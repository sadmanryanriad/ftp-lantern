package parser

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	resRegex       = regexp.MustCompile(`(?i)\b(4k|2160p|1080p|720p|480p)\b`)
	sourceRegex    = regexp.MustCompile(`(?i)\b(web-?dl|webrip|bluray|bdrip|hdrip|hdtv)\b`)
	hdrRegex       = regexp.MustCompile(`(?i)\b(hdr10\+?|hdr10|hdr|dv|dolby\s*vision)\b|\bhdr10\+`)
	seasonNumRegex = regexp.MustCompile(`(?i)\bseason\s*(\d+)\b`)
	episodeRegex   = regexp.MustCompile(`(?i)\bs?(\d{1,2})[ex](\d{1,3})\b`)
	titleYearRegex = regexp.MustCompile(`^(.+?)\s*\((?:TV\s*(?:Mini\s*)?Series\s*)?(\d{4})`)
	multiSpace     = regexp.MustCompile(`\s+`)
)

type ParsedMetadata struct {
	Title         string
	TitleNorm     string
	Year          int
	Type          string // 'series' or 'movie'
	Resolution    string // '1080p', '720p', '4K', 'Unknown'
	Source        string // 'WEB-DL', 'BluRay', 'Unknown'
	HasHDR        bool
	SeasonNumber  *int
	EpisodeNumber *int
	EpisodeCode   string
}

// NormalizeTitle strips punctuation, brackets, diacritics, and spaces for trigram matching
func NormalizeTitle(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ParseFolderMetadata extracts title, year, resolution, source, and HDR from a title directory name
func ParseFolderMetadata(folderName string) ParsedMetadata {
	itemType := "movie"
	if strings.Contains(strings.ToLower(folderName), "tv series") || strings.Contains(strings.ToLower(folderName), "mini series") {
		itemType = "series"
	}

	meta := ParsedMetadata{
		Type:       itemType,
		Resolution: "Unknown",
		Source:     "Unknown",
	}

	if m := resRegex.FindString(folderName); m != "" {
		meta.Resolution = strings.ToLower(m)
		if meta.Resolution == "2160p" {
			meta.Resolution = "4k"
		}
	}

	if m := sourceRegex.FindString(folderName); m != "" {
		meta.Source = strings.ToUpper(m)
	}

	meta.HasHDR = hdrRegex.MatchString(folderName)

	matches := titleYearRegex.FindStringSubmatch(folderName)
	if len(matches) >= 3 {
		meta.Title = strings.TrimSpace(matches[1])
		if y, err := strconv.Atoi(matches[2]); err == nil {
			meta.Year = y
		}
	} else {
		// Clean up common quality/source tags if no year pattern in parentheses
		clean := resRegex.ReplaceAllString(folderName, "")
		clean = sourceRegex.ReplaceAllString(clean, "")
		clean = hdrRegex.ReplaceAllString(clean, "")
		for _, tag := range []string{"[Dual Audio]", "Dual Audio", "[Multi Audio]", "Multi Audio", "+"} {
			clean = strings.ReplaceAll(clean, tag, "")
		}
		clean = multiSpace.ReplaceAllString(clean, " ")
		meta.Title = strings.TrimSpace(clean)
		meta.Year = 0
	}

	meta.TitleNorm = NormalizeTitle(meta.Title)
	return meta
}

// ParseSeasonNumber extracts integer season number from folder names like "Season 1 (Hindi Language)"
func ParseSeasonNumber(folderName string) int {
	matches := seasonNumRegex.FindStringSubmatch(folderName)
	if len(matches) >= 2 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			return n
		}
	}
	return 1
}

// ParseEpisodeCode extracts S01E05 or episode numbers from file names
func ParseEpisodeCode(fileName string, seasonDefault int) (season int, episode int, code string) {
	season = seasonDefault
	episode = 0

	matches := episodeRegex.FindStringSubmatch(fileName)
	if len(matches) >= 3 {
		sNum, _ := strconv.Atoi(matches[1])
		eNum, _ := strconv.Atoi(matches[2])
		if sNum > 0 {
			season = sNum
		}
		episode = eNum
		code = fmt.Sprintf("S%02dE%02d", season, episode)
		return
	}

	// Fallback: look for "Episode 01" or "EP01" or ".01."
	epStandalone := regexp.MustCompile(`(?i)\b(?:ep(?:isode)?\s*|e)(\d{1,3})\b`)
	if m := epStandalone.FindStringSubmatch(fileName); len(m) >= 2 {
		episode, _ = strconv.Atoi(m[1])
		code = fmt.Sprintf("S%02dE%02d", season, episode)
		return
	}

	code = fmt.Sprintf("S%02d", season)
	return
}

// ParseSizeBytes parses strings like "1047848 KB", "784 MB", "1.2 GB" into bytes (int64)
func ParseSizeBytes(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}

	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return 0
	}

	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}

	unit := "b"
	if len(parts) >= 2 {
		unit = strings.ToLower(parts[1])
	}

	switch {
	case strings.HasPrefix(unit, "kb"):
		return int64(num * 1024)
	case strings.HasPrefix(unit, "mb"):
		return int64(num * 1024 * 1024)
	case strings.HasPrefix(unit, "gb"):
		return int64(num * 1024 * 1024 * 1024)
	case strings.HasPrefix(unit, "tb"):
		return int64(num * 1024 * 1024 * 1024 * 1024)
	default:
		return int64(num)
	}
}

// BuildSafeURL guarantees percent-encoding of path components without breaking host or query
func BuildSafeURL(baseURL, relPath string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}

	rel, err := url.Parse(relPath)
	if err != nil {
		return baseURL
	}

	resolved := base.ResolveReference(rel)
	// Normalize escaped path
	resolved.Path = path.Clean(resolved.Path)
	return resolved.String()
}
