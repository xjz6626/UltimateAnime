package crawler

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

type Crawler struct {
	client *resty.Client
	ApiUrl string
}

func NewCrawler(apiUrl, proxy string) *Crawler {
	if apiUrl == "" {
		apiUrl = "https://api.animes.garden/resources"
	}
	client := resty.New().
		SetTimeout(15*time.Second).
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	if proxy != "" {
		client.SetProxy(proxy)
	}

	return &Crawler{
		client: client,
		ApiUrl: apiUrl,
	}
}

// --- 数据结构 ---

// SearchResp 包装 API 返回的列表
type SearchResp struct {
	Resources []ResourceItem `json:"resources"`
}

type ResourceItem struct {
	Title     string      `json:"title"`
	Magnet    string      `json:"magnet"`
	Size      interface{} `json:"size"` // number (bytes) or string
	Type      string      `json:"type"`
	Publisher struct {
		Name string      `json:"name"`
		Id   interface{} `json:"id"`
	} `json:"publisher"`
	CreatedAt string `json:"createdAt"` // JSON key is camelCase
}

type TorrentItem struct {
	Title       string `json:"title"`
	Magnet      string `json:"magnet"`
	Size        string `json:"size"`
	PublishDate string `json:"publish_date"`
	Source      string `json:"source"`
}

// --- 辅助函数 ---

// parseSize 智能处理 Size 字段
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
		return val // 如果已经是字符串，直接返回
	default:
		return fmt.Sprintf("%v", val)
	}

	// 转换为易读格式
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%.0f B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", bytes/float64(div), "KMGTPE"[exp])
}

// ParseEpisodeNumber 从标题提取集数
func ParseEpisodeNumber(title string) float64 {
	// 1. [数字] 或 【数字】 (排除年份和分辨率)
	reBracket := regexp.MustCompile(`[\[【](\d{1,3}(?:\.\d{1,2})?)(?:v\d+)?[\]】]`)
	matches := reBracket.FindAllStringSubmatch(title, -1)
	for _, m := range matches {
		if num, err := strconv.ParseFloat(m[1], 64); err == nil {
			// 排除 1080, 720, 480, 2160, 20xx (年份)
			if (num > 0 && num < 1900) && num != 1080 && num != 720 && num != 480 && num != 2160 {
				// 检查是否包含 p/P (如 1080p)
				if !regexp.MustCompile(`(?i)\d+p`).MatchString(m[0]) {
					return num
				}
			}
		}
	}

	// 2. - 数字
	reDash := regexp.MustCompile(`[-\-]\s*(\d{1,3}(?:\.\d)?)(?:\s|[\[【]|$)`)
	if m := reDash.FindStringSubmatch(title); len(m) > 1 {
		if num, err := strconv.ParseFloat(m[1], 64); err == nil {
			return num
		}
	}

	// 3. 第X话
	reChinese := regexp.MustCompile(`第(\d{1,3}(?:\.\d)?)[话話集]`)
	if m := reChinese.FindStringSubmatch(title); len(m) > 1 {
		if num, err := strconv.ParseFloat(m[1], 64); err == nil {
			return num
		}
	}

	// 4. 简单匹配 (作为最后的手段)
	reSimple := regexp.MustCompile(`[\s\.\-_](\d{1,3}(?:\.\d)?)[\s\.\-_\]]`)
	matchesSimple := reSimple.FindAllStringSubmatch(title, -1)
	var candidates []float64
	for _, m := range matchesSimple {
		if num, err := strconv.ParseFloat(m[1], 64); err == nil {
			if num > 0 && num < 1000 && num != 720 && num != 1080 {
				candidates = append(candidates, num)
			}
		}
	}
	if len(candidates) > 0 {
		// 返回最后一个匹配项 (通常集数在标题后部)
		return candidates[len(candidates)-1]
	}

	return -1
}

// --- 方法实现 ---

// SearchResource 搜索资源
func (c *Crawler) SearchResource(keyword string) ([]TorrentItem, error) {
	// 使用 GET 请求
	var respData SearchResp

	// 构造 JSON 数组字符串作为 search 参数
	// API 似乎支持 search=["keyword"] 格式
	searchParam := fmt.Sprintf("[\"%s\"]", keyword)

	_, err := c.client.R().
		SetQueryParams(map[string]string{
			"search":   searchParam,
			"pageSize": "100",
			"page":     "1",
		}).
		SetResult(&respData).
		Get(c.ApiUrl)

	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	var items []TorrentItem
	for _, res := range respData.Resources {
		pubDate := res.CreatedAt
		if t, err := time.Parse(time.RFC3339, res.CreatedAt); err == nil {
			pubDate = t.Format("2006-01-02 15:04")
		}

		items = append(items, TorrentItem{
			Title:       res.Title,
			Magnet:      res.Magnet,
			Size:        parseSize(res.Size),
			PublishDate: pubDate,
			Source:      res.Publisher.Name,
		})
	}

	return items, nil
}

// SearchEpisode 搜索特定集数
func (c *Crawler) SearchEpisode(keywords []string, episodeNum float64) (*TorrentItem, error) {
	// 1. 搜索所有相关资源
	var respData SearchResp

	// 构造 search 参数 (JSON 数组)
	// 优化：添加集数关键字以缩小搜索范围
	searchTerms := []string{}
	if len(keywords) > 0 {
		searchTerms = append(searchTerms, keywords[0])
	}

	// 尝试添加集数作为关键词
	// 策略：如果是整数，尝试添加 "01" 这种格式 (绝大多数番剧都是 01, 02...)
	// "1" 在 1080p, 2023 中太常见，过滤效果差，且 API 可能是模糊匹配。
	if episodeNum == float64(int(episodeNum)) {
		searchTerms = append(searchTerms, fmt.Sprintf("%02d", int(episodeNum)))
	} else {
		searchTerms = append(searchTerms, fmt.Sprintf("%g", episodeNum))
	}

	searchParamBytes, _ := json.Marshal(searchTerms)
	searchParam := string(searchParamBytes)

	fmt.Printf("🔍 [Crawler] Searching: %s, Target Ep: %.1f\n", searchParam, episodeNum)

	_, err := c.client.R().
		SetQueryParams(map[string]string{
			"search":   searchParam,
			"pageSize": "100",
			"page":     "1",
		}).
		SetResult(&respData).
		Get(c.ApiUrl)

	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	fmt.Printf("🔍 [Crawler] Found %d items\n", len(respData.Resources))

	// 2. 筛选特定集数
	var candidates []ResourceItem
	for _, res := range respData.Resources {
		ep := ParseEpisodeNumber(res.Title)
		fmt.Printf("  - Check: [%.1f] %s\n", ep, res.Title)

		// 使用 epsilon 比较浮点数，防止精度问题
		if math.Abs(ep-episodeNum) < 0.1 {
			candidates = append(candidates, res)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("episode %.1f not found", episodeNum)
	}

	// 3. 优选最佳资源 (优先简中 > 繁中 > 其他，排除纯英)
	var bestRes ResourceItem
	bestScore := -10000

	for _, res := range candidates {
		score := 0
		upperTitle := strings.ToUpper(res.Title)

		// 简中权重最高
		if strings.Contains(upperTitle, "CHS") || strings.Contains(upperTitle, "GB") || strings.Contains(upperTitle, "简体") || strings.Contains(upperTitle, "简中") {
			score += 100
		} else if strings.Contains(upperTitle, "CHT") || strings.Contains(upperTitle, "BIG5") || strings.Contains(upperTitle, "繁体") || strings.Contains(upperTitle, "繁中") || strings.Contains(upperTitle, "TC") {
			// 繁中次之
			score += 50
		} else if strings.Contains(upperTitle, "CN") || strings.Contains(upperTitle, "ZH") {
			// 通用中文
			score += 10
		}

		// 英文降权
		if strings.Contains(upperTitle, "ENG") || strings.Contains(upperTitle, "ENGLISH") {
			score -= 10
		}

		// 优先选择 1080P
		if strings.Contains(upperTitle, "1080") {
			score += 5
		}

		fmt.Printf("  - Candidate: %s (Score: %d)\n", res.Title, score)

		if score > bestScore {
			bestScore = score
			bestRes = res
		}
	}

	fmt.Printf("  ✅ Best Match: %s\n", bestRes.Title)
	pubDate := bestRes.CreatedAt
	if t, err := time.Parse(time.RFC3339, bestRes.CreatedAt); err == nil {
		pubDate = t.Format("2006-01-02 15:04")
	}

	return &TorrentItem{
		Title:       bestRes.Title,
		Magnet:      bestRes.Magnet,
		Size:        parseSize(bestRes.Size),
		PublishDate: pubDate,
		Source:      bestRes.Publisher.Name,
	}, nil
}
