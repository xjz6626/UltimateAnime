package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAtomicPrivateJSONBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := WriteJSON(path, map[string]int{"version": 1}); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(path, map[string]int{"version": 2}); err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]int{path: 2, path + ".bak": 1} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]int
		if json.Unmarshal(data, &got) != nil || got["version"] != want {
			t.Fatalf("%s: %s", file, data)
		}
		info, _ := os.Stat(file)
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
			t.Fatalf("permissions: %v", info.Mode())
		}
	}
}
