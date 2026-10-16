// Package bilibili retrieves public bangumi metadata and the current danmaku pool.
package bilibili

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const maxResponseSize = 16 << 20

type Client struct {
	http         *http.Client
	baseURL      string
	wbiKey       string
	legacySearch bool
	passportURL  string
}

type Season struct {
	ID       int       `json:"season_id"`
	Title    string    `json:"title"`
	Episodes []Episode `json:"episodes"`
}

type Episode struct {
	ID        int    `json:"id"`
	CID       int64  `json:"cid"`
	Title     string `json:"title"`
	LongTitle string `json:"long_title"`
}

type searchResult struct {
	ID           int    `json:"season_id"`
	Title        string `json:"title"`
	Original     string `json:"org_title"`
	PubTime      int64  `json:"pubtime"`
	BVID         string `json:"bvid"`
	Description  string `json:"description"`
	Author       string `json:"author"`
	EpisodeCount int    `json:"ep_size"`
}

func NewClient(proxy, cookie string) (*Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		parsed, err := url.Parse(proxy)
		if err != nil || parsed.Host == "" {
			return nil, fmt.Errorf("B 站代理地址无效")
		}
		transport.Proxy = http.ProxyURL(parsed)
	}
	jar, _ := cookiejar.New(nil)
	base, _ := url.Parse("https://api.bilibili.com")
	request := &http.Request{Header: http.Header{"Cookie": []string{cookie}}}
	jar.SetCookies(base, request.Cookies())
	return &Client{
		http:        &http.Client{Transport: transport, Jar: jar, Timeout: 12 * time.Second},
		baseURL:     base.String(),
		passportURL: "https://passport.bilibili.com",
	}, nil
}

// SearchSeason only accepts matching full titles/original titles. In particular,
// a search hit for another season must never supply this episode's comments.
func (c *Client) SearchSeason(ctx context.Context, names []string, year int) (int, error) {
	var candidates []searchResult
	seen := make(map[string]bool)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		results, err := c.search(ctx, name, "media_bangumi", 1)
		if err != nil {
			return 0, err
		}
		candidates = append(candidates, results...)
	}
	return chooseSeason(candidates, names, year)
}

func (c *Client) Season(ctx context.Context, id int) (Season, error) {
	var result Season
	err := c.api(ctx, "/pgc/view/web/season", url.Values{"season_id": {strconv.Itoa(id)}}, &result)
	if err == nil && (result.ID != id || len(result.Episodes) == 0) {
		err = fmt.Errorf("B 站番剧 %d 未返回正片剧集", id)
	}
	return result, err
}

func (c *Client) Comments(ctx context.Context, cid int64) ([]Comment, error) {
	if cid <= 0 {
		return nil, fmt.Errorf("弹幕 cid 无效")
	}
	body, err := c.get(ctx, "/x/v1/dm/list.so", url.Values{"oid": {strconv.FormatInt(cid, 10)}})
	if err != nil {
		return nil, err
	}
	return ParseXML(body)
}

func (c *Client) initSearch(ctx context.Context) error {
	if c.wbiKey != "" {
		return nil
	}
	base, _ := url.Parse(c.baseURL)
	hasBuvid := false
	for _, cookie := range c.http.Jar.Cookies(base) {
		hasBuvid = hasBuvid || cookie.Name == "buvid3"
	}
	if !hasBuvid {
		var device struct {
			Buvid string `json:"b_3"`
		}
		if err := c.api(ctx, "/x/frontend/finger/spi", nil, &device); err != nil {
			return err
		}
		if device.Buvid == "" {
			return fmt.Errorf("B 站未返回搜索所需的设备 Cookie")
		}
		c.http.Jar.SetCookies(base, []*http.Cookie{{Name: "buvid3", Value: device.Buvid, Path: "/"}})
	}
	var nav struct {
		Images struct {
			Image string `json:"img_url"`
			Sub   string `json:"sub_url"`
		} `json:"wbi_img"`
	}
	if err := c.api(ctx, "/x/web-interface/nav", nil, &nav); err != nil {
		return err
	}
	keyPart := func(address string) string {
		u, _ := url.Parse(address)
		if u == nil {
			return ""
		}
		return strings.TrimSuffix(path.Base(u.Path), path.Ext(u.Path))
	}
	key := keyPart(nav.Images.Image) + keyPart(nav.Images.Sub)
	if len(key) != 64 {
		return fmt.Errorf("B 站未返回有效的 WBI 搜索密钥")
	}
	order := [...]int{46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13}
	mixed := make([]byte, len(order))
	for i, index := range order {
		mixed[i] = key[index]
	}
	c.wbiKey = string(mixed)
	return nil
}

func signWBI(query url.Values, key string, timestamp int64) url.Values {
	query.Set("wts", strconv.FormatInt(timestamp, 10))
	for name, values := range query {
		for i, value := range values {
			values[i] = strings.Map(func(r rune) rune {
				if strings.ContainsRune("!'()*", r) {
					return -1
				}
				return r
			}, value)
		}
		query[name] = values
	}
	encoded := strings.ReplaceAll(query.Encode(), "+", "%20")
	sum := md5.Sum([]byte(encoded + key))
	query.Set("w_rid", hex.EncodeToString(sum[:]))
	return query
}

func (c *Client) api(ctx context.Context, endpoint string, query url.Values, target any) error {
	body, err := c.get(ctx, endpoint, query)
	if err != nil {
		return err
	}
	var envelope struct {
		Code   int             `json:"code"`
		Data   json.RawMessage `json:"data"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("B 站 %s 响应格式无效", endpoint)
	}
	// Anonymous nav responses still include valid public WBI keys.
	if envelope.Code != 0 && !(endpoint == "/x/web-interface/nav" && envelope.Code == -101) {
		return fmt.Errorf("B 站 %s 返回错误码 %d", endpoint, envelope.Code)
	}
	payload := envelope.Data
	if len(envelope.Result) > 0 && string(envelope.Result) != "null" {
		payload = envelope.Result
	}
	if len(payload) == 0 || string(payload) == "null" {
		return fmt.Errorf("B 站 %s 未返回数据", endpoint)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("B 站 %s 数据格式无效", endpoint)
	}
	return nil
}

func (c *Client) get(ctx context.Context, endpoint string, query url.Values) ([]byte, error) {
	base := c.baseURL
	if strings.HasPrefix(endpoint, "/x/passport-login/") {
		base = c.passportURL
	}
	address := base + endpoint
	if len(query) > 0 {
		address += "?" + strings.ReplaceAll(query.Encode(), "+", "%20")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	resp, err := c.http.Do(req)
	if err != nil {
		// Do not include the request URL or user-supplied credentials in logs.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("B 站 %s 网络请求失败", endpoint)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("B 站 %s HTTP %d", endpoint, resp.StatusCode)
	}
	body, err := readLimited(resp.Body)
	if err != nil {
		return nil, err
	}
	var reader io.ReadCloser
	switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
	case "gzip":
		reader, err = gzip.NewReader(bytes.NewReader(body))
	case "deflate":
		reader, err = zlib.NewReader(bytes.NewReader(body))
		if err != nil {
			reader, err = flate.NewReader(bytes.NewReader(body)), nil
		}
	default:
		// The legacy XML endpoint can return raw DEFLATE without a header.
		trimmed := bytes.TrimSpace(body)
		if endpoint == "/x/v1/dm/list.so" && len(trimmed) > 0 && trimmed[0] != '<' && trimmed[0] != '{' {
			reader, err = zlib.NewReader(bytes.NewReader(body))
			if err != nil {
				reader, err = flate.NewReader(bytes.NewReader(body)), nil
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("B 站响应解压失败")
	}
	if reader != nil {
		defer reader.Close()
		body, err = readLimited(reader)
		if err != nil {
			return nil, fmt.Errorf("B 站响应解压失败: %w", err)
		}
	}
	return body, nil
}

func readLimited(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxResponseSize+1))
	if err == nil && len(body) > maxResponseSize {
		err = fmt.Errorf("B 站响应超过大小限制")
	}
	return body, err
}
