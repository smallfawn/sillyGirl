package common

import "regexp"

type Function struct {
	Rules          []string                 `json:"-"`
	RulePatterns   []*regexp.Regexp         `json:"-"`
	RuleErrors     []error                  `json:"-"`
	Params         [][]string               `json:"-"`
	Admin          bool                     `json:"admin"`
	Handle         func(Sender) interface{} `json:"-"`
	Cron           map[string]string        `json:"cron"`
	Priority       int                      `json:"-"`
	Status         *bool                    `json:"status"`
	Hidden         bool                     `json:"-"`
	CronIds        []int                    `json:"-"`
	Origin         string                   `json:"-"`
	UUID           string                   `json:"id"`
	Title          string                   `json:"title"`
	Type           string                   `json:"type"`   //脚本类型
	Suffix         string                   `json:"suffix"` //脚本后缀
	Desc           string                   `json:"desc"`
	Rule           string                   `json:"rule,omitempty"`
	Public         bool                     `json:"public"`
	Open           bool                     `json:"open"`
	Icon           string                   `json:"icon"`
	Version        string                   `json:"version"`
	CurrentVersion string                   `json:"current_version,omitempty"`
	LatestVersion  string                   `json:"latest_version,omitempty"`
	UpdateContent  string                   `json:"update_content,omitempty"`
	Author         string                   `json:"author"`
	Class          string                   `json:"class"`
	InstallStatus  int                      `json:"install_status"` //0未安装 1可更新 2已安装
	Address        string                   `json:"-"`
	CreateAt       string                   `json:"create_at"`
	Module         bool                     `json:"module"`
	OnStart        bool                     `json:"on_start"`
	Web            bool                     `json:"web"`
	PluginPublisher
	Running            bool        `json:"running"`
	Downloads          int         `json:"downloads"`
	HasForm            bool        `json:"has_form"`
	HasUserForm        bool        `json:"has_user_form"`
	ConfigRegistered   bool        `json:"config_registered"`
	UsesSmallCat       bool        `json:"uses_smallcat"`
	Carry              bool        `json:"carry"`
	Messages           interface{} `json:"messages"`
	Classes            []string    `json:"-"`
	Dependencies       []string    `json:"dependencies,omitempty"`
	ModuleDependencies []string    `json:"module_dependencies,omitempty"`
	Debug              bool        `json:"debug"`
	Path               string      `json:"-"`
	Reload             func()      `json:"-"`
}

type PluginPublisher struct {
	Address      string `json:"address"`
	Organization string `json:"organization"`
	Identified   bool   `json:"identified"`
}
