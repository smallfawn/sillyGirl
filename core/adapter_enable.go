package core

import (
	"strings"

	"github.com/smallfawn/sillyGirl/core/storage"
)

func init() {
	for _, platform := range []string{"clawbot", "dingtalk", "pagermaid", "qqguild"} {
		platform := platform
		storage.Watch(MakeBucket(platform), "enable", func(old, new, key string) *storage.Final {
			if !adapterConfigEnabledValue(new) {
				DestroyAdaptersByPlatform(platform)
			}
			return nil
		})
	}
}

func AdapterConfigEnabled(platform string) bool {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" || platform == "web" {
		return true
	}
	return adapterConfigEnabledValue(MakeBucket(platform).GetString("enable"))
}

func adapterConfigEnabledValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0", "off", "no":
		return false
	default:
		return true
	}
}

func DestroyAdaptersByPlatform(platform string) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return
	}
	BotsLocker.RLock()
	items := make([]*Factory, 0)
	for key, bot := range Bots {
		if strings.EqualFold(key[0], platform) {
			items = append(items, bot)
		}
	}
	BotsLocker.RUnlock()
	for _, bot := range items {
		bot.Destroy()
	}
}

func AdapterConfigManageable(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "clawbot", "dingtalk", "qq", "qqguild", "telegram", "pagermaid":
		return true
	default:
		return false
	}
}
