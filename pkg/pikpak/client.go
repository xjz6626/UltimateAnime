package pikpak

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

// 🔥🔥🔥 补全了缺失的结构体定义 🔥🔥🔥
type downloadJob struct {
	Index int64
	Start int64
	End   int64
}

type PikPakClient struct {
	Client       *resty.Client
	Username     string
	Password     string
	AccessToken  string
	RefreshToken string
	DeviceID     string
	UserAgent    string
	CaptchaToken string
	UserID       string
	ProxyAddr    string // 新增：保存代理地址
}

func NewPikPakClient(username, password, proxy string) *PikPakClient {
	client := resty.New()
	client.SetRetryCount(2)
	client.SetTimeout(30 * time.Second)

	// 设置代理
	if proxy != "" {
		client.SetProxy(proxy)
	}

	deviceID := md5Str(username + password)

	return &PikPakClient{
		Client:    client,
		Username:  username,
		Password:  password,
		DeviceID:  deviceID,
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
		ProxyAddr: proxy, // 保存下来给下载器用
	}
}

func (d *PikPakClient) request(url string, method string, callback func(req *resty.Request), resp interface{}) ([]byte, error) {
	authRetried, captchaRetried := false, false
	for {
		req := d.Client.R()

		// 动态 User-Agent 逻辑 (复刻 Python get_headers)
		// 如果有 CaptchaToken，则使用 Android UA；否则使用默认 (Chrome) UA
		ua := d.UserAgent
		if d.CaptchaToken != "" {
			ua = BuildCustomUserAgent(d.DeviceID, AndroidClientID, AndroidPackageName, AndroidSdkVersion, AndroidClientVersion, AndroidPackageName, d.UserID)
		}

		req.SetHeaders(map[string]string{
			"User-Agent":   ua,
			"X-Device-ID":  d.DeviceID,
			"Content-Type": "application/json; charset=utf-8",
		})
		if d.CaptchaToken != "" {
			req.SetHeader("X-Captcha-Token", d.CaptchaToken)
		}
		if d.AccessToken != "" {
			req.SetHeader("Authorization", "Bearer "+d.AccessToken)
		}
		if callback != nil {
			callback(req)
		}
		if resp != nil {
			req.SetResult(resp)
		}

		res, err := req.Execute(method, url)
		if err != nil {
			return nil, err
		}
		var apiError ErrResp
		_ = json.Unmarshal(res.Body(), &apiError)
		if !res.IsError() && !apiError.IsError() {
			return res.Body(), nil
		}
		if (apiError.ErrorCode == 4122 || apiError.ErrorCode == 4121 || apiError.ErrorCode == 16) && !authRetried {
			authRetried = true
			if loginErr := d.Login(); loginErr != nil {
				return nil, fmt.Errorf("PikPak 重新登录失败: %w", loginErr)
			}
			continue
		}
		if apiError.ErrorCode == 9 && !captchaRetried {
			captchaRetried = true
			if captchaErr := d.RefreshCaptchaTokenAtLogin(GetAction(method, url)); captchaErr != nil {
				return nil, fmt.Errorf("PikPak 验证码刷新失败: %w", captchaErr)
			}
			continue
		}
		if apiError.IsError() {
			return nil, fmt.Errorf("api error %d: %s %s", apiError.ErrorCode, apiError.ErrorMsg, apiError.ErrorDescription)
		}
		return nil, fmt.Errorf("PikPak 接口返回 HTTP %d", res.StatusCode())
	}
}

func (d *PikPakClient) TriggerCaptcha(action string, meta map[string]string) error {
	param := CaptchaTokenRequest{
		Action: action, CaptchaToken: d.CaptchaToken, ClientID: AndroidClientID, DeviceID: d.DeviceID, Meta: meta, RedirectUri: "xlaccsdk01://xbase.cloud/callback?state=harbor",
	}
	var e ErrResp
	var resp CaptchaTokenResponse
	response, err := d.Client.R().SetBody(param).SetQueryParam("client_id", AndroidClientID).SetError(&e).SetResult(&resp).SetHeader("User-Agent", d.UserAgent).Post("https://user.mypikpak.com/v1/shield/captcha/init")
	if err != nil {
		return err
	}
	_ = json.Unmarshal(response.Body(), &e)
	if response.IsError() && !e.IsError() {
		return fmt.Errorf("PikPak 验证码接口返回 HTTP %d", response.StatusCode())
	}
	if e.IsError() {
		return errors.New(e.Error())
	}
	if resp.Url != "" {
		return fmt.Errorf("verify: %s", resp.Url)
	}
	if resp.CaptchaToken == "" {
		return errors.New("PikPak 验证码响应缺少 captcha_token")
	}
	d.CaptchaToken = resp.CaptchaToken
	return nil
}

func (d *PikPakClient) RefreshCaptchaTokenAtLogin(action string) error {
	ts, sig := GetCaptchaSign(AndroidClientID, AndroidClientVersion, AndroidPackageName, d.DeviceID)
	metas := map[string]string{"client_version": AndroidClientVersion, "package_name": AndroidPackageName, "user_id": d.UserID, "timestamp": ts, "captcha_sign": sig}
	return d.TriggerCaptcha(action, metas)
}

func (d *PikPakClient) Login() error {
	url := "https://user.mypikpak.com/v1/auth/signin"
	metas := map[string]string{}
	if strings.Contains(d.Username, "@") {
		metas["email"] = d.Username
	} else {
		metas["username"] = d.Username
	}
	d.CaptchaToken = ""

	if err := d.TriggerCaptcha("POST:"+url, metas); err != nil {
		return fmt.Errorf("获取 PikPak 登录验证码失败: %w", err)
	}

	reqBody := map[string]interface{}{"client_id": AndroidClientID, "client_secret": AndroidClientSecret, "username": d.Username, "password": d.Password, "captcha_token": d.CaptchaToken}
	var e ErrResp
	res, err := d.Client.R().SetError(&e).SetBody(reqBody).SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36").Post(url)
	if err != nil {
		return err
	}
	_ = json.Unmarshal(res.Body(), &e)
	if e.IsError() {
		return fmt.Errorf("PikPak 登录失败: %s %s", e.ErrorMsg, e.ErrorDescription)
	}
	if res.IsError() {
		return fmt.Errorf("PikPak 登录接口返回 HTTP %d", res.StatusCode())
	}
	d.CaptchaToken = ""

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		UserID       string `json:"sub"`
	}
	if err := json.Unmarshal(res.Body(), &result); err != nil {
		return fmt.Errorf("解析 PikPak 登录响应失败: %w", err)
	}
	if result.AccessToken == "" {
		return errors.New("PikPak 登录响应缺少 access_token")
	}
	d.AccessToken = result.AccessToken
	d.RefreshToken = result.RefreshToken
	d.UserID = result.UserID
	return nil
}

func (d *PikPakClient) FileList(parentID string) ([]File, error) {
	return d.listFiles(parentID, `{"phase":{"eq":"PHASE_TYPE_COMPLETE"},"trashed":{"eq":false}}`)
}

func (d *PikPakClient) listActiveFiles(parentID string) ([]File, error) {
	return d.listFiles(parentID, `{"trashed":{"eq":false}}`)
}

func (d *PikPakClient) listFiles(parentID, filters string) ([]File, error) {
	var allFiles []File
	pageToken := ""
	d.CaptchaToken = ""
	for {
		query := map[string]string{"parent_id": parentID, "thumbnail_size": "SIZE_LARGE", "limit": "100", "filters": filters}
		if pageToken != "" {
			query["page_token"] = pageToken
		}
		var resp Files
		_, err := d.request("https://api-drive.mypikpak.com/drive/v1/files", http.MethodGet, func(req *resty.Request) { req.SetQueryParams(query) }, &resp)
		if err != nil {
			return nil, err
		}
		allFiles = append(allFiles, resp.Files...)
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return allFiles, nil
}

// IsStorageFull 查询云盘容量。limit 为 0 时表示没有明确的容量上限。
func (d *PikPakClient) IsStorageFull() (bool, error) {
	body, err := d.request("https://api-drive.mypikpak.com/drive/v1/about", http.MethodGet, nil, nil)
	if err != nil {
		return false, err
	}
	var result struct {
		Quota struct {
			Limit json.RawMessage `json:"limit"`
			Usage json.RawMessage `json:"usage"`
		} `json:"quota"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}
	parseQuota := func(raw json.RawMessage) (int64, error) {
		if len(raw) == 0 {
			return 0, errors.New("PikPak 容量响应缺少必要字段")
		}
		return strconv.ParseInt(strings.Trim(string(raw), `"`), 10, 64)
	}
	limit, err := parseQuota(result.Quota.Limit)
	if err != nil {
		return false, err
	}
	usage, err := parseQuota(result.Quota.Usage)
	if err != nil {
		return false, err
	}
	return limit > 0 && usage >= limit, nil
}

func (d *PikPakClient) GetDownloadUrl(fileID string) (string, error) {
	action := fmt.Sprintf("GET:/drive/v1/files/%s", fileID)
	if err := d.RefreshCaptchaTokenAtLogin(action); err != nil {
		return "", err
	}
	var resp File
	_, err := d.request("https://api-drive.mypikpak.com/drive/v1/files/"+fileID, http.MethodGet, func(req *resty.Request) {
		req.SetQueryParams(map[string]string{"usage": "FETCH", "thumbnail_size": "SIZE_LARGE"})
	}, &resp)
	d.CaptchaToken = ""
	if err != nil {
		return "", err
	}
	return resp.WebContentLink, nil
}

func (d *PikPakClient) BatchTrash(fileIDs []string) error {
	d.CaptchaToken = ""
	// 使用用户提供的验证过的 API 逻辑
	_, err := d.request("https://api-drive.mypikpak.com/drive/v1/files:batchTrash", http.MethodPost, func(req *resty.Request) { req.SetBody(map[string]interface{}{"ids": fileIDs}) }, nil)
	return err
}

func (d *PikPakClient) BatchDelete(fileIDs []string) error {
	d.CaptchaToken = ""
	_, err := d.request("https://api-drive.mypikpak.com/drive/v1/files:batchDelete", http.MethodPost, func(req *resty.Request) { req.SetBody(map[string]interface{}{"ids": fileIDs}) }, nil)
	return err
}

func (d *PikPakClient) EmptyTrash() error {
	d.CaptchaToken = ""
	_, err := d.request("https://api-drive.mypikpak.com/drive/v1/files/trash:empty", http.MethodPatch, nil, nil)
	return err
}

// 递归删除文件夹及其所有内容
func (d *PikPakClient) DeleteFolderRecursive(folderID string) error {
	// 获取文件夹内容
	files, err := d.listActiveFiles(folderID)
	if err != nil {
		return err
	}

	// 遍历删除所有子项
	for _, file := range files {
		if file.Kind == "drive#folder" {
			// 递归删除子文件夹
			if err := d.DeleteFolderRecursive(file.Id); err != nil {
				return err
			}
		} else {
			// 删除文件
			if err := d.BatchDelete([]string{file.Id}); err != nil {
				return err
			}
		}
	}

	// 删除文件夹本身
	return d.BatchDelete([]string{folderID})
}

func (d *PikPakClient) folderContainsFiles(folderID string) (bool, error) {
	children, err := d.listActiveFiles(folderID)
	if err != nil {
		return false, err
	}
	for _, child := range children {
		if child.Kind != "drive#folder" {
			return true, nil
		}
		hasFiles, err := d.folderContainsFiles(child.Id)
		if err != nil || hasFiles {
			return hasFiles, err
		}
	}
	return false, nil
}

// ClearStorage 永久删除云盘文件，并复查是否只剩空的系统文件夹。
func (d *PikPakClient) ClearStorage() error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		files, err := d.listActiveFiles("")
		if err != nil {
			return fmt.Errorf("获取云盘文件失败: %w", err)
		}
		if len(files) == 0 {
			return d.EmptyTrash()
		}

		var fileIDs []string
		for _, file := range files {
			if file.Kind == "drive#folder" {
				if err := d.DeleteFolderRecursive(file.Id); err != nil {
					lastErr = err // 系统文件夹可能无法删除，下面检查其中是否仍有文件。
				}
			} else {
				fileIDs = append(fileIDs, file.Id)
			}
		}
		if len(fileIDs) > 0 {
			if err := d.BatchDelete(fileIDs); err != nil {
				lastErr = err
			}
		}

		time.Sleep(time.Second)
		remaining, err := d.listActiveFiles("")
		if err != nil {
			return fmt.Errorf("复查云盘文件失败: %w", err)
		}
		hasFiles := false
		for _, file := range remaining {
			if file.Kind != "drive#folder" {
				hasFiles = true
				break
			}
			folderHasFiles, err := d.folderContainsFiles(file.Id)
			if err != nil {
				return fmt.Errorf("复查文件夹失败: %w", err)
			}
			if folderHasFiles {
				hasFiles = true
				break
			}
		}
		if !hasFiles {
			return d.EmptyTrash()
		}
	}
	if lastErr != nil {
		return fmt.Errorf("云盘清理后仍有文件: %w", lastErr)
	}
	return errors.New("云盘清理后仍有文件")
}

func GetAction(method string, rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed.Scheme != "" {
		return method + ":" + parsed.EscapedPath()
	}
	return method + ":" + rawURL
}

// DownloadFileConcurrent 并发下载分块，全部完成后替换目标文件。
func (d *PikPakClient) DownloadFileConcurrent(fileID string, fileName string, fileSize int64, threadNum int, progress func(current, total int64)) error {
	return d.downloadFileConcurrent(fileID, fileName, fileSize, threadNum, progress, downloadOptions{})
}

type downloadOptions struct {
	rangeTransport http.RoundTripper
	retryWait      func(time.Duration)
}

func (d *PikPakClient) downloadFileConcurrent(fileID string, fileName string, fileSize int64, threadNum int, progress func(current, total int64), options downloadOptions) error {
	if fileSize <= 0 {
		return fmt.Errorf("PikPak 文件大小无效: %d", fileSize)
	}
	urlStr, err := d.GetDownloadUrl(fileID)
	if err != nil {
		return err
	}
	if urlStr == "" {
		return errors.New("PikPak 未返回文件下载地址")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var linkMu sync.Mutex
	var linkVersion uint64
	currentLink := func() (string, uint64) {
		linkMu.Lock()
		defer linkMu.Unlock()
		return urlStr, linkVersion
	}
	refreshLink := func(staleVersion uint64) error {
		linkMu.Lock()
		defer linkMu.Unlock()
		if linkVersion != staleVersion {
			return nil
		}
		link, err := d.GetDownloadUrl(fileID)
		if err != nil {
			return err
		}
		if link == "" {
			return errors.New("PikPak 未返回新的文件下载地址")
		}
		urlStr = link
		linkVersion++
		return nil
	}
	retryWait := options.retryWait
	if retryWait == nil {
		retryWait = func(delay time.Duration) {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
			}
		}
	}
	var proxyFunc func(*http.Request) (*url.URL, error)
	if d.ProxyAddr != "" {
		proxyURL, err := url.Parse(d.ProxyAddr)
		if err != nil || proxyURL.Scheme == "" || proxyURL.Host == "" {
			return fmt.Errorf("无效的代理地址: %q", d.ProxyAddr)
		}
		proxyFunc = http.ProxyURL(proxyURL)
	}

	out, err := os.CreateTemp(filepath.Dir(fileName), "."+filepath.Base(fileName)+".*.part")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())
	defer out.Close()
	if err := out.Truncate(fileSize); err != nil {
		return err
	}

	if threadNum <= 0 {
		threadNum = 16
	}
	if threadNum > 32 {
		threadNum = 32
	}

	// 1. 切片：固定每块 4MB
	const BlockSize = 4 * 1024 * 1024
	totalBlocks := (fileSize + BlockSize - 1) / BlockSize
	if int64(threadNum) > totalBlocks {
		threadNum = int(totalBlocks)
	}

	// 2. 任务池
	// 每个分块只入队一次，失败时在工作线程中有限重试。
	jobs := make(chan downloadJob, totalBlocks+100)
	results := make(chan error, totalBlocks)
	progressChan := make(chan int64, 2000)

	// 初始填装任务
	for i := int64(0); i < totalBlocks; i++ {
		start := i * BlockSize
		end := start + BlockSize - 1
		if end >= fileSize {
			end = fileSize - 1
		}
		jobs <- downloadJob{Index: i, Start: start, End: end}
	}

	// 3. 启动工人
	var wg sync.WaitGroup
	for w := 0; w < threadNum; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// 复用 client 提高效率
			transport := &http.Transport{
				Proxy:             proxyFunc,
				ForceAttemptHTTP2: false,
				MaxIdleConns:      10,
				IdleConnTimeout:   30 * time.Second,
			}
			defer transport.CloseIdleConnections()
			var requestTransport http.RoundTripper = transport
			if options.rangeTransport != nil {
				requestTransport = options.rangeTransport
			}
			client := &http.Client{Transport: requestTransport, Timeout: 60 * time.Second}

			for job := range jobs {
				if err := ctx.Err(); err != nil {
					results <- err
					continue
				}
				success := false
				var lastErr error
				var refreshErr error
				attempts := 0
				for retry := 0; retry < 10; retry++ {
					if err := ctx.Err(); err != nil {
						lastErr = err
						break
					}
					link, linkVersion := currentLink()
					req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
					if reqErr != nil {
						lastErr = fmt.Errorf("分块 %d 下载地址无效: %w", job.Index, reqErr)
						break
					}
					req.Header.Set("User-Agent", d.UserAgent)
					req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", job.Start, job.End))
					attempts++
					resp, err := client.Do(req)
					if err == nil && (resp.StatusCode == 206 || (totalBlocks == 1 && resp.StatusCode == 200)) {
						// 读取数据
						buf := make([]byte, 128*1024)
						var currentOffset = job.Start
						var bytesRead int64 = 0
						expectedBytes := job.End - job.Start + 1
						copyErr := error(nil)

						for {
							n, rErr := resp.Body.Read(buf)
							if n > 0 {
								if bytesRead+int64(n) > expectedBytes {
									copyErr = fmt.Errorf("分块 %d 返回的数据超出预期", job.Index)
									break
								}
								// 写入文件
								_, wErr := out.WriteAt(buf[:n], currentOffset)
								if wErr != nil {
									copyErr = wErr
									break
								}
								currentOffset += int64(n)
								bytesRead += int64(n)
								progressChan <- int64(n)
							}
							if rErr != nil {
								if rErr != io.EOF {
									copyErr = rErr
								}
								break
							}
						}
						resp.Body.Close()

						// 关键修复：检查下载的字节数是否符合预期
						if copyErr == nil {
							if bytesRead == expectedBytes {
								success = true
								break // 成功，跳出重试循环
							} else {
								// 下载不完整，视为失败，回滚进度
								lastErr = fmt.Errorf("分块 %d 下载不完整: 预期 %d 字节，收到 %d 字节", job.Index, expectedBytes, bytesRead)
								progressChan <- -bytesRead // 扣除进度
							}
						} else {
							// Read 过程中报错，也要回滚进度
							lastErr = copyErr
							progressChan <- -bytesRead
						}
					} else {
						status := 0
						if resp != nil {
							status = resp.StatusCode
							lastErr = fmt.Errorf("分块 %d 下载返回 HTTP %d", job.Index, resp.StatusCode)
							resp.Body.Close()
						} else {
							lastErr = fmt.Errorf("分块 %d 下载失败: %w", job.Index, err)
						}
						if (status == 200 && totalBlocks > 1) || (status >= 400 && status < 500 && status != 401 && status != 403 && status != 404 && status != 429) {
							break
						}
						if ((status == 401 || status == 403 || status == 404) && (retry == 0 || retry == 3 || retry == 6)) || ((retry == 2 || retry == 5 || retry == 8) && (status == 429 || status >= 500)) {
							if err := refreshLink(linkVersion); err != nil {
								refreshErr = err
							}
						}
					}
					if retry < 9 {
						delay := time.Duration(1<<retry) * 500 * time.Millisecond
						if delay > 5*time.Second {
							delay = 5 * time.Second
						}
						retryWait(delay)
					}
				}

				if success {
					results <- nil
				} else {
					if attempts > 1 {
						lastErr = fmt.Errorf("分块 %d 尝试 %d 次后仍失败: %w", job.Index, attempts, lastErr)
					}
					if refreshErr != nil {
						lastErr = errors.Join(lastErr, fmt.Errorf("刷新下载地址失败: %w", refreshErr))
					}
					results <- lastErr
					cancel()
				}
			}
		}()
	}

	// 4. 监控进度
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		var totalDownloaded int64 = 0
		for n := range progressChan {
			totalDownloaded += n
			if progress != nil {
				progress(totalDownloaded, fileSize)
			}
		}
	}()

	// 5. 等待所有块完成
	var downloadErr error
	for i := int64(0); i < totalBlocks; i++ {
		if resultErr := <-results; resultErr != nil && downloadErr == nil {
			downloadErr = resultErr
		}
	}

	close(jobs)
	wg.Wait()
	close(progressChan)
	<-progressDone
	if downloadErr != nil {
		return downloadErr
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(out.Name(), fileName)
}

// OfflineDownload 添加离线下载任务
// magnetOrUrl: 磁力链或 HTTP 链接
// parentID: 目标文件夹 ID (传 "" 则默认存入云盘根目录/下载目录)
// fileName: 自定义文件名 (传 "" 则自动识别)
func (d *PikPakClient) OfflineDownload(magnetOrUrl string, parentID string, fileName string) (*OfflineTask, error) {
	// 1. 构造请求体
	var namePtr *string
	if fileName != "" {
		namePtr = &fileName
	}
	var parentPtr *string
	if parentID != "" {
		parentPtr = &parentID
	}

	reqData := OfflineDownloadReq{
		Kind:       "drive#file",
		UploadType: "UPLOAD_TYPE_URL",
		Url:        map[string]string{"url": magnetOrUrl},
		Name:       namePtr,
		ParentID:   parentPtr,
	}

	// 🔥 关键逻辑复刻 (参考 Python 源码):
	// 只有在不指定父目录时，folder_type 才是 "DOWNLOAD"
	// 指定了父目录（比如 "mypak" 文件夹）后，folder_type 必须为空，否则报错或乱飞
	if parentID == "" {
		reqData.FolderType = "DOWNLOAD"
	} else {
		reqData.FolderType = ""
	}

	var resp OfflineDownloadResp

	// 2. 发送请求
	_, err := d.request("https://api-drive.mypikpak.com/drive/v1/files", http.MethodPost, func(req *resty.Request) {
		req.SetBody(reqData)
	}, &resp)

	if err != nil {
		return nil, err
	}

	// 3. 检查任务状态
	// 安全地提取 File ID (因为 File 现在是 interface{})
	var respFileID, respFileName string
	if resp.File != nil {
		if fMap, ok := resp.File.(map[string]interface{}); ok {
			if id, ok := fMap["id"].(string); ok {
				respFileID = id
			}
			if name, ok := fMap["name"].(string); ok {
				respFileName = name
			}
		}
	}

	fmt.Printf("📥 [PikPak] OfflineDownload response: TaskID=%s, FileID=%s\n", resp.Task.ID, respFileID)

	// 如果秒传（文件已存在），PikPak 可能直接返回 File 信息而 Task 为空
	// 这种情况下我们视为成功，伪造一个 Completed 任务返回
	if resp.Task.ID == "" {
		if respFileID != "" {
			return &OfflineTask{
				Phase:    "PHASE_TYPE_COMPLETE",
				Message:  "Instant upload success",
				FileID:   respFileID,
				FileName: respFileName,
			}, nil
		}
		return nil, fmt.Errorf("task creation failed, no task id returned")
	}

	return &resp.Task, nil
} // OfflineList 获取离线任务列表
// includeDone: false=只看进行中/出错, true=包含已完成
func (d *PikPakClient) OfflineList(includeDone bool) ([]OfflineTask, error) {
	var allTasks []OfflineTask
	pageToken := ""

	// 🔥 关键逻辑复刻: 构造状态过滤器
	// Python 版逻辑: filters={"phase": {"in": "RUNNING,ERROR..."}}
	phases := []string{"PHASE_TYPE_RUNNING", "PHASE_TYPE_ERROR", "PHASE_TYPE_PENDING"}
	if includeDone {
		phases = append(phases, "PHASE_TYPE_COMPLETE")
	}

	// 手动构造 JSON 字符串 (比定义结构体更轻量)
	filtersStr := fmt.Sprintf(`{"phase":{"in":"%s"}}`, strings.Join(phases, ","))

	for {
		query := map[string]string{
			"type":           "offline",
			"thumbnail_size": "SIZE_SMALL",
			"limit":          "100",
			"filters":        filtersStr,           // 必传：筛选状态
			"with":           "reference_resource", // 必传：获取关联文件 Hash/ID
		}
		if pageToken != "" {
			query["page_token"] = pageToken
		}

		var resp OfflineListResp
		_, err := d.request("https://api-drive.mypikpak.com/drive/v1/tasks", http.MethodGet, func(req *resty.Request) {
			req.SetQueryParams(query)
		}, &resp)

		if err != nil {
			return nil, err
		}

		allTasks = append(allTasks, resp.Tasks...)

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return allTasks, nil
}

// DeleteTask 删除离线任务；deleteFile 控制是否同时删除关联文件。
func (d *PikPakClient) DeleteTask(taskID string, deleteFile bool) error {
	query := map[string]string{
		"task_ids":     taskID,
		"delete_files": strconv.FormatBool(deleteFile),
	}

	_, err := d.request("https://api-drive.mypikpak.com/drive/v1/tasks", http.MethodDelete, func(req *resty.Request) {
		req.SetQueryParams(query)
	}, nil)

	return err
}

// GetFile 获取文件信息
func (d *PikPakClient) GetFile(fileID string) (*File, error) {
	var resp File
	_, err := d.request("https://api-drive.mypikpak.com/drive/v1/files/"+fileID, http.MethodGet, func(req *resty.Request) {
		req.SetQueryParams(map[string]string{"usage": "FETCH", "thumbnail_size": "SIZE_LARGE"})
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
