package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"UltimateAnime/pkg/bangumi"
)

const (
	airingWeekTTL    = 7 * 24 * time.Hour
	airingRetryDelay = 5 * time.Minute
	airingMappingTTL = 30 * 24 * time.Hour
	airingMappingURL = "https://raw.githubusercontent.com/bangumi-data/bangumi-data/master/dist/data.json"
	airingMappingCDN = "https://cdn.jsdelivr.net/npm/bangumi-data@0.3/dist/data.json"
	airingGraphQLURL = "https://graphql.anilist.co"
	airingUserAgent  = "UltimateAnime/0.1 (https://github.com/xjz6626/UltimateAnime)"
)

// The schedule cache is separate from followed.json so download and watch data
// are never rewritten by a background metadata lookup.
type followedAiring struct {
	AniListID     int    `json:"anilist_id,omitempty"`
	ReleaseStatus string `json:"release_status,omitempty"`
	NextEpisode   int    `json:"next_episode,omitempty"`
	NextAiringAt  int64  `json:"next_airing_at,omitempty"`
	TodayEpisode  int    `json:"today_episode,omitempty"`
	TodayAiringAt int64  `json:"today_airing_at,omitempty"`
}

type followedAiringCache struct {
	WeeklyCheckedAt time.Time              `json:"weekly_checked_at"`
	WeeklyAttemptAt time.Time              `json:"weekly_attempt_at,omitempty"`
	DailyDate       string                 `json:"daily_date,omitempty"`
	DailyAttemptAt  time.Time              `json:"daily_attempt_at,omitempty"`
	Entries         map[int]followedAiring `json:"entries"`
}

type airingMappingCache struct {
	FetchedAt time.Time   `json:"fetched_at"`
	AttemptAt time.Time   `json:"attempt_at,omitempty"`
	Entries   map[int]int `json:"entries"`
}

type airingSource struct {
	client     *http.Client
	mappingURL string
	graphqlURL string
}

func newAiringSource(proxy string) airingSource {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		if parsed, err := url.Parse(proxy); err == nil && parsed.Scheme != "" {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}
	return airingSource{
		client:     &http.Client{Transport: transport, Timeout: 20 * time.Second},
		mappingURL: airingMappingURL,
		graphqlURL: airingGraphQLURL,
	}
}

func (s airingSource) allMappings(ctx context.Context) (map[int]int, error) {
	urls := []string{s.mappingURL}
	if s.mappingURL == airingMappingURL {
		urls = append(urls, airingMappingCDN)
	}
	var lastErr error
	for _, address := range urls {
		mapped, err := s.fetchMappings(ctx, address)
		if err == nil {
			return mapped, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, lastErr
}

func (s airingSource) fetchMappings(ctx context.Context, address string) (map[int]int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", airingUserAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bangumi-data HTTP %d", resp.StatusCode)
	}
	var dataset struct {
		Items []struct {
			Sites []struct {
				Site string `json:"site"`
				ID   string `json:"id"`
			} `json:"sites"`
		} `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 24<<20)).Decode(&dataset); err != nil {
		return nil, fmt.Errorf("解析 bangumi-data 失败: %w", err)
	}
	if len(dataset.Items) == 0 {
		return nil, fmt.Errorf("bangumi-data 返回空目录")
	}
	mapped := make(map[int]int)
	for _, item := range dataset.Items {
		var bangumiID, aniListID int
		for _, site := range item.Sites {
			switch site.Site {
			case "bangumi":
				bangumiID, _ = strconv.Atoi(site.ID)
			case "aniList":
				aniListID, _ = strconv.Atoi(site.ID)
			}
		}
		if bangumiID > 0 && aniListID > 0 {
			mapped[bangumiID] = aniListID
		}
	}
	if len(mapped) == 0 {
		return nil, fmt.Errorf("bangumi-data 未提供 Bangumi 与 AniList 映射")
	}
	return mapped, nil
}

func (s airingSource) mappings(ctx context.Context, follows []FollowedItem) (map[int]int, error) {
	all, err := s.allMappings(ctx)
	if err != nil {
		return nil, err
	}
	return selectAiringMappings(all, follows), nil
}

func selectAiringMappings(all map[int]int, follows []FollowedItem) map[int]int {
	mapped := make(map[int]int, len(follows))
	for _, item := range follows {
		if id := all[item.SubjectID]; id > 0 {
			mapped[item.SubjectID] = id
		}
	}
	return mapped
}

func (s airingSource) query(ctx context.Context, query string, variables any, target any) error {
	body, err := json.Marshal(struct {
		Query     string `json:"query"`
		Variables any    `json:"variables"`
	}{query, variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.graphqlURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", airingUserAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("AniList HTTP %d", resp.StatusCode)
	}
	var result struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&result); err != nil {
		return fmt.Errorf("解析 AniList 响应失败: %w", err)
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("AniList: %s", result.Errors[0].Message)
	}
	if len(result.Data) == 0 || string(result.Data) == "null" {
		return fmt.Errorf("AniList 返回空数据")
	}
	return json.Unmarshal(result.Data, target)
}

func (s airingSource) nextEpisodes(ctx context.Context, ids []int) (map[int]followedAiring, error) {
	const query = `query($ids:[Int]) { Page(page:1,perPage:50) { media(id_in:$ids,type:ANIME) { id status nextAiringEpisode { episode airingAt } } } }`
	result := make(map[int]followedAiring)
	for start := 0; start < len(ids); start += 50 {
		end := start + 50
		if end > len(ids) {
			end = len(ids)
		}
		var response struct {
			Page struct {
				Media []struct {
					ID     int    `json:"id"`
					Status string `json:"status"`
					Next   *struct {
						Episode  int   `json:"episode"`
						AiringAt int64 `json:"airingAt"`
					} `json:"nextAiringEpisode"`
				} `json:"media"`
			} `json:"Page"`
		}
		if err := s.query(ctx, query, map[string]any{"ids": ids[start:end]}, &response); err != nil {
			return nil, err
		}
		if len(response.Page.Media) == 0 {
			return nil, fmt.Errorf("AniList 未返回请求的番剧")
		}
		for _, media := range response.Page.Media {
			entry := followedAiring{AniListID: media.ID, ReleaseStatus: media.Status}
			if media.Next != nil {
				entry.NextEpisode = media.Next.Episode
				entry.NextAiringAt = media.Next.AiringAt
			}
			result[media.ID] = entry
		}
	}
	return result, nil
}

func (s airingSource) todayEpisodes(ctx context.Context, ids []int, now time.Time) (map[int]followedAiring, error) {
	const query = `query($ids:[Int],$start:Int,$end:Int,$page:Int) { Page(page:$page,perPage:50) { airingSchedules(mediaId_in:$ids,airingAt_greater:$start,airingAt_lesser:$end,sort:TIME) { mediaId episode airingAt } } }`
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	result := make(map[int]followedAiring)
	for page := 1; page <= 20; page++ {
		var response struct {
			Page struct {
				Schedules []struct {
					MediaID  int   `json:"mediaId"`
					Episode  int   `json:"episode"`
					AiringAt int64 `json:"airingAt"`
				} `json:"airingSchedules"`
			} `json:"Page"`
		}
		variables := map[string]any{"ids": ids, "start": start.Unix() - 1, "end": end.Unix(), "page": page}
		if err := s.query(ctx, query, variables, &response); err != nil {
			return nil, err
		}
		for _, schedule := range response.Page.Schedules {
			result[schedule.MediaID] = followedAiring{TodayEpisode: schedule.Episode, TodayAiringAt: schedule.AiringAt}
		}
		if len(response.Page.Schedules) < 50 {
			return result, nil
		}
	}
	return nil, fmt.Errorf("AniList 当日排期超过分页上限")
}

func sameLocalDay(a, b time.Time) bool {
	if a.IsZero() {
		return false
	}
	a = a.In(b.Location())
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func (a *App) cachedAiringMappings(ctx context.Context, source airingSource, follows []FollowedItem, now time.Time) (map[int]int, error) {
	path := filepath.Join(a.getCacheDir(), "airing_mappings.json")
	var cache airingMappingCache
	loaded, err := a.loadJSONCache(path, &cache)
	if err != nil {
		a.Log("WARN", fmt.Sprintf("读取番剧 ID 对照缓存失败: %v", err))
	}
	refresh := !loaded || len(cache.Entries) == 0 || now.Sub(cache.FetchedAt) >= airingMappingTTL
	if !refresh && now.Sub(cache.FetchedAt) >= 24*time.Hour {
		for _, follow := range follows {
			if _, ok := cache.Entries[follow.SubjectID]; !ok {
				refresh = true
				break
			}
		}
	}
	if refresh && (cache.AttemptAt.IsZero() || now.Sub(cache.AttemptAt) >= airingRetryDelay) {
		cache.AttemptAt = now
		all, lookupErr := source.allMappings(ctx)
		if lookupErr == nil {
			cache.Entries = all
			cache.FetchedAt = now
			cache.AttemptAt = time.Time{}
		}
		if saveErr := a.saveJSONCache(path, cache); saveErr != nil {
			a.Log("WARN", fmt.Sprintf("保存番剧 ID 对照缓存失败: %v", saveErr))
		}
		if lookupErr != nil {
			if len(cache.Entries) == 0 {
				return nil, lookupErr
			}
			a.Log("WARN", fmt.Sprintf("更新番剧 ID 对照失败，使用本地缓存: %v", lookupErr))
		}
	}
	if len(cache.Entries) == 0 {
		return nil, fmt.Errorf("番剧 ID 对照缓存为空，等待重试")
	}
	return selectAiringMappings(cache.Entries, follows), nil
}

func (a *App) followedAiringSnapshot() followedAiringCache {
	a.airingMu.Lock()
	defer a.airingMu.Unlock()
	path := filepath.Join(a.getCacheDir(), "followed_airings.json")
	var cache followedAiringCache
	loaded, err := a.loadJSONCache(path, &cache)
	if err != nil {
		a.Log("WARN", fmt.Sprintf("读取追番排期缓存失败: %v", err))
	}
	if !loaded || cache.Entries == nil {
		cache.Entries = make(map[int]followedAiring)
	}
	follows := a.GetLocalFollows()
	now := time.Now()
	if len(follows) == 0 {
		if len(cache.Entries) != 0 || cache.WeeklyCheckedAt.IsZero() {
			cache.Entries = make(map[int]followedAiring)
			cache.WeeklyCheckedAt = now
			cache.DailyDate = now.Format("2006-01-02")
			_ = a.saveJSONCache(path, cache)
		}
		return cache
	}
	source := newAiringSource(a.configMgr.Snapshot().GlobalSettings.Proxy)
	changed := false
	weeklyDue := cache.WeeklyCheckedAt.IsZero() || now.Sub(cache.WeeklyCheckedAt) >= airingWeekTTL
	for _, item := range follows {
		entry, exists := cache.Entries[item.SubjectID]
		if !exists || (entry.AniListID > 0 && entry.ReleaseStatus == "") {
			weeklyDue = true
			break
		}
	}
	if weeklyDue && (cache.WeeklyAttemptAt.IsZero() || now.Sub(cache.WeeklyAttemptAt) >= airingRetryDelay) {
		cache.WeeklyAttemptAt = now
		changed = true
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		mapping, lookupErr := a.cachedAiringMappings(ctx, source, follows, now)
		if lookupErr == nil {
			ids := make([]int, 0, len(mapping))
			seen := make(map[int]bool)
			for _, id := range mapping {
				if !seen[id] {
					ids = append(ids, id)
					seen[id] = true
				}
			}
			sort.Ints(ids)
			next := make(map[int]followedAiring)
			if len(ids) > 0 {
				next, lookupErr = source.nextEpisodes(ctx, ids)
			}
			if lookupErr == nil {
				entries := make(map[int]followedAiring, len(follows))
				for _, follow := range follows {
					entry := next[mapping[follow.SubjectID]]
					entry.AniListID = mapping[follow.SubjectID]
					if cache.DailyDate == now.Format("2006-01-02") {
						previous := cache.Entries[follow.SubjectID]
						entry.TodayEpisode = previous.TodayEpisode
						entry.TodayAiringAt = previous.TodayAiringAt
					}
					entries[follow.SubjectID] = entry
				}
				cache.Entries = entries
				cache.WeeklyCheckedAt = now
				cache.WeeklyAttemptAt = time.Time{}
				cache.DailyDate = "" // A newly followed title may also air today.
				changed = true
			}
		}
		cancel()
		if lookupErr != nil {
			a.Log("WARN", fmt.Sprintf("更新追番下集排期失败: %v", lookupErr))
		}
	}
	today := now.Format("2006-01-02")
	dailyDue := cache.DailyDate != today
	if dailyDue && (cache.DailyAttemptAt.IsZero() || now.Sub(cache.DailyAttemptAt) >= airingRetryDelay) {
		cache.DailyAttemptAt = now
		changed = true
		ids := make([]int, 0, len(cache.Entries))
		seen := make(map[int]bool)
		for _, follow := range follows {
			id := cache.Entries[follow.SubjectID].AniListID
			if id > 0 && !seen[id] {
				ids = append(ids, id)
				seen[id] = true
			}
		}
		if len(ids) > 0 {
			sort.Ints(ids)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			todays, lookupErr := source.todayEpisodes(ctx, ids, now)
			cancel()
			if lookupErr != nil {
				a.Log("WARN", fmt.Sprintf("更新今日追番排期失败: %v", lookupErr))
			} else {
				for subjectID, entry := range cache.Entries {
					todayEntry := todays[entry.AniListID]
					entry.TodayEpisode = todayEntry.TodayEpisode
					entry.TodayAiringAt = todayEntry.TodayAiringAt
					cache.Entries[subjectID] = entry
				}
				cache.DailyDate = today
				cache.DailyAttemptAt = time.Time{}
			}
		}
	}
	if changed {
		if err := a.saveJSONCache(path, cache); err != nil {
			a.Log("WARN", fmt.Sprintf("保存追番排期缓存失败: %v", err))
		}
	}
	return cache
}

// GetFollowAirings returns only airing metadata; followed.json remains the
// source of truth for collection and download state.
func (a *App) GetFollowAirings() map[int]followedAiring {
	cache := a.followedAiringSnapshot()
	result := make(map[int]followedAiring, len(cache.Entries))
	now := time.Now()
	today := now.Format("2006-01-02")
	for id, entry := range cache.Entries {
		if cache.WeeklyCheckedAt.IsZero() || now.Sub(cache.WeeklyCheckedAt) >= airingWeekTTL {
			entry.NextEpisode = 0
			entry.NextAiringAt = 0
		}
		if cache.DailyDate != today {
			entry.TodayEpisode = 0
			entry.TodayAiringAt = 0
		}
		result[id] = entry
	}
	return result
}

func quarterStart(now time.Time) time.Time {
	month := time.Month((int(now.Month())-1)/3*3 + 1)
	return time.Date(now.Year(), month, 1, 0, 0, 0, 0, now.Location())
}

func mergeFollowedContinuations(calendar []bangumi.CalendarItem, follows []FollowedItem, schedules map[int]followedAiring, now time.Time) []bangumi.CalendarItem {
	start := quarterStart(now)
	end := start.AddDate(0, 3, 0)
	nextEnd := end.AddDate(0, 3, 0)
	result := make([]bangumi.CalendarItem, len(calendar))
	copy(result, calendar)
	seen := make(map[int]bool)
	dayIndex := make(map[int]int)
	followed := make(map[int]FollowedItem, len(follows))
	for _, follow := range follows {
		followed[follow.SubjectID] = follow
	}
	for i, day := range calendar {
		result[i].Items = append([]bangumi.Subject(nil), day.Items...)
		dayIndex[day.Weekday.ID] = i
		for j, item := range day.Items {
			seen[item.ID] = true
			follow, ok := followed[item.ID]
			if !ok {
				continue
			}
			premiere, err := time.ParseInLocation("2006-01-02", follow.AirDate, now.Location())
			schedule := schedules[item.ID]
			if err != nil || schedule.NextAiringAt <= 0 {
				continue
			}
			next := time.Unix(schedule.NextAiringAt, 0).In(now.Location())
			if premiere.Before(start) && !next.Before(start) && next.Before(end) {
				item.FollowedContinuation = true
			} else if premiere.Before(end) && !next.Before(end) && next.Before(nextEnd) {
				item.NextQuarterContinuation = true
			}
			if item.FollowedContinuation || item.NextQuarterContinuation {
				item.NextAiringAt = schedule.NextAiringAt
				result[i].Items[j] = item
			}
		}
	}
	weekdayNames := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	for _, follow := range follows {
		if seen[follow.SubjectID] {
			continue
		}
		premiere, err := time.ParseInLocation("2006-01-02", follow.AirDate, now.Location())
		if err != nil || !premiere.Before(start) {
			continue
		}
		schedule := schedules[follow.SubjectID]
		if schedule.NextAiringAt <= 0 {
			continue
		}
		next := time.Unix(schedule.NextAiringAt, 0).In(now.Location())
		if next.Before(start) || !next.Before(end) {
			continue
		}
		weekday := int(next.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		index, ok := dayIndex[weekday]
		if !ok {
			var day bangumi.CalendarItem
			day.Weekday.ID = weekday
			day.Weekday.CN = weekdayNames[next.Weekday()]
			result = append(result, day)
			index = len(result) - 1
			dayIndex[weekday] = index
		}
		item := bangumi.Subject{ID: follow.SubjectID, Name: follow.Name, NameCN: follow.NameCN, AirDate: follow.AirDate, AirWeekday: weekday, NextAiringAt: schedule.NextAiringAt, FollowedContinuation: true}
		item.Images.Common = follow.Image
		result[index].Items = append(result[index].Items, item)
		seen[item.ID] = true
	}
	return result
}
