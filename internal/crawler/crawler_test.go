package crawler

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sadmanryanriad/ftp-lantern/internal/db"
	"github.com/sadmanryanriad/ftp-lantern/internal/thumbs"
)

func TestCrawlHotSetNoDeadlock(t *testing.T) {
	tempDir := t.TempDir()
	database, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

	thumbsMgr, err := thumbs.NewManager(filepath.Join(tempDir, "thumbs"), 10)
	if err != nil {
		t.Fatalf("failed to init thumbs: %v", err)
	}
	defer thumbsMgr.Close()

	c := New(database, thumbsMgr)

	// Insert title followed or modified recently
	res, err := database.Exec(`
		INSERT INTO titles (name, name_norm, year, type, is_following, last_modified)
		VALUES ('Test Show', 'testshow', 2026, 'series', 1, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		t.Fatalf("insert title failed: %v", err)
	}
	titleID, _ := res.LastInsertId()

	resRel, err := database.Exec(`
		INSERT INTO releases (title_id, category, folder_url, folder_name, resolution, source)
		VALUES (?, 'TV-WEB-Series', 'http://127.0.0.1:9999/dummy/', 'Test Show (2026)', '1080p', 'WEB-DL')
	`, titleID)
	if err != nil {
		t.Fatalf("insert release failed: %v", err)
	}
	releaseID, _ := resRel.LastInsertId()

	_, err = database.Exec(`
		INSERT INTO files (release_id, title_id, season_number, episode_number, episode_code, name, file_url, last_modified)
		VALUES (?, ?, 1, 1, 'S01E01', 'Test.Show.S01E01.mkv', 'http://127.0.0.1:9999/dummy/S01E01.mkv', CURRENT_TIMESTAMP)
	`, releaseID, titleID)
	if err != nil {
		t.Fatalf("insert file failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		// crawlHotSet should run and complete within 1 second without SQLite connection deadlock
		c.crawlHotSet()
		close(done)
	}()

	select {
	case <-done:
		// Succeeded without deadlock!
	case <-time.After(2 * time.Second):
		t.Fatal("crawlHotSet deadlocked on single SQLite connection!")
	}
}
