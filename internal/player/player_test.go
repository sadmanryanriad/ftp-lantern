package player

import (
	"strings"
	"testing"
)

func TestBuildSeasonPlaylist(t *testing.T) {
	files := []PlayableFile{
		{
			ID:           2,
			TitleName:    "Game of Thrones",
			EpisodeCode:  "S01E02",
			FileName:     "GoT.S01E02.mkv",
			FileURL:      "http://172.16.x.x/GoT/S01E02.mkv",
			Resolution:   "1080p",
			SeasonNumber: 1,
			EpNumber:     2,
		},
		{
			ID:           1,
			TitleName:    "Game of Thrones",
			EpisodeCode:  "S01E01",
			FileName:     "GoT.S01E01.mkv",
			FileURL:      "http://172.16.x.x/GoT/S01E01.mkv",
			Resolution:   "1080p",
			SeasonNumber: 1,
			EpNumber:     1,
		},
	}

	playlist := BuildSeasonPlaylist("Game of Thrones", files)
	if !strings.HasPrefix(playlist, "#EXTM3U\n") {
		t.Errorf("expected playlist to start with #EXTM3U")
	}

	// Verify natural sorting: S01E01 must appear before S01E02
	idxE01 := strings.Index(playlist, "S01E01")
	idxE02 := strings.Index(playlist, "S01E02")
	if idxE01 == -1 || idxE02 == -1 || idxE01 > idxE02 {
		t.Errorf("expected S01E01 before S01E02, got idxE01=%d, idxE02=%d", idxE01, idxE02)
	}
}

func TestBuildAndroidIntent(t *testing.T) {
	url := "http://172.16.50.12/Dhaka-Flix/Series/Show.S01E01.mkv"
	intent := BuildAndroidIntent(url)

	if !strings.HasPrefix(intent, "intent://172.16.50.12/Dhaka-Flix/Series/Show.S01E01.mkv#Intent;") {
		t.Errorf("unexpected intent URI: %s", intent)
	}
	if !strings.Contains(intent, "type=video/*") || !strings.Contains(intent, "scheme=http") {
		t.Errorf("missing MIME type or scheme in intent: %s", intent)
	}
}
