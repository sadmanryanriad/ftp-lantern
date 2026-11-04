package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sadmanryanriad/ftp-lantern/internal/config"
	"github.com/sadmanryanriad/ftp-lantern/internal/crawler"
	"github.com/sadmanryanriad/ftp-lantern/internal/db"
	"github.com/sadmanryanriad/ftp-lantern/internal/thumbs"
)

type Notifier struct {
	cfg       *config.Config
	database  *db.DB
	crawler   *crawler.Crawler
	thumbsMgr *thumbs.Manager
	client    *http.Client
	stopChan  chan struct{}
	mu        sync.Mutex
	isPaused  bool
}

func New(cfg *config.Config, database *db.DB, c *crawler.Crawler, tm *thumbs.Manager) *Notifier {
	return &Notifier{
		cfg:       cfg,
		database:  database,
		crawler:   c,
		thumbsMgr: tm,
		client:    &http.Client{Timeout: 35 * time.Second},
		stopChan:  make(chan struct{}),
	}
}

func (n *Notifier) Start() {
	if n.cfg.TelegramBotToken == "" || n.cfg.TelegramAdminChatID == 0 {
		log.Println("[Notifier] Telegram not configured, skipping alerts and bot listener")
		return
	}

	go n.botListener()
}

func (n *Notifier) Stop() {
	close(n.stopChan)
}

type EventRow struct {
	ID          int64
	Kind        string
	TitleID     int64
	TitleName   string
	Year        int
	ThumbHash   string
	PayloadJSON string
}

func (n *Notifier) DispatchEvents() {
	if n.cfg.TelegramBotToken == "" || n.cfg.TelegramAdminChatID == 0 {
		return
	}

	n.mu.Lock()
	if n.isPaused {
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()

	rows, err := n.database.Query(`
		SELECT e.id, e.kind, e.title_id, t.name, t.year, t.thumb_hash, e.payload_json
		FROM events e
		JOIN titles t ON t.id = e.title_id
		WHERE e.digest_sent_at IS NULL AND e.failed_at IS NULL
		ORDER BY e.created_at ASC
		LIMIT 50
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	var events []EventRow
	for rows.Next() {
		var ev EventRow
		var thumbHash *string
		_ = rows.Scan(&ev.ID, &ev.Kind, &ev.TitleID, &ev.TitleName, &ev.Year, &thumbHash, &ev.PayloadJSON)
		if thumbHash != nil {
			ev.ThumbHash = *thumbHash
		}
		events = append(events, ev)
	}
	_ = rows.Close()

	if len(events) == 0 {
		return
	}

	if len(events) <= 2 {
		for _, ev := range events {
			n.sendSingleEvent(ev)
		}
	} else {
		n.sendBatchDigest(events)
	}
}

func (n *Notifier) DispatchFollowedEvents() {
	if n.cfg.TelegramBotToken == "" || n.cfg.TelegramAdminChatID == 0 {
		return
	}

	n.mu.Lock()
	if n.isPaused {
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()

	rows, err := n.database.Query(`
		SELECT e.id, e.kind, e.title_id, t.name, t.year, t.thumb_hash, e.payload_json
		FROM events e
		JOIN titles t ON t.id = e.title_id
		WHERE e.instant_sent_at IS NULL AND e.failed_at IS NULL AND t.is_following = 1
		ORDER BY e.created_at ASC
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	var events []EventRow
	for rows.Next() {
		var ev EventRow
		var thumbHash *string
		_ = rows.Scan(&ev.ID, &ev.Kind, &ev.TitleID, &ev.TitleName, &ev.Year, &thumbHash, &ev.PayloadJSON)
		if thumbHash != nil {
			ev.ThumbHash = *thumbHash
		}
		events = append(events, ev)
	}
	_ = rows.Close()

	for _, ev := range events {
		n.sendFollowedInstantAlert(ev)
	}
}

func (n *Notifier) sendFollowedInstantAlert(ev EventRow) {
	var payload map[string]interface{}
	_ = json.Unmarshal([]byte(ev.PayloadJSON), &payload)

	titleName := ev.TitleName
	if ev.Year > 0 && !strings.Contains(titleName, fmt.Sprintf("(%d)", ev.Year)) {
		titleName = fmt.Sprintf("%s (%d)", titleName, ev.Year)
	}
	titleEsc := html.EscapeString(titleName)

	var epDesc string
	if eps, ok := payload["episodes"].([]interface{}); ok && len(eps) > 0 {
		var epCodes []string
		for _, e := range eps {
			if s, ok := e.(string); ok {
				epCodes = append(epCodes, s)
			}
		}
		epDesc = fmt.Sprintf("\n⚡ New: <b>%s</b>", html.EscapeString(strings.Join(epCodes, ", ")))
	}

	res := "1080p"
	if r, ok := payload["resolution"].(string); ok && r != "" {
		res = r
	}

	caption := fmt.Sprintf(
		"🔔 <b>Instant Alert: Followed Content</b>\n📺 <b>%s</b>%s\n🎞 Quality: %s\n🔗 <a href=\"%s/#/title/%d\">Stream Now in FTP-Lantern</a>",
		titleEsc,
		epDesc,
		html.EscapeString(res),
		n.cfg.PublicBaseURL,
		ev.TitleID,
	)

	thumbPath := n.thumbsMgr.ThumbPath(ev.TitleID, ev.ThumbHash)
	var err error
	if _, statErr := os.Stat(thumbPath); statErr == nil {
		err = n.sendPhoto(thumbPath, caption)
	} else {
		err = n.sendMessage(caption)
	}

	if err != nil {
		n.handleError(ev.ID, err)
	} else {
		_, _ = n.database.Exec("UPDATE events SET instant_sent_at = CURRENT_TIMESTAMP, notified_at = CURRENT_TIMESTAMP WHERE id = ?", ev.ID)
	}
}

func (n *Notifier) DispatchDailyDigest() {
	n.DispatchEvents()
}

func (n *Notifier) sendSingleEvent(ev EventRow) {
	var payload map[string]interface{}
	_ = json.Unmarshal([]byte(ev.PayloadJSON), &payload)

	titleName := ev.TitleName
	if ev.Year > 0 && !strings.Contains(titleName, fmt.Sprintf("(%d)", ev.Year)) {
		titleName = fmt.Sprintf("%s (%d)", titleName, ev.Year)
	}
	titleEsc := html.EscapeString(titleName)
	res := "1080p"
	if r, ok := payload["resolution"].(string); ok && r != "" {
		res = r
	}

	caption := fmt.Sprintf(
		"🎬 <b>New Content Added [%s]</b>\n📺 <b>%s</b>\n🎞 Quality: %s\n🔗 <a href=\"%s/#/title/%d\">Open in FTP-Lantern</a>",
		html.EscapeString(res),
		titleEsc,
		html.EscapeString(res),
		n.cfg.PublicBaseURL,
		ev.TitleID,
	)

	thumbPath := n.thumbsMgr.ThumbPath(ev.TitleID, ev.ThumbHash)
	var err error

	if _, statErr := os.Stat(thumbPath); statErr == nil {
		err = n.sendPhoto(thumbPath, caption)
	} else {
		err = n.sendMessage(caption)
	}

	if err != nil {
		n.handleError(ev.ID, err)
	} else {
		_, _ = n.database.Exec("UPDATE events SET instant_sent_at = CURRENT_TIMESTAMP, notified_at = CURRENT_TIMESTAMP WHERE id = ?", ev.ID)
	}
}

func (n *Notifier) sendBatchDigest(events []EventRow) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🎬 <b>FTP-Lantern — %d New Updates</b>\n\n", len(events)))

	addedCount := 0
	var successIDs []int64

	for _, ev := range events {
		var payload map[string]interface{}
		_ = json.Unmarshal([]byte(ev.PayloadJSON), &payload)

		res := "1080p"
		if r, ok := payload["resolution"].(string); ok && r != "" {
			res = r
		}

		titleName := ev.TitleName
		if ev.Year > 0 && !strings.Contains(titleName, fmt.Sprintf("(%d)", ev.Year)) {
			titleName = fmt.Sprintf("%s (%d)", titleName, ev.Year)
		}

		line := fmt.Sprintf("• <b>%s</b> — [%s]\n", html.EscapeString(titleName), html.EscapeString(res))

		// Check item-boundary character threshold (3800)
		if sb.Len()+len(line) > 3800 {
			break
		}

		sb.WriteString(line)
		addedCount++
		successIDs = append(successIDs, ev.ID)
	}

	remaining := len(events) - addedCount
	if remaining > 0 {
		sb.WriteString(fmt.Sprintf("\n<i>...and %d more updates</i>\n", remaining))
	}
	sb.WriteString(fmt.Sprintf("🔗 <a href=\"%s\">Open Library</a>", n.cfg.PublicBaseURL))

	err := n.sendMessage(sb.String())
	if err != nil {
		log.Printf("[Notifier] Batch digest failed (%v), falling back to individual dispatches", err)
		for _, ev := range events {
			n.sendSingleEvent(ev)
		}
	} else {
		for _, id := range successIDs {
			_, _ = n.database.Exec("UPDATE events SET digest_sent_at = CURRENT_TIMESTAMP, notified_at = CURRENT_TIMESTAMP WHERE id = ?", id)
		}
	}
}

func (n *Notifier) sendMessage(text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.cfg.TelegramBotToken)
	body, _ := json.Marshal(map[string]interface{}{
		"chat_id":    n.cfg.TelegramAdminChatID,
		"text":       text,
		"parse_mode": "HTML",
	})

	resp, err := n.client.Post(apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (n *Notifier) sendPhoto(imagePath, caption string) error {
	file, err := os.Open(imagePath)
	if err != nil {
		return n.sendMessage(caption)
	}
	defer file.Close()

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	_ = w.WriteField("chat_id", strconv.FormatInt(n.cfg.TelegramAdminChatID, 10))
	_ = w.WriteField("caption", caption)
	_ = w.WriteField("parse_mode", "HTML")

	part, err := w.CreateFormFile("photo", "poster.jpg")
	if err != nil {
		return err
	}
	_, _ = io.Copy(part, file)
	_ = w.Close()

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", n.cfg.TelegramBotToken)
	req, err := http.NewRequest("POST", apiURL, &b)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (n *Notifier) handleError(eventID int64, err error) {
	errStr := err.Error()
	log.Printf("[Notifier] Telegram dispatch error: %v", err)

	// Auth/Config errors: pause notifier
	if strings.Contains(errStr, "status 401") || strings.Contains(errStr, "status 403") || strings.Contains(errStr, "chat not found") {
		n.mu.Lock()
		n.isPaused = true
		n.mu.Unlock()
		log.Println("[Notifier] CRITICAL: Telegram credentials invalid or chat blocked. Pausing notifier.")
		return
	}

	// Rate limit error (HTTP 429)
	if strings.Contains(errStr, "status 429") || strings.Contains(errStr, "Too Many Requests") {
		log.Printf("[Notifier] WARNING: Telegram rate limit hit (429). Will retry on next cycle.")
		return
	}

	// Payload errors (malformed HTML entity, text too long): mark event failed so queue doesn't deadlock
	if strings.Contains(errStr, "status 400") {
		_, _ = n.database.Exec("UPDATE events SET failed_at = CURRENT_TIMESTAMP WHERE id = ?", eventID)
		log.Printf("[Notifier] Marked event %d as failed to unblock queue", eventID)
	}
}

func (n *Notifier) botListener() {
	var offset int64 = 0
	log.Println("[Notifier] Telegram long-polling bot listener started")

	for {
		select {
		case <-n.stopChan:
			return
		default:
		}

		pollURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=25", n.cfg.TelegramBotToken, offset)
		resp, err := n.client.Get(pollURL)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
			_ = resp.Body.Close()
			log.Printf("[Notifier] Bot getUpdates returned HTTP %d (check bot token). Sleeping 30s.", resp.StatusCode)
			time.Sleep(30 * time.Second)
			continue
		}

		var updateResp struct {
			Ok     bool `json:"ok"`
			Result []struct {
				UpdateID int64 `json:"update_id"`
				Message  *struct {
					Chat struct {
						ID int64 `json:"id"`
					} `json:"chat"`
					Text string `json:"text"`
				} `json:"message"`
			} `json:"result"`
		}

		_ = json.NewDecoder(resp.Body).Decode(&updateResp)
		resp.Body.Close()

		if updateResp.Ok {
			for _, upd := range updateResp.Result {
				offset = upd.UpdateID + 1

				if upd.Message == nil || upd.Message.Chat.ID != n.cfg.TelegramAdminChatID {
					continue
				}

				text := strings.TrimSpace(upd.Message.Text)
				n.handleCommand(text)
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func (n *Notifier) handleCommand(text string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/status":
		titleCount, _ := n.database.TitleCount()
		fileCount, _ := n.database.FileCount()
		msg := fmt.Sprintf("🏮 <b>FTP-Lantern Status</b>\n• Titles: %d\n• Files: %d\n• Base URL: %s", titleCount, fileCount, n.cfg.PublicBaseURL)
		_ = n.sendMessage(msg)

	case "/turbo":
		hours := 10
		if len(parts) > 1 {
			if h, err := strconv.Atoi(parts[1]); err == nil && h > 0 {
				hours = h
			}
		}
		turboUntil := time.Now().Add(time.Duration(hours) * time.Hour).Format(time.RFC3339)
		_ = n.database.SetSetting("turbo_until", turboUntil)
		_ = n.database.SetSetting("turbo_interval_minutes", "15")
		_ = n.sendMessage(fmt.Sprintf("⚡ <b>Turbo Scan Activated!</b>\nChecking every 15 min for the next %d hours.", hours))

	case "/scan":
		_ = n.sendMessage("🚀 <b>Manual Scan Triggered...</b>")
		go func() {
			_ = n.crawler.Crawl(n.cfg.SourceURLs)
			n.DispatchEvents()
		}()
	}
}
