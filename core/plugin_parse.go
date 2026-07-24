package core

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/utils"
)

func pluginParse(script string, uuid string) (*common.Function, []func()) {
	var cbs = []func(){}
	var rules []string
	var admin bool
	var disable bool = plugin_disable.GetString(uuid) == "b:true"
	var priority int
	var title string
	var public bool
	var description string
	var icon string
	var version string = "v1.0.0"
	var author string
	var module bool
	var onStart bool
	var web bool
	var origin = "自定义"
	var crons = map[string]string{}
	var hasForm bool
	var carry bool
	var classes = []string{}
	ks := map[string]bool{}
	ress := regexp.MustCompile(
		`\*\s?@([\d\w+-]+)(?:\s+([^\n]+?))?\n`,
	).FindAllStringSubmatch(script, -1)
	for _, res := range ress {
		switch res[1] {
		case "rule":
			rule := strings.TrimSpace(res[2])
			rule = parseReply3(rule, func(s1, s2 string) {
				k := s1 + "." + s2
				if _, ok := ks[k]; !ok { //已改成表单提交触发
					// cbs = append(cbs, func() {
					// 	storage.Watch(MakeBucket(s1), s2, func(old, new, key string) *storage.Final {
					// 		return &storage.Final{
					// 			EndFunc: func() {
					// 				plugins.Set(uuid, "reload")
					// 			},
					// 		}
					// 	}, uuid)
					// })
					ks[k] = true
				}
			})
			_rs := []string{}
		FR:
			ress := regexp.MustCompile(`\[([^\s\[\]]+)\]`).FindAllStringSubmatch(rule, -1)
			if len(ress) != 0 {
				res := ress[len(ress)-1]
				var inner = res[1]
				slice := strings.SplitN(inner, ":", 2)
				name := slice[0]
				ps := ""
				if len(slice) == 2 {
					ps = slice[1]
				}
				if strings.HasSuffix(name, "?") {
					name = strings.TrimRight(name, "?")
					rep := ""
					if ps == "" {
						rep = fmt.Sprintf("[%s]", name)
					} else {
						rep = fmt.Sprintf("[%s:%s]", name, ps)
					}
					for l := range _rs {
						_rs[l] = strings.Replace(_rs[l], res[0], rep, 1)
					}
					rule1 := strings.Replace(rule, res[0], rep, 1)
					if len(_rs) == 0 {
						_rs = append(_rs, rule1)
					}
					rule = strings.Replace(rule, res[0], "", 1)
					rule = regexp.MustCompile("\x20{2,}").ReplaceAllString(rule, " ")
					rule = strings.TrimSpace(rule)
					_rs = append(_rs, rule)
					goto FR
				}
			}
			if len(_rs) != 0 {
				rules = append(rules, _rs...)
			} else {
				rules = append(rules, rule)
			}
		case "class":
			classes = append(classes, regexp.MustCompile(`[\S]+`).FindAllString(res[2], -1)...)
			classes = utils.Unique(classes)

		case "admin":
			admin = strings.TrimSpace(res[2]) == "true"
		case "priority":
			priority = utils.Int(strings.TrimSpace(res[2]))
		case "title":
			title = strings.TrimSpace(res[2])
		case "public":
			public = strings.TrimSpace(res[2]) == "true"
		case "desc":
			description = strings.TrimSpace(res[2])
		case "icon":
			icon = strings.TrimSpace(res[2])
		case "version":
			version = strings.TrimSpace(res[2])
		case "author":
			author = strings.TrimSpace(res[2])
		case "cron":
			schedule := parseCronMetaValue(res[2])
			if schedule != "" {
				crons["task"] = schedule
			}
		case "origin":
			origin = strings.TrimSpace(res[2])
		case "module":
			module = strings.TrimSpace(res[2]) == "true"
		case "carry":
			carry = strings.TrimSpace(res[2]) != "false"
		case "on_start":
			onStart = strings.TrimSpace(res[2]) == "true"
		case "web":
			web = strings.TrimSpace(res[2]) == "true"
		}
	}
	if !hasForm {
		hasForm = strings.Contains(script, "form(") || strings.Contains(script, "SillyGirlPluginConfig") || strings.Contains(script, "sillyGirlCreateSchema")
	}
	return &common.Function{
		Rules:       rules,
		Admin:       admin,
		Priority:    priority,
		Disable:     disable,
		UUID:        uuid,
		Title:       title,
		Public:      public,
		Description: description,
		Icon:        icon,
		Version:     version,
		Author:      author,
		Class:       strings.Join(classes, " "),
		Module:      module,
		OnStart:     onStart || web,
		Web:         web,
		Origin:      origin,
		Cron:        crons,
		Running:     onStart || web,
		HasForm:     hasForm,
		Carry:       carry,
		Classes:     classes,
	}, cbs
}

func parseCronMetaValue(value string) string {
	fields := regexp.MustCompile(`\S+`).FindAllString(strings.TrimSpace(value), -1)
	if len(fields) == 5 || len(fields) == 6 {
		if !isCronFieldToken(fields[0]) {
			return ""
		}
		return strings.Join(fields, " ")
	}
	return ""
}

func isCronFieldToken(value string) bool {
	if value == "" {
		return false
	}
	first := value[0]
	return first == '*' || first == '?' || (first >= '0' && first <= '9')
}
