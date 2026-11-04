package db

import (
	"path/filepath"
	"testing"
)

func TestDBLifecycleAndFTS(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_lantern.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer database.Close()

	// Verify settings
	if err := database.SetSetting("test_key", "hello"); err != nil {
		t.Fatalf("failed to set setting: %v", err)
	}
	val := database.GetSetting("test_key", "fallback")
	if val != "hello" {
		t.Errorf("expected hello, got %s", val)
	}
	fallback := database.GetSetting("non_existent", "default")
	if fallback != "default" {
		t.Errorf("expected default, got %s", fallback)
	}

	// Verify counts
	tc, err := database.TitleCount()
	if err != nil || tc != 0 {
		t.Errorf("expected 0 titles, got %d (err: %v)", tc, err)
	}
	fc, err := database.FileCount()
	if err != nil || fc != 0 {
		t.Errorf("expected 0 files, got %d (err: %v)", fc, err)
	}

	// Test Title insert and FTS5 synchronization trigger
	res, err := database.Exec(`
		INSERT INTO titles (name, name_norm, year, type)
		VALUES ('Breaking Bad', 'breakingbad', 2008, 'series')
	`)
	if err != nil {
		t.Fatalf("failed to insert title: %v", err)
	}
	titleID, _ := res.LastInsertId()

	// Verify FTS5 trigram index automatically indexed it
	var ftsRowID int64
	err = database.QueryRow("SELECT rowid FROM titles_fts WHERE name_norm LIKE '%' || ? || '%'", "king").Scan(&ftsRowID)
	if err != nil || ftsRowID != titleID {
		t.Errorf("FTS5 trigram query failed, expected rowid %d, got %d (err: %v)", titleID, ftsRowID, err)
	}

	// Test update trigger
	_, err = database.Exec("UPDATE titles SET name_norm = 'bettercallsaul' WHERE id = ?", titleID)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	err = database.QueryRow("SELECT rowid FROM titles_fts WHERE name_norm LIKE '%' || ? || '%'", "saul").Scan(&ftsRowID)
	if err != nil || ftsRowID != titleID {
		t.Errorf("FTS5 update trigger failed, expected rowid %d, got %d (err: %v)", titleID, ftsRowID, err)
	}

	// Test delete trigger
	_, err = database.Exec("DELETE FROM titles WHERE id = ?", titleID)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	err = database.QueryRow("SELECT rowid FROM titles_fts WHERE name_norm LIKE '%' || ? || '%'", "saul").Scan(&ftsRowID)
	if err == nil {
		t.Errorf("expected no rows in FTS after delete, but found rowid %d", ftsRowID)
	}
}
