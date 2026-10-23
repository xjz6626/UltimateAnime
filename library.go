package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"UltimateAnime/pkg/storage"
)

func (a *App) dataDir() string {
	if a.configMgr != nil && a.configMgr.ConfigPath != "" {
		return filepath.Dir(a.configMgr.ConfigPath)
	}
	cwd, _ := os.Getwd()
	return cwd
}

func (a *App) readLibraryLocked() ([]FollowedItem, error) {
	path := filepath.Join(a.dataDir(), "followed.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []FollowedItem{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取片库失败: %w", err)
	}
	var list []FollowedItem
	if err := json.Unmarshal(data, &list); err != nil || list == nil {
		return nil, fmt.Errorf("片库 %s 损坏；请修复文件或从 .bak 恢复，原文件已保留", path)
	}
	if list == nil {
		list = []FollowedItem{}
	}
	return list, nil
}

func (a *App) readLibrary() ([]FollowedItem, error) {
	a.followMu.Lock()
	defer a.followMu.Unlock()
	return a.readLibraryLocked()
}

// GetLocalFollowsChecked propagates load errors to both desktop and web UIs.
func (a *App) GetLocalFollowsChecked() ([]FollowedItem, error) {
	list, err := a.readLibrary()
	if err != nil {
		return nil, err
	}
	follows := make([]FollowedItem, 0, len(list))
	for _, item := range list {
		if !item.Archived {
			follows = append(follows, item)
		}
	}
	return follows, nil
}

// Retained for callers that only need a best-effort schedule snapshot.
func (a *App) GetLocalFollows() []FollowedItem {
	list, err := a.GetLocalFollowsChecked()
	if err != nil {
		a.Log("ERROR", err.Error())
	}
	return list
}

func (a *App) updateLibrary(change func(*[]FollowedItem) error) string {
	a.followMu.Lock()
	defer a.followMu.Unlock()
	list, err := a.readLibraryLocked()
	if err == nil {
		err = change(&list)
	}
	if err == nil {
		err = storage.WriteJSON(filepath.Join(a.dataDir(), "followed.json"), list)
	}
	if err != nil {
		return "Error: " + err.Error()
	}
	return "Success"
}

func (a *App) updateLibraryItem(id int, change func(*FollowedItem) error) string {
	return a.updateLibrary(func(list *[]FollowedItem) error {
		for i := range *list {
			if (*list)[i].SubjectID == id {
				return change(&(*list)[i])
			}
		}
		return fmt.Errorf("未找到片库记录，请先追番")
	})
}

// Used for importing complete snapshots, never for read-modify-write operations.
func (a *App) saveFollowedList(list []FollowedItem) string {
	return a.updateLibrary(func(current *[]FollowedItem) error { *current = list; return nil })
}

func (a *App) librarySnapshot() []FollowedItem {
	list, err := a.readLibrary()
	if err != nil {
		a.Log("ERROR", err.Error())
	}
	return list
}
