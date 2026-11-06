package player

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type PlayableFile struct {
	ID           int64
	TitleName    string
	EpisodeCode  string
	FileName     string
	FileURL      string
	Resolution   string
	SeasonNumber int
	EpNumber     int
}

// BuildSeasonPlaylist creates a formatted UTF-8 M3U playlist for continuous auto-next in VLC / MPV
func BuildSeasonPlaylist(titleName string, files []PlayableFile) string {
	// Sort by SeasonNumber and EpNumber
	sort.Slice(files, func(i, j int) bool {
		if files[i].SeasonNumber != files[j].SeasonNumber {
			return files[i].SeasonNumber < files[j].SeasonNumber
		}
		if files[i].EpNumber != files[j].EpNumber {
			return files[i].EpNumber < files[j].EpNumber
		}
		return files[i].FileName < files[j].FileName
	})

	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")

	for _, f := range files {
		label := fmt.Sprintf("%s - %s [%s]", titleName, f.EpisodeCode, f.Resolution)
		if f.EpisodeCode == "" {
			label = fmt.Sprintf("%s [%s]", f.FileName, f.Resolution)
		}

		sb.WriteString(fmt.Sprintf("#EXTINF:-1 tvg-name=\"%s %s\",%s\n", titleName, f.EpisodeCode, label))
		sb.WriteString(f.FileURL)
		sb.WriteString("\n")
	}

	return sb.String()
}

// BuildSingleFilePlaylist creates an M3U playlist for a single file
func BuildSingleFilePlaylist(titleName string, f PlayableFile) string {
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	label := fmt.Sprintf("%s - %s [%s]", titleName, f.EpisodeCode, f.Resolution)
	if f.EpisodeCode == "" {
		label = fmt.Sprintf("%s [%s]", f.FileName, f.Resolution)
	}
	sb.WriteString(fmt.Sprintf("#EXTINF:-1 tvg-name=\"%s %s\",%s\n", titleName, f.EpisodeCode, label))
	sb.WriteString(f.FileURL)
	sb.WriteString("\n")
	return sb.String()
}

// BuildAndroidIntent generates standard Android Intent URI for VLC/MX Player/Just Player
func BuildAndroidIntent(fileURL string) string {
	// Parse URL
	u, err := url.Parse(fileURL)
	if err != nil {
		return fileURL
	}

	// Format: intent://host/path#Intent;action=android.intent.action.VIEW;type=video/*;scheme=http;end
	scheme := u.Scheme
	if scheme == "" {
		scheme = "http"
	}

	rawWithoutScheme := strings.TrimPrefix(fileURL, scheme+"://")
	return fmt.Sprintf("intent://%s#Intent;action=android.intent.action.VIEW;type=video/*;scheme=%s;end", rawWithoutScheme, scheme)
}
