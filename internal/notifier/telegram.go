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
	"sort"
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

	if n.cfg.TelegramEnableListener {
		log.Println("[Notifier] Telegram inbound bot listener enabled")
		go n.botListener()
	} else {
		log.Println("[Notifier] Telegram outbound notifications active (inbound bot listener disabled: homelab owns the bot)")
	}
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
	FolderURL   string
}

type AggregatedTitleItem struct {
	TitleID      int64
	TitleName    string
	Year         int
	ThumbHash    string
	FolderURL    string
	EventIDs     []int64
	Resolutions  []string
	Seasons      []int
	EpisodeCodes []string
	Kinds        map[string]bool
}

func containsString(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func containsInt(slice []int, val int) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func formatSeasons(seasons []int) string {
	if len(seasons) == 0 {
		return ""
	}
	uniq := make(map[int]bool)
	var sorted []int
	for _, s := range seasons {
		if s > 0 && !uniq[s] {
			uniq[s] = true
			sorted = append(sorted, s)
		}
	}
	sort.Ints(sorted)
	if len(sorted) == 0 {
		return ""
	}
	if len(sorted) == 1 {
		return fmt.Sprintf("S%02d", sorted[0])
	}
	isRange := true
	for i := 1; i < len(sorted); i++ {
		if sorted[i] != sorted[i-1]+1 {
			isRange = false
			break
		}
	}
	if isRange {
		return fmt.Sprintf("S%02d-S%02d", sorted[0], sorted[len(sorted)-1])
	}
	var parts []string
	for _, s := range sorted {
		parts = append(parts, fmt.Sprintf("S%02d", s))
	}
	return strings.Join(parts, ", ")
}

func (n *Notifier) aggregateEventsByTitle(events []EventRow) []*AggregatedTitleItem {
	var orderedItems []*AggregatedTitleItem
	itemMap := make(map[int64]*AggregatedTitleItem)

	for _, ev := range events {
		item, exists := itemMap[ev.TitleID]
		if !exists {
			item = &AggregatedTitleItem{
				TitleID:   ev.TitleID,
				TitleName: ev.TitleName,
				Year:      ev.Year,
				ThumbHash: ev.ThumbHash,
				FolderURL: ev.FolderURL,
				Kinds:     make(map[string]bool),
			}
			itemMap[ev.TitleID] = item
			orderedItems = append(orderedItems, item)
		}

		item.EventIDs = append(item.EventIDs, ev.ID)
		item.Kinds[ev.Kind] = true

		if item.FolderURL == "" && ev.FolderURL != "" {
			item.FolderURL = ev.FolderURL
		}
		if item.ThumbHash == "" && ev.ThumbHash != "" {
			item.ThumbHash = ev.ThumbHash
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(ev.PayloadJSON), &payload); err == nil {
			if res, ok := payload["resolution"].(string); ok && res != "" {
				if !containsString(item.Resolutions, res) {
					item.Resolutions = append(item.Resolutions, res)
				}
			}
			if sVal, ok := payload["season"]; ok {
				var sNum int
				switch v := sVal.(type) {
				case float64:
					sNum = int(v)
				case int:
					sNum = v
				}
				if sNum > 0 && !containsInt(item.Seasons, sNum) {
					item.Seasons = append(item.Seasons, sNum)
				}
			}
			if epList, ok := payload["episodes"].([]interface{}); ok {
				for _, ep := range epList {
					if epStr, ok := ep.(string); ok && epStr != "" {
						if !containsString(item.EpisodeCodes, epStr) {
							item.EpisodeCodes = append(item.EpisodeCodes, epStr)
						}
					}
				}
			}
		}
	}

	return orderedItems
}

func formatDigestLine(index int, baseURL string, item *AggregatedTitleItem) string {
	titleName := item.TitleName
	if item.Year > 0 && !strings.Contains(titleName, fmt.Sprintf("(%d)", item.Year)) {
		titleName = fmt.Sprintf("%s (%d)", titleName, item.Year)
	}

	resStr := "1080p"
	if len(item.Resolutions) > 0 {
		resStr = strings.Join(item.Resolutions, ", ")
	}

	line := fmt.Sprintf("%d. <a href=\"%s/#/title/%d\"><b>%s</b></a> — [%s]", index, baseURL, item.TitleID, html.EscapeString(titleName), html.EscapeString(resStr))

	if len(item.Seasons) > 0 {
		line += fmt.Sprintf(" • <b>%s</b>", formatSeasons(item.Seasons))
	} else if len(item.EpisodeCodes) > 0 && len(item.EpisodeCodes) <= 3 {
		line += fmt.Sprintf(" • <b>%s</b>", html.EscapeString(strings.Join(item.EpisodeCodes, ", ")))
	}

	if item.FolderURL != "" {
		line += fmt.Sprintf(" • <a href=\"%s\">ISP</a>", html.EscapeString(item.FolderURL))
	}
	line += "\n"
	return line
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

	// Clear any stale events older than 48 hours to prevent backlog storms
	_, _ = n.database.Exec("UPDATE events SET digest_sent_at = CURRENT_TIMESTAMP WHERE digest_sent_at IS NULL AND created_at < datetime('now', '-48 hours')")

	rows, err := n.database.Query(`
		SELECT e.id, e.kind, e.title_id, t.name, t.year, t.thumb_hash, e.payload_json, COALESCE(r.folder_url, '')
		FROM events e
		JOIN titles t ON t.id = e.title_id
		LEFT JOIN releases r ON r.id = e.release_id
		WHERE e.digest_sent_at IS NULL AND e.failed_at IS NULL
		ORDER BY e.created_at ASC
		LIMIT 150
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	var events []EventRow
	for rows.Next() {
		var ev EventRow
		var thumbHash *string
		var folderURL string
		_ = rows.Scan(&ev.ID, &ev.Kind, &ev.TitleID, &ev.TitleName, &ev.Year, &thumbHash, &ev.PayloadJSON, &folderURL)
		if thumbHash != nil {
			ev.ThumbHash = *thumbHash
		}
		ev.FolderURL = folderURL
		if ev.FolderURL == "" {
			var fUrl string
			_ = n.database.QueryRow("SELECT folder_url FROM releases WHERE title_id = ? AND gone_at IS NULL ORDER BY id ASC LIMIT 1", ev.TitleID).Scan(&fUrl)
			ev.FolderURL = fUrl
		}
		events = append(events, ev)
	}
	_ = rows.Close()

	if len(events) == 0 {
		return
	}

	// Group and deduplicate events by TitleID
	items := n.aggregateEventsByTitle(events)
	if len(items) == 0 {
		return
	}

	if len(items) == 1 {
		n.sendSingleAggregatedTitle(items[0])
	} else {
		n.sendBatchDigest(items)
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

	notifyMode := n.database.GetSetting("telegram_notify_mode", "following_only")
	if notifyMode == "muted" {
		return
	}

	query := `
		SELECT e.id, e.kind, e.title_id, t.name, t.year, t.thumb_hash, e.payload_json, COALESCE(r.folder_url, '')
		FROM events e
		JOIN titles t ON t.id = e.title_id
		LEFT JOIN releases r ON r.id = e.release_id
		WHERE e.instant_sent_at IS NULL AND e.failed_at IS NULL
	`
	if notifyMode == "following_only" {
		query += " AND t.is_following = 1"
	}
	query += " ORDER BY e.created_at ASC"

	rows, err := n.database.Query(query)
	if err != nil {
		return
	}
	defer rows.Close()

	var events []EventRow
	for rows.Next() {
		var ev EventRow
		var thumbHash *string
		var folderURL string
		_ = rows.Scan(&ev.ID, &ev.Kind, &ev.TitleID, &ev.TitleName, &ev.Year, &thumbHash, &ev.PayloadJSON, &folderURL)
		if thumbHash != nil {
			ev.ThumbHash = *thumbHash
		}
		ev.FolderURL = folderURL
		if ev.FolderURL == "" {
			var fUrl string
			_ = n.database.QueryRow("SELECT folder_url FROM releases WHERE title_id = ? AND gone_at IS NULL ORDER BY id ASC LIMIT 1", ev.TitleID).Scan(&fUrl)
			ev.FolderURL = fUrl
		}
		events = append(events, ev)
	}
	_ = rows.Close()

	if len(events) == 0 {
		return
	}

	// Deduplicate followed items by TitleID so fans don't receive spam photos for every season/file
	items := n.aggregateEventsByTitle(events)
	for _, item := range items {
		n.sendFollowedInstantAlertAggregated(item)
	}
}

func (n *Notifier) sendFollowedInstantAlertAggregated(item *AggregatedTitleItem) {
	titleName := item.TitleName
	if item.Year > 0 && !strings.Contains(titleName, fmt.Sprintf("(%d)", item.Year)) {
		titleName = fmt.Sprintf("%s (%d)", titleName, item.Year)
	}
	titleEsc := html.EscapeString(titleName)

	var epDesc string
	if len(item.EpisodeCodes) > 0 {
		displayCodes := item.EpisodeCodes
		if len(displayCodes) > 8 {
			displayCodes = append(displayCodes[:8], fmt.Sprintf("+%d more", len(displayCodes)-8))
		}
		epDesc = fmt.Sprintf("\n⚡ New: <b>%s</b>", html.EscapeString(strings.Join(displayCodes, ", ")))
	} else if len(item.Seasons) > 0 {
		epDesc = fmt.Sprintf("\n⚡ New: <b>%s</b>", formatSeasons(item.Seasons))
	}

	resStr := "1080p"
	if len(item.Resolutions) > 0 {
		resStr = strings.Join(item.Resolutions, ", ")
	}

	caption := fmt.Sprintf(
		"🔔 <b>Instant Alert: Followed Content</b>\n📺 <b>%s</b>%s\n🎞 Quality: %s\n🔗 <a href=\"%s/#/title/%d\">Stream Now in FTP-Lantern</a>",
		titleEsc,
		epDesc,
		html.EscapeString(resStr),
		n.cfg.PublicBaseURL,
		item.TitleID,
	)

	thumbPath := n.thumbsMgr.ThumbPath(item.TitleID, item.ThumbHash)
	var err error
	if _, statErr := os.Stat(thumbPath); statErr == nil {
		err = n.sendPhoto(thumbPath, caption)
	} else {
		err = n.sendMessage(caption)
	}

	if err != nil {
		for _, id := range item.EventIDs {
			n.handleError(id, err)
		}
	} else {
		for _, id := range item.EventIDs {
			_, _ = n.database.Exec("UPDATE events SET instant_sent_at = CURRENT_TIMESTAMP, notified_at = CURRENT_TIMESTAMP WHERE id = ?", id)
		}
	}
}

func (n *Notifier) DispatchDailyDigest() {
	n.DispatchEvents()
}

func (n *Notifier) sendSingleAggregatedTitle(item *AggregatedTitleItem) {
	baseURL := strings.TrimRight(n.cfg.PublicBaseURL, "/")
	titleName := item.TitleName
	if item.Year > 0 && !strings.Contains(titleName, fmt.Sprintf("(%d)", item.Year)) {
		titleName = fmt.Sprintf("%s (%d)", titleName, item.Year)
	}
	titleEsc := html.EscapeString(titleName)

	resStr := "1080p"
	if len(item.Resolutions) > 0 {
		resStr = strings.Join(item.Resolutions, ", ")
	}

	qualityDetail := resStr
	if len(item.Seasons) > 0 {
		qualityDetail += " • " + formatSeasons(item.Seasons)
	}

	var epDesc string
	if len(item.EpisodeCodes) > 0 {
		displayCodes := item.EpisodeCodes
		if len(displayCodes) > 6 {
			displayCodes = append(displayCodes[:6], fmt.Sprintf("+%d more", len(displayCodes)-6))
		}
		epDesc = fmt.Sprintf("\n⚡ Episodes: <b>%s</b>", html.EscapeString(strings.Join(displayCodes, ", ")))
	}

	caption := fmt.Sprintf(
		"🎬 <b>New Content Added [%s]</b>\n📺 <b><a href=\"%s/#/title/%d\">%s</a></b>%s\n🎞 Quality: %s\n🔗 <a href=\"%s/#/title/%d\">Open in FTP-Lantern</a>",
		html.EscapeString(resStr),
		baseURL,
		item.TitleID,
		titleEsc,
		epDesc,
		html.EscapeString(qualityDetail),
		baseURL,
		item.TitleID,
	)
	if item.FolderURL != "" {
		caption += fmt.Sprintf("\n🌐 <a href=\"%s\">Open ISP Folder</a>", html.EscapeString(item.FolderURL))
	}

	var rowButtons []map[string]string
	rowButtons = append(rowButtons, map[string]string{
		"text": "🔗 Open in FTP-Lantern",
		"url":  fmt.Sprintf("%s/#/title/%d", baseURL, item.TitleID),
	})
	if item.FolderURL != "" {
		rowButtons = append(rowButtons, map[string]string{
			"text": "🌐 Direct ISP Folder",
			"url":  item.FolderURL,
		})
	}
	replyMarkup := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{rowButtons},
	}

	thumbPath := n.thumbsMgr.ThumbPath(item.TitleID, item.ThumbHash)
	var err error

	if _, statErr := os.Stat(thumbPath); statErr == nil {
		err = n.sendPhotoWithKeyboard(thumbPath, caption, replyMarkup)
	} else {
		err = n.sendMessageWithKeyboard(caption, replyMarkup)
	}

	if err != nil {
		for _, id := range item.EventIDs {
			n.handleError(id, err)
		}
	} else {
		for _, id := range item.EventIDs {
			_, _ = n.database.Exec("UPDATE events SET digest_sent_at = CURRENT_TIMESTAMP, instant_sent_at = CURRENT_TIMESTAMP, notified_at = CURRENT_TIMESTAMP WHERE id = ?", id)
		}
	}
}

func (n *Notifier) sendBatchDigest(items []*AggregatedTitleItem) {
	baseURL := strings.TrimRight(n.cfg.PublicBaseURL, "/")
	ispSiteURL := ""
	if len(n.cfg.SourceURLs) > 0 {
		ispSiteURL = n.cfg.SourceURLs[0]
	}

	var inlineKeyboard [][]map[string]string
	rowButtons := []map[string]string{
		{"text": "🔗 Open in FTP-Lantern", "url": baseURL},
	}
	if ispSiteURL != "" {
		rowButtons = append(rowButtons, map[string]string{
			"text": "🌐 Direct ISP Site",
			"url":  ispSiteURL,
		})
	}
	inlineKeyboard = append(inlineKeyboard, rowButtons)
	replyMarkup := map[string]interface{}{
		"inline_keyboard": inlineKeyboard,
	}

	totalTitles := len(items)
	itemIndex := 0

	for itemIndex < totalTitles {
		chunkStart := itemIndex
		var chunkLines []string
		var chunkEventIDs []int64
		currentLen := 0

		for itemIndex < totalTitles {
			item := items[itemIndex]
			line := formatDigestLine(itemIndex+1, baseURL, item)
			if len(chunkLines) > 0 && currentLen+len(line) > 3300 {
				break
			}
			chunkLines = append(chunkLines, line)
			chunkEventIDs = append(chunkEventIDs, item.EventIDs...)
			currentLen += len(line)
			itemIndex++
		}
		chunkEnd := itemIndex

		var sb strings.Builder
		if totalTitles > len(chunkLines) {
			sb.WriteString(fmt.Sprintf("🎬 <b>FTP-Lantern — Updates (%d-%d of %d)</b>\n\n", chunkStart+1, chunkEnd, totalTitles))
		} else {
			sb.WriteString(fmt.Sprintf("🎬 <b>FTP-Lantern — %d New Updates</b>\n\n", totalTitles))
		}

		for _, l := range chunkLines {
			sb.WriteString(l)
		}

		sb.WriteString(fmt.Sprintf("\n🔗 <a href=\"%s\">Open Library</a>", baseURL))
		if ispSiteURL != "" {
			sb.WriteString(fmt.Sprintf(" • 🌐 <a href=\"%s\">Open ISP Media Server</a>", html.EscapeString(ispSiteURL)))
		}

		err := n.sendMessageWithKeyboard(sb.String(), replyMarkup)
		if err != nil {
			log.Printf("[Notifier] Batch digest chunk failed (%v)", err)
			for _, id := range chunkEventIDs {
				n.handleError(id, err)
			}
			break
		} else {
			for _, id := range chunkEventIDs {
				_, _ = n.database.Exec("UPDATE events SET digest_sent_at = CURRENT_TIMESTAMP, notified_at = CURRENT_TIMESTAMP WHERE id = ?", id)
			}
		}

		if itemIndex < totalTitles {
			time.Sleep(1 * time.Second)
		}
	}
}

func (n *Notifier) sendMessage(text string) error {
	return n.sendMessageWithKeyboard(text, nil)
}

func (n *Notifier) sendMessageWithKeyboard(text string, replyMarkup interface{}) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.cfg.TelegramBotToken)
	payload := map[string]interface{}{
		"chat_id":    n.cfg.TelegramAdminChatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}
	body, _ := json.Marshal(payload)

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
	return n.sendPhotoWithKeyboard(imagePath, caption, nil)
}

func (n *Notifier) sendPhotoWithKeyboard(imagePath, caption string, replyMarkup interface{}) error {
	file, err := os.Open(imagePath)
	if err != nil {
		return n.sendMessageWithKeyboard(caption, replyMarkup)
	}
	defer file.Close()

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	_ = w.WriteField("chat_id", strconv.FormatInt(n.cfg.TelegramAdminChatID, 10))
	_ = w.WriteField("caption", caption)
	_ = w.WriteField("parse_mode", "HTML")
	if replyMarkup != nil {
		rmBytes, _ := json.Marshal(replyMarkup)
		_ = w.WriteField("reply_markup", string(rmBytes))
	}

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
