package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Detect returns the session ID from the most recently modified Claude Code
// JSONL file across all project directories (~/.claude/projects/*/*.jsonl).
func Detect() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	projectsDir := filepath.Join(home, ".claude", "projects")

	var files []fileInfo
	err = filepath.WalkDir(projectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable dirs
		}
		if d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, fileInfo{path: path, modTime: info.ModTime().UnixNano()})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk projects: %w", err)
	}
	if len(files) == 0 {
		return "", errors.New("no JSONL session files found")
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	return readSessionID(files[0].path)
}

type fileInfo struct {
	path    string
	modTime int64
}

func readSessionID(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var lastID string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var entry struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(line, &entry); err == nil && entry.SessionID != "" {
			lastID = entry.SessionID
		}
	}
	if lastID == "" {
		return "", fmt.Errorf("no sessionId found in %s", path)
	}
	return lastID, nil
}
