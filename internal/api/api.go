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
	"time"

	"github.com/sadmanryanriad/ftp-lantern/internal/config"
	"github.com/sadmanryanriad/ftp-lantern/internal/crawler"
	"github.com/sadmanryanriad/ftp-lantern/internal/db"
	"github.com/sadmanryanriad/ftp-lantern/internal/parser"
	"github.com/sadmanryanriad/ftp-lantern/internal/player"
	"github.com/sadmanryanriad/ftp-lantern/internal/thumbs"
)

type Server struct {
	cfg       *config.Config
	database  *db.DB
	crawler   *crawler.Crawler
	thumbsMgr *thumbs.Manager
	startTime time.Time
}

func NewServer(cfg *config.Config, database *db.DB, c *crawler.Crawler, tm *thumbs.Manager) *Server {
	return &Server{
		cfg:       cfg,
		database:  database,
		crawler:   c,
		thumbsMgr: tm,
		startTime: time.Now(),
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

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	var results []TitleCard

	if normQuery == "" {
		// Browsing without query
		sqlQuery := `
			SELECT t.id, t.name, t.year, t.type, COALESCE(t.poster_url, ''), COALESCE(t.thumb_hash, ''), t.is_following,
			       COALESCE(MAX(r.resolution), 'Unknown') as resolution,
			       COALESCE(MAX(r.source), 'Unknown') as source,
			       COALESCE(MAX(r.has_hdr), 0) as has_hdr,
			       COALESCE(MAX(r.category), '') as category
			FROM titles t
			LEFT JOIN releases r ON r.title_id = t.id AND r.gone_at IS NULL
			WHERE 1=1
		`
		var args []interface{}
		if resolution != "" {
			sqlQuery += " AND r.resolution = ?"
			args = append(args, resolution)
		}
		if reqType != "" {
			sqlQuery += " AND t.type = ?"
			args = append(args, reqType)
		}
		sqlQuery += " GROUP BY t.id ORDER BY t.last_modified DESC, t.id DESC LIMIT ?"
		args = append(args, limit)

		rows, err := s.database.Query(sqlQuery, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var tc TitleCard
				var thumbHash string
				var hasHDRInt int
				_ = rows.Scan(&tc.ID, &tc.Name, &tc.Year, &tc.Type, &tc.PosterURL, &thumbHash, &tc.IsFollowing, &tc.Resolution, &tc.Source, &hasHDRInt, &tc.Category)
				tc.HasHDR = (hasHDRInt == 1)
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
			rows, err := s.database.Query("SELECT id FROM titles WHERE name_norm LIKE ? || '%' LIMIT ?", normQuery, limit)
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
			rows, err := s.database.Query("SELECT rowid FROM titles_fts WHERE name_norm LIKE '%' || ? || '%' LIMIT ?", normQuery, limit)
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
				fuzzyIDs := s.fuzzyFallback(normQuery, limit)
				existing := make(map[int64]bool)
				for _, id := range titleIDs {
					existing[id] = true
				}
				for _, id := range fuzzyIDs {
					if !existing[id] {
						titleIDs = append(titleIDs, id)
						existing[id] = true
						if len(titleIDs) >= limit {
							break
						}
					}
				}
			}
		}

		if len(titleIDs) > 0 {
			results = s.fetchTitlesByIDs(titleIDs)
		}
	}

	writeJSON(w, results)
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
		       COALESCE(MAX(r.category), '') as category
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
		_ = rows.Scan(&tc.ID, &tc.Name, &tc.Year, &tc.Type, &tc.PosterURL, &thumbHash, &tc.IsFollowing, &tc.Resolution, &tc.Source, &hasHDRInt, &tc.Category)
		tc.HasHDR = (hasHDRInt == 1)
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
		       COALESCE(MAX(r.has_hdr), 0), COALESCE(MAX(r.category), '')
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
		_ = rows.Scan(&tc.ID, &tc.Name, &tc.Year, &tc.Type, &tc.PosterURL, &thumbHash, &tc.Resolution, &tc.Source, &hasHDRInt, &tc.Category)
		tc.HasHDR = (hasHDRInt == 1)
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
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.m3u\"", f.FileName))
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
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s - Complete Season.m3u\"", titleName))
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
		"turbo_mode":              isTurbo,
		"turbo_active":            isTurbo,
		"turbo_until":             turboUntil,
		"turbo_remaining_seconds": remainingSec,
		"last_crawl":              lastCrawl,
		"is_crawling":             s.crawler.IsCrawling(),
	})
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

	// Check ISP gateway reachability
	ispConnected := false
	if len(s.cfg.SourceURLs) > 0 {
		client := http.Client{Timeout: 2 * time.Second}
		resp, err := client.Head(s.cfg.SourceURLs[0])
		if err == nil {
			_ = resp.Body.Close()
			ispConnected = (resp.StatusCode == http.StatusOK)
		}
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	lastCrawl := s.database.GetSetting("last_crawl_time", "")

	writeJSON(w, map[string]interface{}{
		"status":           "ok",
		"bdix_connected":   ispConnected,
		"isp_connected":    ispConnected,
		"titles_count":     titleCount,
		"files_count":      fileCount,
		"releases_count":   releaseCount,
		"uptime":           time.Since(s.startTime).String(),
		"uptime_seconds":   int64(time.Since(s.startTime).Seconds()),
		"memory_alloc_mb":  float64(m.Alloc) / (1024 * 1024),
		"memory_sys_mb":    float64(m.Sys) / (1024 * 1024),
		"goroutines_count": runtime.NumGoroutine(),
		"last_scan_at":     lastCrawl,
		"is_crawling":      s.crawler.IsCrawling(),
		"ram_alloc_mb":     fmt.Sprintf("%.2f MB", float64(m.Alloc)/(1024*1024)),
		"ram_sys_mb":       fmt.Sprintf("%.2f MB", float64(m.Sys)/(1024*1024)),
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
