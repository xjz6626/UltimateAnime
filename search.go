package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"UltimateAnime/pkg/crawler"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) episodeSearchKeywords(subjectID int, custom string) ([]string, error) {
	if custom = strings.TrimSpace(custom); custom != "" {
		return []string{custom}, nil
	}
	for _, follow := range a.GetLocalFollows() {
		if follow.SubjectID != subjectID {
			continue
		}
		var names []string
		for _, name := range []string{follow.NameCN, follow.Name} {
			if name = strings.TrimSpace(name); name != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			return names, nil
		}
	}
	return nil, fmt.Errorf("未找到追番名称")
}

type magnetSearchUpdate struct {
	RequestID string                `json:"request_id"`
	Items     []crawler.TorrentItem `json:"items"`
}

// The final return value also recovers any events missed while a browser was
// reconnecting. Request IDs keep simultaneous searches on different clients apart.
func (a *App) SearchEpisodeMagnetListStream(subjectID int, epSort float64, customKeywords, requestID string) ([]crawler.TorrentItem, error) {
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	return a.searchEpisodeMagnetListStream(parent, subjectID, epSort, customKeywords, requestID)
}

func (a *App) searchEpisodeMagnetListStream(parent context.Context, subjectID int, epSort float64, customKeywords, requestID string) ([]crawler.TorrentItem, error) {
	if strings.TrimSpace(requestID) == "" || len(requestID) > 128 {
		return nil, fmt.Errorf("无效的搜索请求 ID")
	}
	keywords, err := a.episodeSearchKeywords(subjectID, customKeywords)
	if err != nil {
		return nil, err
	}
	if a.crawler == nil {
		return nil, fmt.Errorf("搜索服务未初始化")
	}
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	a.searchMu.Lock()
	if a.searches == nil {
		a.searches = make(map[string]context.CancelFunc)
	}
	if _, exists := a.searches[requestID]; exists {
		a.searchMu.Unlock()
		return nil, fmt.Errorf("搜索请求 ID 已存在")
	}
	a.searches[requestID] = cancel
	a.searchMu.Unlock()
	defer func() {
		a.searchMu.Lock()
		delete(a.searches, requestID)
		a.searchMu.Unlock()
	}()
	a.Log("INFO", fmt.Sprintf("搜索磁力: ID=%d, Ep=%g, Keywords=%s", subjectID, epSort, strings.Join(keywords, " / ")))
	items, err := a.crawler.SearchEpisodeListStream(ctx, keywords, epSort, func(items []crawler.TorrentItem) {
		update := magnetSearchUpdate{RequestID: requestID, Items: items}
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "magnet-search-results", update)
		}
		a.webEvents.publish("magnet-search-results", update)
	})
	// A slow/unavailable source must not turn already usable results into an error.
	if len(items) > 0 {
		return items, nil
	}
	return items, err
}

func (a *App) CancelEpisodeMagnetSearch(requestID string) {
	a.searchMu.Lock()
	cancel := a.searches[requestID]
	a.searchMu.Unlock()
	if cancel != nil {
		cancel()
	}
}
