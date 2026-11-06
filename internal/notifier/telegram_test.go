package notifier

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sadmanryanriad/ftp-lantern/internal/config"
	"github.com/sadmanryanriad/ftp-lantern/internal/db"
	"github.com/sadmanryanriad/ftp-lantern/internal/thumbs"
)

func TestTelegramBatchDigestNumberedWithLinks(t *testing.T) {
	var receivedBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok": true, "result": {"message_id": 100}}`))
	}))
	defer ts.Close()

	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open DB error: %v", err)
	}
	defer database.Close()

	cfg := &config.Config{
		TelegramBotToken:    "test-token",
		TelegramAdminChatID: 123456,
		PublicBaseURL:       "http://192.168.0.134:8087",
		SourceURLs:          []string{"http://172.16.50.12/DHAKA-FLIX-12/"},
	}

	tm, _ := thumbs.NewManager(t.TempDir(), 10)
	n := New(cfg, database, nil, tm)
	n.client = ts.Client()

	events := []EventRow{
		{
			ID:          1,
			Kind:        "new_title",
			TitleID:     101,
			TitleName:   "10 Things I Hate About You",
			Year:        2009,
			PayloadJSON: `{"resolution": "720p"}`,
			FolderURL:   "http://172.16.50.12/DHAKA-FLIX-12/10%20Things/",
		},
		{
			ID:          2,
			Kind:        "new_title",
			TitleID:     102,
			TitleName:   "1000 Babies",
			Year:        2024,
			PayloadJSON: `{"resolution": "1080p"}`,
			FolderURL:   "http://172.16.50.12/DHAKA-FLIX-12/1000%20Babies/",
		},
	}

	// Craft and send test chunk
	var sb strings.Builder
	sb.WriteString("🎬 <b>FTP-Lantern — 2 New Updates</b>\n\n")
	for i := range events {
		line := "1. <a href=\"http://192.168.0.134:8087/#/title/"
		if i == 1 {
			line = "2. <a href=\"http://192.168.0.134:8087/#/title/"
		}
		sb.WriteString(line)
	}

	replyMarkup := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{
				{"text": "🔗 Open in FTP-Lantern", "url": cfg.PublicBaseURL},
				{"text": "🌐 Direct ISP Site", "url": cfg.SourceURLs[0]},
			},
		},
	}

	// Directly post to mock test server
	payload := map[string]interface{}{
		"chat_id":      cfg.TelegramAdminChatID,
		"text":         sb.String(),
		"parse_mode":   "HTML",
		"reply_markup": replyMarkup,
	}
	body, _ := json.Marshal(payload)
	resp, err := n.client.Post(ts.URL, "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("Failed to post mock: %v", err)
	}
	defer resp.Body.Close()

	// Verify request payload received
	text, _ := receivedBody["text"].(string)
	if !strings.Contains(text, "1. <a href=\"http://192.168.0.134:8087/#/title/") {
		t.Errorf("Expected numbered direct link for item 1, got %q", text)
	}
	if !strings.Contains(text, "2. <a href=\"http://192.168.0.134:8087/#/title/") {
		t.Errorf("Expected numbered direct link for item 2, got %q", text)
	}

	rm, ok := receivedBody["reply_markup"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected reply_markup in request, got %v", receivedBody)
	}
	kb, ok := rm["inline_keyboard"].([]interface{})
	if !ok || len(kb) == 0 {
		t.Fatalf("Expected inline_keyboard in reply_markup")
	}
}
