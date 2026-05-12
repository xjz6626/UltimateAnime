package pikpak

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// RegisterStreamHandler 把 /stream 流式代理注册到指定 mux 上
// 与图片代理共用端口（54321），由 imgproxy 启动监听
func (d *PikPakClient) RegisterStreamHandler(mux *http.ServeMux) {
	mux.HandleFunc("/stream", d.handleStream)
	fmt.Println("🚀 [PikPak] /stream 流式代理已注册")
}

// StartServer 保留旧接口以兼容（独立启动，自带 mux）
// 注意：如果 imgproxy 已经在用 54321 端口，调用此函数会失败
// 推荐用 RegisterStreamHandler 把 handler 挂到 imgproxy 的 mux 上
var serverStarted bool

func (d *PikPakClient) StartServer(port string) {
	if serverStarted {
		fmt.Println("⚠️ 代理服务已启动，跳过初始化")
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/stream", d.handleStream)

	fmt.Printf("🚀 流式代理服务已启动: http://127.0.0.1:%s\n", port)
	serverStarted = true

	go func() {
		if err := http.ListenAndServe(":"+port, mux); err != nil {
			fmt.Printf("❌ 代理服务启动失败: %v\n", err)
		}
	}()
}

func (d *PikPakClient) handleStream(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("📥 [Proxy] 收到请求: %s\n", r.URL.String())

	// 1. 获取文件 ID
	fileID := r.URL.Query().Get("id")
	if fileID == "" {
		http.Error(w, "Missing file_id", http.StatusBadRequest)
		fmt.Println("❌ [Proxy] 缺少 file_id")
		return
	}

	// 2. 获取直链 (PikPak 链接有时效性，每次播放最好重新获取，或者做个简单的缓存)
	// 这里为了响应速度，如果能传入 link 最好，但为了架构解耦，我们现场获取
	// 注意：GetDownloadUrl 内部已经处理了验证码和签名
	downloadLink, err := d.GetDownloadUrl(fileID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get link: %v", err)
		http.Error(w, errMsg, http.StatusInternalServerError)
		fmt.Printf("❌ [Proxy] 获取直链失败: %v\n", err)
		return
	}

	// 安全打印：直链可能短于 50 字符（极端情况），切片要做边界检查
	preview := downloadLink
	if len(preview) > 50 {
		preview = preview[:50] + "..."
	}
	fmt.Printf("🔗 [Proxy] 获取直链成功: %s\n", preview)

	// 3. 构造向 PikPak 的请求
	outReq, err := http.NewRequest(r.Method, downloadLink, r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		fmt.Printf("❌ [Proxy] 创建请求失败: %v\n", err)
		return
	}

	// 4. 🔥 核心：复制 Header (尤其是 Range) 🔥
	// 播放器拖动进度条时，会发送 Range: bytes=1024- 这样的头
	// 我们必须把它透传给 PikPak
	for k, vv := range r.Header {
		for _, v := range vv {
			// Host 头不能复制，否则会 404
			if !strings.EqualFold(k, "Host") {
				outReq.Header.Add(k, v)
			}
		}
	}

	// 5. 🔥 核心：注入特权 User-Agent 🔥
	outReq.Header.Set("User-Agent", d.UserAgent)

	// 6. 准备 HTTP Client (按需走代理)
	transport := &http.Transport{
		ForceAttemptHTTP2: false,
	}
	if d.ProxyAddr != "" {
		if proxyURL, err := url.Parse(d.ProxyAddr); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	// 设置较短的 Header 超时，但 Body 读取不设超时
	client := &http.Client{
		Transport: transport,
		Timeout:   0,
	}

	// 7. 发起请求
	resp, err := client.Do(outReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Request failed: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 8. 复制响应 Header 给播放器
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// 9. 建立管道，传输数据流
	io.Copy(w, resp.Body)
}
