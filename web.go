package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"UltimateAnime/pkg/bangumi"
)

const webPort = "54322"

type webEventHub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func newWebEventHub() *webEventHub {
	return &webEventHub{clients: make(map[chan []byte]struct{})}
}

func (h *webEventHub) publish(name string, data any) {
	if h == nil {
		return
	}
	message, err := json.Marshal(struct {
		Name string `json:"name"`
		Data any    `json:"data"`
	}{name, data})
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		select {
		case client <- message:
		default: // A slow browser must not block downloads or logging.
		}
	}
}

func (h *webEventHub) subscribe() (chan []byte, func()) {
	client := make(chan []byte, 32)
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
	return client, func() {
		h.mu.Lock()
		delete(h.clients, client)
		h.mu.Unlock()
	}
}

func (a *App) startWebServer(ctx context.Context) {
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Printf("remote web assets unavailable: %v", err)
		return
	}
	handler := a.webHandler(dist)
	a.listenWeb(ctx, handler, "127.0.0.1")
	if address, err := tailscaleIPv4(); err == nil {
		a.listenWeb(ctx, handler, address)
	} else {
		log.Printf("Tailscale web listener unavailable: %v", err)
	}
}

func (a *App) listenWeb(ctx context.Context, handler http.Handler, host string) {
	address := net.JoinHostPort(host, webPort)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Printf("remote web server unavailable at %s: %v", address, err)
		return
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	go func() {
		log.Printf("remote web interface ready at http://%s", address)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("remote web server stopped: %v", err)
		}
	}()
}

func tailscaleIPv4() (string, error) {
	tailnetRange := netip.MustParsePrefix("100.64.0.0/10")
	check := func(candidate string) (string, bool) {
		address, err := netip.ParseAddr(candidate)
		if err != nil || !address.Is4() || !tailnetRange.Contains(address) {
			return "", false
		}
		return address.String(), true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "tailscale", "ip", "-4").Output(); err == nil {
		for _, candidate := range strings.Fields(string(output)) {
			if address, ok := check(candidate); ok {
				return address, nil
			}
		}
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		if !strings.Contains(strings.ToLower(iface.Name), "tailscale") {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, candidate := range addresses {
			if ipNet, ok := candidate.(*net.IPNet); ok {
				if address, valid := check(ipNet.IP.String()); valid {
					return address, nil
				}
			}
		}
	}
	return "", errors.New("Tailscale IPv4 address not found; connect Tailscale and restart the app")
}

func (a *App) webHandler(dist fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/rpc", a.handleWebRPC)
	mux.HandleFunc("/api/events", a.handleWebEvents)
	mux.HandleFunc("/img", a.handleWebImage)
	static := http.FileServer(http.FS(dist))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			static.ServeHTTP(w, r)
			return
		}
		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			http.Error(w, "web interface unavailable", http.StatusInternalServerError)
			return
		}
		index = []byte(strings.Replace(string(index), "<head>", "<head><script>window.__ULTIMATEANIME_WEB__=true</script>", 1))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(index)
	})
	return webHeaders(mux)
}

func webHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // Non-browser clients can access the loopback service.
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	return strings.EqualFold(u.Host, r.Host) || strings.EqualFold(u.Host, r.Header.Get("X-Forwarded-Host"))
}

type webRPCRequest struct {
	Method string            `json:"method"`
	Args   []json.RawMessage `json:"args"`
}

func webArgs(args []json.RawMessage, values ...any) error {
	if len(args) != len(values) {
		return fmt.Errorf("expected %d arguments", len(values))
	}
	for i, value := range values {
		if err := json.Unmarshal(args[i], value); err != nil {
			return fmt.Errorf("argument %d: %w", i+1, err)
		}
	}
	return nil
}

func (a *App) handleWebRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if !sameOrigin(r) {
		http.Error(w, "cross-origin request denied", http.StatusForbidden)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "JSON required", http.StatusUnsupportedMediaType)
		return
	}
	var request webRPCRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(w, "one JSON object required", http.StatusBadRequest)
		return
	}
	result, err := a.webCall(request.Method, request.Args)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Result any `json:"result"`
	}{result})
}

// Only actions needed for remote discovery and downloading are exposed.
func (a *App) webCall(method string, args []json.RawMessage) (any, error) {
	switch method {
	case "GetPikPakStatus":
		if err := webArgs(args); err != nil {
			return nil, err
		}
		return a.GetPikPakStatus(), nil
	case "GetBangumiCalendar":
		if err := webArgs(args); err != nil {
			return nil, err
		}
		return a.GetBangumiCalendar()
	case "GetLocalFollows":
		if err := webArgs(args); err != nil {
			return nil, err
		}
		return a.GetLocalFollows(), nil
	case "GetLogs":
		if err := webArgs(args); err != nil {
			return nil, err
		}
		return a.GetLogs(), nil
	case "GetAnimeDetail":
		var id int
		if err := webArgs(args, &id); err != nil {
			return nil, err
		}
		return a.GetAnimeDetail(id)
	case "FollowLocal":
		var item bangumi.Subject
		if err := webArgs(args, &item); err != nil {
			return nil, err
		}
		return a.FollowLocal(item), nil
	case "UnfollowLocal":
		var id int
		if err := webArgs(args, &id); err != nil {
			return nil, err
		}
		return a.UnfollowLocal(id), nil
	case "ToggleEpisodeWatched":
		var id int
		var episode float64
		if err := webArgs(args, &id, &episode); err != nil {
			return nil, err
		}
		return a.ToggleEpisodeWatched(id, episode), nil
	case "SearchEpisodeMagnet":
		var id int
		var episode float64
		if err := webArgs(args, &id, &episode); err != nil {
			return nil, err
		}
		return a.SearchEpisodeMagnet(id, episode), nil
	case "SearchEpisodeMagnetList":
		var id int
		var episode float64
		var keywords string
		if err := webArgs(args, &id, &episode, &keywords); err != nil {
			return nil, err
		}
		return a.SearchEpisodeMagnetList(id, episode, keywords)
	case "SaveEpisodeMagnet":
		var id int
		var episode float64
		var magnet string
		if err := webArgs(args, &id, &episode, &magnet); err != nil {
			return nil, err
		}
		return a.SaveEpisodeMagnet(id, episode, magnet), nil
	case "DownloadEpisode":
		var id int
		var episode float64
		var magnet string
		if err := webArgs(args, &id, &episode, &magnet); err != nil {
			return nil, err
		}
		if a.GetPikPakStatus() != "Success" {
			return nil, errors.New("请先在家里的桌面端登录 PikPak")
		}
		return a.DownloadEpisode(id, episode, magnet), nil
	default:
		return nil, fmt.Errorf("method %q is unavailable on the web", method)
	}
}

func (a *App) handleWebEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	if !sameOrigin(r) {
		http.Error(w, "cross-origin request denied", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	client, unsubscribe := a.webEvents.subscribe()
	defer unsubscribe()
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case message := <-client:
			fmt.Fprintf(w, "data: %s\n\n", message)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (a *App) handleWebImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	u, err := url.Parse(r.URL.Query().Get("u"))
	if err != nil || !webImageURLAllowed(u) {
		http.Error(w, "unsupported image URL", http.StatusBadRequest)
		return
	}
	if a.imgProxy == nil {
		http.Error(w, "image proxy unavailable", http.StatusServiceUnavailable)
		return
	}
	a.imgProxy.ServeImage(w, r, webImageURLAllowed)
}

func webImageURLAllowed(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "bgm.tv" || strings.HasSuffix(host, ".bgm.tv") || host == "bangumi.tv" || strings.HasSuffix(host, ".bangumi.tv")
}
