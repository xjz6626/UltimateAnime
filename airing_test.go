package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"UltimateAnime/pkg/bangumi"
)

func TestMergeFollowedContinuationsRequiresConfirmedNextEpisode(t *testing.T) {
	zone := time.FixedZone("SGT", 8*60*60)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, zone)
	next := time.Date(2026, 10, 19, 22, 0, 0, 0, zone).Unix()
	calendar := []bangumi.CalendarItem{{Items: []bangumi.Subject{{ID: 10}}}}
	calendar[0].Weekday.ID = 1
	follows := []FollowedItem{
		{SubjectID: 1, Name: "Continuing", AirDate: "2026-07-01", Image: "cover"},
		{SubjectID: 2, Name: "Finished", AirDate: "2026-07-01"},
		{SubjectID: 3, Name: "New", AirDate: "2026-10-01"},
		{SubjectID: 10, Name: "Already present", AirDate: "2026-07-01"},
	}
	schedules := map[int]followedAiring{
		1:  {NextAiringAt: next},
		3:  {NextAiringAt: next},
		10: {NextAiringAt: next},
	}
	merged := mergeFollowedContinuations(calendar, follows, schedules, now)
	if len(merged) != 1 || len(merged[0].Items) != 2 {
		t.Fatalf("merged calendar = %+v", merged)
	}
	carried := merged[0].Items[1]
	if carried.ID != 1 || !carried.FollowedContinuation || carried.NextAiringAt != next || carried.Images.Common != "cover" {
		t.Fatalf("continuation = %+v", carried)
	}
	if len(calendar[0].Items) != 1 {
		t.Fatal("merge mutated the raw Bangumi calendar")
	}
	schedules[1] = followedAiring{NextAiringAt: time.Date(2027, 1, 1, 12, 0, 0, 0, zone).Unix()}
	if got := mergeFollowedContinuations(calendar, follows, schedules, now); len(got[0].Items) != 1 {
		t.Fatalf("next-quarter airing was added to the wrong quarter: %+v", got)
	}
}

func TestFollowedCalendarItemShowsNextQuarterSchedule(t *testing.T) {
	zone := time.FixedZone("SGT", 8*60*60)
	now := time.Date(2026, 9, 30, 20, 0, 0, 0, zone)
	next := time.Date(2026, 10, 19, 22, 0, 0, 0, zone).Unix()
	calendar := []bangumi.CalendarItem{{Items: []bangumi.Subject{{ID: 530725, Name: "BLEACH 千年血戦篇-禍進譚-"}}}}
	calendar[0].Weekday.ID = 6
	follows := []FollowedItem{{SubjectID: 530725, AirDate: "2026-07-25"}}
	got := mergeFollowedContinuations(calendar, follows, map[int]followedAiring{530725: {NextAiringAt: next}}, now)
	item := got[0].Items[0]
	if !item.NextQuarterContinuation || item.FollowedContinuation || item.NextAiringAt != next {
		t.Fatalf("next-quarter badge = %+v", item)
	}
	if calendar[0].Items[0].NextQuarterContinuation {
		t.Fatal("raw calendar was changed")
	}
}

func TestCachedAiringMappingsAvoidsRepeatDownload(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			_, _ = w.Write([]byte(`{"items":[{"sites":[{"site":"bangumi","id":"530725"},{"site":"aniList","id":"185874"}]}]}`))
			return
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	app := &App{webEvents: newWebEventHub()}
	source := airingSource{client: server.Client(), mappingURL: server.URL}
	follows := []FollowedItem{{SubjectID: 530725}}
	now := time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC)
	for _, checkedAt := range []time.Time{now, now.Add(time.Hour)} {
		got, err := app.cachedAiringMappings(context.Background(), source, follows, checkedAt)
		if err != nil || got[530725] != 185874 {
			t.Fatalf("cached mappings = %v, %v", got, err)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("mapping downloaded %d times", requests.Load())
	}
	got, err := app.cachedAiringMappings(context.Background(), source, follows, now.Add(31*24*time.Hour))
	if err != nil || got[530725] != 185874 || requests.Load() != 2 {
		t.Fatalf("stale mapping fallback = %v, %v; requests = %d", got, err, requests.Load())
	}
}

func TestAiringSourceMappingAndSchedules(t *testing.T) {
	zone := time.FixedZone("SGT", 8*60*60)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, zone)
	airingAt := time.Date(2026, 10, 19, 22, 0, 0, 0, zone).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mapping":
			_, _ = w.Write([]byte(`{"items":[{"sites":[{"site":"bangumi","id":"42"},{"site":"aniList","id":"185874"}]}]}`))
		case "/graphql":
			var request struct {
				Query string `json:"query"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode query: %v", err)
				return
			}
			switch {
			case strings.Contains(request.Query, "nextAiringEpisode"):
				_, _ = w.Write([]byte(`{"data":{"Page":{"media":[{"id":185874,"status":"RELEASING","nextAiringEpisode":{"episode":9,"airingAt":1792418400}}]}}}`))
			case strings.Contains(request.Query, "airingSchedules"):
				_, _ = w.Write([]byte(`{"data":{"Page":{"airingSchedules":[{"mediaId":185874,"episode":9,"airingAt":1792418400}]}}}`))
			default:
				t.Errorf("unexpected GraphQL query: %s", request.Query)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	source := airingSource{client: server.Client(), mappingURL: server.URL + "/mapping", graphqlURL: server.URL + "/graphql"}
	mapping, err := source.mappings(context.Background(), []FollowedItem{{SubjectID: 42}})
	if err != nil || mapping[42] != 185874 {
		t.Fatalf("mappings = %v, %v", mapping, err)
	}
	next, err := source.nextEpisodes(context.Background(), []int{185874})
	if err != nil || next[185874].NextEpisode != 9 || next[185874].NextAiringAt != airingAt || next[185874].ReleaseStatus != "RELEASING" {
		t.Fatalf("next = %v, %v", next, err)
	}
	today, err := source.todayEpisodes(context.Background(), []int{185874}, now)
	if err != nil || today[185874].TodayEpisode != 9 || today[185874].TodayAiringAt != airingAt {
		t.Fatalf("today = %v, %v", today, err)
	}
}

func TestAiringSourceKeepsPausedAnimeDistinctFromFinished(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"Page":{"media":[{"id":1,"status":"FINISHED","nextAiringEpisode":null},{"id":2,"status":"HIATUS","nextAiringEpisode":null}]}}}`))
	}))
	defer server.Close()
	source := airingSource{client: server.Client(), graphqlURL: server.URL}
	entries, err := source.nextEpisodes(context.Background(), []int{1, 2})
	if err != nil || entries[1].ReleaseStatus != "FINISHED" || entries[2].ReleaseStatus != "HIATUS" {
		t.Fatalf("release statuses = %v, %v", entries, err)
	}
}

func TestSameLocalDayAtQuarterBoundary(t *testing.T) {
	zone := time.FixedZone("SGT", 8*60*60)
	before := time.Date(2026, 9, 30, 23, 59, 0, 0, zone)
	after := before.Add(2 * time.Minute)
	if sameLocalDay(before, after) || !sameLocalDay(after, after) {
		t.Fatal("daily calendar cache did not refresh at local midnight")
	}
}
