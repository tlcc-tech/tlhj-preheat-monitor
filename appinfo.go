package main

var AppVersion = "1.1.0"

const (
	AppName   = "新服预约监控"
	AppAuthor = "怀旧天龙CC科技"
)

type AppInfo struct {
	Name    string `json:"name"`
	Author  string `json:"author"`
	Version string `json:"version"`
}
