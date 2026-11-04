package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	mu sync.RWMutex
}

func Open(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", dbPath)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	sqlDB.SetMaxOpenConns(1) // SQLite single writer avoids locks
	sqlDB.SetMaxIdleConns(1)

	d := &DB{DB: sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return d, nil
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS titles (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		name          TEXT NOT NULL,
		name_norm     TEXT NOT NULL,
		year          INTEGER NOT NULL DEFAULT 0,
		type          TEXT NOT NULL DEFAULT 'series',
		poster_url    TEXT,
		thumb_hash    TEXT,
		is_following  INTEGER NOT NULL DEFAULT 0,
		first_seen    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_modified DATETIME,
		UNIQUE(name_norm, year)
	);

	CREATE TABLE IF NOT EXISTS releases (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		title_id      INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
		source_server TEXT NOT NULL DEFAULT 'DHAKA-FLIX-12',
		category      TEXT NOT NULL,
		folder_url    TEXT NOT NULL UNIQUE,
		folder_name   TEXT NOT NULL,
		resolution    TEXT NOT NULL DEFAULT 'Unknown',
		source        TEXT NOT NULL DEFAULT 'Unknown',
		has_hdr       INTEGER NOT NULL DEFAULT 0,
		first_seen    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_modified DATETIME,
		gone_at       DATETIME
	);

	CREATE TABLE IF NOT EXISTS files (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		release_id       INTEGER NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
		title_id         INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
		season_number    INTEGER,
		episode_number   INTEGER,
		episode_code     TEXT,
		name             TEXT NOT NULL,
		file_url         TEXT NOT NULL UNIQUE,
		file_size_bytes  INTEGER NOT NULL DEFAULT 0,
		resolution       TEXT NOT NULL DEFAULT 'Unknown',
		source           TEXT NOT NULL DEFAULT 'Unknown',
		has_hdr          INTEGER NOT NULL DEFAULT 0,
		first_seen       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_modified    DATETIME,
		gone_at          DATETIME
	);

	CREATE TABLE IF NOT EXISTS plays (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		file_id    INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
		title_id   INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
		played_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS events (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		kind            TEXT NOT NULL,
		title_id        INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
		release_id      INTEGER REFERENCES releases(id) ON DELETE CASCADE,
		payload_json    TEXT NOT NULL,
		created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		notified_at     DATETIME,
		instant_sent_at DATETIME,
		digest_sent_at  DATETIME,
		failed_at       DATETIME
	);

	CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS titles_fts USING fts5(
		name_norm,
		content='titles',
		content_rowid='id',
		tokenize='trigram'
	);

	CREATE TRIGGER IF NOT EXISTS titles_ai AFTER INSERT ON titles BEGIN
		INSERT INTO titles_fts(rowid, name_norm) VALUES (new.id, new.name_norm);
	END;

	CREATE TRIGGER IF NOT EXISTS titles_ad AFTER DELETE ON titles BEGIN
		INSERT INTO titles_fts(titles_fts, rowid, name_norm) VALUES('delete', old.id, old.name_norm);
	END;

	CREATE TRIGGER IF NOT EXISTS titles_au AFTER UPDATE OF name_norm ON titles BEGIN
		INSERT INTO titles_fts(titles_fts, rowid, name_norm) VALUES('delete', old.id, old.name_norm);
		INSERT INTO titles_fts(rowid, name_norm) VALUES (new.id, new.name_norm);
	END;
	`

	_, err := d.Exec(schema)
	if err != nil {
		return err
	}

	// Safe idempotent column migrations
	_, _ = d.Exec("ALTER TABLE events ADD COLUMN instant_sent_at DATETIME")
	_, _ = d.Exec("ALTER TABLE events ADD COLUMN digest_sent_at DATETIME")

	return nil
}

// Setting retrieves a string value from settings table with fallback
func (d *DB) GetSetting(key, defaultVal string) string {
	var val string
	err := d.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	if err != nil {
		return defaultVal
	}
	return val
}

// SetSetting updates or inserts a setting
func (d *DB) SetSetting(key, val string) error {
	_, err := d.Exec("INSERT INTO settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, val)
	return err
}

// TitleCount returns total active titles in database
func (d *DB) TitleCount() (int, error) {
	var count int
	err := d.QueryRow("SELECT COUNT(*) FROM titles").Scan(&count)
	return count, err
}

// FileCount returns total active files
func (d *DB) FileCount() (int, error) {
	var count int
	err := d.QueryRow("SELECT COUNT(*) FROM files WHERE gone_at IS NULL").Scan(&count)
	return count, err
}

// SetLastCrawlTime records the crawl completion timestamp
func (d *DB) SetLastCrawlTime(t time.Time) error {
	return d.SetSetting("last_crawl_time", t.Format(time.RFC3339))
}
