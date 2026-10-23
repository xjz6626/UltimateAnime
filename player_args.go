package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// parsePlayerArgs accepts quoted arguments without invoking a shell. Backslashes
// in Windows paths remain literal; only quote/backslash escaping is interpreted.
func parsePlayerArgs(value string) ([]string, error) {
	var args []string
	var word []rune
	var quote rune
	started := false
	runes := []rune(value)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\\' && i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\'' && quote != '"') {
			word = append(word, runes[i+1])
			i++
			started = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word = append(word, r)
			}
			started = true
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			started = true
			continue
		}
		if unicode.IsSpace(r) {
			if started {
				args = append(args, string(word))
				word = nil
				started = false
			}
			continue
		}
		word = append(word, r)
		started = true
	}
	if quote != 0 {
		return nil, fmt.Errorf("播放器参数引号未闭合")
	}
	if started {
		args = append(args, string(word))
	}
	return args, nil
}

func (a *App) playerExecutable(configured string) string {
	if configured != "" {
		if !filepath.IsAbs(configured) && strings.ContainsAny(configured, `/\`) {
			return filepath.Join(a.dataDir(), configured)
		}
		return configured
	}
	for _, relative := range []string{"mpv.exe", filepath.Join("bin", "mpv.exe")} {
		path := filepath.Join(a.dataDir(), relative)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return "mpv"
}
