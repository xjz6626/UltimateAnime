package bangumi

import (
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

type BangumiClient struct {
	client *resty.Client
	Token  string
}

// NewClient 初始化客户端
// token: 从 config.json 读取的 bangumi_api_token
func NewClient(token, proxy string) *BangumiClient {
	// 复刻 Python 中的 User-Agent 设置
	const userAgent = "xjz6626/bangmi-anime-tracker (https://github.com/xjz6626/bangmi)"

	client := resty.New().
		SetTimeout(10*time.Second).
		SetHeader("User-Agent", userAgent). // 关键：设置 UA 防止被风控
		SetHeader("Accept", "application/json")

	if proxy != "" {
		client.SetProxy(proxy)
	}

	// 如果有 Token，自动注入认证头
	if token != "" {
		client.SetHeader("Authorization", "Bearer "+token)
	}

	return &BangumiClient{
		client: client,
		Token:  token,
	}
}

// SetToken 更新 Token
func (b *BangumiClient) SetToken(token string) {
	b.Token = token
	if token != "" {
		b.client.SetHeader("Authorization", "Bearer "+token)
	} else {
		b.client.Header.Del("Authorization")
	}
}

// --- 数据结构 (对应 API 返回) ---

// CalendarItem 每日放送数据结构
type CalendarItem struct {
	Weekday struct {
		CN string `json:"cn"` // e.g. "星期一"
		ID int    `json:"id"`
	} `json:"weekday"`
	Items []Subject `json:"items"`
}

// Subject 番剧条目详情
type Subject struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	NameCN     string `json:"name_cn"`
	AirDate    string `json:"air_date"`
	AirWeekday int    `json:"air_weekday"`
	Images     struct {
		Large  string `json:"large"`
		Common string `json:"common"`
		Medium string `json:"medium"`
		Small  string `json:"small"`
		Grid   string `json:"grid"`
	} `json:"images"`
	Rating struct {
		Score float64        `json:"score"`
		Total int            `json:"total"`
		Count map[string]int `json:"count"`
	} `json:"rating"`
	Collection struct {
		Doing   int `json:"doing"`   // 在看
		OnHold  int `json:"on_hold"` // 搁置
		Dropped int `json:"dropped"` // 抛弃
		Wish    int `json:"wish"`    // 想看
		Collect int `json:"collect"` // 看过
	} `json:"collection"`
	Rank int `json:"rank"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Results int       `json:"results"`
	List    []Subject `json:"list"`
}

// UserCollection 用户收藏条目
type UserCollection struct {
	SubjectID int     `json:"subject_id"`
	Subject   Subject `json:"subject"`
	Type      int     `json:"type"` // 1: wish, 2: collect, 3: do, 4: on_hold, 5: dropped
	Rate      int     `json:"rate"`
	Comment   string  `json:"comment"`
	UpdatedAt string  `json:"updated_at"`
}

// SubjectDetail 番剧详细信息
type SubjectDetail struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	NameCN        string `json:"name_cn"`
	Summary       string `json:"summary"`
	Date          string `json:"date"`           // 放送开始日期
	TotalEpisodes int    `json:"total_episodes"` // 总集数
	Images        struct {
		Large  string `json:"large"`
		Common string `json:"common"`
		Medium string `json:"medium"`
		Small  string `json:"small"`
		Grid   string `json:"grid"`
	} `json:"images"`
	Rating struct {
		Score float64 `json:"score"`
		Total int     `json:"total"`
	} `json:"rating"`
}

// Episode 剧集信息
type Episode struct {
	ID      int     `json:"id"`
	Sort    float64 `json:"sort"`    // 集数
	Name    string  `json:"name"`    // 标题
	NameCN  string  `json:"name_cn"` // 中文标题
	AirDate string  `json:"airdate"` // 放送日期
	Type    int     `json:"type"`    // 0=本篇, 1=SP, 2=OP, 3=ED
}

// --- API 方法 ---

// GetCalendar 获取每日放送表 (对应 bangumi_api.py 中的功能)
func (b *BangumiClient) GetCalendar() ([]CalendarItem, error) {
	var result []CalendarItem
	// API: GET /calendar
	_, err := b.client.R().
		SetResult(&result).
		Get("https://api.bgm.tv/calendar")

	if err != nil {
		return nil, fmt.Errorf("请求 Bangumi 日历失败: %v", err)
	}
	return result, nil
}

// SearchSubject 搜索番剧
func (b *BangumiClient) SearchSubject(keywords string) ([]Subject, error) {
	var result SearchResult
	// API: GET /search/subject/{keywords}?type=2 (2代表动画)
	_, err := b.client.R().
		SetResult(&result).
		SetQueryParam("type", "2").
		SetQueryParam("responseGroup", "small").
		Get("https://api.bgm.tv/search/subject/" + keywords)

	if err != nil {
		return nil, fmt.Errorf("搜索失败: %v", err)
	}
	return result.List, nil
}

// GetSubject 获取指定番剧详情
func (b *BangumiClient) GetSubject(subjectID int) (*Subject, error) {
	var result Subject
	_, err := b.client.R().
		SetResult(&result).
		Get(fmt.Sprintf("https://api.bgm.tv/v0/subjects/%d", subjectID))

	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetUserCollection 获取用户在看列表
func (b *BangumiClient) GetUserCollection(uid string) ([]UserCollection, error) {
	var result struct {
		Data []UserCollection `json:"data"`
	}
	// API: GET /v0/users/{username}/collections?subject_type=2&type=3&limit=30
	// subject_type=2 (Anime), type=3 (Doing/Watching)
	_, err := b.client.R().
		SetResult(&result).
		SetQueryParam("subject_type", "2").
		SetQueryParam("type", "3").
		SetQueryParam("limit", "50").
		Get(fmt.Sprintf("https://api.bgm.tv/v0/users/%s/collections", uid))

	if err != nil {
		return nil, fmt.Errorf("获取在看列表失败: %v", err)
	}
	return result.Data, nil
}

// UpdateCollectionStatus 更新收藏状态
// status: 1=想看, 2=看过, 3=在看, 4=搁置, 5=抛弃
func (b *BangumiClient) UpdateCollectionStatus(subjectID int, status int) error {
	// API: POST /v0/users/-/collections/{subject_id}
	// Body: {"type": status}
	body := map[string]int{
		"type": status,
	}

	resp, err := b.client.R().
		SetBody(body).
		Post(fmt.Sprintf("https://api.bgm.tv/v0/users/-/collections/%d", subjectID))

	if err != nil {
		return fmt.Errorf("更新收藏状态失败: %v", err)
	}
	if resp.IsError() {
		return fmt.Errorf("API Error: %s", resp.String())
	}
	return nil
}

// GetSubjectDetail 获取番剧详情
func (b *BangumiClient) GetSubjectDetail(subjectID int) (*SubjectDetail, error) {
	var result SubjectDetail
	// API: GET /v0/subjects/{subject_id}
	_, err := b.client.R().
		SetResult(&result).
		Get(fmt.Sprintf("https://api.bgm.tv/v0/subjects/%d", subjectID))

	if err != nil {
		return nil, fmt.Errorf("获取番剧详情失败: %v", err)
	}
	return &result, nil
}

// GetSubjectEpisodes 获取番剧剧集列表
func (b *BangumiClient) GetSubjectEpisodes(subjectID int) ([]Episode, error) {
	var result struct {
		Data  []Episode `json:"data"`
		Total int       `json:"total"`
	}
	// API: GET /v0/episodes?subject_id={id}&type=0 (只获取本篇)
	// 默认 limit=100，对于大多数番剧够用了
	_, err := b.client.R().
		SetResult(&result).
		SetQueryParam("subject_id", fmt.Sprintf("%d", subjectID)).
		SetQueryParam("type", "0"). // 0 = 本篇
		SetQueryParam("limit", "100").
		Get("https://api.bgm.tv/v0/episodes")

	if err != nil {
		return nil, fmt.Errorf("获取剧集列表失败: %v", err)
	}
	return result.Data, nil
}
