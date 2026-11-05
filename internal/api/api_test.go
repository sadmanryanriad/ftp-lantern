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

	// Request Continue Watching
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

	if len(items) != 1 {
		t.Fatalf("expected 1 continue watching item, got %d", len(items))
	}

	if items[0].TitleName != "Severance" {
		t.Errorf("expected title Severance, got %s", items[0].TitleName)
	}

	// Verify next episode was populated
	if items[0].NextEpisodeCode != "S01E02" {
		t.Errorf("expected next episode S01E02, got %s", items[0].NextEpisodeCode)
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
