package crawler

import (
	"encoding/base32"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

type Crawler struct {
	client      *resty.Client
	mikanClient *resty.Client
	mikanURL    string
	ApiUrl      string
	mu          sync.RWMutex
}

const mikanSearchURL = "https://mikanani.me/RSS/Search"

func NewCrawler(apiUrl, proxy string) *Crawler {
	if apiUrl == "" {
		apiUrl = "https://api.animes.garden/resources"
	}
	client := resty.New().SetTimeout(15*time.Second).
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	if proxy != "" {
		client.SetProxy(proxy)
	}
	mikanClient := resty.New().SetTimeout(15*time.Second).
		SetHeader("User-Agent", "UltimateAnime/0.1")
	if proxy != "" {
		mikanClient.SetProxy(proxy)
	}
	return &Crawler{client: client, mikanClient: mikanClient, mikanURL: mikanSearchURL, ApiUrl: apiUrl}
}

func (c *Crawler) SetAPIURL(apiURL string) {
	if strings.TrimSpace(apiURL) == "" {
		apiURL = "https://api.animes.garden/resources"
	}
	c.mu.Lock()
	c.ApiUrl = apiURL
	c.mu.Unlock()
}

type SearchResp struct {
	Resources  []ResourceItem `json:"resources"`
	Pagination struct {
		Complete bool `json:"complete"`
	} `json:"pagination"`
}

type ResourceItem struct {
	Title     string      `json:"title"`
	Magnet    string      `json:"magnet"`
	Size      interface{} `json:"size"`
	Type      string      `json:"type"`
	Publisher struct {
		Name string      `json:"name"`
		Id   interface{} `json:"id"`
	} `json:"publisher"`
	CreatedAt string `json:"createdAt"`
}

type mikanFeed struct {
	XMLName xml.Name `xml:"rss"`
	Items   []struct {
		Title     string `xml:"title"`
		Enclosure struct {
			URL string `xml:"url,attr"`
		} `xml:"enclosure"`
		Torrent struct {
			ContentLength string `xml:"contentLength"`
			PubDate       string `xml:"pubDate"`
		} `xml:"torrent"`
	} `xml:"channel>item"`
}

type TorrentItem struct {
	Title       string `json:"title"`
	Magnet      string `json:"magnet"`
	Size        string `json:"size"`
	PublishDate string `json:"publish_date"`
	Source      string `json:"source"`
}

func parseSize(v interface{}) string {
	if v == nil {
		return "0 B"
	}
	var bytes float64
	switch val := v.(type) {
	case float64:
		bytes = val
	case int:
		bytes = float64(val)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			return val
		}
		bytes = parsed
	default:
		return fmt.Sprintf("%v", val)
	}
	if bytes < 1024 {
		return fmt.Sprintf("%.0f B", bytes)
	}
	div, exp := float64(1024), 0
	for bytes/div >= 1024 && exp < 5 {
		div *= 1024
		exp++
	}
	return fmt.Sprintf("%.1f %cB", bytes/div, "KMGTPE"[exp])
}

var (
	episodeRange    = regexp.MustCompile(`(?i)(?:\[|【|\b)\d{1,3}\s*[-~～]\s*\d{1,3}(?:\]|】|\b)`)
	episodePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bS\d{1,2}E(\d{1,3}(?:\.\d)?)\b`),
		regexp.MustCompile(`第\s*(\d{1,3}(?:\.\d)?)\s*[话話集]`),
		regexp.MustCompile(`(?i)[\[【](\d{1,3}(?:\.\d)?)(?:v\d+)?[\]】]`),
		regexp.MustCompile(`(?:^|\s)-\s*(\d{1,3}(?:\.\d)?)(?:\s|[\[【]|$)`),
		regexp.MustCompile(`(?:^|[\s._-])(\d{1,3}(?:\.\d)?)(?:[\s._\-\]】]|$)`),
	}
	seasonPatterns = []*regexp.Regexp{
		regexp.MustCompile(`第\s*([一二三四五六七八九十\d]{1,3})\s*[季期]`),
		regexp.MustCompile(`(?i)\bseason\s*0?(\d{1,2})\b`),
		regexp.MustCompile(`(?i)\b0?(\d{1,2})(?:st|nd|rd|th)\s+season\b`),
		regexp.MustCompile(`(?i)(?:^|[\s/\[【(])S0?(\d{1,2})(?:\b|E\d)`),
	}
	infoHashPattern = regexp.MustCompile(`(?i)^[0-9a-f]{40}$`)
)

// ParseEpisodeNumber rejects multi-episode packs so they cannot be selected automatically.
func ParseEpisodeNumber(title string) float64 {
	if episodeRange.MatchString(title) || strings.Contains(title, "合集") || strings.Contains(strings.ToLower(title), "batch") {
		return -1
	}
	for _, pattern := range episodePatterns {
		for _, match := range pattern.FindAllStringSubmatch(title, -1) {
			number, err := strconv.ParseFloat(match[1], 64)
			if err == nil && number > 0 && number <= 200 {
				return number
			}
		}
	}
	return -1
}

func seasonNumber(title string) (int, bool) {
	for _, pattern := range seasonPatterns {
		match := pattern.FindStringSubmatch(title)
		if len(match) < 2 {
			continue
		}
		if number, err := strconv.Atoi(match[1]); err == nil && number > 0 {
			return number, true
		}
		chinese := map[rune]int{'一': 1, '二': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
		runes := []rune(match[1])
		if len(runes) == 1 && runes[0] == '十' {
			return 10, true
		}
		if len(runes) == 1 && chinese[runes[0]] > 0 {
			return chinese[runes[0]], true
		}
		if len(runes) == 2 && runes[0] == '十' && chinese[runes[1]] > 0 {
			return 10 + chinese[runes[1]], true
		}
	}
	return 0, false
}

func (c *Crawler) fetchResources(terms []string) ([]ResourceItem, error) {
	if len(terms) == 0 || strings.TrimSpace(terms[0]) == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}
	// AnimeGarden reads each search term from its own search= parameter.
	params := url.Values{"search": terms, "pageSize": {"100"}}
	c.mu.RLock()
	apiURL := c.ApiUrl
	c.mu.RUnlock()
	var all []ResourceItem
	for page := 1; page <= 3; page++ {
		params.Set("page", strconv.Itoa(page))
		var result SearchResp
		response, err := c.client.R().SetQueryParamsFromValues(params).SetResult(&result).Get(apiURL)
		if err != nil {
			if len(all) > 0 {
				return all, nil
			}
			return nil, fmt.Errorf("搜索请求失败: %w", err)
		}
		if response.IsError() {
			if len(all) > 0 {
				return all, nil
			}
			return nil, fmt.Errorf("搜索接口返回 HTTP %d", response.StatusCode())
		}
		all = append(all, result.Resources...)
		if result.Pagination.Complete || len(result.Resources) < 100 {
			break
		}
	}
	return all, nil
}

func mikanInfoHash(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ""
	}
	filename := path.Base(parsed.Path)
	if !strings.HasSuffix(strings.ToLower(filename), ".torrent") {
		return ""
	}
	hash := filename[:len(filename)-len(".torrent")]
	if !infoHashPattern.MatchString(hash) {
		return ""
	}
	return strings.ToLower(hash)
}

func (c *Crawler) fetchMikanResources(keyword string) ([]ResourceItem, error) {
	if c.mikanURL == "" {
		return nil, nil
	}
	response, err := c.mikanClient.R().SetQueryParam("searchstr", keyword).Get(c.mikanURL)
	if err != nil {
		return nil, fmt.Errorf("蜜柑计划搜索请求失败: %w", err)
	}
	if response.IsError() {
		return nil, fmt.Errorf("蜜柑计划搜索接口返回 HTTP %d", response.StatusCode())
	}
	if len(response.Body()) > 2<<20 {
		return nil, fmt.Errorf("蜜柑计划 RSS 响应过大")
	}
	var feed mikanFeed
	if err := xml.Unmarshal(response.Body(), &feed); err != nil {
		return nil, fmt.Errorf("解析蜜柑计划 RSS 失败: %w", err)
	}
	items := make([]ResourceItem, 0, len(feed.Items))
	for _, item := range feed.Items {
		hash := mikanInfoHash(item.Enclosure.URL)
		if hash == "" || item.Title == "" {
			continue
		}
		resource := ResourceItem{
			Title:     item.Title,
			Magnet:    "magnet:?xt=urn:btih:" + hash,
			Size:      item.Torrent.ContentLength,
			CreatedAt: item.Torrent.PubDate,
		}
		resource.Publisher.Name = "蜜柑计划"
		items = append(items, resource)
	}
	return items, nil
}

func (c *Crawler) fetchAll(animeTerms []string, mikanQuery string) ([]ResourceItem, error, error) {
	var animeResources, mikanResources []ResourceItem
	var animeErr, mikanErr error
	var wg sync.WaitGroup
	searchMikan := c.mikanURL != "" && mikanQuery != ""
	wg.Add(1)
	go func() {
		defer wg.Done()
		animeResources, animeErr = c.fetchResources(animeTerms)
	}()
	if searchMikan {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mikanResources, mikanErr = c.fetchMikanResources(mikanQuery)
		}()
	}
	wg.Wait()
	resources := append(animeResources, mikanResources...)
	if animeErr != nil {
		animeErr = fmt.Errorf("AnimeGarden: %w", animeErr)
	}
	// One reachable source is enough to make the search usable, even if it is empty.
	if animeErr == nil || (searchMikan && mikanErr == nil) {
		return resources, nil, mikanErr
	}
	return resources, errors.Join(animeErr, mikanErr), mikanErr
}

func magnetKey(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "magnet" {
		return raw
	}
	for _, xt := range parsed.Query()["xt"] {
		if len(xt) < len("urn:btih:") || !strings.EqualFold(xt[:len("urn:btih:")], "urn:btih:") {
			continue
		}
		hash := xt[len("urn:btih:"):]
		if infoHashPattern.MatchString(hash) {
			return strings.ToLower(hash)
		}
		if len(hash) == 32 {
			decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(hash))
			if err == nil && len(decoded) == 20 {
				return hex.EncodeToString(decoded)
			}
		}
	}
	return raw
}

func toTorrentItem(resource ResourceItem) TorrentItem {
	date := resource.CreatedAt
	if parsed, err := time.Parse(time.RFC3339, date); err == nil {
		date = parsed.Format("2006-01-02 15:04")
	} else if parsed, err := time.Parse("2006-01-02T15:04:05", date); err == nil {
		date = parsed.Format("2006-01-02 15:04")
	}
	return TorrentItem{
		Title: resource.Title, Magnet: resource.Magnet,
		Size: parseSize(resource.Size), PublishDate: date, Source: resource.Publisher.Name,
	}
}

func (c *Crawler) SearchResource(keyword string) ([]TorrentItem, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}
	resources, err, _ := c.fetchAll([]string{keyword}, keyword)
	if err != nil && len(resources) == 0 {
		return nil, err
	}
	items := make([]TorrentItem, 0, len(resources))
	seen := make(map[string]bool)
	for _, resource := range resources {
		key := magnetKey(resource.Magnet)
		if resource.Magnet == "" || seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, toTorrentItem(resource))
	}
	return items, nil
}

func episodeSearchTerm(episode float64) string {
	if episode == math.Trunc(episode) {
		return fmt.Sprintf("%02d", int(episode))
	}
	return strconv.FormatFloat(episode, 'f', -1, 64)
}

func scoreTitle(title string) int {
	upper := strings.ToUpper(title)
	score := 0
	if strings.Contains(upper, "CHS") || strings.Contains(upper, "简体") || strings.Contains(upper, "简中") || strings.Contains(upper, "GB") {
		score += 100
	} else if strings.Contains(upper, "CHT") || strings.Contains(upper, "繁体") || strings.Contains(upper, "繁中") || strings.Contains(upper, "BIG5") {
		score += 50
	}
	if strings.Contains(upper, "1080P") {
		score += 5
	}
	if strings.Contains(upper, "ENG") || strings.Contains(upper, "ENGLISH") {
		score -= 10
	}
	return score
}

func matchingEpisodes(resources []ResourceItem, episode float64, name string) []TorrentItem {
	items := make([]TorrentItem, 0)
	seen := make(map[string]bool)
	wantSeason, explicitSeason := seasonNumber(name)
	if !explicitSeason {
		wantSeason = 1
	}
	for _, resource := range resources {
		key := magnetKey(resource.Magnet)
		if resource.Magnet == "" || seen[key] || math.Abs(ParseEpisodeNumber(resource.Title)-episode) >= 0.01 {
			continue
		}
		season, hasSeason := seasonNumber(resource.Title)
		if (hasSeason && season != wantSeason) || (explicitSeason && !hasSeason) {
			continue
		}
		seen[key] = true
		items = append(items, toTorrentItem(resource))
	}
	sort.SliceStable(items, func(i, j int) bool { return scoreTitle(items[i].Title) > scoreTitle(items[j].Title) })
	return items
}

func (c *Crawler) SearchEpisodeList(keywords []string, episode float64) ([]TorrentItem, error) {
	if len(keywords) == 0 || episode <= 0 {
		return nil, fmt.Errorf("缺少番剧名称或集数")
	}
	name := strings.TrimSpace(keywords[0])
	if name == "" {
		return nil, fmt.Errorf("缺少番剧名称或集数")
	}
	resources, firstErr, mikanErr := c.fetchAll([]string{name, episodeSearchTerm(episode)}, name+" "+episodeSearchTerm(episode))
	items := matchingEpisodes(resources, episode, name)
	if len(items) > 0 {
		return items, nil
	}
	// Publishers may use 1 instead of 01; retry by title and filter locally.
	fallbackMikanQuery := name
	if mikanErr != nil {
		// A failed source should not add another full timeout to the title fallback.
		fallbackMikanQuery = ""
	}
	resources, fallbackErr, _ := c.fetchAll([]string{name}, fallbackMikanQuery)
	items = matchingEpisodes(resources, episode, name)
	if len(items) > 0 {
		return items, nil
	}
	if firstErr == nil || fallbackErr == nil {
		return nil, nil
	}
	return nil, errors.Join(firstErr, fallbackErr)
}

func (c *Crawler) SearchEpisode(keywords []string, episode float64) (*TorrentItem, error) {
	items, err := c.SearchEpisodeList(keywords, episode)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("未找到第 %g 集的磁力链接", episode)
	}
	return &items[0], nil
}
