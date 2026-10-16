package bangumi

import (
	"encoding/json"
	"io"
	"net/http"
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
