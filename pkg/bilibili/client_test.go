package bilibili

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestSearchSeasonInitializesDeviceAndSignsRequests(t *testing.T) {
	client, err := NewClient("", "SESSDATA=private-value")
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string]int{}
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls[r.URL.Path]++
		if r.Header.Get("Referer") != "https://www.bilibili.com/" || r.Header.Get("User-Agent") == "" {
			t.Fatal("missing Bilibili request headers")
		}
		if cookie, _ := r.Cookie("SESSDATA"); cookie == nil || cookie.Value != "private-value" {
			t.Fatal("optional cookie was not forwarded")
		}
		switch r.URL.Path {
		case "/x/frontend/finger/spi":
			return response(`{"code":0,"data":{"b_3":"device-cookie"}}`), nil
		case "/x/web-interface/nav":
			return response(`{"code":-101,"data":{"wbi_img":{"img_url":"https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png","sub_url":"https://i0.hdslb.com/bfs/wbi/4932caff0ff746eab6f01bf08b70ac45.png"}}}`), nil
		case "/x/web-interface/wbi/search/type":
			if cookie, _ := r.Cookie("buvid3"); cookie == nil || cookie.Value != "device-cookie" {
				t.Fatal("missing device cookie")
			}
			query := r.URL.Query()
			signature := query.Get("w_rid")
			query.Del("w_rid")
			if signature == "" || !strings.Contains(r.URL.RawQuery, "%20") || strings.Contains(r.URL.RawQuery, "+") {
				t.Fatalf("invalid signed search query: %s", r.URL.RawQuery)
			}
			if query.Get("search_type") != "media_bangumi" || query.Get("keyword") != "测试 第二季" {
				t.Fatal(query)
			}
			return response(`{"code":0,"data":{"result":[{"season_id":1,"title":"测试"},{"season_id":2,"title":"<em>测试</em> 第二季"}]}}`), nil
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
			return nil, nil
		}
	})
	for i := 0; i < 2; i++ {
		id, err := client.SearchSeason(context.Background(), []string{"测试 第二季"}, 0)
		if err != nil || id != 2 {
			t.Fatalf("id=%d error=%v", id, err)
		}
	}
	if calls["/x/frontend/finger/spi"] != 1 || calls["/x/web-interface/nav"] != 1 {
		t.Fatal("device and keys should be reused for this client", calls)
	}
}

func TestWBISignatureKnownVector(t *testing.T) {
	query := signWBI(url.Values{"foo": {"114"}, "bar": {"514"}, "zab": {"1919810"}}, "ea1db124af3c7062474693fa704f4ff8", 1702204169)
	if query.Get("w_rid") != "8f6f2b5b3d485fe1886cec6a0be8c5d4" {
		t.Fatal(query)
	}
}

func TestSearchFallsBackToPublicLegacyEndpoint(t *testing.T) {
	client, _ := NewClient("", "")
	client.wbiKey = "already-initialized-wbi-key"
	wbiCalls, legacyCalls := 0, 0
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/x/web-interface/wbi/search/type":
			wbiCalls++
			return response(`{"code":0,"data":{"v_voucher":"verification-required"}}`), nil
		case "/x/web-interface/search/type":
			legacyCalls++
			if r.URL.Query().Has("w_rid") || r.URL.Query().Has("wts") {
				t.Fatal("legacy request should not reuse WBI parameters")
			}
			return response(`{"code":0,"data":{"result":[{"season_id":42,"title":"测试","org_title":"Test"}]}}`), nil
		default:
			t.Fatalf("unexpected request %s", r.URL.Path)
			return nil, nil
		}
	})
	id, err := client.SearchSeason(context.Background(), []string{"测试", "Test"}, 0)
	if err != nil || id != 42 || wbiCalls != 1 || legacyCalls != 2 {
		t.Fatalf("id=%d error=%v WBI=%d legacy=%d", id, err, wbiCalls, legacyCalls)
	}
}

func TestChooseSeasonAvoidsOtherSeasonsAndAmbiguousRemakes(t *testing.T) {
	pubtime := func(year int) int64 { return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).Unix() }
	for _, test := range []struct {
		name    string
		results []searchResult
		names   []string
		year    int
		want    int
	}{
		{"season", []searchResult{{ID: 1, Title: "测试"}, {ID: 2, Title: "测试 第2季"}}, []string{"测试 第二季"}, 0, 2},
		{"english", []searchResult{{ID: 2, Title: "测试 Season 2"}}, []string{"测试 第2季"}, 0, 2},
		{"region", []searchResult{{ID: 2, Title: "测试（仅限港澳台地区）"}}, []string{"测试"}, 0, 2},
		{"remake", []searchResult{{ID: 1, Title: "测试", PubTime: pubtime(1999)}, {ID: 2, Title: "测试", PubTime: pubtime(2026)}}, []string{"测试"}, 2026, 2},
		{"wrong season", []searchResult{{ID: 1, Title: "测试"}}, []string{"测试 第二季"}, 0, 0},
		{"partial title", []searchResult{{ID: 1, Title: "测试剧场版"}}, []string{"测试"}, 0, 0},
		{"ambiguous", []searchResult{{ID: 1, Title: "测试"}, {ID: 2, Title: "测试"}}, []string{"测试"}, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			id, err := chooseSeason(test.results, test.names, test.year)
			if id != test.want || (err != nil) != (test.want == 0) {
				t.Fatalf("id=%d err=%v, want %d", id, err, test.want)
			}
		})
	}
}

func TestEpisodeMatchesNumberRatherThanPosition(t *testing.T) {
	season := Season{Title: "测试", Episodes: []Episode{
		{CID: 99, Title: "PV"}, {CID: 20, Title: "02"}, {CID: 10, Title: "第1话"}, {CID: 25, Title: "2.5"},
	}}
	for sort, cid := range map[float64]int64{1: 10, 2: 20, 2.5: 25} {
		ep, err := season.Episode(sort)
		if err != nil || ep.CID != cid {
			t.Fatalf("sort=%g episode=%+v err=%v", sort, ep, err)
		}
	}
	if _, err := season.Episode(3); err == nil {
		t.Fatal("missing episode was inferred from list order")
	}
}

func TestCommentsDecodePlainAndCompressedXML(t *testing.T) {
	xml := `<i><d p="1.5,1,25,16711680,0,0,hash,1">测试 &amp; 弹幕</d><d p="NaN,1,25,1">invalid</d></i>`
	for _, encoding := range []string{"plain", "raw", "deflate", "gzip"} {
		t.Run(encoding, func(t *testing.T) {
			var body bytes.Buffer
			var compressor io.WriteCloser
			switch encoding {
			case "raw":
				compressor, _ = flate.NewWriter(&body, flate.DefaultCompression)
			case "deflate":
				compressor = zlib.NewWriter(&body)
			case "gzip":
				compressor = gzip.NewWriter(&body)
			default:
				body.WriteString(xml)
			}
			if compressor != nil {
				_, _ = io.WriteString(compressor, xml)
				_ = compressor.Close()
			}
			client, _ := NewClient("", "")
			client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Query().Get("oid") != "123" {
					t.Fatal(r.URL)
				}
				resp := response(body.String())
				if encoding == "gzip" || encoding == "deflate" {
					resp.Header.Set("Content-Encoding", encoding)
				}
				return resp, nil
			})
			comments, err := client.Comments(context.Background(), 123)
			if err != nil || len(comments) != 1 || comments[0].Text != "测试 & 弹幕" || comments[0].Color != 0xff0000 {
				t.Fatalf("comments=%+v err=%v", comments, err)
			}
		})
	}
}

func TestClientRejectsAPIErrorAndCancelledRequests(t *testing.T) {
	client, _ := NewClient("", "")
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(`{"code":-412,"message":"private-error-text"}`), nil
	})
	if _, err := client.Season(context.Background(), 1); err == nil || strings.Contains(err.Error(), "private-error-text") {
		t.Fatal("API errors must be recognized without echoing server text", err)
	}
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("request contained private credentials")
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Comments(ctx, 1); err != context.Canceled {
		t.Fatal(err)
	}
}
