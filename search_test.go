package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"UltimateAnime/pkg/crawler"
)

func TestWebSearchPublishesCandidatesBeforeRPCCompletes(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			http.Error(w, "source unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			<-release
			_, _ = w.Write([]byte(`{"resources":[],"pagination":{"complete":true}}`))
			return
		}
		resources := make([]crawler.ResourceItem, 100)
		for i := range resources {
			resources[i] = crawler.ResourceItem{Title: "番剧 - 01 [CHS]", Magnet: "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567"}
		}
		_ = json.NewEncoder(w).Encode(crawler.SearchResp{Resources: resources})
	}))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	app := &App{crawler: crawler.NewCrawler(server.URL, server.URL), webEvents: newWebEventHub()}
	events, unsubscribe := app.webEvents.subscribe()
	defer unsubscribe()
	done := make(chan error, 1)
	go func() {
		_, err := app.webCall("SearchEpisodeMagnetListStream", []json.RawMessage{json.RawMessage(`42`), json.RawMessage(`1`), json.RawMessage(`"番剧"`), json.RawMessage(`"browser-one"`)})
		done <- err
	}()
	deadline := time.After(3 * time.Second)
	found := false
	for !found {
		select {
		case message := <-events:
			var event struct {
				Name string          `json:"name"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(message, &event); err != nil {
				t.Fatal(err)
			}
			if event.Name != "magnet-search-results" {
				continue
			}
			var update magnetSearchUpdate
			if err := json.Unmarshal(event.Data, &update); err != nil {
				t.Fatal(err)
			}
			if update.RequestID != "browser-one" || len(update.Items) != 1 {
				t.Fatalf("unexpected event: %+v", update)
			}
			found = true
		case <-deadline:
			t.Fatal("no results were pushed while later pages were still pending")
		}
	}
	select {
	case <-done:
		t.Fatal("RPC completed before second page was released")
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	app.searchMu.Lock()
	defer app.searchMu.Unlock()
	if len(app.searches) != 0 {
		t.Fatal("finished search was not cleaned up")
	}
}

func TestWebSearchStopsWhenRequestIsCanceled(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			http.Error(w, "source unavailable", http.StatusBadGateway)
			return
		}
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := &App{crawler: crawler.NewCrawler(server.URL, server.URL)}
	done := make(chan error, 1)
	go func() {
		_, err := app.webCallContext(ctx, "SearchEpisodeMagnetListStream", []json.RawMessage{json.RawMessage(`42`), json.RawMessage(`1`), json.RawMessage(`"番剧"`), json.RawMessage(`"canceled-browser"`)})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("search did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("cancel error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP cancellation did not stop the upstream search")
	}
}
