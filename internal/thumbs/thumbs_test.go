package thumbs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestThumbsManagerPruning(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize manager with a small 1MB limit for testing pruning
	mgr, err := NewManager(tempDir, 1)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close()

	// Verify hash computation
	h1 := mgr.Hash("http://example.com/poster1.jpg")
	h2 := mgr.Hash("http://example.com/poster2.jpg")
	if len(h1) != 8 || len(h2) != 8 || h1 == h2 {
		t.Errorf("expected distinct 8-char hashes, got h1=%s, h2=%s", h1, h2)
	}

	// Create 3 dummy files exceeding 1MB total
	// File 1: 500KB (oldest)
	p1 := filepath.Join(tempDir, "1-hash1.jpg")
	_ = os.WriteFile(p1, make([]byte, 500*1024), 0644)
	_ = os.Chtimes(p1, time.Now().Add(-1*time.Hour), time.Now().Add(-1*time.Hour))

	// File 2: 400KB
	p2 := filepath.Join(tempDir, "2-hash2.jpg")
	_ = os.WriteFile(p2, make([]byte, 400*1024), 0644)
	_ = os.Chtimes(p2, time.Now().Add(-30*time.Minute), time.Now().Add(-30*time.Minute))

	// File 3: 300KB (newest)
	p3 := filepath.Join(tempDir, "3-hash3.jpg")
	_ = os.WriteFile(p3, make([]byte, 300*1024), 0644)

	// Total size is 1200 KB (> 1024 KB limit).
	// Pruning should prune oldest file (p1) to get below 80% of 1MB (819 KB).
	mgr.PruneDiskIfNeeded()

	if _, err := os.Stat(p1); !os.IsNotExist(err) {
		t.Errorf("expected oldest file %s to be pruned", p1)
	}
	if _, err := os.Stat(p3); err != nil {
		t.Errorf("expected newest file %s to remain", p3)
	}
}
