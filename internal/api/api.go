package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sadmanryanriad/ftp-lantern/internal/config"
	"github.com/sadmanryanriad/ftp-lantern/internal/crawler"
	"github.com/sadmanryanriad/ftp-lantern/internal/db"
	"github.com/sadmanryanriad/ftp-lantern/internal/parser"
	"github.com/sadmanryanriad/ftp-lantern/internal/player"
	"github.com/sadmanryanriad/ftp-lantern/internal/thumbs"
)

type Server struct {
	cfg          *config.Config
	database     *db.DB
	crawler      *crawler.Crawler
	thumbsMgr    *thumbs.Manager
	startTime    time.Time
	ispConnected atomic.Bool
	ispLatencyMs atomic.Int64
}

func NewServer(cfg *config.Config, database *db.DB, c *crawler.Crawler, tm *thumbs.Manager) *Server {
	s := &Server{
		cfg:       cfg,
		database:  database,
		crawler:   c,
		thumbsMgr: tm,
		startTime: time.Now(),
	}
	s.ispConnected.Store(true) // Optimistically true on start
	go s.startIspHealthChecker()
	return s
}

func (s *Server) startIspHealthChecker() {
	client := &http.Client{Timeout: 3 * time.Second}
	for {
		if len(s.cfg.SourceURLs) > 0 {
			start := time.Now()
			req, err := http.NewRequest("GET", s.cfg.SourceURLs[0], nil)
			if err == nil {
				req.Header.Set("Range", "bytes=0-0")
				resp, err := client.Do(req)
				if err == nil {
					_ = resp.Body.Close()
					s.ispConnected.Store(resp.StatusCode < 500)
					s.ispLatencyMs.Store(time.Since(start).Milliseconds())
				} else {
					s.ispConnected.Store(false)
				}
			}
		}
		time.Sleep(15 * time.Second)
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/titles", s.handleTitles)
	mux.HandleFunc("GET /api/titles/{id}", s.handleTitleDetails)
	mux.HandleFunc("POST /api/titles/{id}/follow", s.handleToggleFollow)
	mux.HandleFunc("POST /api/plays", s.handleRecordPlay)
	mux.HandleFunc("POST /api/plays/mark-watched", s.handleMarkWatched)
	mux.HandleFunc("GET /api/new", s.handleWhatsNew)
	mux.HandleFunc("GET /api/thumbs/{filename}", s.handleServeThumb)
	mux.HandleFunc("GET /play/file/{file}", s.handleSingleFilePlaylist)
	mux.HandleFunc("GET /play/season/{season}", s.handleSeasonPlaylist)
	mux.HandleFunc("GET /api/scheduler", s.handleGetScheduler)
	mux.HandleFunc("POST /api/scheduler/turbo", s.handleSetTurbo)
	mux.HandleFunc("POST /api/scheduler/frequency", s.handleSetFrequency)
	mux.HandleFunc("POST /api/backup", s.handleBackup)
	mux.HandleFunc("POST /api/backup/import", s.handleImportBackup)
	mux.HandleFunc("GET /api/settings/notifications", s.handleGetNotificationMode)
	mux.HandleFunc("POST /api/settings/notifications", s.handleSetNotificationMode)
	mux.HandleFunc("POST /api/crawl/trigger", s.handleTriggerCrawl)
	mux.HandleFunc("GET /api/status", s.handleStatus)
}

type TitleCard struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Year        int    `json:"year"`
	Type        string `json:"type"`
	Resolution  string `json:"resolution"`
	Source      string `json:"source"`
	HasHDR      bool   `json:"has_hdr"`
	PosterURL   string `json:"poster_url"`
	ThumbURL    string `json:"thumb_url"`
	IsFollowing bool   `json:"is_following"`
	Category    string `json:"category"`
	SizeBytes   int64  `json:"size_bytes"`
	SizeHuman   string `json:"size_human"`
}

type ContinueWatchingItem struct {
	TitleID          int64  `json:"title_id"`
	TitleName        string `json:"title_name"`
	PosterURL        string `json:"poster_url"`
	ThumbHash        string `json:"thumb_hash"`
	LastPlayedFileID int64  `json:"last_played_file_id"`
	LastPlayedCode   string `json:"last_played_code"`
	LastPlayedAt     string `json:"last_played_at"`
	NextFileID       *int64 `json:"next_file_id,omitempty"`
	NextEpisodeCode  string `json:"next_episode_code,omitempty"`
	NextFileURL      string `json:"next_file_url,omitempty"`
	Resolution       string `json:"resolution"`
}

func (s *Server) handleTitles(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("continue_watching") == "true" {
		query := `
			SELECT t.id, t.name, COALESCE(t.poster_url, ''), COALESCE(t.thumb_hash, ''),
			       p.file_id, p.played_at, f.episode_code,
			       COALESCE(MAX(r.resolution), 'Unknown') as resolution
			FROM plays p
			JOIN titles t ON t.id = p.title_id
			JOIN files f ON f.id = p.file_id
			LEFT JOIN releases r ON r.title_id = t.id AND r.gone_at IS NULL
			GROUP BY t.id
			ORDER BY p.played_at DESC
			LIMIT 15
		`
		rows, err := s.database.Query(query)
		if err != nil {
			writeJSON(w, []ContinueWatchingItem{})
			return
		}
		defer rows.Close()

		var list []ContinueWatchingItem
		for rows.Next() {
			var item ContinueWatchingItem
			err := rows.Scan(&item.TitleID, &item.TitleName, &item.PosterURL, &item.ThumbHash, &item.LastPlayedFileID, &item.LastPlayedAt, &item.LastPlayedCode, &item.Resolution)
			if err == nil {
				list = append(list, item)
			}
		}
		_ = rows.Close()

		for i := range list {
			// Find next episode
			var nextID int64
			var nextCode, nextURL string
			errNext := s.database.QueryRow(`
				SELECT id, episode_code, file_url
				FROM files
				WHERE title_id = ? AND id > ? AND gone_at IS NULL
				ORDER BY season_number ASC, episode_number ASC, id ASC
				LIMIT 1`, list[i].TitleID, list[i].LastPlayedFileID).Scan(&nextID, &nextCode, &nextURL)

			if errNext == nil {
				list[i].NextFileID = &nextID
				list[i].NextEpisodeCode = nextCode
				list[i].NextFileURL = nextURL
			}
		}

		writeJSON(w, list)
		return
	}

	// Default fallback: route to search
	s.handleSearch(w, r)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	normQuery := parser.NormalizeTitle(query)

	resolution := strings.TrimSpace(r.URL.Query().Get("resolution"))
	reqType := strings.TrimSpace(r.URL.Query().Get("type"))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	sortParam := strings.TrimSpace(r.URL.Query().Get("sort"))
	yearParam := strings.TrimSpace(r.URL.Query().Get("year"))
	dateAddedParam := strings.TrimSpace(r.URL.Query().Get("date_added"))
	followingParam := r.URL.Query().Get("following") == "true"

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	offset := (page - 1) * limit

	var results []TitleCard
	var totalCount int

	if normQuery == "" {
		// Browsing without query
		whereClause := " WHERE 1=1"
		var args []interface{}
		var countArgs []interface{}

		if followingParam {
			whereClause += " AND t.is_following = 1"
		}
		if resolution != "" && resolution != "All" {
			whereClause += " AND r.resolution = ?"
			args = append(args, resolution)
			countArgs = append(countArgs, resolution)
		}
		if reqType != "" && reqType != "all" {
			whereClause += " AND t.type = ?"
			args = append(args, reqType)
			countArgs = append(countArgs, reqType)
		}
		if category != "" && category != "All" {
			whereClause += " AND (r.category LIKE ? OR r.folder_url LIKE ?)"
			args = append(args, "%"+category+"%", "%"+category+"%")
			countArgs = append(countArgs, "%"+category+"%", "%"+category+"%")
		}
		if yearParam != "" && yearParam != "All" {
			if strings.Contains(yearParam, "-") {
				parts := strings.Split(yearParam, "-")
				if len(parts) == 2 {
					yMin, _ := strconv.Atoi(parts[0])
					yMax, _ := strconv.Atoi(parts[1])
					if yMin > 0 && yMax >= yMin {
						whereClause += " AND t.year BETWEEN ? AND ?"
						args = append(args, yMin, yMax)
						countArgs = append(countArgs, yMin, yMax)
					}
				}
			} else if strings.HasPrefix(yearParam, "<") {
				yMax, _ := strconv.Atoi(strings.TrimPrefix(yearParam, "<"))
				if yMax > 0 {
					whereClause += " AND t.year > 0 AND t.year < ?"
					args = append(args, yMax)
					countArgs = append(countArgs, yMax)
				}
			} else {
				y, _ := strconv.Atoi(yearParam)
				if y > 0 {
					whereClause += " AND t.year = ?"
					args = append(args, y)
					countArgs = append(countArgs, y)
				}
			}
		}
		if dateAddedParam != "" && dateAddedParam != "all" {
			switch dateAddedParam {
			case "today":
				whereClause += " AND t.first_seen >= date('now', 'start of day')"
			case "7d", "week":
				whereClause += " AND t.first_seen >= datetime('now', '-7 days')"
			case "30d", "month":
				whereClause += " AND t.first_seen >= datetime('now', '-30 days')"
			case "90d", "3months":
				whereClause += " AND t.first_seen >= datetime('now', '-90 days')"
			}
		}

		// Count query
		countSQL := `
			SELECT COUNT(DISTINCT t.id)
			FROM titles t
			LEFT JOIN releases r ON r.title_id = t.id AND r.gone_at IS NULL
		` + whereClause
		_ = s.database.QueryRow(countSQL, countArgs...).Scan(&totalCount)

		// Main query
		orderBy := " ORDER BY t.last_modified DESC, t.id DESC"
		if sortParam == "year" || sortParam == "year_desc" {
			orderBy = " ORDER BY t.year DESC, t.name ASC"
		} else if sortParam == "year_asc" {
			orderBy = " ORDER BY t.year ASC, t.name ASC"
		} else if sortParam == "name" || sortParam == "name_asc" {
			orderBy = " ORDER BY t.name ASC"
		} else if sortParam == "name_desc" {
			orderBy = " ORDER BY t.name DESC"
		}

		sqlQuery := `
			SELECT t.id, t.name, t.year, t.type, COALESCE(t.poster_url, ''), COALESCE(t.thumb_hash, ''), t.is_following,
			       COALESCE(MAX(r.resolution), 'Unknown') as resolution,
			       COALESCE(MAX(r.source), 'Unknown') as source,
			       COALESCE(MAX(r.has_hdr), 0) as has_hdr,
			       COALESCE(MAX(r.category), '') as category,
			       COALESCE((SELECT SUM(f.file_size_bytes) FROM files f WHERE f.title_id = t.id AND f.gone_at IS NULL), 0) as size_bytes
			FROM titles t
			LEFT JOIN releases r ON r.title_id = t.id AND r.gone_at IS NULL
		` + whereClause + " GROUP BY t.id " + orderBy + " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)

		rows, err := s.database.Query(sqlQuery, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var tc TitleCard
				var thumbHash string
				var hasHDRInt int
				_ = rows.Scan(&tc.ID, &tc.Name, &tc.Year, &tc.Type, &tc.PosterURL, &thumbHash, &tc.IsFollowing, &tc.Resolution, &tc.Source, &hasHDRInt, &tc.Category, &tc.SizeBytes)
				tc.HasHDR = (hasHDRInt == 1)
				if tc.SizeBytes > 0 {
					tc.SizeHuman = formatBytes(tc.SizeBytes)
				}
				if thumbHash != "" {
					tc.ThumbURL = fmt.Sprintf("/api/thumbs/%d-%s.jpg", tc.ID, thumbHash)
				}
				results = append(results, tc)
			}
		}
	} else {
		// Search with query
		var titleIDs []int64

		if len(normQuery) < 3 {
			// Prefix search for short query
			rows, err := s.database.Query("SELECT id FROM titles WHERE name_norm LIKE ? || '%' LIMIT 100", normQuery)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var id int64
					_ = rows.Scan(&id)
					titleIDs = append(titleIDs, id)
				}
			}
		} else {
			// Trigram FTS search accelerated via parameterized LIKE
			rows, err := s.database.Query("SELECT rowid FROM titles_fts WHERE name_norm LIKE '%' || ? || '%' LIMIT 100", normQuery)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var id int64
					_ = rows.Scan(&id)
					titleIDs = append(titleIDs, id)
				}
			}

			// If < 3 hits, fallback to sliding-window Levenshtein
			if len(titleIDs) < 3 {
				fuzzyIDs := s.fuzzyFallback(normQuery, 100)
				existing := make(map[int64]bool)
				for _, id := range titleIDs {
					existing[id] = true
				}
				for _, id := range fuzzyIDs {
					if !existing[id] {
						titleIDs = append(titleIDs, id)
						existing[id] = true
					}
				}
			}
		}

		totalCount = len(titleIDs)
		if len(titleIDs) > 0 {
			// Slice for pagination
			start := offset
			if start > len(titleIDs) {
				start = len(titleIDs)
			}
			end := start + limit
			if end > len(titleIDs) {
				end = len(titleIDs)
			}
			slicedIDs := titleIDs[start:end]
			if len(slicedIDs) > 0 {
				results = s.fetchTitlesByIDs(slicedIDs)
			}
		}
	}

	if results == nil {
		results = []TitleCard{}
	}

	writeJSON(w, map[string]interface{}{
		"results": results,
		"total":   totalCount,
		"page":    page,
		"limit":   limit,
	})
}

func (s *Server) fuzzyFallback(query string, limit int) []int64 {
	rows, err := s.database.Query("SELECT id, name_norm FROM titles LIMIT 2000")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var matchedIDs []int64
	for rows.Next() {
		var id int64
		var nameNorm string
		if err := rows.Scan(&id, &nameNorm); err == nil {
			if SlidingWindowMatch(query, nameNorm, 2) {
				matchedIDs = append(matchedIDs, id)
				if len(matchedIDs) >= limit {
					break
				}
			}
		}
	}
	return matchedIDs
}

func (s *Server) fetchTitlesByIDs(ids []int64) []TitleCard {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT t.id, t.name, t.year, t.type, COALESCE(t.poster_url, ''), COALESCE(t.thumb_hash, ''), t.is_following,
		       COALESCE(MAX(r.resolution), 'Unknown') as resolution,
		       COALESCE(MAX(r.source), 'Unknown') as source,
		       COALESCE(MAX(r.has_hdr), 0) as has_hdr,
		       COALESCE(MAX(r.category), '') as category,
		       COALESCE((SELECT SUM(f.file_size_bytes) FROM files f WHERE f.title_id = t.id AND f.gone_at IS NULL), 0) as size_bytes
		FROM titles t
		LEFT JOIN releases r ON r.title_id = t.id AND r.gone_at IS NULL
		WHERE t.id IN (%s)
		GROUP BY t.id
	`, strings.Join(placeholders, ","))

	rows, err := s.database.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var cards []TitleCard
	for rows.Next() {
		var tc TitleCard
		var thumbHash string
		var hasHDRInt int
		_ = rows.Scan(&tc.ID, &tc.Name, &tc.Year, &tc.Type, &tc.PosterURL, &thumbHash, &tc.IsFollowing, &tc.Resolution, &tc.Source, &hasHDRInt, &tc.Category, &tc.SizeBytes)
		tc.HasHDR = (hasHDRInt == 1)
		if tc.SizeBytes > 0 {
			tc.SizeHuman = formatBytes(tc.SizeBytes)
		}
		if thumbHash != "" {
			tc.ThumbURL = fmt.Sprintf("/api/thumbs/%d-%s.jpg", tc.ID, thumbHash)
		}
		cards = append(cards, tc)
	}

	// Reorder cards according to input ids order
	idOrder := make(map[int64]int, len(ids))
	for i, id := range ids {
		idOrder[id] = i
	}
	sort.Slice(cards, func(i, j int) bool {
		return idOrder[cards[i].ID] < idOrder[cards[j].ID]
	})

	return cards
}

type EpisodeInfo struct {
	ID           int64  `json:"id"`
	SeasonNumber int    `json:"season_number"`
	EpNumber     int    `json:"episode_number"`
	EpisodeCode  string `json:"episode_code"`
	Name         string `json:"name"`
	FileURL      string `json:"file_url"`
	IntentURL    string `json:"intent_url"`
	SizeBytes    int64  `json:"size_bytes"`
	SizeHuman    string `json:"size_human"`
	Resolution   string `json:"resolution"`
	Source       string `json:"source"`
	HasHDR       bool   `json:"has_hdr"`
}

type TitleDetails struct {
	TitleCard
	Episodes    []EpisodeInfo `json:"episodes"`
	Files       []EpisodeInfo `json:"files"`
	LastPlayed  string        `json:"last_played_code"`
	NextEpisode *EpisodeInfo  `json:"next_episode"`
}

func (s *Server) handleTitleDetails(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	cards := s.fetchTitlesByIDs([]int64{id})
	if len(cards) == 0 {
		http.Error(w, "Title not found", http.StatusNotFound)
		return
	}

	details := TitleDetails{TitleCard: cards[0]}

	// Fetch files
	filesQuery := `
		SELECT id, season_number, episode_number, episode_code, name, file_url, file_size_bytes, resolution, source, has_hdr
		FROM files
		WHERE title_id = ? AND gone_at IS NULL
		ORDER BY season_number ASC, episode_number ASC, name ASC
	`
	rows, err := s.database.Query(filesQuery, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ep EpisodeInfo
			var sNum, epNum *int
			var hasHDRInt int
			_ = rows.Scan(&ep.ID, &sNum, &epNum, &ep.EpisodeCode, &ep.Name, &ep.FileURL, &ep.SizeBytes, &ep.Resolution, &ep.Source, &hasHDRInt)
			if sNum != nil {
				ep.SeasonNumber = *sNum
			}
			if epNum != nil {
				ep.EpNumber = *epNum
			}
			ep.HasHDR = (hasHDRInt == 1)
			ep.IntentURL = player.BuildAndroidIntent(ep.FileURL)
			ep.SizeHuman = formatBytes(ep.SizeBytes)
			details.Episodes = append(details.Episodes, ep)
		}
	}

	// Fetch last played
	var lastFileID int64
	err = s.database.QueryRow(`
		SELECT p.file_id 
		FROM plays p 
		WHERE p.title_id = ? 
		ORDER BY p.played_at DESC 
		LIMIT 1`, id).Scan(&lastFileID)

	if err == nil {
		for i, ep := range details.Episodes {
			if ep.ID == lastFileID {
				details.LastPlayed = ep.EpisodeCode
				if i+1 < len(details.Episodes) {
					details.NextEpisode = &details.Episodes[i+1]
				}
				break
			}
		}
	}

	details.Files = details.Episodes
	writeJSON(w, details)
}

func (s *Server) handleToggleFollow(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	_, err := s.database.Exec("UPDATE titles SET is_following = 1 - is_following WHERE id = ?", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var isFollowing bool
	_ = s.database.QueryRow("SELECT is_following FROM titles WHERE id = ?", id).Scan(&isFollowing)
	writeJSON(w, map[string]interface{}{"is_following": isFollowing})
}

func (s *Server) handleRecordPlay(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req struct {
		FileID int64 `json:"file_id"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var titleID int64
	err = s.database.QueryRow("SELECT title_id FROM files WHERE id = ?", req.FileID).Scan(&titleID)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	_, _ = s.database.Exec("INSERT INTO plays (file_id, title_id) VALUES (?, ?)", req.FileID, titleID)
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleMarkWatched(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	var req struct {
		FileID int64 `json:"file_id"`
	}
	_ = json.Unmarshal(body, &req)

	var titleID int64
	var seasonNum, epNum int
	err := s.database.QueryRow("SELECT title_id, season_number, episode_number FROM files WHERE id = ?", req.FileID).Scan(&titleID, &seasonNum, &epNum)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Insert play record
	_, _ = s.database.Exec("INSERT INTO plays (file_id, title_id) VALUES (?, ?)", req.FileID, titleID)
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleWhatsNew(w http.ResponseWriter, r *http.Request) {
	rows, err := s.database.Query(`
		SELECT t.id, t.name, t.year, t.type, COALESCE(t.poster_url, ''), COALESCE(t.thumb_hash, ''),
		       COALESCE(MAX(r.resolution), 'Unknown'), COALESCE(MAX(r.source), 'Unknown'),
		       COALESCE(MAX(r.has_hdr), 0), COALESCE(MAX(r.category), ''),
		       COALESCE((SELECT SUM(f.file_size_bytes) FROM files f WHERE f.title_id = t.id AND f.gone_at IS NULL), 0) as size_bytes
		FROM titles t
		JOIN releases r ON r.title_id = t.id AND r.gone_at IS NULL
		GROUP BY t.id
		ORDER BY t.first_seen DESC, t.id DESC
		LIMIT 20
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []TitleCard
	for rows.Next() {
		var tc TitleCard
		var thumbHash string
		var hasHDRInt int
		_ = rows.Scan(&tc.ID, &tc.Name, &tc.Year, &tc.Type, &tc.PosterURL, &thumbHash, &tc.Resolution, &tc.Source, &hasHDRInt, &tc.Category, &tc.SizeBytes)
		tc.HasHDR = (hasHDRInt == 1)
		if tc.SizeBytes > 0 {
			tc.SizeHuman = formatBytes(tc.SizeBytes)
		}
		if thumbHash != "" {
			tc.ThumbURL = fmt.Sprintf("/api/thumbs/%d-%s.jpg", tc.ID, thumbHash)
		}
		results = append(results, tc)
	}

	writeJSON(w, results)
}

func (s *Server) handleServeThumb(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	cleanName := filepath.Base(filename)
	fullPath := filepath.Join(s.cfg.ThumbsDir, cleanName)

	if info, err := os.Stat(fullPath); err == nil && info.Size() > 0 {
		_ = os.Chtimes(fullPath, time.Now(), time.Now())
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Content-Type", "image/jpeg")
		http.ServeFile(w, r, fullPath)
		return
	}

	// On-demand re-fetch if thumbnail was evicted by disk LRU
	parts := strings.Split(strings.TrimSuffix(cleanName, ".jpg"), "-")
	if len(parts) >= 2 {
		if id, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
			var posterURL string
			if err := s.database.QueryRow("SELECT poster_url FROM titles WHERE id = ?", id).Scan(&posterURL); err == nil && posterURL != "" {
				if err := s.thumbsMgr.ProcessSync(id, parts[1], posterURL); err == nil {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					w.Header().Set("Content-Type", "image/jpeg")
					http.ServeFile(w, r, fullPath)
					return
				}
			}
		}
	}

	http.Error(w, "Thumbnail not found", http.StatusNotFound)
}

func (s *Server) handleSingleFilePlaylist(w http.ResponseWriter, r *http.Request) {
	fileParam := r.PathValue("file")
	idStr := strings.TrimSuffix(fileParam, ".m3u")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	var f player.PlayableFile
	err := s.database.QueryRow(`
		SELECT f.id, t.name, f.episode_code, f.name, f.file_url, f.resolution, f.season_number, f.episode_number
		FROM files f
		JOIN titles t ON t.id = f.title_id
		WHERE f.id = ?`, id).Scan(&f.ID, &f.TitleName, &f.EpisodeCode, &f.FileName, &f.FileURL, &f.Resolution, &f.SeasonNumber, &f.EpNumber)

	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Record play
	var titleID int64
	_ = s.database.QueryRow("SELECT title_id FROM files WHERE id = ?", id).Scan(&titleID)
	_, _ = s.database.Exec("INSERT INTO plays (file_id, title_id) VALUES (?, ?)", id, titleID)

	playlist := player.BuildSingleFilePlaylist(f.TitleName, f)

	w.Header().Set("Content-Type", "audio/x-mpegurl; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	cleanFilename := strings.ReplaceAll(f.FileName, `"`, `_`)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.m3u\"", cleanFilename))
	w.Write([]byte(playlist))
}

func (s *Server) handleSeasonPlaylist(w http.ResponseWriter, r *http.Request) {
	seasonParam := r.PathValue("season")
	titleIDStr := strings.TrimSuffix(seasonParam, ".m3u")
	titleID, _ := strconv.ParseInt(titleIDStr, 10, 64)

	var titleName string
	err := s.database.QueryRow("SELECT name FROM titles WHERE id = ?", titleID).Scan(&titleName)
	if err != nil {
		http.Error(w, "title not found", http.StatusNotFound)
		return
	}

	seasonQuery := r.URL.Query().Get("season")
	var seasonFilter int
	if seasonQuery != "" {
		seasonFilter, _ = strconv.Atoi(seasonQuery)
	}

	filesQuery := `
		SELECT id, season_number, episode_number, episode_code, name, file_url, resolution
		FROM files
		WHERE title_id = ? AND gone_at IS NULL`
	var args []interface{}
	args = append(args, titleID)

	if seasonFilter > 0 {
		filesQuery += " AND season_number = ?"
		args = append(args, seasonFilter)
	}

	filesQuery += " ORDER BY season_number ASC, episode_number ASC, name ASC"

	rows, err := s.database.Query(filesQuery, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var files []player.PlayableFile
	for rows.Next() {
		var f player.PlayableFile
		var sNum, epNum *int
		_ = rows.Scan(&f.ID, &sNum, &epNum, &f.EpisodeCode, &f.FileName, &f.FileURL, &f.Resolution)
		if sNum != nil {
			f.SeasonNumber = *sNum
		}
		if epNum != nil {
			f.EpNumber = *epNum
		}
		f.TitleName = titleName
		files = append(files, f)
	}

	if len(files) == 0 {
		http.Error(w, "no files found for season", http.StatusNotFound)
		return
	}

	// Record play for first episode
	_, _ = s.database.Exec("INSERT INTO plays (file_id, title_id) VALUES (?, ?)", files[0].ID, titleID)

	playlist := player.BuildSeasonPlaylist(titleName, files)

	w.Header().Set("Content-Type", "audio/x-mpegurl; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	cleanTitleName := strings.ReplaceAll(titleName, `"`, `_`)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s - Complete Season.m3u\"", cleanTitleName))
	w.Write([]byte(playlist))
}

func (s *Server) handleGetScheduler(w http.ResponseWriter, r *http.Request) {
	turboUntil := s.database.GetSetting("turbo_until", "")
	isTurbo := false
	var remainingSec int64 = 0
	if turboUntil != "" {
		if t, err := time.Parse(time.RFC3339, turboUntil); err == nil && time.Now().Before(t) {
			isTurbo = true
			remainingSec = int64(time.Until(t).Seconds())
		}
	}

	intervalMin := s.cfg.BaseCrawlIntervalMinutes
	if isTurbo {
		turboMin, _ := strconv.Atoi(s.database.GetSetting("turbo_interval_minutes", "15"))
		if turboMin > 0 {
			intervalMin = turboMin
		}
	}

	lastCrawl := s.database.GetSetting("last_crawl_time", "")

	writeJSON(w, map[string]interface{}{
		"interval_minutes":        intervalMin,
		"base_interval_minutes":   s.cfg.BaseCrawlIntervalMinutes,
		"frequency":               s.database.GetSetting("crawl_frequency", "15m"),
		"turbo_mode":              isTurbo,
		"turbo_active":            isTurbo,
		"turbo_until":             turboUntil,
		"turbo_remaining_seconds": remainingSec,
		"last_crawl":              lastCrawl,
		"is_crawling":             s.crawler.IsCrawling(),
	})
}

func (s *Server) handleSetFrequency(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Frequency string `json:"frequency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	freq := strings.ToLower(strings.TrimSpace(req.Frequency))
	if freq != "15m" && freq != "30m" && freq != "1h" && freq != "daily" && freq != "manual" {
		freq = "15m"
	}
	_ = s.database.SetSetting("crawl_frequency", freq)
	writeJSON(w, map[string]interface{}{"ok": true, "frequency": freq})
}

func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	backupPath := filepath.Join(s.cfg.DataDir, "lantern_backup.db")
	_ = os.Remove(backupPath)
	_, err := s.database.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupPath))
	if err != nil {
		http.Error(w, fmt.Sprintf("Backup failed: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "backup_path": backupPath})
}

func (s *Server) handleImportBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("database")
	if err != nil {
		file, header, err = r.FormFile("file")
		if err != nil {
			http.Error(w, "Missing database file upload (field: database or file)", http.StatusBadRequest)
			return
		}
	}
	defer file.Close()

	// Verify SQLite magic header: first 16 bytes must start with "SQLite format 3"
	magic := make([]byte, 16)
	if _, err := io.ReadFull(file, magic); err != nil || string(magic[:15]) != "SQLite format 3" {
		http.Error(w, "Invalid SQLite database file header", http.StatusBadRequest)
		return
	}

	if seeker, ok := file.(io.Seeker); ok {
		_, _ = seeker.Seek(0, io.SeekStart)
	}

	tempPath := filepath.Join(s.cfg.DataDir, "lantern_import_temp.db")
	_ = os.Remove(tempPath)
	out, err := os.Create(tempPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temporary file: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		http.Error(w, fmt.Sprintf("Failed to save database file: %v", err), http.StatusInternalServerError)
		return
	}
	out.Close()

	dbPath := filepath.Join(s.cfg.DataDir, "lantern.db")
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	if err := os.Rename(tempPath, dbPath); err != nil {
		http.Error(w, fmt.Sprintf("Failed to replace database: %v", err), http.StatusInternalServerError)
		return
	}

	titleCount, _ := s.database.TitleCount()
	writeJSON(w, map[string]interface{}{
		"ok":           true,
		"filename":     header.Filename,
		"titles_count": titleCount,
		"message":      "Database restored successfully",
	})
}

func (s *Server) handleGetNotificationMode(w http.ResponseWriter, r *http.Request) {
	mode := s.database.GetSetting("telegram_notify_mode", "following_only")
	writeJSON(w, map[string]interface{}{"mode": mode})
}

func (s *Server) handleSetNotificationMode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode != "following_only" && mode != "all" && mode != "muted" {
		mode = "following_only"
	}
	_ = s.database.SetSetting("telegram_notify_mode", mode)
	writeJSON(w, map[string]interface{}{"ok": true, "mode": mode})
}

func (s *Server) handleSetTurbo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IntervalMinutes int `json:"interval_minutes"`
		DurationHours   int `json:"duration_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.IntervalMinutes <= 0 {
		req.IntervalMinutes = 15
	}
	if req.DurationHours <= 0 {
		req.DurationHours = 10
	}

	turboUntil := time.Now().Add(time.Duration(req.DurationHours) * time.Hour).Format(time.RFC3339)
	_ = s.database.SetSetting("turbo_until", turboUntil)
	_ = s.database.SetSetting("turbo_interval_minutes", strconv.Itoa(req.IntervalMinutes))

	writeJSON(w, map[string]interface{}{
		"ok":          true,
		"turbo_until": turboUntil,
	})
}

func (s *Server) handleTriggerCrawl(w http.ResponseWriter, r *http.Request) {
	go func() {
		_ = s.crawler.Crawl(s.cfg.SourceURLs)
	}()
	writeJSON(w, map[string]string{"status": "crawl triggered"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	titleCount, _ := s.database.TitleCount()
	fileCount, _ := s.database.FileCount()

	var releaseCount int
	_ = s.database.QueryRow("SELECT COUNT(*) FROM releases WHERE gone_at IS NULL").Scan(&releaseCount)

	var moviesCount, seriesCount, othersCount int
	_ = s.database.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN type = 'movie' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN type = 'series' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN type NOT IN ('movie', 'series') THEN 1 ELSE 0 END), 0)
		FROM titles
	`).Scan(&moviesCount, &seriesCount, &othersCount)

	// Background ISP gateway reachability
	ispConnected := s.ispConnected.Load()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	lastCrawl := s.database.GetSetting("last_crawl_time", "")
	telemetry := s.crawler.GetTelemetry()

	var dbSizeBytes int64
	dbPath := filepath.Join(s.cfg.DataDir, "lantern.db")
	if fi, err := os.Stat(dbPath); err == nil {
		dbSizeBytes += fi.Size()
	}
	if fi, err := os.Stat(dbPath + "-wal"); err == nil {
		dbSizeBytes += fi.Size()
	}
	if fi, err := os.Stat(dbPath + "-shm"); err == nil {
		dbSizeBytes += fi.Size()
	}

	var thumbsSizeBytes int64
	if entries, err := os.ReadDir(s.cfg.ThumbsDir); err == nil {
		for _, e := range entries {
			if fi, err := e.Info(); err == nil {
				thumbsSizeBytes += fi.Size()
			}
		}
	}

	totalVolumeBytes := dbSizeBytes + thumbsSizeBytes

	writeJSON(w, map[string]interface{}{
		"status":                  "ok",
		"bdix_connected":          ispConnected,
		"isp_connected":           ispConnected,
		"isp_latency_ms":          s.ispLatencyMs.Load(),
		"titles_count":            titleCount,
		"files_count":             fileCount,
		"releases_count":          releaseCount,
		"movies_count":            moviesCount,
		"series_count":            seriesCount,
		"others_count":            othersCount,
		"uptime":                  time.Since(s.startTime).String(),
		"uptime_seconds":          int64(time.Since(s.startTime).Seconds()),
		"memory_alloc_mb":         float64(m.Alloc) / (1024 * 1024),
		"memory_sys_mb":           float64(m.Sys) / (1024 * 1024),
		"goroutines_count":        runtime.NumGoroutine(),
		"last_scan_at":            lastCrawl,
		"is_crawling":             telemetry.IsCrawling,
		"current_source":          telemetry.CurrentSource,
		"current_folder":          telemetry.CurrentFolder,
		"files_indexed_session":   telemetry.FilesIndexedSession,
		"folders_indexed_session": telemetry.FoldersIndexedSession,
		"items_per_second":        telemetry.ItemsPerSecond,
		"last_scan_duration_sec":  telemetry.LastScanDurationSec,
		"db_size_bytes":           dbSizeBytes,
		"db_size_human":           formatBytes(dbSizeBytes),
		"thumbs_size_bytes":       thumbsSizeBytes,
		"thumbs_size_human":       formatBytes(thumbsSizeBytes),
		"total_volume_bytes":      totalVolumeBytes,
		"total_volume_human":      formatBytes(totalVolumeBytes),
		"ram_alloc_mb":            fmt.Sprintf("%.2f MB", float64(m.Alloc)/(1024*1024)),
		"ram_sys_mb":              fmt.Sprintf("%.2f MB", float64(m.Sys)/(1024*1024)),
	})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
