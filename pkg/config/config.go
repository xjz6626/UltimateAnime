package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// AppConfig 根配置结构
type AppConfig struct {
	GlobalSettings  GlobalSettings  `json:"global_settings"`
	LocalStorage    LocalStorage    `json:"local_storage"`
	SeasonalFetcher SeasonalFetcher `json:"seasonal_fetcher"`
	TorrentSearcher TorrentSearcher `json:"torrent_searcher"`
	BTDownloader    BTDownloader    `json:"bt_downloader"`
	Player          PlayerSettings  `json:"player"`
}

// PlayerSettings 播放器配置
type PlayerSettings struct {
	MPVPath string `json:"mpv_path"` // MPV 播放器路径
	MPVArgs string `json:"mpv_args"` // 启动参数
}

// GlobalSettings 全局设置
type GlobalSettings struct {
	BangumiDataUrl      string   `json:"bangumi_data_url"`
	BangumiApiToken     string   `json:"bangumi_api_token"` // Bangumi Access Token
	BangumiUserID       string   `json:"bangumi_user_id"`   // Bangumi User ID
	TorrentApiUrl       string   `json:"torrent_api_url"`
	JstTimezoneOffset   int      `json:"jst_timezone_offset"`
	ChineseWeekdays     []string `json:"chinese_weekdays"`
	DownloadHistoryFile string   `json:"download_history_file"`
	PikPakUsers         []string `json:"pikpak_users"`    // 支持多账号轮询 (预留)
	PikPakPassword      string   `json:"pikpak_password"` // 目前共用一个密码
	AutoLogin           bool     `json:"auto_login"`      // 是否自动登录 PikPak
	Proxy               string   `json:"proxy"`           // 代理地址 (http://127.0.0.1:7897)
}

// LocalStorage 本地存储路径
type LocalStorage struct {
	AnimeDir string `json:"anime_dir"` // 视频下载目录
}

// SeasonalFetcher 新番抓取配置
type SeasonalFetcher struct {
	TargetYear   int    `json:"target_year"`
	TargetMonths []int  `json:"target_months"`
	OutputFile   string `json:"output_file"`
}

// TorrentSearcher 磁力搜索配置
type TorrentSearcher struct {
	WatchlistFile string `json:"watchlist_file"`
	OutputFile    string `json:"output_file"`
}

// BTDownloader 下载器配置
type BTDownloader struct {
	ClientType string `json:"client_type"` // 例如 "pikpak"
}

// Manager 配置管理器
type Manager struct {
	ConfigPath string
	Data       AppConfig
	mu         sync.RWMutex
}

// NewManager 初始化管理器
func NewManager() *Manager {
	// 获取当前执行目录
	cwd, _ := os.Getwd()
	// 默认读取当前目录下的 config.json
	// 如果你的项目结构是 data/config.json，请修改这里的路径
	path := filepath.Join(cwd, "config.json")

	mgr := &Manager{
		ConfigPath: path,
		Data:       NewDefaultConfig(), // 加载默认值防止空指针
	}

	// 尝试从文件加载，如果失败（文件不存在）则保持默认值
	_ = mgr.Load()
	return mgr
}

// NewDefaultConfig 提供默认配置 (防止 config.json 缺失时崩盘)
func NewDefaultConfig() AppConfig {
	cfg := AppConfig{}
	cfg.GlobalSettings.BangumiApiToken = ""
	cfg.GlobalSettings.PikPakUsers = []string{}
	cfg.GlobalSettings.Proxy = ""
	cfg.LocalStorage.AnimeDir = "anime"
	cfg.BTDownloader.ClientType = "pikpak"
	return cfg
}

// Load 读取配置
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	file, err := os.ReadFile(m.ConfigPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(file, &m.Data)
}

// Save 保存配置
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := json.MarshalIndent(m.Data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.ConfigPath, data, 0644)
}

// SetBangumiToken 设置 Bangumi Token 和 UserID
func (m *Manager) SetBangumiToken(token, uid string) error {
	m.mu.Lock()
	m.Data.GlobalSettings.BangumiApiToken = token
	m.Data.GlobalSettings.BangumiUserID = uid
	m.mu.Unlock()
	return m.Save()
}

// 辅助方法：快速获取核心凭证

func (m *Manager) GetBangumiToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Data.GlobalSettings.BangumiApiToken
}

func (m *Manager) GetPikPakCredential() (username, password string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// 默认取第一个用户
	if len(m.Data.GlobalSettings.PikPakUsers) > 0 {
		return m.Data.GlobalSettings.PikPakUsers[0], m.Data.GlobalSettings.PikPakPassword
	}
	return "", ""
}

func (m *Manager) GetProxy() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Data.GlobalSettings.Proxy
}
