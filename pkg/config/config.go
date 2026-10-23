package config

import (
	"UltimateAnime/pkg/storage"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	Danmaku         DanmakuSettings `json:"danmaku"`
}

type DanmakuSettings struct {
	Enabled        bool   `json:"enabled"`
	AutoSelect     bool   `json:"auto_select"`
	BilibiliCookie string `json:"bilibili_cookie"`
}

// UpdateBilibiliCookie changes only the login session, preserving concurrent
// player, account and download settings. The login cookie is never logged.
func (m *Manager) UpdateBilibiliCookie(cookie string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.Data
	next.Danmaku.BilibiliCookie = cookie
	if err := m.writeLocked(next); err != nil {
		return err
	}
	m.Data = next
	return nil
}

// PlayerSettings 播放器配置
type PlayerSettings struct {
	AutoNext bool   `json:"auto_next"`
	MPVPath  string `json:"mpv_path"` // MPV 播放器路径
	MPVArgs  string `json:"mpv_args"` // 启动参数
}

// GlobalSettings 全局设置
type GlobalSettings struct {
	BangumiDataUrl      string            `json:"bangumi_data_url"`
	BangumiApiToken     string            `json:"bangumi_api_token"` // Bangumi Access Token
	BangumiUserID       string            `json:"bangumi_user_id"`   // Bangumi User ID
	TorrentApiUrl       string            `json:"torrent_api_url"`
	JstTimezoneOffset   int               `json:"jst_timezone_offset"`
	ChineseWeekdays     []string          `json:"chinese_weekdays"`
	DownloadHistoryFile string            `json:"download_history_file"`
	PikPakUsers         []string          `json:"pikpak_users"`    // 支持多账号轮询 (预留)
	PikPakPassword      string            `json:"pikpak_password"` // 目前共用一个密码
	DisabledAccounts    map[string]bool   `json:"disabled_accounts,omitempty"`
	QuotaBlockedUntil   map[string]string `json:"quota_blocked_until,omitempty"`
	Proxy               string            `json:"proxy"` // 代理地址 (http://127.0.0.1:7897)
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
	WatchlistFile    string `json:"watchlist_file"`
	OutputFile       string `json:"output_file"`
	AutoSelectMagnet bool   `json:"auto_select_magnet"`
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
	loadErr    error
}

// NewManager 初始化管理器
func NewManager() *Manager {
	// 获取当前执行目录
	cwd := DataDir()
	// 默认读取当前目录下的 config.json
	// 如果你的项目结构是 data/config.json，请修改这里的路径
	path := filepath.Join(cwd, "config.json")

	mgr := &Manager{
		ConfigPath: path,
		Data:       NewDefaultConfig(), // 加载默认值防止空指针
	}

	// 尝试从文件加载，如果失败（文件不存在）则保持默认值
	if err := mgr.Load(); err != nil && !os.IsNotExist(err) {
		mgr.loadErr = err
	}
	return mgr
}

// NewDefaultConfig 提供默认配置 (防止 config.json 缺失时崩盘)
func NewDefaultConfig() AppConfig {
	cfg := AppConfig{}
	cfg.GlobalSettings.BangumiApiToken = ""
	cfg.GlobalSettings.PikPakUsers = []string{}
	cfg.GlobalSettings.Proxy = ""
	cfg.LocalStorage.AnimeDir = "Downloads"
	cfg.BTDownloader.ClientType = "pikpak"
	cfg.Danmaku.Enabled = true
	return cfg
}

// Load 读取配置
func (m *Manager) Load() (loadErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	defer func() {
		if !os.IsNotExist(loadErr) {
			m.loadErr = loadErr
		}
	}()

	file, err := os.ReadFile(m.ConfigPath)
	if err != nil {
		return err
	}
	// Older config.json files lack fields added by newer releases. Start with
	// current defaults, and only replace the live configuration after a full parse.
	next := NewDefaultConfig()
	if strings.TrimSpace(string(file)) == "null" {
		return fmt.Errorf("配置文件不能为 null")
	}
	if err := json.Unmarshal(file, &next); err != nil {
		return err
	}
	m.Data = next
	m.loadErr = nil
	return nil
}

// Save 保存配置
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.writeLocked(m.Data)
}

func (m *Manager) Snapshot() AppConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneConfig(m.Data)
}

func (m *Manager) Replace(data AppConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.writeLocked(data); err != nil {
		return err
	}
	m.Data = cloneConfig(data)
	return nil
}

func (m *Manager) writeLocked(data AppConfig) error {
	if m.loadErr != nil {
		return fmt.Errorf("配置加载失败，请修复原文件后重新加载: %w", m.loadErr)
	}
	return storage.WriteJSON(m.ConfigPath, data)
}

func (m *Manager) LoadError() error { m.mu.RLock(); defer m.mu.RUnlock(); return m.loadErr }

func (m *Manager) SetBangumiToken(token, uid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := cloneConfig(m.Data)
	next.GlobalSettings.BangumiApiToken = token
	next.GlobalSettings.BangumiUserID = uid
	if err := m.writeLocked(next); err != nil {
		return err
	}
	m.Data = next
	return nil
}

func (m *Manager) SetAccountState(user string, disabled *bool, quotaUntil string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := cloneConfig(m.Data)
	if disabled != nil {
		if next.GlobalSettings.DisabledAccounts == nil {
			next.GlobalSettings.DisabledAccounts = map[string]bool{}
		}
		next.GlobalSettings.DisabledAccounts[user] = *disabled
	} else {
		if next.GlobalSettings.QuotaBlockedUntil == nil {
			next.GlobalSettings.QuotaBlockedUntil = map[string]string{}
		}
		next.GlobalSettings.QuotaBlockedUntil[user] = quotaUntil
	}
	if err := m.writeLocked(next); err != nil {
		return err
	}
	m.Data = next
	return nil
}

func cloneConfig(data AppConfig) AppConfig {
	data.GlobalSettings.PikPakUsers = slices.Clone(data.GlobalSettings.PikPakUsers)
	data.GlobalSettings.ChineseWeekdays = slices.Clone(data.GlobalSettings.ChineseWeekdays)
	data.GlobalSettings.DisabledAccounts = maps.Clone(data.GlobalSettings.DisabledAccounts)
	data.GlobalSettings.QuotaBlockedUntil = maps.Clone(data.GlobalSettings.QuotaBlockedUntil)
	data.SeasonalFetcher.TargetMonths = slices.Clone(data.SeasonalFetcher.TargetMonths)
	return data
}

// Portable releases use the executable directory, independent of launch cwd.
// Development go run/test binaries use the project cwd; an explicit root wins.
func DataDir() string {
	if root := os.Getenv("ULTIMATEANIME_DATA_DIR"); root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			return abs
		}
	}
	executable, err := os.Executable()
	if err == nil && !strings.HasSuffix(executable, ".test") && !strings.Contains(executable, "go-build") {
		directory := filepath.Dir(executable)
		if filepath.Base(directory) == "bin" && filepath.Base(filepath.Dir(directory)) == "build" {
			project := filepath.Dir(filepath.Dir(directory))
			if _, err := os.Stat(filepath.Join(project, "go.mod")); err == nil {
				return project
			}
		}
		return directory
	}
	cwd, _ := os.Getwd()
	return cwd
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
