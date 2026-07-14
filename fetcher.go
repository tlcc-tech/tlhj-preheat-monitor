package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	voteInfoURL   = "https://tlhj-activity.changyou.com/tlgl/routineServer/voteInfo?phone=&code="
	activityReferer = "https://tlhj-activity.changyou.com/tlhj/preheat/20211215/pc/index.shtml"
)

type voteInfoResponse struct {
	Code json.Number `json:"code"`
	Data struct {
		ReserveCount int `json:"reserveCount"`
	} `json:"data"`
	Message string `json:"message"`
}

func fetchReserveCount(ctx context.Context, client *http.Client) (int, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, voteInfoURL, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Referer", activityReferer)
	req.Header.Set("APP", "tlgl")
	req.Header.Set("ACTIVITY", "routineserver")
	req.Header.Set("VERSIONCODE", "20211202")
	req.Header.Set("PLAT", "phone")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, errors.New("HTTP " + resp.Status + ": " + strings.TrimSpace(string(body)))
	}

	var parsed voteInfoResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, err
	}

	codeStr := strings.TrimSpace(parsed.Code.String())
	if codeStr != "10000" {
		msg := strings.TrimSpace(parsed.Message)
		if msg == "" {
			msg = string(body)
		}
		return 0, fmt.Errorf("API code=%s: %s", codeStr, msg)
	}

	return parsed.Data.ReserveCount, nil
}
