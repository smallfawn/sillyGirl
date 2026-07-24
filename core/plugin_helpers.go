package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/beego/beego/v2/client/httplib"
)

type ScriptUtils struct {
	matched bool
	ress    [][]string
	script  string
}

func (su *ScriptUtils) match() {
	su.ress = regexp.MustCompile(
		`(\x20?\*[ ]?@([^\s]+)\s+([^\n]+?)\n)`,
	).FindAllStringSubmatch(su.script, -1)
	su.matched = true
}

func (su *ScriptUtils) GetValue(key string) string {
	if !su.matched {
		su.match()
	}
	value := ""
	for _, res := range su.ress {
		if res[2] == key {
			value = res[3]
		}
	}
	return value
}

func (su *ScriptUtils) SetValue(key, value string) {
	if !su.matched {
		su.match()
	}
	exists := []string{}
	first := ""
	for _, res := range su.ress {
		if first == "" {
			first = res[1]
		}
		if res[2] == key {
			exists = append(exists, res[1])
		}
	}
	if len(exists) != 0 {
		for i := range exists {
			if i == len(exists)-1 {
				su.script = strings.Replace(su.script, exists[i], fmt.Sprintf(" * @%s %s\n", key, value), 1)
			} else {
				su.script = strings.Replace(su.script, exists[i], "", 1)
			}
		}
	} else if first != "" {
		su.script = strings.Replace(su.script, first, first+fmt.Sprintf(" * @%s %s\n", key, value), 1)
	} else {
		su.script = fmt.Sprintf("/**\n * @%s %s\n */\n", key, value) + su.script
	}
	su.match()
}

func (su *ScriptUtils) DeleteValue(key string) {
	if !su.matched {
		su.match()
	}
	exists := []string{}
	for _, res := range su.ress {
		if res[2] == key {
			exists = append(exists, res[1])
		}
	}
	for i := range exists {
		su.script = strings.Replace(su.script, exists[i], "", 1)
	}
	su.match()
}

func ErrStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func EncryptPlugin(script string) string {
	res := strings.SplitN(script, "*/\n", 2)
	if len(res) != 2 {
		return script
	}
	str, err := EncryptByAes([]byte(res[1]))
	if err != nil {
		return script
	}
	su := ScriptUtils{script: res[0]}
	if su.GetValue("encrypt_data") != "" {
		return script
	}
	su.SetValue("encrypt_data", str)
	return su.script + "*/\n"
}

func DecryptPlugin(script string) string {
	su := ScriptUtils{script: script}
	encryptData := su.GetValue("encrypt_data")
	if encryptData == "" {
		return script
	}
	su.DeleteValue("encrypt_data")
	str, err := DecryptByAes(encryptData)
	if err != nil {
		return script
	}
	return fmt.Sprintf("%s%s", su.script, str)
}

func fetchScript(address, uuid string) (data []byte) {
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		if strings.HasSuffix(strings.ToLower(address), ".js") {
			data, _ = httplib.Get(address).Bytes()
			return
		}
	}
	prefix := "?uuid=" + uuid
	if !strings.HasSuffix(address, "list.json") {
		address = address + "/api/plugins/download" + prefix
	} else {
		address = strings.ReplaceAll(address, "list.json", "download"+prefix)
	}
	data, _ = httplib.Get(address).Bytes()
	return
}

func formatRule(rule string) []string {
	_rs := []string{}
FR:
	ress := regexp.MustCompile(`\[([^\s\[\]]+)\]`).FindAllStringSubmatch(rule, -1)
	if len(ress) != 0 {
		res := ress[len(ress)-1]
		inner := res[1]
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
	return _rs
}

func isFlatNodePluginPath(path string) bool {
	root := filepath.Clean(nodePluginsRoot())
	clean := filepath.Clean(path)
	return strings.EqualFold(filepath.Dir(clean), root) && strings.EqualFold(filepath.Ext(clean), ".js")
}
