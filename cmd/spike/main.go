package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Item struct {
	Name         string
	URL          string
	LastModified string
	Size         string
	IsDir        bool
}

var (
	resRegex    = regexp.MustCompile(`(?i)\b(4k|2160p|1080p|720p|480p)\b`)
	sourceRegex = regexp.MustCompile(`(?i)\b(web-?dl|webrip|bluray|bdrip|hdrip|hdtv)\b`)
	hdrRegex    = regexp.MustCompile(`(?i)\b(hdr10\+?|hdr|dv|dolby\s*vision)\b`)
	titleRegex  = regexp.MustCompile(`^(.+?)\s*\((?:TV\s*(?:Mini\s*)?Series\s*)?(\d{4})`)
)

func fetchDir(targetURL string) ([]Item, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(targetURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var items []Item
	baseURL, _ := url.Parse(targetURL)

	doc.Find("#fallback table tr").Each(func(i int, s *goquery.Selection) {
		nameLink := s.Find("td.fb-n a")
		if nameLink.Length() == 0 {
			return
		}
		name := strings.TrimSpace(nameLink.Text())
		if name == "Parent Directory" || name == ".." {
			return
		}

		href, exists := nameLink.Attr("href")
		if !exists {
			return
		}

		relURL, err := url.Parse(href)
		if err != nil {
			return
		}
		fullURL := baseURL.ResolveReference(relURL).String()

		date := strings.TrimSpace(s.Find("td.fb-d").Text())
		size := strings.TrimSpace(s.Find("td.fb-s").Text())

		imgAlt, _ := s.Find("td.fb-i img").Attr("alt")
		isDir := imgAlt == "folder" || strings.HasSuffix(href, "/")

		items = append(items, Item{
			Name:         name,
			URL:          fullURL,
			LastModified: date,
			Size:         size,
			IsDir:        isDir,
		})
	})

	return items, nil
}

func parseMetadata(folderName string) (title string, year string, res string, source string, hasHDR bool) {
	resMatch := resRegex.FindString(folderName)
	if resMatch != "" {
		res = strings.ToLower(resMatch)
	} else {
		res = "Unknown"
	}

	srcMatch := sourceRegex.FindString(folderName)
	if srcMatch != "" {
		source = strings.ToUpper(srcMatch)
	} else {
		source = "Unknown"
	}

	hasHDR = hdrRegex.MatchString(folderName)

	matches := titleRegex.FindStringSubmatch(folderName)
	if len(matches) >= 3 {
		title = strings.TrimSpace(matches[1])
		year = matches[2]
	} else {
		title = folderName
		year = "0"
	}

	return
}

func main() {
	baseURL := os.Getenv("SOURCE_URL")
	if baseURL == "" {
		baseURL = "http://172.16.50.12/DHAKA-FLIX-12/TV-WEB-Series/"
	}
	fmt.Println("=== STEP 1: Crawling Root Categories ===")
	categories, err := fetchDir(baseURL)
	if err != nil {
		panic(err)
	}

	for _, cat := range categories {
		if cat.IsDir {
			fmt.Printf("📂 Category: %s | URL: %s | LastModified: %s\n", cat.Name, cat.URL, cat.LastModified)
		}
	}

	// Pick "TV Series ♦ S — Z"
	var szCat *Item
	for i := range categories {
		if strings.Contains(categories[i].Name, "S  —  Z") {
			szCat = &categories[i]
			break
		}
	}

	if szCat == nil {
		fmt.Println("S - Z category not found, using first dir")
		szCat = &categories[0]
	}

	fmt.Printf("\n=== STEP 2: Crawling Category %s ===\n", szCat.Name)
	titles, err := fetchDir(szCat.URL)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Total titles found in category: %d\n", len(titles))

	// Find Sacred Games or VisionQuest or sample first 3
	var sampleTitle *Item
	for i := range titles {
		if strings.Contains(titles[i].Name, "Sacred Games") {
			sampleTitle = &titles[i]
			break
		}
	}
	if sampleTitle == nil && len(titles) > 0 {
		sampleTitle = &titles[0]
	}

	cleanTitle, year, res, source, hasHDR := parseMetadata(sampleTitle.Name)
	fmt.Printf("🎯 Inspecting Title: %s\n", sampleTitle.Name)
	fmt.Printf("   Clean Title: %s\n   Year: %s\n   Resolution: %s\n   Source: %s\n   HDR: %v\n   LastModified: %s\n",
		cleanTitle, year, res, source, hasHDR, sampleTitle.LastModified)

	fmt.Printf("\n=== STEP 3: Crawling Title Folder ===\n")
	titleContents, err := fetchDir(sampleTitle.URL)
	if err != nil {
		panic(err)
	}

	var seasons []Item
	var posters []Item
	for _, it := range titleContents {
		if it.IsDir {
			seasons = append(seasons, it)
			fmt.Printf("   📂 Subfolder/Season: %s (Modified: %s)\n", it.Name, it.LastModified)
		} else {
			if strings.HasSuffix(strings.ToLower(it.Name), ".jpg") || strings.HasSuffix(strings.ToLower(it.Name), ".png") {
				posters = append(posters, it)
				fmt.Printf("   🖼️  Poster: %s (Size: %s)\n", it.Name, it.Size)
			}
		}
	}

	if len(seasons) > 0 {
		fmt.Printf("\n=== STEP 4: Crawling Episodes in %s ===\n", seasons[0].Name)
		episodes, err := fetchDir(seasons[0].URL)
		if err != nil {
			panic(err)
		}

		for _, ep := range episodes {
			if !ep.IsDir {
				fmt.Printf("   🎬 Episode: %s | Size: %s | Modified: %s\n", ep.Name, ep.Size, ep.LastModified)
			}
		}
	}

	fmt.Println("\n✅ Crawler spike completed with 100% success!")
}
