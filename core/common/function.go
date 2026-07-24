package common

type Function struct {
	Rules          []string                 `json:"-"`
	Params         [][]string               `json:"-"`
	Admin          bool                     `json:"-"`
	Handle         func(Sender) interface{} `json:"-"`
	Cron           map[string]string        `json:"cron"`
	Priority       int                      `json:"-"`
	Disable        bool                     `json:"disable"`
	Hidden         bool                     `json:"-"`
	CronIds        []int                    `json:"-"`
	Origin         string                   `json:"-"`
	UUID           string                   `json:"id"`
	Title          string                   `json:"title"`
	Type           string                   `json:"type"`   //脚本类型
	Suffix         string                   `json:"suffix"` //脚本后缀
	Description    string                   `json:"desc"`
	Public         bool                     `json:"public"`
	Icon           string                   `json:"icon"`
	Version        string                   `json:"version"`
	CurrentVersion string                   `json:"current_version,omitempty"`
	LatestVersion  string                   `json:"latest_version,omitempty"`
	UpdateContent  string                   `json:"update_content,omitempty"`
	Author         string                   `json:"author"`
	Class          string                   `json:"class"`
	Status         int                      `json:"status"` //0未安装 1可更新 2已安装
	Address        string                   `json:"-"`
	CreateAt       string                   `json:"create_at"`
	Module         bool                     `json:"module"`
	OnStart        bool                     `json:"on_start"`
	Web            bool                     `json:"web"`
	PluginPublisher
	Running      bool        `json:"running"`
	Downloads    int         `json:"downloads"`
	HasForm      bool        `json:"has_form"`
	Carry        bool        `json:"carry"`
	Messages     interface{} `json:"messages"`
	Classes      []string    `json:"-"`
	Dependencies []string    `json:"dependencies,omitempty"`
	Debug        bool        `json:"debug"`
	Path         string      `json:"-"`
	Reload       func()      `json:"-"`
}

type PluginPublisher struct {
	Address      string `json:"address"`
	Organization string `json:"organization"`
	Identified   bool   `json:"identified"`
}
