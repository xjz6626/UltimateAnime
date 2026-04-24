package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	// 统一使用 UltimateAnime 作为模块前缀
	"UltimateAnime/pkg/bangumi"
	"UltimateAnime/pkg/config"
	"UltimateAnime/pkg/crawler"
	"UltimateAnime/pkg/pikpak"
)

// App application struct
type App struct {
	ctx                 context.Context
	configMgr           *config.Manager
	pikpakClient        *pikpak.PikPakClient
	bangumiClient       *bangumi.BangumiClient
	crawler             *crawler.Crawler
	logHistory          []map[string]string // 日志历史
	currentAccountIndex int                 // 当前使用的账号索引
	blockedAccounts     map[string]string   // 账号封禁状态 map[username]date (YYYY-MM-DD)
}

var windowsPathReplacer = strings.NewReplacer(
	"<", "_",
	">", "_",
	":", "_",
	"\"", "_",
	"/", "_",
	"\\", "_",
	"|", "_",
	"?", "_",
	"*", "_",
)

var windowsReservedNames = map[string]struct{}{
	"CON":  {},
	"PRN":  {},
	"AUX":  {},
	"NUL":  {},
	"COM1": {},
	"COM2": {},
	"COM3": {},
	"COM4": {},
	"COM5": {},
	"COM6": {},
	"COM7": {},
	"COM8": {},
	"COM9": {},
	"LPT1": {},
	"LPT2": {},
	"LPT3": {},
	"LPT4": {},
	"LPT5": {},
	"LPT6": {},
	"LPT7": {},
	"LPT8": {},
	"LPT9": {},
}

func sanitizePathSegment(name string) string {
	cleaned := strings.TrimSpace(name)
	cleaned = windowsPathReplacer.Replace(cleaned)
	cleaned = strings.TrimRight(cleaned, ". ")
	if cleaned == "" {
		return "unnamed"
	}

	if _, isReserved := windowsReservedNames[strings.ToUpper(cleaned)]; isReserved {
		cleaned = "_" + cleaned
	}

	return cleaned
}

// NewApp creates a new App application struct
func NewApp() *App {
	// 1. 初始化配置管理器 (自动读取 config.json)
	cfgMgr := config.NewManager()

	// 2. 初始化 Bangumi 客户端
	// 从配置中读取 Token (复刻 bangumi_api.py 的逻辑)
	bgmToken := cfgMgr.Data.GlobalSettings.BangumiApiToken
	// proxy := cfgMgr.Data.GlobalSettings.Proxy
	// 用户反馈 Bangumi API 走代理可能超时，强制不使用代理
	bgmClient := bangumi.NewClient(bgmToken, "")

	// 3. 初始化资源爬虫 (复刻 search_torrents.py 的逻辑)
	// 从配置中读取 torrent_api_url (默认 api.animes.garden)
	torrentApiUrl := cfgMgr.Data.GlobalSettings.TorrentApiUrl
	// 用户反馈搜索走代理会导致搜不到中文资源，因此这里强制不使用代理
	crawlerClient := crawler.NewCrawler(torrentApiUrl, "")

	return &App{
		configMgr:       cfgMgr,
		bangumiClient:   bgmClient,
		crawler:         crawlerClient,
		logHistory:      make([]map[string]string, 0),
		blockedAccounts: make(map[string]string),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// ⚡ 自动登录 PikPak (如果 config.json 里填了且开启了自动登录)
	users := a.configMgr.Data.GlobalSettings.PikPakUsers
	password := a.configMgr.Data.GlobalSettings.PikPakPassword
	autoLogin := a.configMgr.Data.GlobalSettings.AutoLogin

	if len(users) > 0 && password != "" && autoLogin {
		// 异步登录，防止卡住启动画面
		go func() {
			for i, user := range users {
				fmt.Printf("⚡ [AutoLogin] 尝试账号 (%d/%d): %s ...\n", i+1, len(users), user)
				res := a.Login(user, password)
				if res == "Success" {
					fmt.Printf("✅ [AutoLogin] 账号 %s 登录成功！\n", user)
					return
				}
				fmt.Printf("❌ [AutoLogin] 账号 %s 登录失败: %s，尝试下一个...\n", user, res)
				// 稍微等待一下，避免请求过快
				time.Sleep(1 * time.Second)
			}
			fmt.Println("❌ [AutoLogin] 所有账号均登录失败，请检查配置或手动登录。")
		}()
	} else {
		fmt.Println("ℹ️ [AutoLogin] 自动登录已关闭或未配置账号")
	}
}

// --- 1. 用户认证 & 基础功能 ---

// Login 登录 PikPak (前端调用)
func (a *App) Login(username, password string) string {
	fmt.Printf("💻 登录请求: %s\n", username)

	// 初始化 PikPak 客户端
	proxy := a.configMgr.Data.GlobalSettings.Proxy
	client := pikpak.NewPikPakClient(username, password, proxy)

	// 尝试登录
	if err := client.Login(); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	// 登录成功，保存实例
	a.pikpakClient = client
	a.Log("SUCCESS", "PikPak 登录成功")
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "pikpak-status", "Success")
	}

	// 🔥 启动本地流式代理 (固定端口 54321，供前端播放器使用)
	client.StartServer("54321")

	return "Success"
}

// SaveBangumiToken 保存 Bangumi Token 到配置文件
func (a *App) SaveBangumiToken(token string, uid string) string {
	if err := a.configMgr.SetBangumiToken(token, uid); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	// 实时更新当前客户端状态，无需重启
	a.bangumiClient.SetToken(token)

	return "Success"
}

// --- 2. 资源管理 (PikPak) ---

// GetFileList 获取网盘文件列表
func (a *App) GetFileList(parentID string) ([]pikpak.File, error) {
	fmt.Printf("📂 [PikPak] 获取文件列表, ParentID: %s\n", parentID)
	if a.pikpakClient == nil {
		return nil, fmt.Errorf("请先登录 PikPak")
	}
	return a.pikpakClient.FileList(parentID)
}

// GetPlayLink 获取播放链接 (返回本地代理地址)
func (a *App) GetPlayLink(fileID string) string {
	fmt.Printf("▶️ [PikPak] 获取播放链接: %s\n", fileID)
	// 直接返回流式代理地址，前端 <video> 标签可用
	return fmt.Sprintf("http://127.0.0.1:54321/stream?id=%s", fileID)
}

// AddTask 添加离线下载任务 (磁力/URL)
func (a *App) AddTask(magnet string) string {
	if a.pikpakClient == nil {
		return "Error: 请先登录 PikPak"
	}

	fmt.Printf("⬇️ [PikPak] 添加离线任务: %s\n", magnet)
	// parentID 传空字符串，默认存入云盘根目录
	task, err := a.pikpakClient.OfflineDownload(magnet, "", "")
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	// 返回格式: Success|任务ID|文件名
	return fmt.Sprintf("Success|%s|%s", task.ID, task.Name)
}

// GetTasks 获取离线任务列表 (用于前端进度条)
func (a *App) GetTasks() ([]pikpak.OfflineTask, error) {
	// fmt.Println("🔄 [PikPak] 刷新任务列表") // 轮询太频繁，暂不打印
	if a.pikpakClient == nil {
		return nil, fmt.Errorf("not logged in")
	}
	// false = 只获取进行中/出错的任务，不看已完成的历史
	return a.pikpakClient.OfflineList(false)
}

// DeleteTask 删除任务
func (a *App) DeleteTask(taskID string) string {
	fmt.Printf("🗑️ [PikPak] 删除任务: %s\n", taskID)
	if a.pikpakClient == nil {
		return "Error: Not logged in"
	}
	// true 表示同时删除源文件，false 表示只删除任务记录
	if err := a.pikpakClient.DeleteTask(taskID, false); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return "Success"
}

// --- 3. 动漫信息 (Bangumi API) ---

// GetBangumiCalendar 获取新番日历
func (a *App) GetBangumiCalendar() ([]bangumi.CalendarItem, error) {
	fmt.Println("📅 [Bangumi] 获取新番日历...")
	return a.bangumiClient.GetCalendar()
}

// SearchBangumi 搜索番剧信息
func (a *App) SearchBangumi(keyword string) ([]bangumi.Subject, error) {
	fmt.Printf("🔍 [Bangumi] 搜索番剧: %s\n", keyword)
	return a.bangumiClient.SearchSubject(keyword)
}

// GetMyCollection 获取我的在看列表 (需要 Token)
func (a *App) GetMyCollection() ([]bangumi.UserCollection, error) {
	fmt.Println("📚 [Bangumi] 获取我的收藏...")
	// 如果 config.json 里配了 user_id 就用，否则用 "me"
	uid := a.configMgr.Data.GlobalSettings.BangumiUserID
	if uid == "" {
		uid = "me"
	}
	return a.bangumiClient.GetUserCollection(uid)
}

// --- 4. 资源搜索 (Crawler API) ---

// SearchResource 搜索磁力链接 (动漫花园)
func (a *App) SearchResource(keyword string) ([]crawler.TorrentItem, error) {
	fmt.Printf("🔍 [Crawler] 搜索资源: %s\n", keyword)

	// 使用 config.json 里配置的 API URL 进行搜索
	items, err := a.crawler.SearchResource(keyword)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %v", err)
	}

	return items, nil
}

// Log 发送日志到前端
func (a *App) Log(level, message string) {
	// 打印到控制台
	fmt.Printf("[%s] %s\n", level, message)

	logEntry := map[string]string{
		"level":   level,
		"message": message,
		"time":    time.Now().Format("15:04:05"),
	}

	// 保存到历史
	a.logHistory = append(a.logHistory, logEntry)
	// 限制历史长度
	if len(a.logHistory) > 1000 {
		a.logHistory = a.logHistory[1:]
	}

	// 发送到前端
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "log-message", logEntry)
	}
}

// GetLogs 获取日志历史
func (a *App) GetLogs() []map[string]string {
	return a.logHistory
}

// GetPikPakStatus 获取 PikPak 登录状态
func (a *App) GetPikPakStatus() string {
	if a.pikpakClient != nil {
		return "Success"
	}
	return "未登录"
}

// UpdateCollectionStatus 更新番剧收藏状态
func (a *App) UpdateCollectionStatus(subjectID int, status int) string {
	a.Log("INFO", fmt.Sprintf("更新收藏状态: ID=%d, Status=%d", subjectID, status))
	if err := a.bangumiClient.UpdateCollectionStatus(subjectID, status); err != nil {
		a.Log("ERROR", fmt.Sprintf("更新失败: %v", err))
		return fmt.Sprintf("Error: %v", err)
	}
	return "Success"
}

// GetAppConfig 获取当前配置
func (a *App) GetAppConfig() config.AppConfig {
	return a.configMgr.Data
}

// SaveAppConfig 保存配置
func (a *App) SaveAppConfig(jsonStr string) string {
	a.Log("INFO", "保存配置...")
	var newConfig config.AppConfig
	if err := json.Unmarshal([]byte(jsonStr), &newConfig); err != nil {
		return fmt.Sprintf("Error: 解析配置失败 %v", err)
	}

	// 更新内存
	a.configMgr.Data = newConfig
	// 保存到文件
	if err := a.configMgr.Save(); err != nil {
		return fmt.Sprintf("Error: 保存文件失败 %v", err)
	}

	// 实时应用部分配置
	if newConfig.GlobalSettings.BangumiApiToken != "" {
		a.bangumiClient.SetToken(newConfig.GlobalSettings.BangumiApiToken)
	}

	return "Success"
}

// FollowedItem 本地追番条目
type FollowedItem struct {
	SubjectID      int               `json:"subject_id"`
	Name           string            `json:"name"`
	NameCN         string            `json:"name_cn"`
	Image          string            `json:"image"`
	AirDate        string            `json:"air_date"`
	AddedAt        string            `json:"added_at"`
	WatchedEps     []float64         `json:"watched_eps"`     // 已观看的集数 (Sort)
	DownloadedEps  []float64         `json:"downloaded_eps"`  // 已下载的集数 (Sort)
	EpisodeMagnets map[string]string `json:"episode_magnets"` // 集数对应的磁力链接 (key: sort, value: magnet)
	LocalFiles     map[string]string `json:"local_files"`     // 本地文件路径 (key: sort, value: path)
}

// FollowLocal 添加本地追番
func (a *App) FollowLocal(item bangumi.Subject) string {
	a.Log("INFO", fmt.Sprintf("添加本地追番: %s", item.Name))

	// 读取现有列表
	list := a.GetLocalFollows()

	// 查重
	for _, v := range list {
		if v.SubjectID == item.ID {
			return "Already followed"
		}
	}

	// 构造新条目
	newItem := FollowedItem{
		SubjectID:      item.ID,
		Name:           item.Name,
		NameCN:         item.NameCN,
		Image:          item.Images.Large, // 优先用 Large (高清)
		AirDate:        item.AirDate,
		AddedAt:        time.Now().Format("2006-01-02 15:04:05"),
		WatchedEps:     []float64{},
		DownloadedEps:  []float64{},
		EpisodeMagnets: make(map[string]string),
		LocalFiles:     make(map[string]string),
	}
	if newItem.Image == "" {
		newItem.Image = item.Images.Common
	}

	list = append(list, newItem)
	return a.saveFollowedList(list)
}

// ToggleEpisodeWatched 切换集数观看状态
func (a *App) ToggleEpisodeWatched(subjectID int, epSort float64) string {
	a.Log("INFO", fmt.Sprintf("切换观看状态: ID=%d, Ep=%v", subjectID, epSort))

	list := a.GetLocalFollows()
	found := false

	for i, v := range list {
		if v.SubjectID == subjectID {
			found = true
			// 检查是否已观看
			isWatched := false
			newWatched := []float64{}
			for _, s := range v.WatchedEps {
				if s == epSort {
					isWatched = true
				} else {
					newWatched = append(newWatched, s)
				}
			}

			if !isWatched {
				newWatched = append(newWatched, epSort)
			}
			list[i].WatchedEps = newWatched
			break
		}
	}

	if !found {
		return "Not followed"
	}

	return a.saveFollowedList(list)
}

// SearchEpisodeMagnet 搜索集数磁力链接
func (a *App) SearchEpisodeMagnet(subjectID int, epSort float64) string {
	a.Log("INFO", fmt.Sprintf("搜索磁力: ID=%d, Ep=%v", subjectID, epSort))

	// 1. 获取追番信息
	list := a.GetLocalFollows()
	var targetItem *FollowedItem
	var targetIndex int

	for i, v := range list {
		if v.SubjectID == subjectID {
			targetItem = &list[i]
			targetIndex = i
			break
		}
	}

	if targetItem == nil {
		return "Error: 请先追番"
	}

	// 2. 检查本地缓存
	epKey := fmt.Sprintf("%v", epSort)
	if magnet, ok := targetItem.EpisodeMagnets[epKey]; ok && magnet != "" {
		a.Log("INFO", "命中本地磁力缓存")
		return magnet
	}

	// 3. 构造搜索关键词
	keywords := []string{}
	if targetItem.NameCN != "" {
		keywords = append(keywords, targetItem.NameCN)
	} else {
		keywords = append(keywords, targetItem.Name)
	}

	// 4. 调用爬虫
	res, err := a.crawler.SearchEpisode(keywords, epSort)
	if err != nil {
		a.Log("WARN", fmt.Sprintf("搜索失败: %v", err))
		return fmt.Sprintf("Error: %v", err)
	}

	// 5. 保存结果
	if targetItem.EpisodeMagnets == nil {
		targetItem.EpisodeMagnets = make(map[string]string)
	}
	targetItem.EpisodeMagnets[epKey] = res.Magnet

	// 更新列表
	list[targetIndex] = *targetItem
	if res := a.saveFollowedList(list); res != "Success" {
		a.Log("ERROR", "保存磁力链接失败: "+res)
	}

	return res.Magnet
}

// SearchEpisodeMagnetList 搜索集数磁力链接，返回候选列表
func (a *App) SearchEpisodeMagnetList(subjectID int, epSort float64, customKeywords string) ([]crawler.TorrentItem, error) {
	a.Log("INFO", fmt.Sprintf("搜索磁力列表: ID=%d, Ep=%v, Keywords=%s", subjectID, epSort, customKeywords))

	// 1. 构造搜索关键词
	var keywords []string
	if customKeywords != "" {
		// 使用自定义关键词
		keywords = []string{customKeywords}
	} else {
		// 使用默认关键词
		list := a.GetLocalFollows()
		for _, v := range list {
			if v.SubjectID == subjectID {
				if v.NameCN != "" {
					keywords = append(keywords, v.NameCN)
				} else {
					keywords = append(keywords, v.Name)
				}
				break
			}
		}
		if len(keywords) == 0 {
			return nil, fmt.Errorf("未找到追番记录")
		}
	}

	// 2. 调用爬虫获取候选列表
	return a.crawler.SearchEpisodeList(keywords, epSort)
}

// SaveEpisodeMagnet 手动保存选定的磁力链接
func (a *App) SaveEpisodeMagnet(subjectID int, epSort float64, magnet string) string {
	a.Log("INFO", fmt.Sprintf("保存磁力: ID=%d, Ep=%v", subjectID, epSort))

	list := a.GetLocalFollows()
	var targetItem *FollowedItem
	var targetIndex int

	for i, v := range list {
		if v.SubjectID == subjectID {
			targetItem = &list[i]
			targetIndex = i
			break
		}
	}

	if targetItem == nil {
		return "Error: 请先追番"
	}

	// 保存磁力链接
	epKey := fmt.Sprintf("%v", epSort)
	if targetItem.EpisodeMagnets == nil {
		targetItem.EpisodeMagnets = make(map[string]string)
	}
	targetItem.EpisodeMagnets[epKey] = magnet

	// 更新列表
	list[targetIndex] = *targetItem
	return a.saveFollowedList(list)
}

// UnfollowLocal 取消本地追番
func (a *App) UnfollowLocal(subjectID int) string {
	a.Log("INFO", fmt.Sprintf("取消本地追番: %d", subjectID))

	list := a.GetLocalFollows()
	newList := []FollowedItem{}
	found := false

	for _, v := range list {
		if v.SubjectID == subjectID {
			found = true
			continue
		}
		newList = append(newList, v)
	}

	if !found {
		return "Not found"
	}

	return a.saveFollowedList(newList)
}

// GetLocalFollows 获取本地追番列表
func (a *App) GetLocalFollows() []FollowedItem {
	// 简单实现：每次都读文件
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, "followed.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return []FollowedItem{}
	}

	var list []FollowedItem
	_ = json.Unmarshal(data, &list)
	return list
}

// saveFollowedList 保存追番列表到文件
func (a *App) saveFollowedList(list []FollowedItem) string {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, "followed.json")

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return "Success"
}

// AnimeDetail 聚合的动漫详情
type AnimeDetail struct {
	Subject        bangumi.SubjectDetail `json:"subject"`
	Episodes       []bangumi.Episode     `json:"episodes"`
	CurrentEpisode float64               `json:"current_episode"` // 当前更新到的集数
	IsFollowed     bool                  `json:"is_followed"`
	WatchedEps     []float64             `json:"watched_eps"`     // 已观看集数
	DownloadedEps  []float64             `json:"downloaded_eps"`  // 已下载集数
	EpisodeMagnets map[string]string     `json:"episode_magnets"` // 集数对应的磁力链接
}

// GetAnimeDetail 获取动漫详细信息 (包含集数更新情况)
func (a *App) GetAnimeDetail(subjectID int) (*AnimeDetail, error) {
	a.Log("INFO", fmt.Sprintf("获取动漫详情: %d", subjectID))

	// 1. 获取基本详情
	subject, err := a.bangumiClient.GetSubjectDetail(subjectID)
	if err != nil {
		return nil, err
	}

	// 2. 获取剧集列表
	episodes, err := a.bangumiClient.GetSubjectEpisodes(subjectID)
	if err != nil {
		// 如果获取剧集失败，降级处理，只返回详情
		a.Log("WARN", fmt.Sprintf("获取剧集失败: %v", err))
		episodes = []bangumi.Episode{}
	}

	// 3. 计算当前更新到了第几集
	// 逻辑：遍历剧集，找到 AirDate <= 今天 的最大 Sort
	currentEp := 0.0
	today := time.Now().Format("2006-01-02")

	for _, ep := range episodes {
		// 如果没有放送日期，或者放送日期在今天之前(或今天)
		if ep.AirDate != "" && ep.AirDate <= today {
			if ep.Sort > currentEp {
				currentEp = ep.Sort
			}
		}
	}

	// 4. 检查是否已追番及观看进度
	isFollowed := false
	watchedEps := []float64{}
	downloadedEps := []float64{}
	episodeMagnets := make(map[string]string)
	localFollows := a.GetLocalFollows()
	for _, f := range localFollows {
		if f.SubjectID == subjectID {
			isFollowed = true
			watchedEps = f.WatchedEps
			downloadedEps = f.DownloadedEps
			episodeMagnets = f.EpisodeMagnets
			break
		}
	}

	return &AnimeDetail{
		Subject:        *subject,
		Episodes:       episodes,
		CurrentEpisode: currentEp,
		IsFollowed:     isFollowed,
		WatchedEps:     watchedEps,
		DownloadedEps:  downloadedEps,
		EpisodeMagnets: episodeMagnets,
	}, nil
}

// getPikPakFileFromMagnet 从磁力链接获取 PikPak 文件信息 (自动处理文件夹)
func (a *App) getPikPakFileFromMagnet(magnet string) (string, string, int64, error) {
	if a.pikpakClient == nil {
		if err := a.tryAutoLogin(); err != nil {
			return "", "", 0, fmt.Errorf("请先登录 PikPak (%v)", err)
		}
	}

	var task *pikpak.OfflineTask
	var err error

	// 尝试添加任务，如果遇到限额错误则切换账号重试
	maxRetries := len(a.configMgr.Data.GlobalSettings.PikPakUsers)
	if maxRetries == 0 {
		maxRetries = 1
	}

	for i := 0; i < maxRetries+1; i++ {
		a.Log("INFO", fmt.Sprintf("正在添加磁力任务: %s", magnet))
		task, err = a.pikpakClient.OfflineDownload(magnet, "", "")
		if err == nil {
			break
		}

		// 检查是否为限额错误 (api error 11) 或 空间不足 (api error 8)
		errMsg := err.Error()
		if strings.Contains(errMsg, "task_daily_create_limit") || strings.Contains(errMsg, "api error 11") ||
			strings.Contains(errMsg, "file_space_not_enough") || strings.Contains(errMsg, "api error 8") {

			reason := "每日配额已满"
			if strings.Contains(errMsg, "file_space_not_enough") || strings.Contains(errMsg, "api error 8") {
				reason = "云盘空间不足"
				// 空间不足时，异步清理当前账号空间，并立即切换账号
				a.Log("WARN", "当前账号空间不足，启动后台彻底清理（永久删除所有文件）...")

				// 捕获当前客户端实例和账号名，用于后台清理
				currentClient := a.pikpakClient
				currentUsername := ""
				if currentClient != nil {
					currentUsername = currentClient.Username
				}

				go func(client *pikpak.PikPakClient, username string) {
					if client != nil {
						a.Log("INFO", fmt.Sprintf("🧹 [后台清理] 开始清空账号 %s 的云盘空间...", username))
						if clearErr := client.ClearStorage(); clearErr != nil {
							a.Log("ERROR", fmt.Sprintf("❌ [后台清理] 账号 %s 清理失败: %v", username, clearErr))
						} else {
							a.Log("SUCCESS", fmt.Sprintf("✅ [后台清理] 账号 %s 空间清理完成，所有文件已永久删除", username))
						}
					}
				}(currentClient, currentUsername)
			}

			a.Log("WARN", fmt.Sprintf("当前账号不可用 (%s)，尝试切换账号...", reason))

			// 标记当前账号今日不可用
			if a.pikpakClient != nil {
				a.markAccountBlocked(a.pikpakClient.Username, reason)
			}

			if switchErr := a.switchToNextAccount(); switchErr != nil {
				a.Log("ERROR", fmt.Sprintf("切换账号失败: %v", switchErr))
				return "", "", 0, fmt.Errorf("添加任务失败 (%v) 且切换账号失败 (%v)", err, switchErr)
			}
			continue
		}
		return "", "", 0, fmt.Errorf("添加任务失败: %v", err)
	}

	if err != nil {
		return "", "", 0, fmt.Errorf("所有账号均无法添加任务: %v", err)
	}

	a.Log("INFO", fmt.Sprintf("任务添加成功，TaskID: %s, FileID: %s, Phase: %s", task.ID, task.FileID, task.Phase))

	fileID := task.FileID
	finalTaskName := task.FileName
	if finalTaskName == "" {
		finalTaskName = task.Name
	}

	// 即使有 FileID，如果状态不是 COMPLETE，也需要等待
	if fileID == "" || task.Phase != "PHASE_TYPE_COMPLETE" {
		a.Log("INFO", "任务正在进行中，等待离线下载完成...")
		// 轮询任务状态
		for i := 0; i < 60; i++ { // 最多等待 60秒
			time.Sleep(1 * time.Second)
			// includeDone=true 以便能查到已完成的任务
			tasks, err := a.pikpakClient.OfflineList(true)
			if err != nil {
				a.Log("WARN", fmt.Sprintf("轮询任务列表失败: %v", err))
				continue
			}
			for _, t := range tasks {
				if t.ID == task.ID {
					if t.Phase == "PHASE_TYPE_COMPLETE" {
						fileID = t.FileID
						if t.FileName != "" {
							finalTaskName = t.FileName
						} else {
							finalTaskName = t.Name
						}
						a.Log("INFO", fmt.Sprintf("任务已完成，FileID: %s", fileID))
						goto Found
					} else if t.Phase == "PHASE_TYPE_ERROR" {
						return "", "", 0, fmt.Errorf("离线下载失败 %s", t.Message)
					}
					// 仍在运行中...
				}
			}
		}
		return "", "", 0, fmt.Errorf("离线下载超时或未完成")
	}

Found:
	a.Log("INFO", fmt.Sprintf("离线完成，初始ID: %s", fileID))

	// 检查是否为文件夹
	fileInfo, err := a.pikpakClient.GetFile(fileID)

	// 容错：如果 ID 找不到文件，尝试通过文件名在根目录搜索
	if err != nil {
		a.Log("WARN", fmt.Sprintf("通过 ID 获取文件失败 (%v)，尝试搜索同名文件: %s", err, finalTaskName))
		if finalTaskName != "" {
			// 只获取前 100 个文件，避免太慢
			// 注意：这里调用 FileList 会获取所有文件，如果文件太多可能会慢
			// 暂时先这样，后续可以优化 FileList 支持 limit
			files, listErr := a.pikpakClient.FileList("")
			if listErr == nil {
				for _, f := range files {
					if f.Name == finalTaskName {
						a.Log("INFO", fmt.Sprintf("✅ 找到同名文件: %s (ID: %s)", f.Name, f.Id))
						fCopy := f
						fileInfo = &fCopy
						fileID = f.Id
						err = nil
						break
					}
				}
			}
		}
	}

	if err != nil {
		return "", "", 0, fmt.Errorf("无法获取文件信息: %v", err)
	}

	if fileInfo.Kind == "drive#folder" {
		a.Log("INFO", "检测到文件夹，正在寻找视频文件...")
		files, err := a.pikpakClient.FileList(fileID)
		if err != nil {
			return "", "", 0, fmt.Errorf("获取文件夹内容失败 %v", err)
		}

		var largestFile *pikpak.File
		for _, f := range files {
			if f.Kind == "drive#file" && strings.HasPrefix(f.MimeType, "video/") {
				if largestFile == nil {
					fCopy := f
					largestFile = &fCopy
				} else {
					s1, _ := strconv.ParseInt(f.Size, 10, 64)
					s2, _ := strconv.ParseInt(largestFile.Size, 10, 64)
					if s1 > s2 {
						fCopy := f
						largestFile = &fCopy
					}
				}
			}
		}

		if largestFile != nil {
			size, _ := strconv.ParseInt(largestFile.Size, 10, 64)
			a.Log("INFO", fmt.Sprintf("找到最大视频文件: %s (%d bytes)", largestFile.Name, size))
			return largestFile.Id, largestFile.Name, size, nil
		}
		return "", "", 0, fmt.Errorf("文件夹中未找到视频文件")
	}

	// 如果是单文件
	if fileInfo != nil {
		size, _ := strconv.ParseInt(fileInfo.Size, 10, 64)
		return fileInfo.Id, fileInfo.Name, size, nil
	}

	return "", "", 0, fmt.Errorf("无法获取文件信息")
}

// DownloadEpisode 下载单集
func (a *App) DownloadEpisode(subjectID int, epSort float64, magnet string) string {
	a.Log("INFO", fmt.Sprintf("开始下载任务: ID=%d, Ep=%v", subjectID, epSort))

	go func() {
		// 1. 获取文件信息
		fileID, fileName, fileSize, err := a.getPikPakFileFromMagnet(magnet)
		if err != nil {
			a.Log("ERROR", fmt.Sprintf("下载准备失败: %v", err))
			return
		}

		// 2. 确定保存路径
		// 获取番剧名称
		animeName := fmt.Sprintf("Anime_%d", subjectID)
		localFollows := a.GetLocalFollows()
		for _, f := range localFollows {
			if f.SubjectID == subjectID {
				animeName = f.Name
				break
			}
		}
		animeName = sanitizePathSegment(animeName)

		cwd, _ := os.Getwd()
		downloadDir := filepath.Join(cwd, "Downloads", animeName)
		if err := os.MkdirAll(downloadDir, 0755); err != nil {
			a.Log("ERROR", fmt.Sprintf("创建目录失败: %v", err))
			return
		}

		safeFileName := sanitizePathSegment(fileName)
		savePath := filepath.Join(downloadDir, safeFileName)
		a.Log("INFO", fmt.Sprintf("开始下载到: %s", savePath))

		// 3. 开始下载
		var lastTime = time.Now()
		var lastBytes int64 = 0
		var speed int64 = 0
		lastProgressLogTime := time.Now().Add(-10 * time.Second)

		err = a.pikpakClient.DownloadFileConcurrent(fileID, savePath, fileSize, 16, func(current, total int64) {
			now := time.Now()
			duration := now.Sub(lastTime)

			// 每秒更新一次速度和进度
			if duration >= time.Second {
				bytesDiff := current - lastBytes
				speed = int64(float64(bytesDiff) / duration.Seconds())

				lastTime = now
				lastBytes = current

				progressVal := float64(current) / float64(total) * 100

				// 打印进度到控制台
				fmt.Printf("\r⬇️ [下载中] 进度: %.2f%% | 速度: %s/s | 已下载: %s | 总大小: %s   ",
					progressVal, formatBytes(speed), formatBytes(current), formatBytes(total))

				// 打包版通常看不到控制台输出，这里节流写入应用日志。
				if now.Sub(lastProgressLogTime) >= 5*time.Second || current == total {
					a.Log("INFO", fmt.Sprintf("下载进度: %.2f%% | 速度: %s/s | 已下载: %s/%s",
						progressVal, formatBytes(speed), formatBytes(current), formatBytes(total)))
					lastProgressLogTime = now
				}

				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "download-progress", map[string]interface{}{
						"subject_id": subjectID,
						"ep_sort":    epSort,
						"progress":   progressVal,
						"speed":      speed, // bytes per second
						"total":      total,
						"current":    current,
					})
				}
			}
		})
		fmt.Println() // 换行

		if err != nil {
			a.Log("ERROR", fmt.Sprintf("下载失败: %v", err))
			return
		}

		a.Log("INFO", fmt.Sprintf("下载完成: %s", fileName))

		// 4. 标记为已下载
		a.markEpisodeDownloaded(subjectID, epSort, savePath)

		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "download-complete", map[string]interface{}{
				"subject_id": subjectID,
				"ep_sort":    epSort,
				"path":       savePath,
			})
		}
	}()

	return "Started"
}

// markEpisodeDownloaded 标记集数已下载
func (a *App) markEpisodeDownloaded(subjectID int, epSort float64, filePath string) {
	list := a.GetLocalFollows()

	for i, v := range list {
		if v.SubjectID == subjectID {
			// 检查是否已存在
			exists := false
			for _, s := range v.DownloadedEps {
				if s == epSort {
					exists = true
					break
				}
			}
			if !exists {
				list[i].DownloadedEps = append(list[i].DownloadedEps, epSort)
			}

			// 更新文件路径
			if list[i].LocalFiles == nil {
				list[i].LocalFiles = make(map[string]string)
			}
			epKey := fmt.Sprintf("%v", epSort)
			list[i].LocalFiles[epKey] = filePath

			a.saveFollowedList(list)
			break
		}
	}
}

// PlayLocalEpisode 播放本地已下载的剧集
func (a *App) PlayLocalEpisode(subjectID int, epSort float64) string {
	a.Log("INFO", fmt.Sprintf("请求播放本地文件: ID=%d, Ep=%v", subjectID, epSort))

	// 1. 查找文件路径
	list := a.GetLocalFollows()
	var filePath string
	found := false

	for _, v := range list {
		if v.SubjectID == subjectID {
			if v.LocalFiles != nil {
				epKey := fmt.Sprintf("%v", epSort)
				filePath = v.LocalFiles[epKey]
				if filePath != "" {
					found = true
				}
			}
			break
		}
	}

	if !found {
		return "Error: 未找到本地文件记录"
	}

	// 2. 检查文件是否存在
	if _, err := os.Stat(filePath); err != nil {
		a.Log("WARN", fmt.Sprintf("文件不存在，自动清理记录: %s", filePath))
		a.removeEpisodeRecord(subjectID, epSort)
		return "Error: FileMissing"
	}

	// 3. 启动 MPV
	a.Log("INFO", fmt.Sprintf("启动 MPV 播放本地文件: %s", filePath))

	// 优先使用配置中的 MPV 路径
	mpvPath := a.configMgr.Data.Player.MPVPath
	if mpvPath == "" {
		mpvPath = "mpv" // 默认尝试环境变量
		if _, err := os.Stat("mpv.exe"); err == nil {
			mpvPath = ".\\mpv.exe"
		} else if _, err := os.Stat("bin/mpv.exe"); err == nil {
			mpvPath = "bin\\mpv.exe"
		}
	}

	// 构造参数
	args := []string{filePath, "--force-window"}
	if a.configMgr.Data.Player.MPVArgs != "" {
		// 简单分割参数，不支持带空格的引号参数，后续可优化
		userArgs := strings.Fields(a.configMgr.Data.Player.MPVArgs)
		args = append(args, userArgs...)
	}

	// 构造命令
	cmd := exec.Command(mpvPath, args...)

	// 设置工作目录
	cwd, _ := os.Getwd()
	cmd.Dir = cwd

	// 绑定输出以便调试
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		a.Log("ERROR", fmt.Sprintf("启动 MPV 失败: %v", err))
		return fmt.Sprintf("Error: 启动 MPV 失败 %v", err)
	}

	return "Success"
}

// PlayMagnet 添加磁力并调用 MPV 播放
func (a *App) PlayMagnet(magnet string) string {
	fileID, fileName, _, err := a.getPikPakFileFromMagnet(magnet)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	a.Log("INFO", fmt.Sprintf("准备播放文件: %s (ID: %s)", fileName, fileID))

	// 获取播放链接
	playUrl, err := a.pikpakClient.GetDownloadUrl(fileID)
	if err != nil {
		return fmt.Sprintf("Error: 获取播放链接失败 %v", err)
	}

	// 启动 MPV
	a.Log("INFO", "启动 MPV 播放器...")

	// 优先使用配置中的 MPV 路径
	mpvPath := a.configMgr.Data.Player.MPVPath
	if mpvPath == "" {
		mpvPath = "mpv" // 默认尝试环境变量
		if _, err := os.Stat("mpv.exe"); err == nil {
			mpvPath = ".\\mpv.exe"
		} else if _, err := os.Stat("bin/mpv.exe"); err == nil {
			mpvPath = "bin\\mpv.exe"
		}
	}

	// 构造参数
	// 使用 --force-window 确保窗口显示
	// 使用 --http-header-fields 设置 User-Agent，防止 403
	args := []string{playUrl, "--force-window", "--http-header-fields=User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"}

	if a.configMgr.Data.Player.MPVArgs != "" {
		userArgs := strings.Fields(a.configMgr.Data.Player.MPVArgs)
		args = append(args, userArgs...)
	}

	cmd := exec.Command(mpvPath, args...)

	// 设置工作目录，确保能读取到 input.conf
	cwd, _ := os.Getwd()
	cmd.Dir = cwd

	// 绑定输出以便调试
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		a.Log("ERROR", fmt.Sprintf("启动 MPV 失败: %v", err))
		return fmt.Sprintf("Error: 启动 MPV 失败 %v", err)
	}

	return "Success"
}

// GetBlockedAccounts 获取当前被封禁的账号列表
func (a *App) GetBlockedAccounts() map[string]string {
	if a.blockedAccounts == nil {
		return map[string]string{}
	}
	return a.blockedAccounts
}

// SetAccountBlockStatus 手动设置账号封禁状态
func (a *App) SetAccountBlockStatus(username string, blocked bool) string {
	if a.blockedAccounts == nil {
		a.blockedAccounts = make(map[string]string)
	}

	if blocked {
		// 封禁：设置为今天
		today := time.Now().Format("2006-01-02")
		a.blockedAccounts[username] = today
		a.Log("INFO", fmt.Sprintf("手动封禁账号: %s", username))
	} else {
		// 解封：删除记录
		delete(a.blockedAccounts, username)
		a.Log("INFO", fmt.Sprintf("手动解封账号: %s", username))
	}
	return "Success"
}

// isAccountBlocked 检查账号是否今日不可用
func (a *App) isAccountBlocked(username string) bool {
	if a.blockedAccounts == nil {
		return false
	}
	date, ok := a.blockedAccounts[username]
	if !ok {
		return false
	}
	// 检查是否是今天
	today := time.Now().Format("2006-01-02")
	return date == today
}

// markAccountBlocked 标记账号今日不可用
func (a *App) markAccountBlocked(username string, reason string) {
	if a.blockedAccounts == nil {
		a.blockedAccounts = make(map[string]string)
	}
	today := time.Now().Format("2006-01-02")
	a.blockedAccounts[username] = today
	a.Log("WARN", fmt.Sprintf("账号 %s 已被标记为今日不可用 (原因: %s)", username, reason))
}

// tryAutoLogin 尝试自动登录 (遍历所有账号直到成功)
func (a *App) tryAutoLogin() error {
	users := a.configMgr.Data.GlobalSettings.PikPakUsers
	if len(users) == 0 {
		return fmt.Errorf("未配置 PikPak 账号")
	}

	// 确保索引有效
	if a.currentAccountIndex >= len(users) {
		a.currentAccountIndex = 0
	}

	startIndex := a.currentAccountIndex
	count := len(users)

	for i := 0; i < count; i++ {
		idx := (startIndex + i) % count
		username := users[idx]

		if a.isAccountBlocked(username) {
			a.Log("WARN", fmt.Sprintf("跳过今日不可用账号: %s", username))
			continue
		}

		password := a.configMgr.Data.GlobalSettings.PikPakPassword

		a.Log("INFO", fmt.Sprintf("尝试登录账号 (%d/%d): %s", i+1, count, username))

		proxy := a.configMgr.Data.GlobalSettings.Proxy
		client := pikpak.NewPikPakClient(username, password, proxy)
		err := client.Login()
		if err == nil {
			a.pikpakClient = client
			a.currentAccountIndex = idx
			a.Log("INFO", fmt.Sprintf("账号 %s 登录成功", username))
			return nil
		}

		// 如果是风控错误，标记
		errMsg := err.Error()
		if strings.Contains(errMsg, "captcha") || strings.Contains(errMsg, "verify") || strings.Contains(errMsg, "risk") {
			a.markAccountBlocked(username, "登录风控/验证码")
		}

		a.Log("WARN", fmt.Sprintf("账号 %s 登录失败: %v，尝试下一个...", username, err))
	}

	return fmt.Errorf("所有账号均登录失败或不可用")
}

// switchToNextAccount 切换到下一个账号
func (a *App) switchToNextAccount() error {
	users := a.configMgr.Data.GlobalSettings.PikPakUsers
	if len(users) <= 1 {
		return fmt.Errorf("只有一个账号，无法切换")
	}

	a.currentAccountIndex = (a.currentAccountIndex + 1) % len(users)
	return a.tryAutoLogin()
}

// formatBytes 格式化字节数
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// removeEpisodeRecord 移除集数下载记录
func (a *App) removeEpisodeRecord(subjectID int, epSort float64) {
	list := a.GetLocalFollows()
	for i, v := range list {
		if v.SubjectID == subjectID {
			// 移除 DownloadedEps
			newEps := []float64{}
			for _, e := range v.DownloadedEps {
				if e != epSort {
					newEps = append(newEps, e)
				}
			}
			list[i].DownloadedEps = newEps

			// 移除 LocalFiles
			epKey := fmt.Sprintf("%v", epSort)
			if list[i].LocalFiles != nil {
				delete(list[i].LocalFiles, epKey)
			}
			a.saveFollowedList(list)
			break
		}
	}
}

// DeleteEpisodeData 删除集数的磁力链接和/或本地文件
func (a *App) DeleteEpisodeData(subjectID int, epSort float64) string {
	a.Log("INFO", fmt.Sprintf("删除集数数据: ID=%d, Ep=%v", subjectID, epSort))

	list := a.GetLocalFollows()
	var targetItem *FollowedItem
	var targetIndex int

	for i, v := range list {
		if v.SubjectID == subjectID {
			targetItem = &list[i]
			targetIndex = i
			break
		}
	}

	if targetItem == nil {
		return "Error: 未找到追番记录"
	}

	epKey := fmt.Sprintf("%v", epSort)
	deleted := false

	// 1. 删除本地文件
	if targetItem.LocalFiles != nil {
		if filePath, ok := targetItem.LocalFiles[epKey]; ok && filePath != "" {
			if err := os.Remove(filePath); err != nil {
				a.Log("WARN", fmt.Sprintf("删除本地文件失败: %v", err))
			} else {
				a.Log("INFO", fmt.Sprintf("✅ 已删除本地文件: %s", filePath))
				deleted = true
			}
			delete(targetItem.LocalFiles, epKey)
		}
	}

	// 2. 删除磁力链接记录
	if targetItem.EpisodeMagnets != nil {
		if _, ok := targetItem.EpisodeMagnets[epKey]; ok {
			delete(targetItem.EpisodeMagnets, epKey)
			a.Log("INFO", "✅ 已删除磁力链接记录")
			deleted = true
		}
	}

	// 3. 移除下载标记
	newDownloadedEps := []float64{}
	for _, e := range targetItem.DownloadedEps {
		if e != epSort {
			newDownloadedEps = append(newDownloadedEps, e)
		}
	}
	targetItem.DownloadedEps = newDownloadedEps

	// 保存更新
	list[targetIndex] = *targetItem
	if res := a.saveFollowedList(list); res != "Success" {
		return res
	}

	if !deleted {
		return "Error: 未找到可删除的数据"
	}

	return "Success"
}

// ClearPikPakStorage 手动清空指定 PikPak 账号的云盘空间
// username: 要清空的账号，如果为空则清空当前登录账号
func (a *App) ClearPikPakStorage(username string) string {
	// 如果指定了账号，需要先登录
	if username != "" && (a.pikpakClient == nil || a.pikpakClient.Username != username) {
		password := a.configMgr.Data.GlobalSettings.PikPakPassword
		if password == "" {
			return "Error: 未配置密码"
		}

		a.Log("INFO", fmt.Sprintf("🔐 切换到账号 %s 进行清空操作...", username))
		proxy := a.configMgr.Data.GlobalSettings.Proxy
		client := pikpak.NewPikPakClient(username, password, proxy)

		if err := client.Login(); err != nil {
			return fmt.Sprintf("Error: 登录失败 %v", err)
		}

		// 临时使用这个客户端进行清空
		a.Log("INFO", fmt.Sprintf("🧹 开始清空账号 %s 的云盘空间...", username))
		go func(c *pikpak.PikPakClient, user string) {
			if err := c.ClearStorage(); err != nil {
				a.Log("ERROR", fmt.Sprintf("❌ 账号 %s 清空失败: %v", user, err))
			} else {
				a.Log("SUCCESS", fmt.Sprintf("✅ 账号 %s 的云盘空间已清空（所有文件已永久删除）", user))
			}
		}(client, username)

		return "Started"
	}

	// 清空当前账号
	if a.pikpakClient == nil {
		return "Error: 请先登录 PikPak"
	}

	currentUsername := a.pikpakClient.Username
	a.Log("INFO", fmt.Sprintf("🧹 开始清空账号 %s 的云盘空间...", currentUsername))

	// 异步执行清理，避免阻塞前端
	go func() {
		if err := a.pikpakClient.ClearStorage(); err != nil {
			a.Log("ERROR", fmt.Sprintf("❌ 清空失败: %v", err))
		} else {
			a.Log("SUCCESS", fmt.Sprintf("✅ 账号 %s 的云盘空间已清空（所有文件已永久删除）", currentUsername))
		}
	}()

	return "Started"
}
