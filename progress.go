package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"UltimateAnime/pkg/bangumi"
)

const episodeProgressTTL = time.Hour

type episodeAiringProgress struct {
	Available    bool  `json:"available"`
	AiredEpisode int   `json:"aired_episode"`
	NextEpisode  int   `json:"next_episode,omitempty"`
	NextAiringAt int64 `json:"next_airing_at,omitempty"`
}

type episodeProgressCache struct {
	FetchedAt time.Time             `json:"fetched_at"`
	AttemptAt time.Time             `json:"attempt_at,omitempty"`
	Progress  episodeAiringProgress `json:"progress"`
}

// Query actual episode schedules, rather than estimating one episode per week.
// Both episodes of a double premiere are counted as soon as they have aired.
func (s airingSource) episodeProgress(ctx context.Context, id int, now time.Time) (episodeAiringProgress, error) {
	const query = `query($id:Int,$now:Int) {
  Page(page:1,perPage:1) { airingSchedules(mediaId:$id,airingAt_lesser:$now,sort:EPISODE_DESC) { episode airingAt } }
  Media(id:$id,type:ANIME) { status episodes nextAiringEpisode { episode airingAt } }
}`
	var response struct {
		Page struct {
			Schedules []struct {
				Episode  int   `json:"episode"`
				AiringAt int64 `json:"airingAt"`
			} `json:"airingSchedules"`
		} `json:"Page"`
		Media *struct {
			Status   string `json:"status"`
			Episodes int    `json:"episodes"`
			Next     *struct {
				Episode  int   `json:"episode"`
				AiringAt int64 `json:"airingAt"`
			} `json:"nextAiringEpisode"`
		} `json:"Media"`
	}
	if err := s.query(ctx, query, map[string]any{"id": id, "now": now.Unix() + 1}, &response); err != nil {
		return episodeAiringProgress{}, err
	}
	if response.Media == nil {
		return episodeAiringProgress{}, fmt.Errorf("AniList 未返回剧集进度")
	}
	progress := episodeAiringProgress{}
	for _, schedule := range response.Page.Schedules {
		if schedule.AiringAt <= now.Unix() && schedule.Episode > progress.AiredEpisode {
			progress.Available = true
			progress.AiredEpisode = schedule.Episode
		}
	}
	if next := response.Media.Next; next != nil && next.AiringAt > now.Unix() && next.Episode > 0 {
		progress.NextEpisode = next.Episode
		progress.NextAiringAt = next.AiringAt
		// Some titles have future schedules but no historical schedules indexed.
		if !progress.Available {
			progress.Available = true
			progress.AiredEpisode = next.Episode - 1
		}
	}
	if response.Media.Status == "FINISHED" && response.Media.Episodes > 0 {
		progress.Available = true
		if response.Media.Episodes > progress.AiredEpisode {
			progress.AiredEpisode = response.Media.Episodes
		}
	} else if response.Media.Status == "NOT_YET_RELEASED" && !progress.Available {
		progress.Available = true
	}
	return progress, nil
}

func progressCacheFresh(cache episodeProgressCache, now time.Time) bool {
	return !cache.FetchedAt.IsZero() && now.Sub(cache.FetchedAt) < episodeProgressTTL &&
		!(cache.Progress.NextAiringAt > 0 && cache.FetchedAt.Unix() < cache.Progress.NextAiringAt && now.Unix() >= cache.Progress.NextAiringAt)
}

func (a *App) subjectEpisodeProgress(subjectID int, now time.Time) episodeAiringProgress {
	a.progressMu.Lock()
	defer a.progressMu.Unlock()
	path := filepath.Join(a.getCacheDir(), fmt.Sprintf("episode_progress_%d.json", subjectID))
	var cache episodeProgressCache
	_, _ = a.loadJSONCache(path, &cache)
	if progressCacheFresh(cache, now) {
		return cache.Progress
	}
	fallback := cache.Progress
	fallback.Available = false
	if a.configMgr == nil || (!cache.AttemptAt.IsZero() && now.Sub(cache.AttemptAt) < airingRetryDelay) {
		return fallback
	}
	cache.AttemptAt = now
	source := newAiringSource(a.configMgr.Snapshot().GlobalSettings.Proxy)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	mappings, err := a.cachedAiringMappings(ctx, source, []FollowedItem{{SubjectID: subjectID}}, now)
	if err == nil && mappings[subjectID] > 0 {
		var progress episodeAiringProgress
		progress, err = source.episodeProgress(ctx, mappings[subjectID], now)
		if err == nil {
			cache.Progress = progress
			cache.FetchedAt = now
			cache.AttemptAt = time.Time{}
		}
	} else if err == nil {
		// Cache missing mappings too; new titles may not be in bangumi-data yet.
		cache.Progress = episodeAiringProgress{}
		cache.FetchedAt = now
		cache.AttemptAt = time.Time{}
	}
	if err != nil {
		a.Log("WARN", fmt.Sprintf("获取番剧 %d 的放送进度失败，保留已有数据: %v", subjectID, err))
	}
	if saveErr := a.saveJSONCache(path, cache); saveErr != nil {
		a.Log("WARN", fmt.Sprintf("保存剧集进度失败: %v", saveErr))
	}
	if err != nil {
		return fallback
	}
	return cache.Progress
}

func currentEpisode(episodes []bangumi.Episode, progress episodeAiringProgress, now time.Time) float64 {
	if progress.Available {
		return float64(progress.AiredEpisode)
	}
	current := float64(progress.AiredEpisode)
	for _, ep := range episodes {
		date, err := time.ParseInLocation("2006-01-02", ep.AirDate, now.Location())
		if ep.Type == 0 && err == nil && !date.After(now) && ep.Sort > current {
			current = ep.Sort
		}
	}
	return current
}

func includeAiredEpisodes(episodes []bangumi.Episode, current float64) []bangumi.Episode {
	seen := make(map[float64]bool, len(episodes))
	result := append([]bangumi.Episode(nil), episodes...)
	for _, episode := range episodes {
		seen[episode.Sort] = true
	}
	// Missing Bangumi episode entries must not hide confirmed released episodes.
	for number := 1; number <= int(current) && number <= 10000; number++ {
		if !seen[float64(number)] {
			result = append(result, bangumi.Episode{ID: -number, Sort: float64(number)})
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Sort < result[j].Sort })
	return result
}
