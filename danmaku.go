package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"UltimateAnime/pkg/bilibili"
)

type danmakuSource interface {
	SearchSeason(context.Context, []string, int) (int, error)
	Season(context.Context, int) (bilibili.Season, error)
	Comments(context.Context, int64) ([]bilibili.Comment, error)
}

type danmakuSeasonCache struct {
	FetchedAt time.Time       `json:"fetched_at"`
	Names     []string        `json:"names"`
	Year      int             `json:"year"`
	Season    bilibili.Season `json:"season"`
}

func danmakuPath(video string) string {
	return strings.TrimSuffix(video, filepath.Ext(video)) + ".danmaku.ass"
}

func (a *App) prepareEpisodeDanmaku(parent context.Context, subjectID int, epSort float64, video string) {
	a.prepareDanmaku(parent, subjectID, epSort, video, false)
}

func (a *App) prepareDanmaku(parent context.Context, subjectID int, epSort float64, video string, refresh bool) {
	if a.configMgr == nil {
		return
	}
	cfg := a.configMgr.Snapshot()
	if !cfg.Danmaku.Enabled {
		return
	}
	choice, err := a.loadDanmakuChoice(subjectID, epSort)
	if err != nil {
		a.Log("WARN", "读取弹幕选择失败: "+err.Error())
		return
	}
	if choice.Kind == "skip" {
		if err := os.Remove(danmakuPath(video)); err != nil && !os.IsNotExist(err) {
			a.Log("WARN", "清理已禁用的弹幕失败: "+err.Error())
		}
		return
	}
	if info, err := os.Stat(danmakuPath(video)); !refresh && choice.Kind == "" && err == nil && info.Size() > 0 {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	source, err := bilibili.NewClient(cfg.GlobalSettings.Proxy, cfg.Danmaku.BilibiliCookie)
	if err == nil {
		err = a.downloadEpisodeDanmaku(ctx, source, subjectID, epSort, video)
	}
	if err != nil {
		a.Log("WARN", fmt.Sprintf("第 %g 集弹幕未下载，视频可正常下载和播放: %v", epSort, err))
	}
}

func (a *App) downloadEpisodeDanmaku(ctx context.Context, source danmakuSource, subjectID int, epSort float64, video string) error {
	var names []string
	year := 0
	for _, follow := range a.librarySnapshot() {
		if follow.SubjectID != subjectID {
			continue
		}
		for _, name := range []string{follow.NameCN, follow.Name} {
			if name = strings.TrimSpace(name); name != "" {
				names = append(names, name)
			}
		}
		if len(follow.AirDate) >= 4 {
			year, _ = strconv.Atoi(follow.AirDate[:4])
		}
		break
	}
	if len(names) == 0 {
		return fmt.Errorf("缺少追番名称，无法搜索 B 站番剧")
	}
	a.Log("INFO", fmt.Sprintf("搜索 B 站弹幕: %s 第 %g 集", names[0], epSort))
	choice, err := a.loadDanmakuChoice(subjectID, epSort)
	if err != nil {
		return err
	}
	if choice.Kind == "skip" {
		a.Log("INFO", fmt.Sprintf("第 %g 集已选择不下载弹幕", epSort))
		return nil
	}
	var episode bilibili.Episode
	if choice.Kind == "season" || choice.Kind == "video" {
		if choice.CID <= 0 {
			return fmt.Errorf("已保存的弹幕来源无效，请重新选择")
		}
		episode = bilibili.Episode{CID: choice.CID, Title: choice.PartTitle}
		a.Log("INFO", fmt.Sprintf("使用手选 B 站弹幕: %s / %s (cid=%d)", choice.Title, choice.PartTitle, choice.CID))
	} else {
		season, err := a.danmakuSeason(ctx, source, subjectID, names, year, epSort)
		if err != nil {
			return err
		}
		episode, err = season.Episode(epSort)
		if err != nil {
			return err
		}
		a.Log("INFO", fmt.Sprintf("已匹配 B 站番剧: %s / 第 %s 集 (cid=%d)", season.Title, episode.Title, episode.CID))
	}
	comments, err := source.Comments(ctx, episode.CID)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	output := danmakuPath(video)
	file, err := os.CreateTemp(filepath.Dir(output), ".danmaku-*.ass")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	count, err := bilibili.WriteASS(file, comments)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// A source can be changed while its previous comments are in flight. Only
	// publish an ASS if it still belongs to the current selection.
	a.danmakuMu.Lock()
	defer a.danmakuMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	choicePath, err := a.danmakuChoicePath(subjectID, epSort)
	if err != nil {
		return err
	}
	var latest DanmakuChoice
	if _, err := a.loadJSONCache(choicePath, &latest); err != nil {
		return err
	}
	if latest != choice {
		return fmt.Errorf("弹幕来源已更换，已取消旧来源的写入")
	}
	if err := os.Rename(file.Name(), output); err != nil {
		return err
	}
	a.Log("SUCCESS", fmt.Sprintf("已保存 %d 条 B 站弹幕: %s", count, output))
	return nil
}

func (a *App) danmakuSeason(ctx context.Context, source danmakuSource, subjectID int, names []string, year int, epSort float64) (bilibili.Season, error) {
	// Concurrent episode downloads share one successful season lookup and never
	// rewrite followed.json with Bilibili metadata.
	a.danmakuMu.Lock()
	defer a.danmakuMu.Unlock()
	if err := ctx.Err(); err != nil {
		return bilibili.Season{}, err
	}
	path := filepath.Join(a.getCacheDir(), fmt.Sprintf("bilibili_subject_%d.json", subjectID))
	var cache danmakuSeasonCache
	loaded, _ := a.loadJSONCache(path, &cache)
	loaded = loaded && cache.Season.ID > 0 && strings.Join(cache.Names, "\x00") == strings.Join(names, "\x00") && cache.Year == year
	if loaded && time.Since(cache.FetchedAt) < 24*time.Hour {
		if _, err := cache.Season.Episode(epSort); err == nil {
			return cache.Season, nil
		}
	}
	id := cache.Season.ID
	if !loaded {
		var err error
		id, err = source.SearchSeason(ctx, names, year)
		if err != nil {
			return bilibili.Season{}, err
		}
	}
	season, err := source.Season(ctx, id)
	if err != nil {
		return bilibili.Season{}, err
	}
	if err := a.saveJSONCache(path, danmakuSeasonCache{FetchedAt: time.Now(), Names: names, Year: year, Season: season}); err != nil {
		a.Log("WARN", fmt.Sprintf("保存 B 站番剧匹配缓存失败: %v", err))
	}
	return season, nil
}

func (a *App) localDanmakuArgs(video string) []string {
	if a.configMgr == nil || !a.configMgr.Snapshot().Danmaku.Enabled {
		return nil
	}
	path := danmakuPath(video)
	if info, err := os.Stat(path); err != nil || info.IsDir() || info.Size() == 0 {
		return nil
	}
	a.Log("INFO", "加载 B 站弹幕: "+path)
	// Keep the video's normal subtitle track available alongside the danmaku.
	return []string{"--sub-file=" + path, "--sub-ass-override=no", "--secondary-sid=auto", "--secondary-sub-pos=100"}
}
