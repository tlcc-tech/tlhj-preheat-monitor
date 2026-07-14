package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const pollIntervalSec = 300

type MonitorStatus struct {
	Running     bool   `json:"running"`
	LastCount   int    `json:"lastCount"`
	LastChecked string `json:"lastChecked"`
	LastError   string `json:"lastError"`
	NextPollIn  int    `json:"nextPollIn"`
}

type AppSettings struct {
	ChannelKey string `json:"channelKey"`
}

type countEvent struct {
	Count int    `json:"count"`
	At    string `json:"at"`
}

type Monitor struct {
	mu sync.Mutex

	appCtx context.Context

	running bool
	cancel  context.CancelFunc

	channelKey    string
	lastMilestone int
	lastCount     int
	lastChecked   time.Time
	lastError     string
	nextPollAt    time.Time

	history []HistoryPoint

	httpClient *http.Client
}

func NewMonitor() *Monitor {
	return &Monitor{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (m *Monitor) Attach(appCtx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appCtx = appCtx

	if s, err := loadSettings(); err == nil {
		m.channelKey = strings.TrimSpace(s.ChannelKey)
		m.lastMilestone = s.LastMilestone
	}
	if h, err := loadHistory(); err == nil {
		m.history = h
		if len(h) > 0 {
			last := h[len(h)-1]
			m.lastCount = last.Count
			if t, err := time.Parse(time.RFC3339, last.At); err == nil {
				m.lastChecked = t
			}
		}
	}
}

func (m *Monitor) GetSettings() AppSettings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return AppSettings{ChannelKey: m.channelKey}
}

func (m *Monitor) GetHistory(limit int) []HistoryPoint {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limit <= 0 || limit >= len(m.history) {
		out := make([]HistoryPoint, len(m.history))
		copy(out, m.history)
		return out
	}
	start := len(m.history) - limit
	out := make([]HistoryPoint, limit)
	copy(out, m.history[start:])
	return out
}

func (m *Monitor) Status() MonitorStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	status := MonitorStatus{
		Running:   m.running,
		LastCount: m.lastCount,
		LastError: m.lastError,
	}
	if !m.lastChecked.IsZero() {
		status.LastChecked = m.lastChecked.Format(time.RFC3339)
	}
	if m.running && !m.nextPollAt.IsZero() {
		sec := int(time.Until(m.nextPollAt).Seconds())
		if sec < 0 {
			sec = 0
		}
		status.NextPollIn = sec
	}
	return status
}

func (m *Monitor) Start(channelKey string) error {
	channelKey = strings.TrimSpace(channelKey)

	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return errors.New("监控已在运行")
	}
	m.running = true
	m.channelKey = channelKey
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	appCtx := m.appCtx
	m.mu.Unlock()

	_ = saveSettings(persistedSettings{
		ChannelKey:    channelKey,
		LastMilestone: m.getLastMilestone(),
	})

	m.emitLog(appCtx, "INFO", "监控已启动，每 5 分钟采集一次")
	if channelKey == "" {
		m.emitLog(appCtx, "WARN", "未填写推送链接/Key：将跳过微信推送")
	}

	go func() {
		defer func() {
			m.mu.Lock()
			m.running = false
			m.cancel = nil
			m.mu.Unlock()
			m.emitLog(appCtx, "INFO", "监控已停止")
		}()

		if err := m.pollOnce(ctx, appCtx); err != nil {
			m.emitLog(appCtx, "ERROR", "采集失败: "+err.Error())
		}

		for {
			nextAt := time.Now().Add(pollIntervalSec * time.Second)
			m.mu.Lock()
			m.nextPollAt = nextAt
			m.mu.Unlock()

			m.emitLog(appCtx, "INFO", fmt.Sprintf("下次采集将在 %s 后", (pollIntervalSec * time.Second).String()))

			t := time.NewTimer(pollIntervalSec * time.Second)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}

			if err := m.pollOnce(ctx, appCtx); err != nil {
				m.emitLog(appCtx, "ERROR", "采集失败: "+err.Error())
			}
		}
	}()

	return nil
}

func (m *Monitor) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	m.running = false
	m.cancel = nil
	appCtx := m.appCtx
	m.mu.Unlock()

	if cancel != nil {
		m.emitLog(appCtx, "INFO", "收到停止请求")
		cancel()
	}
}

// ResetData clears local history and milestone baseline so recording starts fresh.
// Returns true when data was reset, false when the user cancelled.
func (m *Monitor) ResetData() (bool, error) {
	m.mu.Lock()
	appCtx := m.appCtx
	m.mu.Unlock()

	if appCtx != nil {
		sel, err := runtime.MessageDialog(appCtx, runtime.MessageDialogOptions{
			Type:          runtime.QuestionDialog,
			Title:         "重置数据",
			Message:       "确定重置数据？将清空本地历史记录与里程碑，并重新开始记录。",
			Buttons:       []string{"重置", "取消"},
			DefaultButton: "取消",
			CancelButton:  "取消",
		})
		if err != nil {
			return false, err
		}
		// macOS/Windows may return custom labels or Yes/No depending on runtime.
		if sel != "重置" && sel != "Yes" && sel != "OK" && sel != "Ok" {
			return false, nil
		}
	}

	if err := clearHistory(); err != nil {
		return false, err
	}

	m.mu.Lock()
	m.history = []HistoryPoint{}
	m.lastCount = 0
	m.lastChecked = time.Time{}
	m.lastError = ""
	m.lastMilestone = 0
	channelKey := m.channelKey
	m.mu.Unlock()

	if err := saveSettings(persistedSettings{
		ChannelKey:    channelKey,
		LastMilestone: 0,
	}); err != nil {
		return false, err
	}

	m.emitLog(appCtx, "INFO", "已重置历史记录与里程碑，将重新开始记录")
	return true, nil
}

func (m *Monitor) getLastMilestone() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastMilestone
}

func (m *Monitor) pollOnce(ctx context.Context, appCtx context.Context) error {
	count, err := fetchReserveCount(ctx, m.httpClient)
	now := time.Now()

	m.mu.Lock()
	m.lastChecked = now
	if err != nil {
		m.lastError = err.Error()
		m.mu.Unlock()
		return err
	}
	m.lastError = ""
	m.lastCount = count
	m.mu.Unlock()

	at := now.Format(time.RFC3339)
	point := HistoryPoint{At: at, Count: count}

	history, err := appendHistory(point)
	if err != nil {
		m.emitLog(appCtx, "WARN", "保存历史记录失败: "+err.Error())
	} else {
		m.mu.Lock()
		m.history = history
		m.mu.Unlock()
	}

	m.emitLog(appCtx, "INFO", fmt.Sprintf("预约人数：%d", count))
	runtime.EventsEmit(appCtx, "count", countEvent{Count: count, At: at})

	m.handleMilestones(ctx, appCtx, count)
	return nil
}

func milestoneFor(count int) int {
	return (count / 1000) * 1000
}

func (m *Monitor) handleMilestones(ctx context.Context, appCtx context.Context, count int) {
	current := milestoneFor(count)

	m.mu.Lock()
	last := m.lastMilestone
	channelKey := m.channelKey
	m.mu.Unlock()

	if last == 0 {
		m.mu.Lock()
		m.lastMilestone = current
		m.mu.Unlock()
		_ = saveSettings(persistedSettings{
			ChannelKey:    channelKey,
			LastMilestone: current,
		})
		m.emitLog(appCtx, "INFO", fmt.Sprintf("已建立里程碑基线：%d（当前 %d 人）", current, count))
		return
	}

	if current <= last {
		return
	}

	for milestone := last + 1000; milestone <= current; milestone += 1000 {
		if strings.TrimSpace(channelKey) == "" {
			m.emitLog(appCtx, "INFO", fmt.Sprintf("已达 %d 人里程碑，未配置推送已跳过", milestone))
			continue
		}

		title := fmt.Sprintf("新服预约人数突破 %d", milestone)
		content := buildMilestonePushContent(milestone, count)
		if err := sendWechatPush(ctx, channelKey, title, content); err != nil {
			m.emitLog(appCtx, "ERROR", fmt.Sprintf("里程碑 %d 推送失败: %s", milestone, err.Error()))
		} else {
			m.emitLog(appCtx, "INFO", fmt.Sprintf("里程碑 %d 微信推送成功", milestone))
		}
	}

	m.mu.Lock()
	m.lastMilestone = current
	m.mu.Unlock()

	_ = saveSettings(persistedSettings{
		ChannelKey:    channelKey,
		LastMilestone: current,
	})
}

func (m *Monitor) emitLog(appCtx context.Context, level string, msg string) {
	if appCtx == nil {
		return
	}
	line := time.Now().Format("2006-01-02 15:04:05") + " [" + level + "] " + msg
	runtime.EventsEmit(appCtx, "log", line)
}
