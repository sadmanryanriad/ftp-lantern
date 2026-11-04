package crawler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sadmanryanriad/ftp-lantern/internal/db"
	"github.com/sadmanryanriad/ftp-lantern/internal/parser"
	"github.com/sadmanryanriad/ftp-lantern/internal/thumbs"
)

var ErrAlreadyRunning = errors.New("crawl already in progress")

type DirItem struct {
	Name         string
	URL          string
	LastModified string
	SizeBytes    int64
	IsDir        bool
}

type CrawlerTelemetry struct {
	IsCrawling            bool      `json:"is_crawling"`
	CurrentSource         string    `json:"current_source"`
	CurrentFolder         string    `json:"current_folder"`
	FilesIndexedSession   int64     `json:"files_indexed_session"`
	FoldersIndexedSession int64     `json:"folders_indexed_session"`
	ItemsPerSecond        float64   `json:"items_per_second"`
	CrawlStartTime        time.Time `json:"crawl_start_time"`
	LastScanDurationSec   float64   `json:"last_scan_duration_sec"`
}

type Crawler struct {
	database              *db.DB
	thumbsMgr             *thumbs.Manager
	client                *http.Client
	lock                  sync.Mutex
	isCrawling            atomic.Bool
	isSeed                bool
	statsLock             sync.RWMutex
	currentSource         string
	currentFolder         string
	filesIndexedSession   atomic.Int64
	foldersIndexedSession atomic.Int64
	crawlStartTime        time.Time
	lastScanDurationSec   float64
}

func New(database *db.DB, thumbsMgr *thumbs.Manager) *Crawler {
	return &Crawler{
		database:  database,
		thumbsMgr: thumbsMgr,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Crawler) FetchDir(targetURL string) ([]DirItem, error) {
	resp, err := c.client.Get(targetURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	baseURL, _ := url.Parse(targetURL)
	var items []DirItem

	doc.Find("#fallback table tr").Each(func(i int, s *goquery.Selection) {
		nameLink := s.Find("td.fb-n a")
		if nameLink.Length() == 0 {
			return
		}
		name := strings.TrimSpace(nameLink.Text())
		if name == "Parent Directory" || name == ".." {
			return
		}

		href, exists := nameLink.Attr("href")
		if !exists {
			return
		}

		relURL, err := url.Parse(href)
		if err != nil {
			return
		}
		fullURL := baseURL.ResolveReference(relURL).String()

		date := strings.TrimSpace(s.Find("td.fb-d").Text())
		sizeRaw := strings.TrimSpace(s.Find("td.fb-s").Text())

		imgAlt, _ := s.Find("td.fb-i img").Attr("alt")
		isDir := imgAlt == "folder" || strings.HasSuffix(href, "/")

		items = append(items, DirItem{
			Name:         name,
			URL:          fullURL,
			LastModified: date,
			SizeBytes:    parser.ParseSizeBytes(sizeRaw),
			IsDir:        isDir,
		})
	})

	return items, nil
}

func (c *Crawler) IsCrawling() bool {
	return c.isCrawling.Load()
}

func (c *Crawler) GetTelemetry() CrawlerTelemetry {
	c.statsLock.RLock()
	defer c.statsLock.RUnlock()

	isCrawling := c.isCrawling.Load()
	files := c.filesIndexedSession.Load()
	var speed float64
	if isCrawling && !c.crawlStartTime.IsZero() {
		elapsed := time.Since(c.crawlStartTime).Seconds()
		if elapsed > 0.5 {
			speed = float64(files) / elapsed
		}
	}

	return CrawlerTelemetry{
		IsCrawling:            isCrawling,
		CurrentSource:         c.currentSource,
		CurrentFolder:         c.currentFolder,
		FilesIndexedSession:   files,
		FoldersIndexedSession: c.foldersIndexedSession.Load(),
		ItemsPerSecond:        speed,
		CrawlStartTime:        c.crawlStartTime,
		LastScanDurationSec:   c.lastScanDurationSec,
	}
}

func (c *Crawler) Crawl(sourceURLs []string) error {
	if !c.lock.TryLock() {
		return ErrAlreadyRunning
	}
	defer c.lock.Unlock()

	c.isCrawling.Store(true)
	defer c.isCrawling.Store(false)

	startTime := time.Now()
	totalTitles, _ := c.database.TitleCount()
	c.isSeed = (totalTitles == 0)

	c.statsLock.Lock()
	c.crawlStartTime = startTime
	c.statsLock.Unlock()
	c.filesIndexedSession.Store(0)
	c.foldersIndexedSession.Store(0)

	log.Printf("[Crawler] Starting crawl (isSeed=%v, sources=%d)", c.isSeed, len(sourceURLs))

	for _, sourceURL := range sourceURLs {
		if err := c.crawlSource(sourceURL); err != nil {
			log.Printf("[Crawler] Error crawling source %s: %v", sourceURL, err)
		}
	}

	// Dynamic Hot-Set check for new episodes inside existing seasons
	c.crawlHotSet()

	_ = c.database.SetLastCrawlTime(time.Now())
	duration := time.Since(startTime)
	c.statsLock.Lock()
	c.lastScanDurationSec = duration.Seconds()
	c.currentSource = ""
	c.currentFolder = ""
	c.statsLock.Unlock()

	log.Printf("[Crawler] Crawl finished in %v (indexed %d files)", duration, c.filesIndexedSession.Load())
	return nil
}

func DeriveCanonicalCategory(rawURL string) string {
	decoded, _ := url.PathUnescape(rawURL)
	lower := strings.ToLower(decoded)
	if strings.Contains(lower, "1080p") && strings.Contains(lower, "english") {
		return "English Movies (1080p)"
	} else if strings.Contains(lower, "english movies") {
		return "English Movies"
	} else if strings.Contains(lower, "hindi dubbed") {
		return "South Movies (Hindi Dubbed)"
	} else if strings.Contains(lower, "south movies") || strings.Contains(lower, "south indian") {
		return "South Indian Movies"
	} else if strings.Contains(lower, "hindi movies") {
		return "Hindi Movies"
	} else if strings.Contains(lower, "kolkata bangla") || strings.Contains(lower, "satyajit") {
		return "Bangla Movies"
	} else if strings.Contains(lower, "animation movies") {
		return "Animation Movies"
	} else if strings.Contains(lower, "foreign language") {
		return "Foreign Movies"
	} else if strings.Contains(lower, "top-250") || strings.Contains(lower, "top 250") {
		return "IMDb Top-250"
	} else if strings.Contains(lower, "korean") {
		return "Korean Series"
	} else if strings.Contains(lower, "anime") || strings.Contains(lower, "cartoon") {
		return "Anime & Cartoons"
	} else if strings.Contains(lower, "documentary") {
		return "Documentaries"
	} else if strings.Contains(lower, "wrestling") || strings.Contains(lower, "wwe") {
		return "Wrestling"
	} else if strings.Contains(lower, "awards") || strings.Contains(lower, "tv shows") {
		return "Awards & TV Shows"
	} else if strings.Contains(lower, "tv-web-series") || strings.Contains(lower, "tv series") {
		return "TV-WEB-Series"
	}

	u, err := url.Parse(rawURL)
	if err == nil {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) > 0 {
			last := parts[len(parts)-1]
			if last != "" {
				return last
			}
		}
	}
	return "General"
}

func IsSeriesCategory(catName string) bool {
	lower := strings.ToLower(catName)
	return strings.Contains(lower, "series") ||
		strings.Contains(lower, "korean") ||
		strings.Contains(lower, "anime") ||
		strings.Contains(lower, "cartoon") ||
		strings.Contains(lower, "documentar") ||
		strings.Contains(lower, "wrestling") ||
		strings.Contains(lower, "tv shows")
}

func (c *Crawler) crawlSource(sourceURL string) error {
	canonicalCategory := DeriveCanonicalCategory(sourceURL)
	c.statsLock.Lock()
	c.currentSource = canonicalCategory
	c.statsLock.Unlock()

	categories, err := c.FetchDir(sourceURL)
	if err != nil {
		return err
	}
	c.foldersIndexedSession.Add(1)

	for _, cat := range categories {
		if !cat.IsDir {
			continue
		}
		c.crawlCategory(canonicalCategory, cat.Name, cat.URL)
	}

	return nil
}

func (c *Crawler) crawlCategory(canonicalCategory, folderName, categoryURL string) {
	c.statsLock.Lock()
	c.currentSource = canonicalCategory
	c.statsLock.Unlock()

	titles, err := c.FetchDir(categoryURL)
	if err != nil {
		log.Printf("[Crawler] Failed to fetch category %s (%s): %v", canonicalCategory, categoryURL, err)
		return
	}
	c.foldersIndexedSession.Add(1)

	for _, it := range titles {
		if !it.IsDir {
			continue
		}

		// Check if release exists and mtime is unchanged
		var releaseID int64
		var recordedMtime string
		err := c.database.QueryRow("SELECT id, last_modified FROM releases WHERE folder_url = ?", it.URL).Scan(&releaseID, &recordedMtime)

		if err == nil {
			// Title already exists in DB
			if it.LastModified != "" && it.LastModified == recordedMtime {
				// Title folder mtime unchanged, skip descending!
				continue
			}
			// Title folder mtime changed! (New season added to title)
			c.crawlTitleFolder(canonicalCategory, it, releaseID)
		} else {
			// Brand new release
			c.crawlTitleFolder(canonicalCategory, it, 0)
		}
	}
}

func (c *Crawler) crawlTitleFolder(categoryName string, it DirItem, existingReleaseID int64) {
	c.statsLock.Lock()
	c.currentFolder = it.Name
	c.statsLock.Unlock()
	c.foldersIndexedSession.Add(1)

	contents, err := c.FetchDir(it.URL)
	if err != nil {
		log.Printf("[Crawler] Failed to read title folder %s: %v", it.URL, err)
		return
	}

	meta := parser.ParseFolderMetadata(it.Name)
	if IsSeriesCategory(categoryName) {
		meta.Type = "series"
	} else {
		meta.Type = "movie"
	}

	var posterURL string
	var subfolders []DirItem
	var flatFiles []DirItem

	for _, item := range contents {
		if item.IsDir {
			subfolders = append(subfolders, item)
		} else {
			ext := strings.ToLower(path.Ext(item.Name))
			if ext == ".jpg" || ext == ".png" || ext == ".jpeg" {
				if posterURL == "" || strings.Contains(strings.ToLower(item.Name), "poster") || strings.Contains(strings.ToLower(item.Name), "v1") {
					posterURL = item.URL
				}
			} else if ext == ".mkv" || ext == ".mp4" || ext == ".avi" || ext == ".webm" {
				flatFiles = append(flatFiles, item)
			}
		}
	}

	// Upsert Title
	var titleID int64
	err = c.database.QueryRow("SELECT id FROM titles WHERE name_norm = ? AND year = ?", meta.TitleNorm, meta.Year).Scan(&titleID)
	if err != nil {
		// Insert Title
		thumbHash := ""
		if posterURL != "" {
			thumbHash = c.thumbsMgr.Hash(posterURL)
		}
		res, err := c.database.Exec(`
			INSERT INTO titles (name, name_norm, year, type, poster_url, thumb_hash, last_modified)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			meta.Title, meta.TitleNorm, meta.Year, meta.Type, posterURL, thumbHash, it.LastModified,
		)
		if err != nil {
			log.Printf("[Crawler] Insert title error: %v", err)
			return
		}
		titleID, _ = res.LastInsertId()
		if posterURL != "" {
			c.thumbsMgr.Enqueue(titleID, posterURL)
		}
	} else if posterURL != "" {
		thumbHash := c.thumbsMgr.Hash(posterURL)
		_, _ = c.database.Exec("UPDATE titles SET poster_url = ?, thumb_hash = ? WHERE id = ? AND poster_url IS NULL", posterURL, thumbHash, titleID)
		c.thumbsMgr.Enqueue(titleID, posterURL)
	}

	// Upsert Release
	releaseID := existingReleaseID
	if releaseID == 0 {
		hdrVal := 0
		if meta.HasHDR {
			hdrVal = 1
		}
		res, err := c.database.Exec(`
			INSERT INTO releases (title_id, category, folder_url, folder_name, resolution, source, has_hdr, last_modified)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			titleID, categoryName, it.URL, it.Name, meta.Resolution, meta.Source, hdrVal, it.LastModified,
		)
		if err != nil {
			log.Printf("[Crawler] Insert release error: %v", err)
			return
		}
		releaseID, _ = res.LastInsertId()

		// Emit new_title event
		c.emitEvent("new_title", titleID, releaseID, map[string]interface{}{
			"title":      meta.Title,
			"year":       meta.Year,
			"resolution": meta.Resolution,
			"category":   categoryName,
		})
	} else {
		_, _ = c.database.Exec("UPDATE releases SET last_modified = ?, gone_at = NULL WHERE id = ?", it.LastModified, releaseID)
	}

	// If there are subfolders (Seasons)
	for _, sub := range subfolders {
		seasonNum := parser.ParseSeasonNumber(sub.Name)
		c.crawlSeasonFolder(titleID, releaseID, seasonNum, sub)
	}

	// If flat files directly in title folder
	if len(flatFiles) > 0 {
		c.ingestFiles(titleID, releaseID, 1, flatFiles)
	}
}

func (c *Crawler) crawlSeasonFolder(titleID, releaseID int64, seasonNum int, seasonDir DirItem) {
	files, err := c.FetchDir(seasonDir.URL)
	if err != nil {
		log.Printf("[Crawler] Failed to read season folder %s: %v", seasonDir.URL, err)
		return
	}

	var videoFiles []DirItem
	for _, f := range files {
		if !f.IsDir {
			ext := strings.ToLower(path.Ext(f.Name))
			if ext == ".mkv" || ext == ".mp4" || ext == ".avi" || ext == ".webm" {
				videoFiles = append(videoFiles, f)
			}
		}
	}

	c.ingestFiles(titleID, releaseID, seasonNum, videoFiles)
}

func (c *Crawler) ingestFiles(titleID, releaseID int64, seasonNum int, files []DirItem) {
	var newFiles []string

	for _, f := range files {
		var fileID int64
		err := c.database.QueryRow("SELECT id FROM files WHERE file_url = ?", f.URL).Scan(&fileID)
		if err == nil {
			// Already indexed
			continue
		}

		sNum, epNum, epCode := parser.ParseEpisodeCode(f.Name, seasonNum)
		meta := parser.ParseFolderMetadata(f.Name)

		hdrVal := 0
		if meta.HasHDR {
			hdrVal = 1
		}

		_, err = c.database.Exec(`
			INSERT INTO files (release_id, title_id, season_number, episode_number, episode_code, name, file_url, file_size_bytes, resolution, source, has_hdr, last_modified)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			releaseID, titleID, sNum, epNum, epCode, f.Name, f.URL, f.SizeBytes, meta.Resolution, meta.Source, hdrVal, f.LastModified,
		)
		if err == nil {
			newFiles = append(newFiles, epCode)
			c.filesIndexedSession.Add(1)
		}
	}

	if len(newFiles) > 0 && !c.isSeed {
		c.emitEvent("new_episodes", titleID, releaseID, map[string]interface{}{
			"season":   seasonNum,
			"episodes": newFiles,
		})
	}
}

func (c *Crawler) crawlHotSet() {
	query := `
		SELECT t.id, r.id, r.folder_url, r.folder_name, r.category
		FROM titles t
		JOIN releases r ON r.title_id = t.id
		WHERE t.is_following = 1 OR (
			SELECT MAX(f.last_modified) FROM files f WHERE f.title_id = t.id AND f.gone_at IS NULL
		) > datetime('now', '-45 days')
	`
	rows, err := c.database.Query(query)
	if err != nil {
		return
	}

	type hotItem struct {
		titleID    int64
		releaseID  int64
		folderURL  string
		folderName string
		category   string
	}
	var items []hotItem
	for rows.Next() {
		var it hotItem
		if err := rows.Scan(&it.titleID, &it.releaseID, &it.folderURL, &it.folderName, &it.category); err == nil {
			items = append(items, it)
		}
	}
	_ = rows.Close()

	for _, it := range items {
		// Re-list folder contents to scan seasons
		contents, err := c.FetchDir(it.folderURL)
		if err != nil {
			continue
		}

		for _, item := range contents {
			if item.IsDir {
				seasonNum := parser.ParseSeasonNumber(item.Name)
				c.crawlSeasonFolder(it.titleID, it.releaseID, seasonNum, item)
			}
		}
	}
}

func (c *Crawler) emitEvent(kind string, titleID, releaseID int64, payload map[string]interface{}) {
	data, _ := json.Marshal(payload)

	notifiedAtVal := interface{}(nil)
	if c.isSeed {
		notifiedAtVal = time.Now().UTC().Format(time.RFC3339)
	}

	_, _ = c.database.Exec(`
		INSERT INTO events (kind, title_id, release_id, payload_json, notified_at)
		VALUES (?, ?, ?, ?, ?)`,
		kind, titleID, releaseID, string(data), notifiedAtVal,
	)
}
