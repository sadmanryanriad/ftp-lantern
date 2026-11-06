package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sadmanryanriad/ftp-lantern/internal/config"
	"github.com/sadmanryanriad/ftp-lantern/internal/crawler"
	"github.com/sadmanryanriad/ftp-lantern/internal/db"
	"github.com/sadmanryanriad/ftp-lantern/internal/thumbs"
)

func setupTestServer(t *testing.T) (*Server, *http.ServeMux, *db.DB) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		Port:            "8085",
		DataDir:         tempDir,
		ThumbsDir:       filepath.Join(tempDir, "thumbs"),
		MaxThumbsDiskMB: 250,
	}

	database, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	thumbsMgr, err := thumbs.NewManager(cfg.ThumbsDir, cfg.MaxThumbsDiskMB)
	if err != nil {
		t.Fatalf("failed to open thumbs: %v", err)
	}

	c := crawler.New(database, thumbsMgr)
	srv := NewServer(cfg, database, c, thumbsMgr)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	return srv, mux, database
}

func TestContinueWatchingNoDeadlock(t *testing.T) {
	_, mux, database := setupTestServer(t)
	defer database.Close()

	// Seed data: Title -> Release -> Files (Ep 1, Ep 2) -> Play record on Ep 1
	res, err := database.Exec(`INSERT INTO titles (name, name_norm, year, type) VALUES ('Severance', 'severance', 2022, 'series')`)
	if err != nil {
		t.Fatal(err)
	}
	titleID, _ := res.LastInsertId()

	resRel, err := database.Exec(`INSERT INTO releases (title_id, category, folder_url, folder_name, resolution) VALUES (?, 'TV', 'http://test/sev/', 'Severance', '1080p')`, titleID)
	if err != nil {
		t.Fatal(err)
	}
	relID, _ := resRel.LastInsertId()

	resF1, err := database.Exec(`INSERT INTO files (release_id, title_id, season_number, episode_number, episode_code, name, file_url) VALUES (?, ?, 1, 1, 'S01E01', 'Ep1.mkv', 'http://test/sev/1.mkv')`, relID, titleID)
	if err != nil {
		t.Fatal(err)
	}
	f1ID, _ := resF1.LastInsertId()

	_, err = database.Exec(`INSERT INTO files (release_id, title_id, season_number, episode_number, episode_code, name, file_url) VALUES (?, ?, 1, 2, 'S01E02', 'Ep2.mkv', 'http://test/sev/2.mkv')`, relID, titleID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = database.Exec(`INSERT INTO plays (file_id, title_id) VALUES (?, ?)`, f1ID, titleID)
	if err != nil {
		t.Fatal(err)
	}

	// Request Continue Watching (should return empty list as history is disabled)
	req := httptest.NewRequest("GET", "/api/titles?continue_watching=true", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var items []ContinueWatchingItem
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if len(items) != 0 {
		t.Fatalf("expected 0 continue watching items since watch history is disabled, got %d", len(items))
	}
}

func TestTitleDetailsFilesAndEpisodes(t *testing.T) {
	_, mux, database := setupTestServer(t)
	defer database.Close()

	res, _ := database.Exec(`INSERT INTO titles (name, name_norm, year, type) VALUES ('Fallout', 'fallout', 2024, 'series')`)
	titleID, _ := res.LastInsertId()

	resRel, _ := database.Exec(`INSERT INTO releases (title_id, category, folder_url, folder_name, resolution) VALUES (?, 'TV', 'http://test/fo/', 'Fallout', '4k')`, titleID)
	relID, _ := resRel.LastInsertId()

	_, _ = database.Exec(`INSERT INTO files (release_id, title_id, season_number, episode_number, episode_code, name, file_url) VALUES (?, ?, 1, 1, 'S01E01', 'Fallout.S01E01.mkv', 'http://test/fo/1.mkv')`, relID, titleID)

	req := httptest.NewRequest("GET", "/api/titles/1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	// Verify both "episodes" and "files" exist in JSON response for frontend compatibility
	if _, ok := body["episodes"]; !ok {
		t.Errorf("expected 'episodes' key in title details response")
	}
	if _, ok := body["files"]; !ok {
		t.Errorf("expected 'files' key in title details response")
	}
}

func TestSeasonPlaylistFilter(t *testing.T) {
	_, mux, database := setupTestServer(t)
	defer database.Close()

	res, _ := database.Exec(`INSERT INTO titles (name, name_norm, year, type) VALUES ('The Boys', 'theboys', 2019, 'series')`)
	titleID, _ := res.LastInsertId()

	resRel, _ := database.Exec(`INSERT INTO releases (title_id, category, folder_url, folder_name) VALUES (?, 'TV', 'http://test/boys/', 'The Boys')`, titleID)
	relID, _ := resRel.LastInsertId()

	_, _ = database.Exec(`INSERT INTO files (release_id, title_id, season_number, episode_number, episode_code, name, file_url) VALUES (?, ?, 1, 1, 'S01E01', 'S01E01.mkv', 'http://test/boys/1.mkv')`, relID, titleID)
	_, _ = database.Exec(`INSERT INTO files (release_id, title_id, season_number, episode_number, episode_code, name, file_url) VALUES (?, ?, 2, 1, 'S02E01', 'S02E01.mkv', 'http://test/boys/2.mkv')`, relID, titleID)

	// Filter by season=2
	req := httptest.NewRequest("GET", "/play/season/1.m3u?season=2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	playlist := rec.Body.String()
	if strings.Contains(playlist, "S01E01") {
		t.Errorf("expected playlist to only contain season 2, but found S01E01")
	}
	if !strings.Contains(playlist, "S02E01") {
		t.Errorf("expected playlist to contain S02E01")
	}
}

func TestSearchMoviesAndSeries(t *testing.T) {
	_, mux, database := setupTestServer(t)
	defer database.Close()

	// Seed Movie
	resM, err := database.Exec(`INSERT INTO titles (name, name_norm, year, type) VALUES ('Inception', 'inception', 2010, 'movie')`)
	if err != nil {
		t.Fatal(err)
	}
	mID, _ := resM.LastInsertId()
	_, _ = database.Exec(`INSERT INTO releases (title_id, category, folder_url, folder_name, resolution) VALUES (?, 'English Movies', 'http://test/inc/', 'Inception (2010)', '1080p')`, mID)

	// Seed Series
	resS, err := database.Exec(`INSERT INTO titles (name, name_norm, year, type) VALUES ('Dark', 'dark', 2017, 'series')`)
	if err != nil {
		t.Fatal(err)
	}
	sID, _ := resS.LastInsertId()
	_, _ = database.Exec(`INSERT INTO releases (title_id, category, folder_url, folder_name, resolution) VALUES (?, 'TV-WEB-Series', 'http://test/dark/', 'Dark (2017)', '1080p')`, sID)

	// Query movies
	reqM := httptest.NewRequest("GET", "/api/search?type=movie&limit=16&sort=recent", nil)
	recM := httptest.NewRecorder()
	mux.ServeHTTP(recM, reqM)

	if recM.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recM.Code)
	}

	var dataM map[string]interface{}
	if err := json.NewDecoder(recM.Body).Decode(&dataM); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	resultsM, ok := dataM["results"].([]interface{})
	if !ok || len(resultsM) == 0 {
		t.Fatalf("expected at least 1 movie, got: %+v", dataM)
	}

	// Query series
	reqS := httptest.NewRequest("GET", "/api/search?type=series&limit=16&sort=recent", nil)
	recS := httptest.NewRecorder()
	mux.ServeHTTP(recS, reqS)

	if recS.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recS.Code)
	}

	var dataS map[string]interface{}
	if err := json.NewDecoder(recS.Body).Decode(&dataS); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	resultsS, ok := dataS["results"].([]interface{})
	if !ok || len(resultsS) == 0 {
		t.Fatalf("expected at least 1 series, got: %+v", dataS)
	}
}

func TestSearchAPIEnhancementsAndFiles(t *testing.T) {
	srv, mux, database := setupTestServer(t)
	defer database.Close()
	srv.cfg.PublicBaseURL = "http://192.168.0.134:8087"

	// Seed Title with release and files
	res, err := database.Exec(`INSERT INTO titles (name, name_norm, year, type, thumb_hash) VALUES ('Dexter', 'dexter', 2006, 'series', 'abc12345')`)
	if err != nil {
		t.Fatal(err)
	}
	titleID, _ := res.LastInsertId()

	resRel, err := database.Exec(`INSERT INTO releases (title_id, category, folder_url, folder_name, resolution) VALUES (?, 'TV', 'http://172.16.50.12/dexter/', 'Dexter', '1080p')`, titleID)
	if err != nil {
		t.Fatal(err)
	}
	relID, _ := resRel.LastInsertId()

	_, err = database.Exec(`INSERT INTO files (release_id, title_id, season_number, episode_number, episode_code, name, file_url, file_size_bytes, resolution) VALUES (?, ?, 1, 1, 'S01E01', 'Ep1.mkv', 'http://172.16.50.12/dexter/1.mkv', 1048576, '1080p')`, relID, titleID)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Test /api/v1/search endpoint with include_files=true
	req := httptest.NewRequest("GET", "/api/v1/search?q=dexter&include_files=true", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	type SearchResponse struct {
		Query   string      `json:"query"`
		Total   int         `json:"total"`
		Results []TitleCard `json:"results"`
	}

	var resp SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if resp.Total != 1 || len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}

	item := resp.Results[0]
	if item.Name != "Dexter" {
		t.Errorf("expected name Dexter, got %s", item.Name)
	}
	if item.FolderURL != "http://172.16.50.12/dexter/" {
		t.Errorf("expected folder_url http://172.16.50.12/dexter/, got %s", item.FolderURL)
	}
	if item.WebURL != "http://192.168.0.134:8087/#/title/1" {
		t.Errorf("expected web_url http://192.168.0.134:8087/#/title/1, got %s", item.WebURL)
	}
	if item.ThumbFullURL != "http://192.168.0.134:8087/api/thumbs/1-abc12345.jpg" {
		t.Errorf("expected thumb_full_url http://192.168.0.134:8087/api/thumbs/1-abc12345.jpg, got %s", item.ThumbFullURL)
	}
	if len(item.Files) != 1 {
		t.Fatalf("expected 1 file in files array, got %d", len(item.Files))
	}
	if item.Files[0].EpisodeCode != "S01E01" {
		t.Errorf("expected episode code S01E01, got %s", item.Files[0].EpisodeCode)
	}
	if item.Files[0].FileURL != "http://172.16.50.12/dexter/1.mkv" {
		t.Errorf("expected file_url http://172.16.50.12/dexter/1.mkv, got %s", item.Files[0].FileURL)
	}
}

func TestCategorySearchMultiAndStems(t *testing.T) {
	_, mux, database := setupTestServer(t)
	defer database.Close()

	// Seed: Animation Movie, Anime Series, Documentary
	r1, _ := database.Exec(`INSERT INTO titles (name, name_norm, year, type) VALUES ('Spirited Away', 'spiritedaway', 2001, 'movie')`)
	t1, _ := r1.LastInsertId()
	_, _ = database.Exec(`INSERT INTO releases (title_id, category, folder_url, folder_name) VALUES (?, 'Animation Movies', 'http://test/Animation%20Movies/spirited/', 'Spirited Away')`, t1)

	r2, _ := database.Exec(`INSERT INTO titles (name, name_norm, year, type) VALUES ('Naruto', 'naruto', 2002, 'series')`)
	t2, _ := r2.LastInsertId()
	_, _ = database.Exec(`INSERT INTO releases (title_id, category, folder_url, folder_name) VALUES (?, 'Anime & Cartoons', 'http://test/Anime%20%26%20Cartoon/naruto/', 'Naruto')`, t2)

	r3, _ := database.Exec(`INSERT INTO titles (name, name_norm, year, type) VALUES ('13th', '13th', 2016, 'series')`)
	t3, _ := r3.LastInsertId()
	_, _ = database.Exec(`INSERT INTO releases (title_id, category, folder_url, folder_name) VALUES (?, 'Documentaries', 'http://172.16.50.10/DHAKA-FLIX-10/Documentary/13th/', '13th')`, t3)

	type SearchResponse struct {
		Total   int         `json:"total"`
		Results []TitleCard `json:"results"`
	}

	// 1. Query combined categories: Animation Movies,Anime & Cartoons
	req := httptest.NewRequest("GET", "/api/search?category=Animation%20Movies,Anime%20%26%20Cartoons", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var resp SearchResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Total != 2 {
		t.Errorf("expected 2 combined anime/animation titles, got %d", resp.Total)
	}

	// 2. Query Documentary matching /Documentary/ stem
	reqDoc := httptest.NewRequest("GET", "/api/search?category=Documentaries", nil)
	recDoc := httptest.NewRecorder()
	mux.ServeHTTP(recDoc, reqDoc)
	var respDoc SearchResponse
	_ = json.NewDecoder(recDoc.Body).Decode(&respDoc)
	if respDoc.Total != 1 {
		t.Errorf("expected 1 documentary title, got %d", respDoc.Total)
	}
}
