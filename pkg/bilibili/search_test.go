package bilibili

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestManualSearchReturnsAllCandidatesWithoutAutomaticTitleFiltering(t *testing.T) {
	client, _ := NewClient("", "")
	client.legacySearch = true
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("page") != "2" || r.URL.Query().Get("search_type") != "video" {
			t.Fatal(r.URL)
		}
		return response(`{"code":0,"data":{"result":[{"bvid":"BV13x41117TL","title":"<em>测试</em>片段 &amp; 讨论","author":"UP"}]}}`), nil
	})
	items, err := client.Search(context.Background(), "测试", "video", 2)
	if err != nil || len(items) != 1 || items[0].Title != "测试片段 & 讨论" || items[0].ID != "BV13x41117TL" {
		t.Fatalf("items=%+v error=%v", items, err)
	}
}

func TestDirectBVIDAndExplicitPartSelection(t *testing.T) {
	client, _ := NewClient("", "")
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/x/web-interface/view" || r.URL.Query().Get("bvid") != "BV13x41117TL" {
			t.Fatal(r.URL)
		}
		return response(`{"code":0,"data":{"bvid":"BV13x41117TL","title":"合集","pages":[{"cid":501,"page":1,"part":"第一部分"},{"cid":502,"page":2,"part":"第二部分"}]}}`), nil
	})
	items, err := client.Search(context.Background(), "https://www.bilibili.com/video/BV13x41117TL/?p=2", "season", 1)
	if err != nil || len(items) != 1 || items[0].Kind != "video" {
		t.Fatal(items, err)
	}
	details, err := client.Details(context.Background(), items[0].Kind, items[0].ID)
	if err != nil || len(details.Parts) != 2 || details.Parts[1].CID != 502 || !strings.Contains(details.Parts[1].Title, "第二部分") {
		t.Fatal(details, err)
	}
}
