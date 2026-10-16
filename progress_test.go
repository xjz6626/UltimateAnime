package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"UltimateAnime/pkg/bangumi"
)

func TestProgressUsesActualScheduleForDoublePremiereAndHiatus(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name                        string
		aired, next, bangumiEpisode int
	}{
		{"double premiere", 2, 3, 1},
		{"following week", 3, 4, 2},
		{"hiatus does not imply another episode", 3, 4, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Query     string `json:"query"`
					Variables struct {
						Now int64 `json:"now"`
						ID  int   `json:"id"`
					} `json:"variables"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if !strings.Contains(request.Query, "airingAt_lesser:$now") || request.Variables.Now != now.Unix()+1 || request.Variables.ID != 42 {
					t.Errorf("query did not request already aired episodes: %+v", request)
				}
				fmt.Fprintf(w, `{"data":{"Page":{"airingSchedules":[{"episode":%d,"airingAt":%d}]},"Media":{"status":"RELEASING","episodes":12,"nextAiringEpisode":{"episode":%d,"airingAt":%d}}}}`, tc.aired, now.Add(-time.Hour).Unix(), tc.next, now.AddDate(0, 0, 7).Unix())
			}))
			defer server.Close()
			source := airingSource{client: server.Client(), graphqlURL: server.URL}
			progress, err := source.episodeProgress(context.Background(), 42, now)
			episodes := []bangumi.Episode{{Sort: float64(tc.bangumiEpisode), AirDate: "2026-10-01"}}
			if err != nil || currentEpisode(episodes, progress, now) != float64(tc.aired) {
				t.Fatalf("progress = %+v, %v", progress, err)
			}
		})
	}
}

func TestProgressUsesUpcomingEpisodeWhenHistoryIsMissing(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":{"Page":{"airingSchedules":[]},"Media":{"status":"RELEASING","nextAiringEpisode":{"episode":3,"airingAt":%d}}}}`, now.Add(time.Hour).Unix())
	}))
	defer server.Close()
	source := airingSource{client: server.Client(), graphqlURL: server.URL}
	progress, err := source.episodeProgress(context.Background(), 42, now)
	if err != nil || !progress.Available || progress.AiredEpisode != 2 {
		t.Fatalf("premiere progress = %+v, %v", progress, err)
	}
}

func TestProgressCacheExpiresAtNextBroadcast(t *testing.T) {
	now := time.Now()
	cache := episodeProgressCache{FetchedAt: now, Progress: episodeAiringProgress{Available: true, AiredEpisode: 2, NextAiringAt: now.Add(time.Minute).Unix()}}
	if !progressCacheFresh(cache, now.Add(30*time.Second)) || progressCacheFresh(cache, now.Add(time.Minute)) {
		t.Fatal("cache did not expire at the scheduled broadcast")
	}
}

func TestProgressFallbackAndMissingEpisodeEntries(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	episodes := []bangumi.Episode{
		{ID: 10, Sort: 1, AirDate: "2026-10-01"},
		{ID: 30, Sort: 3, AirDate: "2026-10-16"},
		{ID: 40, Sort: 4, AirDate: "2026-10-23"},
		{ID: 90, Sort: 9, AirDate: "unknown"},
	}
	current := currentEpisode(episodes, episodeAiringProgress{}, now)
	if current != 3 {
		t.Fatalf("Bangumi fallback = %g", current)
	}
	filled := includeAiredEpisodes(episodes, current)
	if len(filled) != 5 || filled[1].Sort != 2 || filled[1].ID != -2 || len(episodes) != 4 {
		t.Fatalf("missing episode not restored: %+v", filled)
	}
	if got := currentEpisode(nil, episodeAiringProgress{AiredEpisode: 2}, now); got != 2 {
		t.Fatalf("offline fallback forgot confirmed episodes: %g", got)
	}
}

func TestAnimeDetailCorrectsProgressAndExposesSecondPremiereEpisode(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })
	app := &App{}
	now := time.Now()
	subject := bangumiSubjectCache{
		Subject:  bangumi.SubjectDetail{ID: 42, NameCN: "首播两集"},
		Episodes: []bangumi.Episode{{ID: 100, Sort: 1, AirDate: now.AddDate(0, 0, -1).Format("2006-01-02")}},
	}
	if err := app.saveJSONCache(filepath.Join(app.getCacheDir(), "bangumi_subject_42.json"), bangumiCacheEntry[bangumiSubjectCache]{FetchedAt: now, Data: subject}); err != nil {
		t.Fatal(err)
	}
	if err := app.saveJSONCache(filepath.Join(app.getCacheDir(), "episode_progress_42.json"), episodeProgressCache{FetchedAt: now, Progress: episodeAiringProgress{Available: true, AiredEpisode: 2}}); err != nil {
		t.Fatal(err)
	}
	detail, err := app.GetAnimeDetail(42)
	if err != nil {
		t.Fatal(err)
	}
	if detail.CurrentEpisode != 2 || detail.ProgressSource != "AniList" || len(detail.Episodes) != 2 || detail.Episodes[1].Sort != 2 {
		t.Fatalf("detail still hid the second premiere episode: %+v", detail)
	}
}
