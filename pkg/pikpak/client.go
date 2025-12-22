package pikpak

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url" // 新增
	"os"
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

	var e ErrResp
	req.SetError(&e)
	res, err := req.Execute(method, url)
	if err != nil {
		return nil, err
	}

	if e.ErrorCode != 0 {
		fmt.Printf("⚠️ [PikPak] API Error: %d %s (Action: %s)\n", e.ErrorCode, e.ErrorMsg, method+":"+url)
		if e.ErrorCode == 4122 || e.ErrorCode == 4121 || e.ErrorCode == 16 {
			fmt.Println("🔄 [PikPak] Token expired, trying to relogin...")
			if loginErr := d.Login(); loginErr == nil {
				fmt.Println("✅ [PikPak] Relogin success, retrying request...")
				return d.request(url, method, callback, resp)
			}
			return nil, fmt.Errorf("token: %v", e.Error())
		}
		if e.ErrorCode == 9 {
			fmt.Println("🛡️ [PikPak] Captcha required, trying to refresh token...")
			if refreshErr := d.RefreshCaptchaTokenAtLogin(GetAction(method, url)); refreshErr == nil {
				fmt.Println("✅ [PikPak] Captcha refreshed, retrying request...")
				return d.request(url, method, callback, resp)
			}
		}
		return nil, fmt.Errorf("api error %d: %s", e.ErrorCode, e.ErrorMsg)
	}
	return res.Body(), nil
}

func (d *PikPakClient) TriggerCaptcha(action string, meta map[string]string) error {
	param := CaptchaTokenRequest{
		Action: action, CaptchaToken: d.CaptchaToken, ClientID: AndroidClientID, DeviceID: d.DeviceID, Meta: meta, RedirectUri: "xlaccsdk01://xbase.cloud/callback?state=harbor",
	}
	var e ErrResp
	var resp CaptchaTokenResponse
	_, err := d.Client.R().SetBody(param).SetQueryParam("client_id", AndroidClientID).SetError(&e).SetResult(&resp).SetHeader("User-Agent", d.UserAgent).Post("https://user.mypikpak.com/v1/shield/captcha/init")
	if err != nil {
		return err
	}
	if e.IsError() {
		return errors.New(e.Error())
	}
	if resp.Url != "" {
		return fmt.Errorf("verify: %s", resp.Url)
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

	// 增加日志
	fmt.Printf("Login: TriggerCaptcha action=POST:%s metas=%v\n", url, metas)
	if err := d.TriggerCaptcha("POST:"+url, metas); err != nil {
		fmt.Printf("Login: TriggerCaptcha failed: %v\n", err)
		return err
	}
	fmt.Printf("Login: Got CaptchaToken: %s\n", d.CaptchaToken)

	reqBody := map[string]interface{}{"client_id": AndroidClientID, "client_secret": AndroidClientSecret, "username": d.Username, "password": d.Password, "captcha_token": d.CaptchaToken}
	var e ErrResp
	res, err := d.Client.R().SetError(&e).SetBody(reqBody).SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36").Post(url)
	if err != nil {
		fmt.Printf("Login: Signin request failed: %v\n", err)
		return err
	}
	if e.ErrorCode != 0 {
		fmt.Printf("Login: Signin API error: %d %s\n", e.ErrorCode, e.ErrorMsg)
		return errors.New(e.ErrorMsg)
	}
	d.CaptchaToken = ""

	var result map[string]interface{}
	json.Unmarshal(res.Body(), &result)
	if token, ok := result["access_token"].(string); ok {
		d.AccessToken = token
	}
	if refresh, ok := result["refresh_token"].(string); ok {
		d.RefreshToken = refresh
	}
	if sub, ok := result["sub"].(string); ok {
		d.UserID = sub
		// d.UserAgent = BuildCustomUserAgent(d.DeviceID, AndroidClientID, AndroidPackageName, AndroidSdkVersion, AndroidClientVersion, AndroidPackageName, d.UserID)
	}
	return nil
}

func (d *PikPakClient) FileList(parentID string) ([]File, error) {
	if parentID == "" {
		parentID = ""
	}
	var allFiles []File
	pageToken := ""
	d.CaptchaToken = ""
	for {
		query := map[string]string{"parent_id": parentID, "thumbnail_size": "SIZE_LARGE", "limit": "100", "filters": `{"phase":{"eq":"PHASE_TYPE_COMPLETE"},"trashed":{"eq":false}}`}
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

// 递归删除文件夹及其所有内容
func (d *PikPakClient) DeleteFolderRecursive(folderID string) error {
	// 获取文件夹内容
	files, err := d.FileList(folderID)
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

// ClearStorage 清空云盘空间 (删除根目录下所有文件)
func (d *PikPakClient) ClearStorage() error {
	fmt.Println("🧹 [PikPak] Cleaning up storage...")

	var lastFileCount int = -1

	for {
		// 获取根目录文件
		files, err := d.FileList("")
		if err != nil {
			return fmt.Errorf("list files failed: %v", err)
		}

		// 过滤掉已经在回收站的文件
		var activeFiles []File
		for _, f := range files {
			if !f.Trashed {
				activeFiles = append(activeFiles, f)
			}
		}

		currentFileCount := len(activeFiles)

		// 如果没有文件了，清空完成
		if currentFileCount == 0 {
			fmt.Println("✅ [PikPak] Storage is empty.")
			break
		}

		// 如果文件数量没有变化，说明剩下的都是无法删除的系统文件夹（如 My Pack）
		// 此时认为清空完成
		if lastFileCount == currentFileCount {
			fmt.Printf("ℹ️ [PikPak] Storage cleaned. %d system folders remain (cannot be deleted).\n", currentFileCount)
			break
		}

		lastFileCount = currentFileCount
		fmt.Printf("🗑️ [PikPak] Deleting %d items...\n", len(activeFiles))

		// 逐个删除，文件夹用递归，文件用批量删除
		var fileIDs []string
		for _, f := range activeFiles {
			if f.Kind == "drive#folder" {
				// 文件夹：递归删除所有内容
				fmt.Printf("📁 [PikPak] Recursively deleting folder: %s\n", f.Name)
				if err := d.DeleteFolderRecursive(f.Id); err != nil {
					// 删除失败（如系统文件夹），记录日志但不中断流程
					fmt.Printf("⚠️ [PikPak] Cannot delete folder %s: %v (may be system folder)\n", f.Name, err)
				}
			} else {
				// 文件：收集ID，批量删除
				fileIDs = append(fileIDs, f.Id)
			}
		}

		// 批量删除收集到的文件
		if len(fileIDs) > 0 {
			fmt.Printf("📄 [PikPak] Batch deleting %d files...\n", len(fileIDs))
			if err := d.BatchDelete(fileIDs); err != nil {
				fmt.Printf("⚠️ [PikPak] Batch delete failed: %v\n", err)
				time.Sleep(2 * time.Second)
				continue
			}
		}

		// 稍微等待一下，避免请求过快
		time.Sleep(1 * time.Second)
	}
	return nil
}

func GetAction(method string, url string) string { return method + ":" + url }

// 🔥🔥🔥 DownloadFileConcurrent V3.0 (动态任务池版 - 完整实现) 🔥🔥🔥
func (d *PikPakClient) DownloadFileConcurrent(fileID string, fileName string, fileSize int64, threadNum int, progress func(current, total int64)) error {
	urlStr, err := d.GetDownloadUrl(fileID)
	if err != nil {
		return err
	}

	out, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer out.Close()
	out.Truncate(fileSize)

	if threadNum <= 0 {
		threadNum = 16
	}
	if threadNum > 32 {
		threadNum = 32
	}

	// 1. 切片：固定每块 4MB
	const BlockSize = 4 * 1024 * 1024
	totalBlocks := (fileSize + BlockSize - 1) / BlockSize

	// 2. 任务池
	// 缓冲设大一点，方便重试插队
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

	proxyURL, _ := url.Parse(d.ProxyAddr)

	// 3. 启动工人
	var wg sync.WaitGroup
	for w := 0; w < threadNum; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 复用 client 提高效率
			transport := &http.Transport{
				Proxy:             http.ProxyURL(proxyURL),
				ForceAttemptHTTP2: false,
				MaxIdleConns:      10,
				IdleConnTimeout:   30 * time.Second,
			}
			client := &http.Client{Transport: transport, Timeout: 60 * time.Second}

			for job := range jobs {
				// 执行下载
				req, _ := http.NewRequest("GET", urlStr, nil)
				req.Header.Set("User-Agent", d.UserAgent)
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", job.Start, job.End)) // 每个块最多重试 3 次，如果还不行，扔回大池子
				success := false
				for retry := 0; retry < 3; retry++ {
					// 每次重试前，如果不是第一次，先扣除之前可能已增加的进度
					// 注意：这里的逻辑是，只有在 Read 循环里才会增加进度
					// 如果 Read 了一半失败了，我们在下面会扣除
					// 所以这里不需要预先扣除

					resp, err := client.Do(req)
					if err == nil && (resp.StatusCode == 200 || resp.StatusCode == 206) {
						// 读取数据
						buf := make([]byte, 128*1024)
						var currentOffset = job.Start
						var bytesRead int64 = 0
						expectedBytes := job.End - job.Start + 1
						copyErr := error(nil)

						for {
							n, rErr := resp.Body.Read(buf)
							if n > 0 {
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
								fmt.Printf("⚠️ [Chunk %d] Incomplete download: expected %d, got %d\n", job.Index, expectedBytes, bytesRead)
								progressChan <- -bytesRead // 扣除进度
							}
						} else {
							// Read 过程中报错，也要回滚进度
							progressChan <- -bytesRead
						}
					} else {
						if resp != nil {
							// 打印错误状态码，方便调试
							fmt.Printf("⚠️ [Chunk %d] Download failed: Status %d\n", job.Index, resp.StatusCode)
							resp.Body.Close()
						} else {
							fmt.Printf("⚠️ [Chunk %d] Download failed: %v\n", job.Index, err)
						}
					}
					// 失败休息一下
					time.Sleep(time.Duration(retry+1) * 500 * time.Millisecond)
				}

				if success {
					results <- nil
				} else {
					// 彻底失败，扔回 jobs 通道
					// 注意：这里需要非阻塞或者确保 buffer 够大，否则会死锁
					// 实际上我们 buffer 够大，但在极端的“所有都失败”情况下要注意
					// 这里简单处理：无限重试
					go func(j downloadJob) {
						time.Sleep(2 * time.Second) // 惩罚性延时
						jobs <- j
					}(job)
				}
			}
		}(w)
	}

	// 4. 监控进度
	go func() {
		var totalDownloaded int64 = 0
		for n := range progressChan {
			totalDownloaded += n
			// 防止进度超过 100% (虽然理论上不应该发生，但为了 UI 好看)
			if totalDownloaded > fileSize {
				// totalDownloaded = fileSize // 不要强制修正，方便调试
			}
			if progress != nil {
				progress(totalDownloaded, fileSize)
			}
		}
	}()

	// 5. 等待所有块完成
	for i := int64(0); i < totalBlocks; i++ {
		<-results
	}

	close(jobs)
	wg.Wait()
	close(progressChan)
	return nil
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
	reqJson, _ := json.Marshal(reqData)
	fmt.Printf("🚀 [PikPak] Sending OfflineDownload request: %s\n", string(reqJson))

	// 添加 Panic 捕获，防止程序崩溃
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("🔥 [PikPak] PANIC in OfflineDownload: %v\n", r)
		}
	}()

	_, err := d.request("https://api-drive.mypikpak.com/drive/v1/files", http.MethodPost, func(req *resty.Request) {
		req.SetBody(reqData)
	}, &resp)

	fmt.Printf("🏁 [PikPak] Request finished. Err: %v\n", err)

	if err != nil {
		fmt.Printf("❌ [PikPak] OfflineDownload request failed: %v\n", err)
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

// DeleteTask 删除离线任务
// taskID: 任务 ID
// deleteFile: 是否同时删除源文件 (目前 API 仅支持删除任务记录，此参数暂未生效)
func (d *PikPakClient) DeleteTask(taskID string, deleteFile bool) error {
	// API: DELETE /drive/v1/tasks?task_ids=xxx
	query := map[string]string{
		"task_ids": taskID,
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
