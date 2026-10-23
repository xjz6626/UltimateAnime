package main

import (
	"reflect"
	"testing"
)

func TestQuotedPlayerArgs(t *testing.T) {
	args, err := parsePlayerArgs(`--fullscreen --title="番剧 第 1 集" --config-dir="C:\Program Files\mpv" --script='a b.lua'`)
	want := []string{"--fullscreen", "--title=番剧 第 1 集", `--config-dir=C:\Program Files\mpv`, "--script=a b.lua"}
	if err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("args=%q err=%v", args, err)
	}
	if _, err := parsePlayerArgs(`--title="broken`); err == nil {
		t.Fatal("unclosed quote accepted")
	}
}
