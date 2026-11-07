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

func TestFormatSeasons(t *testing.T) {
	tests := []struct {
		input    []int
		expected string
	}{
		{nil, ""},
		{[]int{}, ""},
		{[]int{1}, "S01"},
		{[]int{2}, "S02"},
		{[]int{1, 2, 3, 4, 5, 6}, "S01-S06"},
		{[]int{6, 3, 1, 2, 5, 4}, "S01-S06"}, // unordered input
		{[]int{1, 1, 2, 2, 3}, "S01-S03"},    // duplicates
		{[]int{1, 3, 5}, "S01, S03, S05"},
		{[]int{1, 2, 4, 5}, "S01, S02, S04, S05"},
	}

	for _, tc := range tests {
		actual := formatSeasons(tc.input)
		if actual != tc.expected {
			t.Errorf("formatSeasons(%v) = %q; want %q", tc.input, actual, tc.expected)
		}
	}
}

func TestAggregateEventsByTitle(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open DB error: %v", err)
	}
	defer database.Close()

	cfg := &config.Config{
		TelegramBotToken:    "test-token",
		TelegramAdminChatID: 123456,
		PublicBaseURL:       "http://192.168.0.134:8087",
	}
	tm, _ := thumbs.NewManager(t.TempDir(), 10)
	n := New(cfg, database, nil, tm)

	// Simulate Angel (1999) with 1 new_title event and 5 new_episodes events
	events := []EventRow{
		{
			ID:          1,
			Kind:        "new_title",
			TitleID:     104,
			TitleName:   "Angel",
			Year:        1999,
			PayloadJSON: `{"category": "TV Series", "resolution": "720p", "title": "Angel", "year": 1999}`,
			FolderURL:   "http://172.16.50.12/DHAKA-FLIX-12/Angel/",
		},
		{
			ID:          2,
			Kind:        "new_episodes",
			TitleID:     104,
			TitleName:   "Angel",
			Year:        1999,
			PayloadJSON: `{"season": 1, "episodes": ["S01E01", "S01E02"]}`,
			FolderURL:   "http://172.16.50.12/DHAKA-FLIX-12/Angel/",
		},
		{
			ID:          3,
			Kind:        "new_episodes",
			TitleID:     104,
			TitleName:   "Angel",
			Year:        1999,
			PayloadJSON: `{"season": 2, "episodes": ["S02E01"]}`,
			FolderURL:   "http://172.16.50.12/DHAKA-FLIX-12/Angel/",
		},
		{
			ID:          4,
			Kind:        "new_episodes",
			TitleID:     104,
			TitleName:   "Angel",
			Year:        1999,
			PayloadJSON: `{"season": 3, "episodes": ["S03E01"]}`,
			FolderURL:   "http://172.16.50.12/DHAKA-FLIX-12/Angel/",
		},
		{
			ID:          5,
			Kind:        "new_episodes",
			TitleID:     104,
			TitleName:   "Angel",
			Year:        1999,
			PayloadJSON: `{"season": 4, "episodes": ["S04E01"]}`,
			FolderURL:   "http://172.16.50.12/DHAKA-FLIX-12/Angel/",
		},
		{
			ID:          6,
			Kind:        "new_episodes",
			TitleID:     104,
			TitleName:   "Angel",
			Year:        1999,
			PayloadJSON: `{"season": 5, "episodes": ["S05E01"]}`,
			FolderURL:   "http://172.16.50.12/DHAKA-FLIX-12/Angel/",
		},
		// And another show Animal Kingdom
		{
			ID:          7,
			Kind:        "new_title",
			TitleID:     106,
			TitleName:   "Animal Kingdom",
			Year:        2016,
			PayloadJSON: `{"category": "TV Series", "resolution": "1080p", "title": "Animal Kingdom", "year": 2016}`,
			FolderURL:   "http://172.16.50.12/DHAKA-FLIX-12/Animal%20Kingdom/",
		},
	}

	items := n.aggregateEventsByTitle(events)
	if len(items) != 2 {
		t.Fatalf("Expected 2 aggregated titles, got %d", len(items))
	}

	// Check Angel
	angel := items[0]
	if angel.TitleID != 104 || angel.TitleName != "Angel" {
		t.Errorf("Unexpected title at index 0: %v", angel)
	}
	if len(angel.EventIDs) != 6 {
		t.Errorf("Expected 6 event IDs collapsed into Angel, got %d", len(angel.EventIDs))
	}
	if formatSeasons(angel.Seasons) != "S01-S05" {
		t.Errorf("Expected season range S01-S05, got %q", formatSeasons(angel.Seasons))
	}

	line := formatDigestLine(1, cfg.PublicBaseURL, angel)
	if !strings.Contains(line, "1. <a href=\"http://192.168.0.134:8087/#/title/104\"><b>Angel (1999)</b></a> — [720p] • <b>S01-S05</b>") {
		t.Errorf("Unexpected formatted line: %s", line)
	}

	// Check Animal Kingdom
	animal := items[1]
	if animal.TitleID != 106 || animal.TitleName != "Animal Kingdom" {
		t.Errorf("Unexpected title at index 1: %v", animal)
	}
	if len(animal.EventIDs) != 1 {
		t.Errorf("Expected 1 event ID for Animal Kingdom, got %d", len(animal.EventIDs))
	}
}
