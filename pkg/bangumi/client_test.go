package bangumi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEpisodesLoadsBeyondFirstHundred(t *testing.T) {
	client := NewClient("", "")
	var offsets []int
	client.client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		offsets = append(offsets, offset)
		count := 100
		if offset == 100 {
			count = 2
		}
		episodes := make([]Episode, count)
		for i := range episodes {
			episodes[i] = Episode{ID: offset + i + 1, Sort: float64(offset + i + 1)}
		}
		body, _ := json.Marshal(map[string]any{"total": 102, "data": episodes})
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	}))
	episodes, err := client.GetSubjectEpisodes(42)
	if err != nil || len(episodes) != 102 || episodes[101].Sort != 102 || len(offsets) != 2 || offsets[1] != 100 {
		t.Fatalf("pagination: episodes=%d offsets=%v error=%v", len(episodes), offsets, err)
	}
}

func TestConfigureClearsAuthorizationAndAppliesProxy(t *testing.T) {
	client := NewClient("old-token", "")
	client.Configure("", "http://127.0.0.1:7890")
	if client.Token != "" || client.client.Header.Get("Authorization") != "" {
		t.Fatal("stale authorization retained")
	}
	proxy, err := client.client.GetClient().Transport.(*http.Transport).Proxy(&http.Request{URL: &url.URL{Scheme: "https", Host: "api.bgm.tv"}})
	if err != nil || proxy.String() != "http://127.0.0.1:7890" {
		t.Fatalf("proxy=%v err=%v", proxy, err)
	}
	client.Configure("new-token", "")
	if client.client.Header.Get("Authorization") != "Bearer new-token" {
		t.Fatal("token not updated")
	}
}

func TestNextRequestHasNoTokenAfterClear(t *testing.T) {
	client := NewClient("old-token", "")
	headers := []string{}
	client.client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		headers = append(headers, r.Header.Get("Authorization"))
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}, nil
	}))
	if _, err := client.GetSubject(1); err != nil {
		t.Fatal(err)
	}
	client.Configure("", "")
	if _, err := client.GetSubject(1); err != nil {
		t.Fatal(err)
	}
	if len(headers) != 2 || headers[0] != "Bearer old-token" || headers[1] != "" {
		t.Fatalf("authorization: %q", headers)
	}
}
