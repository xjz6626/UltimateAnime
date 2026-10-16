package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"UltimateAnime/pkg/bilibili"
)

type DanmakuChoice struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	CID       int64  `json:"cid"`
	Title     string `json:"title"`
	PartTitle string `json:"part_title"`
}

type DanmakuPreferences struct {
	Enabled    bool `json:"enabled"`
	AutoSelect bool `json:"auto_select"`
}

type DanmakuSelectionContext struct {
	Names  []string      `json:"names"`
	Choice DanmakuChoice `json:"choice"`
}

func (a *App) GetDanmakuPreferences() DanmakuPreferences {
	if a.configMgr == nil {
		return DanmakuPreferences{}
	}
	cfg := a.configMgr.Snapshot().Danmaku
	return DanmakuPreferences{Enabled: cfg.Enabled, AutoSelect: cfg.AutoSelect}
}

func (a *App) danmakuChoicePath(subjectID int, episode float64) (string, error) {
	if subjectID <= 0 || episode <= 0 || episode > 10000 || math.IsNaN(episode) || math.IsInf(episode, 0) {
		return "", fmt.Errorf("番剧或集数无效")
	}
	return filepath.Join(a.getCacheDir(), fmt.Sprintf("bilibili_choice_%d_%s.json", subjectID, strconv.FormatFloat(episode, 'f', -1, 64))), nil
}

func (a *App) loadDanmakuChoice(subjectID int, episode float64) (DanmakuChoice, error) {
	path, err := a.danmakuChoicePath(subjectID, episode)
	if err != nil {
		return DanmakuChoice{}, err
	}
	a.danmakuMu.Lock()
	defer a.danmakuMu.Unlock()
	var choice DanmakuChoice
	_, err = a.loadJSONCache(path, &choice)
	return choice, err
}

func (a *App) GetEpisodeDanmakuSource(subjectID int, episode float64) (DanmakuSelectionContext, error) {
	choice, err := a.loadDanmakuChoice(subjectID, episode)
	if err != nil {
		return DanmakuSelectionContext{}, err
	}
	for _, follow := range a.GetLocalFollows() {
		if follow.SubjectID == subjectID {
			return DanmakuSelectionContext{Names: []string{follow.NameCN, follow.Name}, Choice: choice}, nil
		}
	}
	return DanmakuSelectionContext{}, fmt.Errorf("请先追番")
}

func (a *App) SearchBilibiliDanmaku(keyword, kind string, page int) ([]bilibili.Candidate, error) {
	client, err := a.newBilibiliClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := a.bilibiliContext()
	defer cancel()
	return client.Search(ctx, keyword, kind, page)
}

func (a *App) GetBilibiliDanmakuParts(kind, id string) (bilibili.SourceDetails, error) {
	client, err := a.newBilibiliClient()
	if err != nil {
		return bilibili.SourceDetails{}, err
	}
	ctx, cancel := a.bilibiliContext()
	defer cancel()
	return client.Details(ctx, kind, id)
}

func (a *App) SaveEpisodeDanmakuSource(subjectID int, episode float64, choice DanmakuChoice) string {
	path, err := a.danmakuChoicePath(subjectID, episode)
	if err != nil {
		return "Error: " + err.Error()
	}
	var localFile string
	found := false
	for _, follow := range a.GetLocalFollows() {
		if follow.SubjectID == subjectID {
			found = true
			localFile = follow.LocalFiles[fmt.Sprint(episode)]
			break
		}
	}
	if !found {
		return "Error: 请先追番"
	}
	if choice.Kind == "auto" || choice.Kind == "skip" {
		choice = DanmakuChoice{Kind: choice.Kind}
	} else {
		details, err := a.GetBilibiliDanmakuParts(choice.Kind, choice.ID)
		if err != nil {
			return "Error: " + err.Error()
		}
		valid := false
		for _, part := range details.Parts {
			if part.CID == choice.CID && part.CID > 0 {
				valid = true
				choice.Title = details.Title
				choice.PartTitle = part.Title
				break
			}
		}
		if !valid {
			return "Error: 所选集数或分 P 不属于该视频"
		}
	}
	a.danmakuMu.Lock()
	defer a.danmakuMu.Unlock()
	var previous DanmakuChoice
	_, _ = a.loadJSONCache(path, &previous)
	if err := a.saveJSONCache(path, choice); err != nil {
		return "Error: 保存弹幕来源失败"
	}
	if localFile != "" && (choice.Kind != previous.Kind || choice.ID != previous.ID || choice.CID != previous.CID) {
		if err := os.Remove(danmakuPath(localFile)); err != nil && !os.IsNotExist(err) {
			return "Error: 来源已保存，但清理旧弹幕失败"
		}
	}
	return "Success"
}

func (a *App) RefreshEpisodeDanmaku(subjectID int, episode float64) string {
	if !a.GetDanmakuPreferences().Enabled {
		return "Error: 请先开启弹幕功能"
	}
	if _, err := a.danmakuChoicePath(subjectID, episode); err != nil {
		return "Error: " + err.Error()
	}
	for _, follow := range a.GetLocalFollows() {
		if follow.SubjectID != subjectID {
			continue
		}
		file := follow.LocalFiles[fmt.Sprint(episode)]
		if file == "" {
			return "Success"
		} // Selection will apply when the video is downloaded.
		if _, err := os.Stat(file); err != nil {
			return "Error: 本地视频不存在"
		}
		go func() {
			ctx, cancel := a.bilibiliContext()
			defer cancel()
			a.prepareDanmaku(ctx, subjectID, episode, file, true)
		}()
		return "Started"
	}
	return "Error: 未找到追番记录"
}
