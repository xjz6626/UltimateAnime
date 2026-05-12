package imgproxy

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Proxy 图片代理服务，独立于 PikPak，应用启动时就跑
type Proxy struct {
	cacheDir  string
	proxyAddr string
	mux       *http.ServeMux
	mu        sync.RWMutex
	started   bool
}

// New 创建一个图片代理
func New(cacheDir string) *Proxy {
	_ = os.MkdirAll(cacheDir, 0755)
	return &Proxy{
		cacheDir: cacheDir,
		mux:      http.NewServeMux(),
	}
}

// Mux 返回内部 ServeMux，允许其他服务（如 PikPak 流式代理）注册路径
func (p *Proxy) Mux() *http.ServeMux {
	return p.mux
}

// SetProxy 动态更新上游代理地址（用户在设置里改了代理后调用）
func (p *Proxy) SetProxy(addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.proxyAddr = addr
	fmt.Printf("🖼️  [ImgProxy] 上游代理已更新: %s\n", addr)
}

// Start 在指定端口启动 HTTP 服务（用独立的 ServeMux，不污染全局）
func (p *Proxy) Start(port string) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return nil
	}
	p.started = true
	p.mu.Unlock()

	p.mux.HandleFunc("/img", p.handle)
	p.mux.HandleFunc("/img/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	addr := "127.0.0.1:" + port
	go func() {
		fmt.Printf("🖼️  [ImgProxy] 服务启动: http://%s/img\n", addr)
		if err := http.ListenAndServe(addr, p.mux); err != nil {
			fmt.Printf("❌ [ImgProxy] 启动失败: %v\n", err)
		}
	}()
	return nil
}

func (p *Proxy) handle(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("u")
	if raw == "" {
		http.Error(w, "missing u", http.StatusBadRequest)
		return
	}

	// 1. 本地缓存命中？
	sum := md5.Sum([]byte(raw))
	hash := hex.EncodeToString(sum[:])
	ext := ".jpg"
	if i := strings.LastIndex(raw, "."); i > 0 && len(raw)-i <= 5 {
		ext = raw[i:]
		// 去掉 query string
		if q := strings.Index(ext, "?"); q > 0 {
			ext = ext[:q]
		}
	}
	cachePath := filepath.Join(p.cacheDir, hash+ext)

	if data, err := os.ReadFile(cachePath); err == nil && len(data) > 0 {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Header().Set("Content-Type", guessContentType(ext))
		w.Write(data)
		return
	}

	// 2. 准备 HTTP client，按需走上游代理
	p.mu.RLock()
	upstream := p.proxyAddr
	p.mu.RUnlock()

	transport := &http.Transport{
		ForceAttemptHTTP2: false,
	}
	if upstream != "" {
		if proxyURL, err := url.Parse(upstream); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}

	// 3. 拉图
	req, err := http.NewRequest("GET", raw, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://bgm.tv/")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ [ImgProxy] 拉取失败 %s: %v\n", raw, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, resp.Status, resp.StatusCode)
		return
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. 写缓存（异步，避免阻塞响应）
	go func(path string, b []byte) {
		_ = os.WriteFile(path, b, 0644)
	}(cachePath, data)

	// 5. 返回给浏览器
	w.Header().Set("Cache-Control", "public, max-age=604800")
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", guessContentType(ext))
	}
	w.Write(data)
}

func guessContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
