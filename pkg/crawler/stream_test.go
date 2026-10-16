package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestStreamPublishesBeforeOtherSourceAndNextPageFinish(t *testing.T) {
	crawler := NewCrawler("https://example.test/resources", "")
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	crawler.client.SetTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("page") == "2" {
			<-release
			return jsonResponse(request, `{"resources":[],"pagination":{"complete":true}}`), nil
		}
		resources := make([]ResourceItem, 100)
		for i := range resources {
			resources[i] = ResourceItem{Title: "番剧 - 01 [CHS]", Magnet: "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567"}
		}
		body, _ := json.Marshal(SearchResp{Resources: resources})
		return jsonResponse(request, string(body)), nil
	}))
	crawler.mikanClient.SetTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-release
		return xmlResponse(request, `<rss><channel><item><title>番剧 - 01 [CHT]</title><enclosure url="https://mikanani.me/abcdef0123456789abcdef0123456789abcdef01.torrent"/></item></channel></rss>`), nil
	}))
	updates := make(chan []TorrentItem, 4)
	done := make(chan error, 1)
	go func() {
		items, err := crawler.SearchEpisodeListStream(context.Background(), []string{"番剧"}, 1, func(items []TorrentItem) { updates <- items })
		if err == nil && len(items) != 2 {
			err = fmt.Errorf("final candidates = %v", items)
		}
		done <- err
	}()
	select {
	case first := <-updates:
		if len(first) != 1 || !strings.Contains(first[0].Title, "CHS") {
			t.Fatalf("first candidates = %+v", first)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first page was held until all sources finished")
	}
	select {
	case <-done:
		t.Fatal("search completed while both sources were blocked")
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if second := <-updates; len(second) != 2 {
		t.Fatalf("second candidates = %+v", second)
	}
}

func TestSeasonOnlyChangesOrderAndDoesNotHideCandidates(t *testing.T) {
	resources := []ResourceItem{
		{Title: "番剧 第二季 - 01 [CHT]", Magnet: "magnet:season2"},
		{Title: "番剧 - 01 [CHS]", Magnet: "magnet:unmarked"},
		{Title: "番剧 第三季 - 01 [CHS]", Magnet: "magnet:season3"},
	}
	items := matchingEpisodes(resources, 1, "番剧 第二季")
	if len(items) != 3 || items[0].Magnet != "magnet:season2" || items[1].Magnet != "magnet:unmarked" {
		t.Fatalf("season preferences = %+v", items)
	}
}

func TestSearchFallsBackToPhraseFromFullTitle(t *testing.T) {
	crawler := NewCrawler("https://example.test/resources", "")
	crawler.mikanURL = ""
	var queries []string
	crawler.client.SetTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		query := request.URL.Query()["search"][0]
		queries = append(queries, query)
		body := `{"resources":[]}`
		if query == "番剧" {
			body = `{"resources":[{"title":"番剧 - 01 [CHS]","magnet":"magnet:found"}]}`
		}
		return jsonResponse(request, body), nil
	}))
	items, err := crawler.SearchEpisodeList([]string{"番剧：新的冒险 第二季"}, 1)
	if err != nil || len(items) != 1 || items[0].Magnet != "magnet:found" {
		t.Fatalf("keyword fallback = %+v, %v; queries = %v", items, err, queries)
	}
	if queries[0] != "番剧：新的冒险 第二季" || queries[len(queries)-1] != "番剧" {
		t.Fatalf("fallback order = %v", queries)
	}
}

func TestEpisodeQueriesTryOriginalNameAndBoundKeywordFallbacks(t *testing.T) {
	queries := episodeQueries([]string{"番剧 第二季", "Anime Season 2"})
	if len(queries) > 8 || !reflect.DeepEqual(queries[:2], []string{"番剧 第二季", "Anime Season 2"}) {
		t.Fatalf("queries = %v", queries)
	}
	if !strings.Contains(strings.Join(queries, "/"), "番剧/Anime") {
		t.Fatalf("missing season-free names: %v", queries)
	}
}

func TestEpisodeQueriesExtractWordsFromChineseTitle(t *testing.T) {
	queries := episodeQueries([]string{"关于地球的运动"})
	joined := "/" + strings.Join(queries, "/") + "/"
	if !strings.Contains(joined, "/地球/") || !strings.Contains(joined, "/运动/") {
		t.Fatalf("missing title keywords: %v", queries)
	}
}

func TestStreamCancellationStopsPendingRequests(t *testing.T) {
	crawler := NewCrawler("https://example.test/resources", "")
	crawler.mikanURL = ""
	started := make(chan struct{})
	crawler.client.SetTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		close(started)
		<-request.Context().Done()
		return nil, request.Context().Err()
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := crawler.SearchEpisodeListStream(ctx, []string{"番剧"}, 1, nil)
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("cancel error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled search kept waiting")
	}
}
