package core

import (
	"context"
	"fmt"

	"github.com/smallfawn/sillyGirl/core/logs"
	"github.com/smallfawn/sillyGirl/proto3/srpc"
	"github.com/smallfawn/sillyGirl/utils"
)

func GetScriptNameByUUID(uuid string) string {
	for _, f := range Functions {
		if f.UUID == uuid {
			return fmt.Sprintf("%s%s", f.Title, f.Suffix)
		}
	}
	return "未知脚本"
}

type Console struct {
	UUID string
}

var console = &Console{}
var Logs = &Console{}

func (sg *SillyGirlService) Console(ctx context.Context, req *srpc.ConsoleRequest) (*srpc.Empty, error) {
	log := &Console{
		UUID: req.PluginId,
	}
	switch req.Type {
	case "info", "log":
		log.Info(req.Content)
	case "error":
		log.Error(req.Content)
	case "debug":
		log.Debug(req.Content)
	case "warn":
		log.Warn(req.Content)
	}
	return &srpc.Empty{}, nil
}

func pluginConsole(uuid string) *Console {
	return &Console{
		UUID: uuid,
	}
}

func Broadcast2WebUser(content, class string) {
	if RegistFuncs["Broadcast2WebUser"] == nil {
		return
	}
	RegistFuncs["Broadcast2WebUser"].(func(string, string))(content, class)
}

func (c *Console) Info(v ...interface{}) {
	if len(v) == 0 {
		return
	}
	log := utils.FormatLog(v[0], v[1:]...)
	logs.Info(log)
	Broadcast2WebUser(log, "info")
}

func (c *Console) Debug(v ...interface{}) {
	if len(v) == 0 {
		return
	}
	if c.UUID != "" && plugin_debug.GetString(c.UUID) != "b:true" {
		return
	}
	log := utils.FormatLog(v[0], v[1:]...)
	logs.Debug(log)
	Broadcast2WebUser(log, "debug")
}

func (c *Console) Warn(v ...interface{}) {
	if len(v) == 0 {
		return
	}
	log := utils.FormatLog(v[0], v[1:]...)
	logs.Warn(log)
	WritePluginMessage(c.UUID, "warn", log)
	Broadcast2WebUser(log, "warn")
}

func (c *Console) Error(v ...interface{}) {
	if len(v) == 0 {
		return
	}
	log := utils.FormatLog(v[0], v[1:]...)
	logs.Error(log)
	WritePluginMessage(c.UUID, "error", log)
	Broadcast2WebUser(log, "error")
}

func (c *Console) Log(v ...interface{}) {
	if len(v) == 0 {
		return
	}
	log := utils.FormatLog(v[0], v[1:]...)
	logs.Info(log)
	Broadcast2WebUser(log, "log")
}
