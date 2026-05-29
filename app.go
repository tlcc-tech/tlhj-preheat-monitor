package main

import "context"

type App struct {
	ctx     context.Context
	monitor *Monitor
}

func NewApp() *App {
	return &App{monitor: NewMonitor()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.monitor.Attach(ctx)
}

func (a *App) StartMonitoring(channelKey string) error {
	return a.monitor.Start(channelKey)
}

func (a *App) StopMonitoring() {
	a.monitor.Stop()
}

func (a *App) GetStatus() MonitorStatus {
	return a.monitor.Status()
}

func (a *App) GetSettings() AppSettings {
	return a.monitor.GetSettings()
}

func (a *App) GetHistory(limit int) []HistoryPoint {
	return a.monitor.GetHistory(limit)
}

func (a *App) GetAppInfo() AppInfo {
	return AppInfo{Name: AppName, Author: AppAuthor, Version: AppVersion}
}
