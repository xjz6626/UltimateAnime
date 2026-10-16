package bilibili

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Candidate struct {
	Kind         string `json:"kind"`
	ID           string `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Author       string `json:"author"`
	EpisodeCount int    `json:"episode_count"`
	URL          string `json:"url"`
}

type Part struct {
	CID    int64   `json:"cid"`
	Title  string  `json:"title"`
	Number float64 `json:"number"`
}

type SourceDetails struct {
	Candidate
	Parts []Part `json:"parts"`
}

func plainTitle(value string) string {
	return html.UnescapeString(htmlTags.ReplaceAllString(value, ""))
}

func (c *Client) search(ctx context.Context, keyword, searchType string, page int) ([]searchResult, error) {
	if !c.legacySearch {
		if err := c.initSearch(ctx); err != nil {
			c.legacySearch = true
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var data struct {
		Results []searchResult `json:"result"`
		Voucher string         `json:"v_voucher"`
	}
	query := url.Values{"search_type": {searchType}, "keyword": {keyword}, "page": {strconv.Itoa(page)}}
	if !c.legacySearch {
		err := c.api(ctx, "/x/web-interface/wbi/search/type", signWBI(query, c.wbiKey, time.Now().Unix()), &data)
		if err != nil || data.Voucher != "" {
			c.legacySearch = true
		}
	}
	if c.legacySearch {
		query.Del("w_rid")
		query.Del("wts")
		data.Results, data.Voucher = nil, ""
		if err := c.api(ctx, "/x/web-interface/search/type", query, &data); err != nil {
			return nil, err
		}
	}
	if data.Voucher != "" {
		return nil, fmt.Errorf("B 站搜索需要验证，请在设置中扫码登录后重试")
	}
	return data.Results, nil
}

var (
	bvidPattern = regexp.MustCompile(`BV[0-9A-Za-z]{10}`)
	seasonLink  = regexp.MustCompile(`(?:^|/)ss([0-9]+)(?:[/?#]|$)`)
	episodeLink = regexp.MustCompile(`(?:^|/)ep([0-9]+)(?:[/?#]|$)`)
)

func (c *Client) Search(ctx context.Context, keyword, kind string, page int) ([]Candidate, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || len(keyword) > 300 || page < 1 || page > 50 {
		return nil, fmt.Errorf("搜索关键词或页码无效")
	}
	if kind != "season" && kind != "video" {
		return nil, fmt.Errorf("无效的 B 站搜索类型")
	}
	if bvid := bvidPattern.FindString(keyword); bvid != "" {
		details, err := c.Details(ctx, "video", bvid)
		if err != nil {
			return nil, err
		}
		return []Candidate{details.Candidate}, nil
	}
	if match := seasonLink.FindStringSubmatch(keyword); match != nil {
		details, err := c.Details(ctx, "season", match[1])
		if err != nil {
			return nil, err
		}
		return []Candidate{details.Candidate}, nil
	}
	if match := episodeLink.FindStringSubmatch(keyword); match != nil {
		var season Season
		if err := c.api(ctx, "/pgc/view/web/season", url.Values{"ep_id": {match[1]}}, &season); err != nil {
			return nil, err
		}
		return []Candidate{seasonCandidate(season)}, nil
	}
	searchType := "media_bangumi"
	if kind == "video" {
		searchType = "video"
	}
	results, err := c.search(ctx, keyword, searchType, page)
	if err != nil {
		return nil, err
	}
	candidates := make([]Candidate, 0, len(results))
	for _, result := range results {
		candidate := Candidate{Kind: kind, Title: plainTitle(result.Title), Description: plainTitle(result.Description), Author: result.Author, EpisodeCount: result.EpisodeCount}
		if kind == "season" {
			if result.ID <= 0 {
				continue
			}
			candidate.ID = strconv.Itoa(result.ID)
			candidate.URL = "https://www.bilibili.com/bangumi/play/ss" + candidate.ID
		} else {
			if !bvidPattern.MatchString(result.BVID) {
				continue
			}
			candidate.ID = result.BVID
			candidate.URL = "https://www.bilibili.com/video/" + result.BVID
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func seasonCandidate(season Season) Candidate {
	id := strconv.Itoa(season.ID)
	return Candidate{Kind: "season", ID: id, Title: plainTitle(season.Title), EpisodeCount: len(season.Episodes), URL: "https://www.bilibili.com/bangumi/play/ss" + id}
}

func (c *Client) Details(ctx context.Context, kind, id string) (SourceDetails, error) {
	if kind == "season" {
		number, err := strconv.Atoi(id)
		if err != nil || number <= 0 {
			return SourceDetails{}, fmt.Errorf("番剧 ID 无效")
		}
		season, err := c.Season(ctx, number)
		if err != nil {
			return SourceDetails{}, err
		}
		details := SourceDetails{Candidate: seasonCandidate(season), Parts: []Part{}}
		for _, episode := range season.Episodes {
			if episode.CID <= 0 {
				continue
			}
			number := 0.0
			if match := episodeNumber.FindStringSubmatch(strings.TrimSpace(episode.Title)); match != nil {
				number, _ = strconv.ParseFloat(match[1], 64)
			}
			title := "第 " + episode.Title + " 集"
			if episode.LongTitle != "" {
				title += " · " + episode.LongTitle
			}
			details.Parts = append(details.Parts, Part{CID: episode.CID, Title: title, Number: number})
		}
		return details, nil
	}
	if kind != "video" || bvidPattern.FindString(id) != id || id == "" {
		return SourceDetails{}, fmt.Errorf("视频 BV 号无效")
	}
	var video struct {
		BVID        string `json:"bvid"`
		Title       string `json:"title"`
		Description string `json:"desc"`
		Pages       []struct {
			CID   int64  `json:"cid"`
			Page  int    `json:"page"`
			Title string `json:"part"`
		} `json:"pages"`
	}
	if err := c.api(ctx, "/x/web-interface/view", url.Values{"bvid": {id}}, &video); err != nil {
		return SourceDetails{}, err
	}
	if video.BVID != id || len(video.Pages) == 0 {
		return SourceDetails{}, fmt.Errorf("B 站未返回视频分 P 信息")
	}
	details := SourceDetails{Candidate: Candidate{Kind: "video", ID: id, Title: plainTitle(video.Title), Description: video.Description, EpisodeCount: len(video.Pages), URL: "https://www.bilibili.com/video/" + id}, Parts: []Part{}}
	for _, page := range video.Pages {
		if page.CID > 0 {
			details.Parts = append(details.Parts, Part{CID: page.CID, Title: fmt.Sprintf("P%d · %s", page.Page, page.Title), Number: float64(page.Page)})
		}
	}
	return details, nil
}
