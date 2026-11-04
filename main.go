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

	// Startup crawl logic: Seed if empty, or catch up if last crawl was > 24h ago
	go func() {
		time.Sleep(3 * time.Second)
		count, _ := database.TitleCount()
		if count == 0 {
			log.Println("[Main] Database empty, starting initial seed crawl...")
			_ = c.Crawl(cfg.SourceURLs)
			notif.DispatchFollowedEvents()
		} else {
			lastCrawlStr := database.GetSetting("last_crawl_time", "")
			shouldCatchup := false
			if lastCrawlStr == "" {
				shouldCatchup = true
			} else if t, err := time.Parse(time.RFC3339, lastCrawlStr); err == nil {
				if time.Since(t) > 24*time.Hour {
					shouldCatchup = true
				}
			}
			if shouldCatchup {
				log.Println("[Main] Catch-up crawl starting (last crawl was >24h ago or missing)...")
				_ = c.Crawl(cfg.SourceURLs)
				notif.DispatchFollowedEvents()
			} else {
				log.Println("[Main] Database already seeded and up-to-date. Skipping boot crawl.")
			}
		}
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
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	loc, err := time.LoadLocation("Asia/Dhaka")
	if err != nil {
		loc = time.Local
	}

	var lastDailyCrawlDay int = -1
	var lastDailyDigestDay int = -1
	var lastTurboCrawl time.Time

	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			nowLocal := now.In(loc)
			currentDay := nowLocal.YearDay()

			// 1. Check if Turbo Mode is active
			isTurbo := false
			turboUntil := database.GetSetting("turbo_until", "")
			if turboUntil != "" {
				if t, err := time.Parse(time.RFC3339, turboUntil); err == nil && now.Before(t) {
					isTurbo = true
				}
			}

			// Frequency setting: "15m", "30m", "1h", "daily", "manual"
			frequency := database.GetSetting("crawl_frequency", "15m")

			if isTurbo || frequency == "15m" {
				// Crawl every 15 minutes
				if lastTurboCrawl.IsZero() || now.Sub(lastTurboCrawl) >= 15*time.Minute {
					lastTurboCrawl = now
					go func() {
						log.Printf("[Scheduler] Incremental crawl starting (15m interval)")
						if err := c.Crawl(cfg.SourceURLs); err == nil {
							notif.DispatchFollowedEvents()
						}
					}()
				}
			} else if frequency == "30m" {
				if lastTurboCrawl.IsZero() || now.Sub(lastTurboCrawl) >= 30*time.Minute {
					lastTurboCrawl = now
					go func() {
						log.Printf("[Scheduler] Incremental crawl starting (30m interval)")
						if err := c.Crawl(cfg.SourceURLs); err == nil {
							notif.DispatchFollowedEvents()
						}
					}()
				}
			} else if frequency == "1h" {
				if lastTurboCrawl.IsZero() || now.Sub(lastTurboCrawl) >= 1*time.Hour {
					lastTurboCrawl = now
					go func() {
						log.Printf("[Scheduler] Incremental crawl starting (1h interval)")
						if err := c.Crawl(cfg.SourceURLs); err == nil {
							notif.DispatchFollowedEvents()
						}
					}()
				}
			} else if frequency == "daily" {
				// Normal daily schedule: crawl at 07:00 AM Asia/Dhaka
				if nowLocal.Hour() == 7 && nowLocal.Minute() == 0 && lastDailyCrawlDay != currentDay {
					lastDailyCrawlDay = currentDay
					go func() {
						log.Printf("[Scheduler] Daily scheduled crawl starting at 07:00 AM")
						if err := c.Crawl(cfg.SourceURLs); err == nil {
							notif.DispatchFollowedEvents()
						}
					}()
				}
			}

			// 2. Daily Morning Digest at 08:01 AM Asia/Dhaka
			if nowLocal.Hour() == 8 && nowLocal.Minute() == 1 && lastDailyDigestDay != currentDay {
				lastDailyDigestDay = currentDay
				go func() {
					log.Printf("[Scheduler] Daily morning digest dispatching at 08:01 AM")
					notif.DispatchDailyDigest()
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
