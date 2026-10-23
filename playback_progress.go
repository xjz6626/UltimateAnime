package main

import (
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"time"
)

type ViewedRange struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}
type PlaybackProgress struct {
	SessionID    string        `json:"session_id"`
	MediaVersion string        `json:"media_version"`
	Position     float64       `json:"position"`
	Duration     float64       `json:"duration"`
	Ranges       []ViewedRange `json:"viewed_ranges,omitempty"`
	Completed    bool          `json:"completed"`
	Hidden       bool          `json:"hidden,omitempty"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

type ContinueWatchingItem struct {
	SubjectID int       `json:"subject_id"`
	Episode   float64   `json:"ep_sort"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Position  float64   `json:"position"`
	Duration  float64   `json:"duration"`
	Available bool      `json:"available"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (a *App) absoluteMediaPath(path string) string {
	if path != "" && !filepath.IsAbs(path) {
		return filepath.Join(a.dataDir(), path)
	}
	return path
}
func mediaVersion(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("本地视频不是普通文件")
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", filepath.Clean(path), info.Size(), info.ModTime().UnixNano())))), nil
}
func validSeconds(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func mergeViewed(ranges []ViewedRange, start, end float64) []ViewedRange {
	if !validSeconds(start) || !validSeconds(end) || end <= start {
		return ranges
	}
	merged := append(slices.Clone(ranges), ViewedRange{start, end})
	slices.SortFunc(merged, func(a, b ViewedRange) int {
		if a.Start < b.Start {
			return -1
		}
		if a.Start > b.Start {
			return 1
		}
		return 0
	})
	out := make([]ViewedRange, 0, len(merged))
	for _, r := range merged {
		if !validSeconds(r.Start) || !validSeconds(r.End) || r.End <= r.Start {
			continue
		}
		if len(out) > 0 && r.Start <= out[len(out)-1].End+0.05 {
			out[len(out)-1].End = max(out[len(out)-1].End, r.End)
		} else {
			out = append(out, r)
		}
	}
	return out
}
func watchedDuration(p PlaybackProgress) float64 {
	var total float64
	for _, r := range p.Ranges {
		total += max(0, min(r.End, p.Duration)-r.Start)
	}
	return total
}

// Save only the media and session that this MPV instance was launched for.
func (a *App) savePlayback(subjectID int, episode float64, path string, p PlaybackProgress, markWatched bool) error {
	marked := false
	result := a.updateLibraryItem(subjectID, func(item *FollowedItem) error {
		key := fmt.Sprint(episode)
		if a.absoluteMediaPath(item.LocalFiles[key]) != path {
			return fmt.Errorf("视频记录已更换，停止保存旧播放进度")
		}
		version, err := mediaVersion(path)
		if err != nil {
			return err
		}
		if version != p.MediaVersion {
			return fmt.Errorf("视频版本已更换，停止保存旧播放进度")
		}
		current, ok := item.Playback[key]
		if ok && current.SessionID != p.SessionID {
			return fmt.Errorf("已有新的播放会话")
		}
		p.Hidden = current.Hidden
		if item.Playback == nil {
			item.Playback = map[string]PlaybackProgress{}
		}
		item.Playback[key] = p
		_, manual := item.WatchOverrides[key]
		if markWatched && !manual && !slices.Contains(item.WatchedEps, episode) {
			item.WatchedEps = append(item.WatchedEps, episode)
			marked = true
		}
		return nil
	})
	if result != "Success" {
		return fmt.Errorf("%s", result)
	}
	a.emit("playback-progress", map[string]any{"subject_id": subjectID, "ep_sort": episode, "progress": p, "marked_watched": marked})
	return nil
}

func (a *App) GetContinueWatching() ([]ContinueWatchingItem, error) {
	list, err := a.readLibrary()
	if err != nil {
		return nil, err
	}
	result := []ContinueWatchingItem{}
	for _, item := range list {
		var latest *ContinueWatchingItem
		for key, p := range item.Playback {
			if p.Hidden || p.Completed || p.Position < 1 {
				continue
			}
			var episode float64
			if _, err := fmt.Sscan(key, &episode); err != nil {
				continue
			}
			// Manual watch choices override the automatic continue list too.
			if item.WatchOverrides[key] {
				continue
			}
			path := a.absoluteMediaPath(item.LocalFiles[key])
			version, statErr := mediaVersion(path)
			name := item.NameCN
			if name == "" {
				name = item.Name
			}
			next := ContinueWatchingItem{SubjectID: item.SubjectID, Episode: episode, Name: name, Image: item.Image, Position: p.Position, Duration: p.Duration, Available: statErr == nil && version == p.MediaVersion, UpdatedAt: p.UpdatedAt}
			if latest == nil || latest.UpdatedAt.Before(next.UpdatedAt) {
				latest = &next
			}
		}
		if latest != nil {
			result = append(result, *latest)
		}
	}
	slices.SortFunc(result, func(a, b ContinueWatchingItem) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return result, nil
}

func (a *App) HideContinueWatching(subjectID int, episode float64) string {
	return a.updateLibraryItem(subjectID, func(item *FollowedItem) error {
		key := fmt.Sprint(episode)
		p, ok := item.Playback[key]
		if !ok {
			return fmt.Errorf("未找到播放进度")
		}
		p.Hidden = true
		item.Playback[key] = p
		return nil
	})
}
