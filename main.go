package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/sadmanryanriad/ftp-lantern/internal/api"
	"github.com/sadmanryanriad/ftp-lantern/internal/config"
	"github.com/sadmanryanriad/ftp-lantern/internal/crawler"
	"github.com/sadmanryanriad/ftp-lantern/internal/db"
	"github.com/sadmanryanriad/ftp-lantern/internal/notifier"
	"github.com/sadmanryanriad/ftp-lantern/internal/thumbs"
)

//go:embed all:server/ui/dist
var embeddedUI embed.FS

func main() {
	// Built-in Healthcheck subcommand for minimal scratch Docker container
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8085"
		}
		client := http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/api/status", port))
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		_ = resp.Body.Close()
		os.Exit(0)
	}

	cfg := config.Load()
	log.Printf("[Main] Starting FTP-Lantern on port %s", cfg.Port)

	// Open Database
	dbPath := filepath.Join(cfg.DataDir, "lantern.db")
	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("[Main] Failed to open database: %v", err)
	}
	defer database.Close()

	// Initialize Thumbnail Manager
	thumbsMgr, err := thumbs.NewManager(cfg.ThumbsDir, cfg.MaxThumbsDiskMB)
	if err != nil {
		log.Fatalf("[Main] Failed to init thumbs manager: %v", err)
	}
	defer thumbsMgr.Close()

	// Initialize Crawler
	c := crawler.New(database, thumbsMgr)

	// Initialize Notifier
	notif := notifier.New(cfg, database, c, thumbsMgr)
	notif.Start()
	defer notif.Stop()

	// Initialize API Server
	apiServer := api.NewServer(cfg, database, c, thumbsMgr)
	mux := http.NewServeMux()
	apiServer.RegisterRoutes(mux)

	// Serve Embedded Frontend SPA
	distFS, err := fs.Sub(embeddedUI, "server/ui/dist")
	if err == nil {
		fileServer := http.FileServer(http.FS(distFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			// If request has extension, check if file exists
			f, err := distFS.Open(stringsTrimPrefix(path, "/"))
			if err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
			// SPA fallback: serve index.html
			indexFile, err := distFS.Open("index.html")
			if err == nil {
				_ = indexFile.Close()
				r.URL.Path = "/"
				fileServer.ServeHTTP(w, r)
				return
			}
			// If no index.html yet, serve simple splash
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<h1>FTP-Lantern API Server is running</h1><p><a href="/api/status">/api/status</a></p>`))
		})
	}

	// Dynamic Background Scheduler
	stopScheduler := make(chan struct{})
	go runScheduler(database, c, notif, cfg, stopScheduler)

	// Kick off initial crawl shortly after boot
	go func() {
		time.Sleep(3 * time.Second)
		_ = c.Crawl(cfg.SourceURLs)
		notif.DispatchEvents()
	}()

	// Start HTTP Server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: corsMiddleware(mux),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Main] HTTP server error: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[Main] Shutting down gracefully...")
	close(stopScheduler)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	log.Println("[Main] Shutdown complete.")
}

func runScheduler(database *db.DB, c *crawler.Crawler, notif *notifier.Notifier, cfg *config.Config, stop <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	var lastRun time.Time

	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			// Check if Turbo Mode is active
			intervalMinutes := cfg.BaseCrawlIntervalMinutes
			turboUntil := database.GetSetting("turbo_until", "")
			if turboUntil != "" {
				if t, err := time.Parse(time.RFC3339, turboUntil); err == nil && now.Before(t) {
					turboMin, _ := strconv.Atoi(database.GetSetting("turbo_interval_minutes", "15"))
					if turboMin > 0 {
						intervalMinutes = turboMin
					}
				}
			}

			if lastRun.IsZero() || now.Sub(lastRun) >= time.Duration(intervalMinutes)*time.Minute {
				lastRun = now
				go func() {
					if err := c.Crawl(cfg.SourceURLs); err == nil {
						notif.DispatchEvents()
					}
				}()
			}
		}
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func stringsTrimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[0:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
