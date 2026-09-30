package imgproxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRestrictedImageDoesNotFollowRedirectToAnotherHost(t *testing.T) {
	var destinationCalled atomic.Bool
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinationCalled.Store(true)
		_, _ = w.Write([]byte("secret"))
	}))
	defer destination.Close()
	source := httptest.NewServer(http.RedirectHandler(destination.URL, http.StatusFound))
	defer source.Close()

	proxy := New(t.TempDir())
	request := httptest.NewRequest(http.MethodGet, "/img?u="+url.QueryEscape(source.URL), nil)
	response := httptest.NewRecorder()
	allowedHost := strings.TrimPrefix(source.URL, "http://")
	proxy.ServeImage(response, request, func(u *url.URL) bool {
		return u.Host == allowedHost
	})

	if response.Code != http.StatusBadGateway || destinationCalled.Load() {
		t.Fatalf("redirect escaped allowlist: status %d, destination called %v", response.Code, destinationCalled.Load())
	}
}

func TestRestrictedImageRejectsHTML(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<script>alert('x')</script>"))
	}))
	defer source.Close()

	proxy := New(t.TempDir())
	request := httptest.NewRequest(http.MethodGet, "/img?u="+url.QueryEscape(source.URL), nil)
	response := httptest.NewRecorder()
	proxy.ServeImage(response, request, func(u *url.URL) bool { return u.Host == strings.TrimPrefix(source.URL, "http://") })
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("HTML was served as an image: status %d", response.Code)
	}
}
