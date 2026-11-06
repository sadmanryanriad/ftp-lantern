package parser

import (
	"testing"
)

func TestParseFolderMetadata(t *testing.T) {
	cases := []struct {
		input    string
		title    string
		year     int
		res      string
		source   string
		hasHDR   bool
	}{
		{
			input:  "Sacred Games (TV Series 2018–2019) 1080p",
			title:  "Sacred Games",
			year:   2018,
			res:    "1080p",
			source: "Unknown",
			hasHDR: false,
		},
		{
			input:  "VisionQuest (TV Mini Series 2026) 1080p [Dual Audio] WEB-DL HDR",
			title:  "VisionQuest",
			year:   2026,
			res:    "1080p",
			source: "WEB-DL",
			hasHDR: true,
		},
		{
			input:  "S.W.A.T. (TV Series 2017–2025) 720p [Dual Audio]",
			title:  "S.W.A.T.",
			year:   2017,
			res:    "720p",
			source: "Unknown",
			hasHDR: false,
		},
		{
			input:  "Dune Part Two 2160p 4K Bluray HDR10+",
			title:  "Dune Part Two",
			year:   0,
			res:    "4k",
			source: "BLURAY",
			hasHDR: true,
		},
		{
			input:  "13th (TV Documentary 2016) 720p WEBRip",
			title:  "13th",
			year:   2016,
			res:    "720p",
			source: "WEBRIP",
			hasHDR: false,
		},
		{
			input:  "A Certain Magical Index (TV Cartoon 2008–2019) 1080p [Dual Audio]",
			title:  "A Certain Magical Index",
			year:   2008,
			res:    "1080p",
			source: "Unknown",
			hasHDR: false,
		},
		{
			input:  "World's Most Wanted (TV Documentary 2020-) 720p",
			title:  "World's Most Wanted",
			year:   2020,
			res:    "720p",
			source: "Unknown",
			hasHDR: false,
		},
	}

	for _, c := range cases {
		meta := ParseFolderMetadata(c.input)
		if meta.Title != c.title {
			t.Errorf("expected title %s, got %s", c.title, meta.Title)
		}
		if meta.Year != c.year {
			t.Errorf("expected year %d, got %d", c.year, meta.Year)
		}
		if meta.Resolution != c.res {
			t.Errorf("expected res %s, got %s", c.res, meta.Resolution)
		}
		if meta.Source != c.source {
			t.Errorf("expected source %s, got %s", c.source, meta.Source)
		}
		if meta.HasHDR != c.hasHDR {
			t.Errorf("expected HDR %v, got %v", c.hasHDR, meta.HasHDR)
		}
	}
}

func TestNormalizeTitle(t *testing.T) {
	if NormalizeTitle("Saas, Bahu Aur Flamingo") != "saasbahuaurflamingo" {
		t.Fatalf("unexpected normalized title: %s", NormalizeTitle("Saas, Bahu Aur Flamingo"))
	}
	if NormalizeTitle("S.W.A.T.") != "swat" {
		t.Fatalf("unexpected normalized title: %s", NormalizeTitle("S.W.A.T."))
	}
}

func TestParseSizeBytes(t *testing.T) {
	if ParseSizeBytes("1047848 KB") != 1047848*1024 {
		t.Errorf("expected %d, got %d", 1047848*1024, ParseSizeBytes("1047848 KB"))
	}
	if ParseSizeBytes("784 MB") != 784*1024*1024 {
		t.Errorf("expected %d, got %d", 784*1024*1024, ParseSizeBytes("784 MB"))
	}
}

func TestParseEpisodeCode(t *testing.T) {
	_, _, code := ParseEpisodeCode("Sacred.Games.S01E05.Hindi.1080p.mkv", 1)
	if code != "S01E05" {
		t.Errorf("expected S01E05, got %s", code)
	}

	_, _, code2 := ParseEpisodeCode("Series.Title.EP03.720p.mkv", 2)
	if code2 != "S02E03" {
		t.Errorf("expected S02E03, got %s", code2)
	}
}
