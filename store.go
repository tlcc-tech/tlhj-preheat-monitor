package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	storeDirName      = "tlhj-preheat-monitor"
	settingsFileName  = "settings.json"
	historyFileName   = "history.json"
	maxHistoryRecords = 5000
)

type persistedSettings struct {
	ChannelKey    string `json:"channelKey"`
	LastMilestone int    `json:"lastMilestone"`
	UpdatedAt     string `json:"updatedAt"`
}

type HistoryPoint struct {
	At    string `json:"at"`
	Count int    `json:"count"`
}

func storeDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", errors.New("无法获取用户配置目录")
	}
	return filepath.Join(dir, storeDirName), nil
}

func settingsPath() (string, error) {
	dir, err := storeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, settingsFileName), nil
}

func historyPath() (string, error) {
	dir, err := storeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, historyFileName), nil
}

func loadSettings() (persistedSettings, error) {
	path, err := settingsPath()
	if err != nil {
		return persistedSettings{}, err
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return persistedSettings{}, nil
		}
		return persistedSettings{}, err
	}

	var s persistedSettings
	if err := json.Unmarshal(b, &s); err != nil {
		return persistedSettings{}, err
	}
	return s, nil
}

func saveSettings(s persistedSettings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	s.UpdatedAt = time.Now().Format(time.RFC3339)
	return writeJSONAtomic(path, s)
}

func loadHistory() ([]HistoryPoint, error) {
	path, err := historyPath()
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []HistoryPoint{}, nil
		}
		return nil, err
	}

	var h []HistoryPoint
	if err := json.Unmarshal(b, &h); err != nil {
		return nil, err
	}
	return h, nil
}

func saveHistory(h []HistoryPoint) error {
	if len(h) > maxHistoryRecords {
		h = h[len(h)-maxHistoryRecords:]
	}
	path, err := historyPath()
	if err != nil {
		return err
	}
	return writeJSONAtomic(path, h)
}

func appendHistory(point HistoryPoint) ([]HistoryPoint, error) {
	h, err := loadHistory()
	if err != nil {
		return nil, err
	}
	h = append(h, point)
	if err := saveHistory(h); err != nil {
		return nil, err
	}
	return h, nil
}

func clearHistory() error {
	return saveHistory([]HistoryPoint{})
}

func writeJSONAtomic(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
