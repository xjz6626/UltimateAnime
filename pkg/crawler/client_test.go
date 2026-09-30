package crawler

import (
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestParseEpisodeNumber(t *testing.T) {
	cases := []struct {
		title string
		want  float64
	}{
		{"[字幕组] 番剧 - 01 [1080P][简体]", 1},
		{"番剧 第12话", 12},
		{"番剧 S02E03 WEB", 3},
		{"番剧 [01v2]", 1},
		{"番剧 [01-12] 合集", -1},
		{"番剧 [1080P] [2026]", -1},
	}
	for _, tc := range cases {
		if got := ParseEpisodeNumber(tc.title); got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.title, got, tc.want)
		}
	}
}

func TestSearchEpisodeUsesSeparateTermsAndFilters(t *testing.T) {
	crawler := NewCrawler("https://example.test/resources", "")
	crawler.mikanURL = ""
	var searches [][]string
	crawler.client.SetTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		terms := request.URL.Query()["search"]
		searches = append(searches, append([]string(nil), terms...))
		if request.URL.Query().Get("page") != "1" {
			t.Errorf("unexpected page: %s", request.URL.Query().Get("page"))
		}
		body := `{"resources":[
			{"title":"番剧 - 02 [1080P]","magnet":"magnet:?xt=urn:btih:two"},
			{"title":"番剧 [01-12] 合集","magnet":"magnet:?xt=urn:btih:pack"},
			{"title":"番剧 - 01 [CHT]","magnet":"magnet:?xt=urn:btih:traditional"},
			{"title":"番剧 - 01 [CHS][1080P]","magnet":"magnet:?xt=urn:btih:simplified"}
		]}`
		header := make(http.Header)
		header.Set("Content-Type", "application/json")
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	}))

	items, err := crawler.SearchEpisodeList([]string{"番剧"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(searches, [][]string{{"番剧", "01"}}) {
		t.Fatalf("search terms: %v", searches)
	}
	if len(items) != 2 || items[0].Magnet != "magnet:?xt=urn:btih:simplified" || items[1].Magnet != "magnet:?xt=urn:btih:traditional" {
		t.Fatalf("unexpected filtered candidates: %+v", items)
	}
}

func TestSearchEpisodeFallsBackToTitle(t *testing.T) {
	crawler := NewCrawler("https://example.test/resources", "")
	crawler.mikanURL = ""
	requests := 0
	crawler.client.SetTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		body := `{"resources":[]}`
		if len(request.URL.Query()["search"]) == 1 {
			body = `{"resources":[{"title":"番剧 - 1 [1080P]","magnet":"magnet:?xt=urn:btih:one"}]}`
		}
		header := make(http.Header)
		header.Set("Content-Type", "application/json")
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	}))
	item, err := crawler.SearchEpisode([]string{"番剧"}, 1)
	if err != nil || item.Magnet != "magnet:?xt=urn:btih:one" || requests != 2 {
		t.Fatalf("fallback: item=%+v err=%v requests=%d", item, err, requests)
	}
}

func TestSearchEpisodeKeepsRequestedSeason(t *testing.T) {
	for _, test := range []struct {
		title string
		want  int
	}{
		{"番剧 第二季", 2},
		{"番剧 第十二季", 12},
		{"番剧 S03E01", 3},
		{"番剧 Season 4", 4},
		{"番剧 2nd Season", 2},
	} {
		if got, ok := seasonNumber(test.title); !ok || got != test.want {
			t.Errorf("seasonNumber(%q) = %d, %t", test.title, got, ok)
		}
	}

	crawler := NewCrawler("https://example.test/resources", "")
	crawler.mikanURL = ""
	crawler.client.SetTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"resources":[
			{"title":"番剧 第二季 S2 - 01 [CHS][1080P]","magnet":"magnet:?xt=urn:btih:season2"},
			{"title":"番剧 - 01 [CHT]","magnet":"magnet:?xt=urn:btih:season1"},
			{"title":"番剧 第三季 - 01 [CHS]","magnet":"magnet:?xt=urn:btih:season3"}
		]}`
		header := make(http.Header)
		header.Set("Content-Type", "application/json")
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	}))
	for _, test := range []struct {
		name string
		want string
	}{
		{"番剧", "magnet:?xt=urn:btih:season1"},
		{"番剧 第二季", "magnet:?xt=urn:btih:season2"},
		{"番剧 第三季", "magnet:?xt=urn:btih:season3"},
	} {
		item, err := crawler.SearchEpisode([]string{test.name}, 1)
		if err != nil || item.Magnet != test.want {
			t.Errorf("%q: item=%+v, error=%v", test.name, item, err)
		}
	}
}

func TestSearchResourceReportsHTTPError(t *testing.T) {
	crawler := NewCrawler("https://example.test/resources", "")
	crawler.mikanURL = ""
	crawler.client.SetTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("unavailable")), Request: request}, nil
	}))
	_, err := crawler.SearchResource("番剧")
	if err == nil || !strings.Contains(err.Error(), fmt.Sprint(503)) {
		t.Fatalf("expected HTTP error, got %v", err)
	}
}

func xmlResponse(request *http.Request, body string) *http.Response {
	header := make(http.Header)
	header.Set("Content-Type", "application/xml")
	return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: request}
}

func TestMikanRSSBuildsMagnetFromEnclosureHash(t *testing.T) {
	const hash = "0123456789abcdef0123456789abcdef01234567"
	crawler := NewCrawler("https://example.test/resources", "")
	crawler.mikanClient.SetTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.URL.Query().Get("searchstr"); got != "番剧 第二季 01" {
			t.Errorf("searchstr = %q", got)
		}
		body := `<rss xmlns:mikan="https://mikanani.me/0.1/"><channel><item>` +
			`<title>番剧 第二季 - 01</title>` +
			`<mikan:torrent><mikan:contentLength>1048576</mikan:contentLength>` +
			`<mikan:pubDate>2026-09-30T12:00:00</mikan:pubDate></mikan:torrent>` +
			`<enclosure url="https://mikanani.me/Download/20260930/` + hash + `.torrent"/>` +
			`</item></channel></rss>`
		return xmlResponse(request, body), nil
	}))
	items, err := crawler.fetchMikanResources("番剧 第二季 01")
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%+v, err=%v", items, err)
	}
	item := toTorrentItem(items[0])
	if item.Magnet != "magnet:?xt=urn:btih:"+hash || item.Size != "1.0 MB" || item.PublishDate != "2026-09-30 12:00" || item.Source != "蜜柑计划" {
		t.Fatalf("parsed item=%+v", item)
	}
}

func TestSearchEpisodeMergesAndDeduplicatesSources(t *testing.T) {
	const sharedHash = "0123456789abcdef0123456789abcdef01234567"
	const mikanHash = "abcdef0123456789abcdef0123456789abcdef01"
	decoded, err := hex.DecodeString(sharedHash)
	if err != nil {
		t.Fatal(err)
	}
	base32Hash := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(decoded)
	crawler := NewCrawler("https://example.test/resources", "")
	crawler.client.SetTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := fmt.Sprintf(`{"resources":[{"title":"番剧 - 01 [CHS]","magnet":"magnet:?xt=urn:btih:%s&tr=tracker"}]}`, base32Hash)
		header := make(http.Header)
		header.Set("Content-Type", "application/json")
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	}))
	crawler.mikanClient.SetTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `<rss><channel>` +
			`<item><title>番剧 - 01 [CHS]</title><enclosure url="https://mikanani.me/Download/` + sharedHash + `.torrent"/></item>` +
			`<item><title>番剧 - 01 [CHT]</title><enclosure url="https://mikanani.me/Download/` + mikanHash + `.torrent"/></item>` +
			`</channel></rss>`
		return xmlResponse(request, body), nil
	}))
	items, err := crawler.SearchEpisodeList([]string{"番剧"}, 1)
	if err != nil || len(items) != 2 || items[1].Source != "蜜柑计划" {
		t.Fatalf("merged items=%+v, err=%v", items, err)
	}
}

func TestSearchEpisodeUsesMikanWhenPrimaryFails(t *testing.T) {
	const hash = "abcdef0123456789abcdef0123456789abcdef01"
	crawler := NewCrawler("https://example.test/resources", "")
	crawler.client.SetTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("unavailable")), Request: request}, nil
	}))
	crawler.mikanClient.SetTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `<rss><channel><item><title>番剧 - 01</title><enclosure url="https://mikanani.me/Download/` + hash + `.torrent"/></item></channel></rss>`
		return xmlResponse(request, body), nil
	}))
	item, err := crawler.SearchEpisode([]string{"番剧"}, 1)
	if err != nil || item == nil || item.Source != "蜜柑计划" {
		t.Fatalf("item=%+v, err=%v", item, err)
	}
}
