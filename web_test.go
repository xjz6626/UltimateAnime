package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestWebHandlerServesVueAndLimitsRPC(t *testing.T) {
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		t.Fatal(err)
	}
	a := &App{webEvents: newWebEventHub()}
	handler := a.webHandler(dist)

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "UltimateAnime") || !strings.Contains(page.Body.String(), "window.__ULTIMATEANIME_WEB__=true") {
		t.Fatalf("Vue page: status %d, body %q", page.Code, page.Body.String())
	}

	call := func(origin, body string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/api/rpc", strings.NewReader(body))
		request.Host = "127.0.0.1:54322"
		request.Header.Set("Content-Type", "application/json")
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	status := call("http://127.0.0.1:54322", `{"method":"GetPikPakStatus","args":[]}`)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), "未登录") {
		t.Fatalf("status RPC: %d %q", status.Code, status.Body.String())
	}
	blocked := call("http://127.0.0.1:54322", `{"method":"GetAppConfig","args":[]}`)
	if blocked.Code != http.StatusBadRequest {
		t.Fatalf("settings RPC should be blocked: %d", blocked.Code)
	}
	foreign := call("https://attacker.example", `{"method":"GetPikPakStatus","args":[]}`)
	if foreign.Code != http.StatusForbidden {
		t.Fatalf("cross-origin RPC should be blocked: %d", foreign.Code)
	}
	noLogin := call("http://127.0.0.1:54322", `{"method":"DownloadEpisode","args":[42,1,"magnet:?xt=urn:btih:test"]}`)
	if noLogin.Code != http.StatusBadRequest {
		t.Fatalf("download without login should fail: %d", noLogin.Code)
	}
	forwarded := httptest.NewRequest(http.MethodPost, "/api/rpc", strings.NewReader(`{"method":"GetPikPakStatus","args":[]}`))
	forwarded.Host = "127.0.0.1:54322"
	forwarded.Header.Set("Content-Type", "application/json")
	forwarded.Header.Set("Origin", "https://home.example.ts.net")
	forwarded.Header.Set("X-Forwarded-Host", "home.example.ts.net")
	forwardedResponse := httptest.NewRecorder()
	handler.ServeHTTP(forwardedResponse, forwarded)
	if forwardedResponse.Code != http.StatusOK {
		t.Fatalf("Tailscale reverse proxy request: %d", forwardedResponse.Code)
	}

	image := httptest.NewRecorder()
	handler.ServeHTTP(image, httptest.NewRequest(http.MethodGet, "/img?u=https://127.0.0.1/secret", nil))
	if image.Code != http.StatusBadRequest {
		t.Fatalf("private image URL should be blocked: %d", image.Code)
	}
}

func TestWebEventHub(t *testing.T) {
	hub := newWebEventHub()
	client, unsubscribe := hub.subscribe()
	hub.publish("download-complete", map[string]int{"subject_id": 42})
	select {
	case event := <-client:
		if !strings.Contains(string(event), `"subject_id":42`) {
			t.Fatalf("unexpected event: %s", event)
		}
	default:
		t.Fatal("event was not delivered")
	}
	unsubscribe()
}

func TestWebImageURLAllowlist(t *testing.T) {
	for _, test := range []struct {
		address string
		allowed bool
	}{
		{"https://lain.bgm.tv/pic.jpg", true},
		{"https://bangumi.tv/pic.jpg", true},
		{"http://lain.bgm.tv/pic.jpg", false},
		{"https://bgm.tv:444/pic.jpg", false},
		{"https://bgm.tv.attacker.example/pic.jpg", false},
		{"https://user@bgm.tv/pic.jpg", false},
	} {
		u, err := url.Parse(test.address)
		if err != nil || webImageURLAllowed(u) != test.allowed {
			t.Errorf("allowlist for %s: got %v, parse error %v", test.address, webImageURLAllowed(u), err)
		}
	}
}
