package thumbs

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"image"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/disintegration/imaging"
)

type Job struct {
	ID        int64
	PosterURL string
	Result    chan error
}

type Manager struct {
	thumbsDir    string
	maxDiskBytes int64
	queue        chan Job
	wg           sync.WaitGroup
	client       *http.Client
}

func NewManager(thumbsDir string, maxDiskMB int) (*Manager, error) {
	if err := os.MkdirAll(thumbsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create thumbs dir: %w", err)
	}

	if maxDiskMB <= 0 {
		maxDiskMB = 250
	}

	m := &Manager{
		thumbsDir:    thumbsDir,
		maxDiskBytes: int64(maxDiskMB) * 1024 * 1024,
		queue:        make(chan Job, 500),
		client:       &http.Client{Timeout: 15 * time.Second},
	}

	// Single worker to protect 128MB RAM limit during large JPEG decodes
	m.wg.Add(1)
	go m.worker()

	return m, nil
}

func (m *Manager) Hash(posterURL string) string {
	h := md5.Sum([]byte(posterURL))
	return hex.EncodeToString(h[:4]) // 8-char short hash
}

func (m *Manager) ThumbPath(id int64, hash string) string {
	return filepath.Join(m.thumbsDir, fmt.Sprintf("%d-%s.jpg", id, hash))
}

func (m *Manager) ThumbExists(id int64, hash string) bool {
	p := m.ThumbPath(id, hash)
	info, err := os.Stat(p)
	if err == nil && info.Size() > 0 {
		// Touch file to keep LRU fresh
		_ = os.Chtimes(p, time.Now(), time.Now())
		return true
	}
	return false
}

func (m *Manager) Enqueue(id int64, posterURL string) {
	if posterURL == "" {
		return
	}
	hash := m.Hash(posterURL)
	if m.ThumbExists(id, hash) {
		return
	}

	select {
	case m.queue <- Job{ID: id, PosterURL: posterURL}:
	default:
		// Queue full, drop non-blocking
	}
}

// ProcessSync generates a thumbnail immediately (used as fallback for evicted/missing posters)
func (m *Manager) ProcessSync(id int64, hash, posterURL string) error {
	if m.ThumbExists(id, hash) {
		return nil
	}
	err := m.process(id, hash, posterURL)
	if err == nil {
		m.PruneDiskIfNeeded()
	}
	return err
}

func (m *Manager) worker() {
	defer m.wg.Done()
	jobCount := 0

	for job := range m.queue {
		hash := m.Hash(job.PosterURL)
		if m.ThumbExists(job.ID, hash) {
			continue
		}

		_ = m.process(job.ID, hash, job.PosterURL)
		jobCount++

		// Prune disk after every 25 new thumbnails
		if jobCount%25 == 0 {
			m.PruneDiskIfNeeded()
		}

		time.Sleep(20 * time.Millisecond)
	}
}

func (m *Manager) process(id int64, hash, posterURL string) error {
	resp, err := m.client.Get(posterURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code: %d", resp.StatusCode)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return fmt.Errorf("decode error: %w", err)
	}

	// Resize to width 300px, height calculated proportionally
	resized := imaging.Resize(img, 300, 0, imaging.Lanczos)

	destPath := m.ThumbPath(id, hash)
	tmpPath := destPath + ".tmp"
	defer os.Remove(tmpPath)

	if err := imaging.Save(resized, tmpPath, imaging.JPEGQuality(75)); err != nil {
		return err
	}

	return os.Rename(tmpPath, destPath)
}

type fileEntry struct {
	path    string
	size    int64
	modTime time.Time
}

// PruneDiskIfNeeded scans /data/thumbs and evicts oldest thumbnails if total size exceeds limit
func (m *Manager) PruneDiskIfNeeded() {
	entries, err := os.ReadDir(m.thumbsDir)
	if err != nil {
		return
	}

	var files []fileEntry
	var totalSize int64

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		totalSize += info.Size()
		files = append(files, fileEntry{
			path:    filepath.Join(m.thumbsDir, e.Name()),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}

	if totalSize <= m.maxDiskBytes {
		return
	}

	log.Printf("[Thumbs] Disk cap reached: %.2f MB / %.2f MB limit. Pruning oldest thumbnails...",
		float64(totalSize)/(1024*1024), float64(m.maxDiskBytes)/(1024*1024))

	// Sort oldest modTime first
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	targetSize := int64(float64(m.maxDiskBytes) * 0.80) // Prune down to 80%
	prunedCount := 0

	for _, f := range files {
		if totalSize <= targetSize {
			break
		}
		if err := os.Remove(f.path); err == nil {
			totalSize -= f.size
			prunedCount++
		}
	}

	log.Printf("[Thumbs] Pruned %d thumbnails. New size: %.2f MB", prunedCount, float64(totalSize)/(1024*1024))
}

func (m *Manager) Close() {
	close(m.queue)
	m.wg.Wait()
	m.PruneDiskIfNeeded()
}
