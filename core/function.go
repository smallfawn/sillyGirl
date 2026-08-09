package core

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goccy/go-json"
	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/core/logs"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
)

var total uint64 = 0
var finished uint64 = 0
var contents sync.Map

var Functions = []*common.Function{}

var Messages chan common.Sender

var (
	ruleParamPattern        = regexp.MustCompile(`\[([^\s\[\]]+)\]`)
	trailingOptionalPattern = regexp.MustCompile(`\?$`)
	cronFieldPattern        = regexp.MustCompile(`\S+`)
)

var ListenOnGroups sync.Map
var NoListenUsers sync.Map
var NoReplyGroups sync.Map
var StaticListenOnGroups sync.Map
var StaticNoReplyGroups sync.Map
var noListenUsers = MakeBucket("noListenUsers")
var listenOnGroups = MakeBucket("listenOnGroups")
var noReplyGroups = MakeBucket("noReplyGroups")

type GroupInfo struct {
	Platform string `json:"platform"`
	Desc     string `json:"desc"`
	Enable   bool   `json:"enable"`
}

var AddNoReplyGroups = func(code string, _ string, plt string) {
	NoReplyGroups.LoadOrStore(code, plt)
}

var AddListenOnGroup = func(code string, _ string, plt string) {
	ListenOnGroups.LoadOrStore(code, plt)
}

var RemNoReplyGroups = func(code string, _ string) {
	_, loaded := NoReplyGroups.Load(code)
	if loaded {
		NoReplyGroups.Delete(code)
	}
}

var RemListenOnGroup = func(code string, _ string) {
	_, loaded := ListenOnGroups.Load(code)
	if loaded {
		ListenOnGroups.Delete(code)
	}
}

var IsNoReplyGroup = func(s common.Sender) bool {
	cid := s.GetChatID()
	if utils.IsZeroOrEmpty(cid) {
		return false
	}
	_, ok1 := NoReplyGroups.Load(cid)
	_, ok2 := StaticNoReplyGroups.Load(cid)
	res := ok1 || ok2
	if res {
		logs.Info("禁言的群组 %v/%v@%v", s.GetImType(), s.GetUserID(), cid)
	}
	return res
}

func initListenReply() {
	listenOnGroups.Foreach(func(b1, data []byte) error {
		groupCode := string(b1)
		info := &GroupInfo{}
		err := json.Unmarshal(data, info)
		if err != nil {
			listenOnGroups.Set(groupCode, "")
		} else {
			if info.Enable {
				StaticListenOnGroups.Store(string(b1), info.Platform)
			}
		}
		return nil
	})
	storage.Watch(listenOnGroups, nil, func(old, new, key string) (fin *storage.Final) {
		if new == "" {
			logs.Info("已删除监听群组%s", key)
			StaticListenOnGroups.Delete(key)
			return
		}
		info := &GroupInfo{}
		json.Unmarshal([]byte(new), info)
		if info.Enable {
			StaticListenOnGroups.Store(key, info.Platform)
			logs.Info("已设置监听群组%s/%s", info.Platform, key)
		} else {
			StaticListenOnGroups.Delete(key)
			logs.Info("已取消监听群组%s/%s", info.Platform, key)
		}
		return
	})
	noReplyGroups.Foreach(func(b1, data []byte) error {
		groupCode := string(b1)
		info := &GroupInfo{}
		err := json.Unmarshal(data, info)
		if err != nil {
			noReplyGroups.Set(groupCode, "")
		} else {
			info := &GroupInfo{}
			json.Unmarshal(data, info)
			if info.Enable {
				StaticNoReplyGroups.Store(string(b1), info.Platform)
			}
		}
		return nil
	})
	storage.Watch(noReplyGroups, nil, func(old, new, key string) (fin *storage.Final) {
		if new == "" {
			logs.Info("已删除禁言群组%s", key)
			StaticNoReplyGroups.Delete(key)
			return
		}
		info := &GroupInfo{}
		json.Unmarshal([]byte(new), info)
		if info.Enable {
			logs.Info("已设置禁言群组%s/%s", info.Platform, key)
			StaticNoReplyGroups.Store(key, info.Platform)
		} else {
			logs.Info("已取消禁言群组%s%s", info.Platform, key)
			StaticNoReplyGroups.Delete(key)
		}
		return
	})
	noListenUsers.Foreach(func(b1, data []byte) error {
		groupCode := string(b1)
		info := &GroupInfo{}
		err := json.Unmarshal(data, info)
		if err != nil {
			noListenUsers.Set(groupCode, "")
		} else {
			info := &GroupInfo{}
			json.Unmarshal(data, info)
			if info.Enable {
				NoListenUsers.Store(string(b1), info.Platform)
			}
		}
		return nil
	})
	storage.Watch(noListenUsers, nil, func(old, new, key string) (fin *storage.Final) {
		if new == "" {
			logs.Info("已取消屏蔽用户%s", key)
			NoListenUsers.Delete(key)
			return
		}
		info := &GroupInfo{}
		json.Unmarshal([]byte(new), info)
		if info.Enable {
			logs.Info("已屏蔽用户%s/%s", info.Platform, key)
			NoListenUsers.Store(key, info.Platform)
		} else {
			logs.Info("已取消屏蔽用户%s%s", info.Platform, key)
			NoListenUsers.Delete(key)
		}
		return
	})
}

func initToHandleMessage() {
	listen_admin := sillyGirl.GetBool("listen_admin", true)
	storage.Watch(sillyGirl, "listen_admin", func(old, new, key string) *storage.Final {
		if new == "false" {
			listen_admin = false
		}
		return nil
	})
	Messages = make(chan common.Sender)
	go func() {
		for {
			s := <-Messages
			ignore := false
			cid := s.GetChatID()
			uid := s.GetUserID()
			imType := s.GetImType()
			isAdmin := s.IsAdmin()
			uname := s.GetUserName()
			ctt := s.GetContent()
			if !utils.IsZeroOrEmpty(cid) {
				cname := s.GetChatName()
				if cname != "" {
					CreateNickName(&Nickname{
						ID:       cid,
						Group:    true,
						Value:    cname,
						Platform: imType,
						BotsID:   []string{s.GetBotID()},
					})
				}
				if isAdmin {
					switch ctt {
					case "listen":
						if data := listenOnGroups.GetBytes(cid); len(data) == 0 {
							listenOnGroups.Set(cid, utils.JsonMarshal(&GroupInfo{
								Platform: imType,
								Enable:   true,
								Desc:     s.GetChatName(),
							}))
						} else {
							info := &GroupInfo{}
							json.Unmarshal(data, info)
							if !info.Enable {
								info.Enable = !info.Enable
								listenOnGroups.Set(cid, utils.JsonMarshal(info))
							}
						}
						s.Reply("ok")
					case "unlisten", "nolisten":
						if data := listenOnGroups.GetBytes(cid); len(data) != 0 {
							info := &GroupInfo{}
							json.Unmarshal(data, info)
							if info.Enable {
								info.Enable = !info.Enable
								listenOnGroups.Set(cid, utils.JsonMarshal(info))
							}
						}
						s.Reply("ok")
					case "reply":
						noReplyGroups.Set(cid, utils.JsonMarshal(&GroupInfo{}))
						s.Reply("ok")
					case "noreply", "unreply":
						if data := noReplyGroups.GetBytes(cid); len(data) == 0 {
							noReplyGroups.Set(cid, utils.JsonMarshal(&GroupInfo{
								Platform: imType,
								Enable:   true,
								Desc:     s.GetChatName(),
							}))
						} else {
							info := &GroupInfo{}
							json.Unmarshal(data, info)
							if !info.Enable {
								info.Enable = !info.Enable
								noReplyGroups.Set(cid, utils.JsonMarshal(info))
							}
						}
						s.Reply("ok")
					}
				}
				_, ok1 := ListenOnGroups.Load(cid)
				if !ok1 {
					if !listen_admin || !isAdmin {
						_, ok2 := StaticListenOnGroups.Load(cid)
						if !ok2 {
							ignore = true
						}
					}
				}
			} else {
				if isAdmin {
					switch ctt {
					case "unlisten", "nolisten":
						if data := noListenUsers.GetBytes(uid); len(data) == 0 {
							noListenUsers.Set(uid, utils.JsonMarshal(&GroupInfo{
								Platform: imType,
								Enable:   true,
								Desc:     s.GetChatName(),
							}))
						} else {
							info := &GroupInfo{}
							json.Unmarshal(data, info)
							if !info.Enable {
								info.Enable = !info.Enable
								noListenUsers.Set(uid, utils.JsonMarshal(info))
							}
						}
						s.Reply("ok")
					case "listen":
						if data := noListenUsers.GetBytes(uid); len(data) != 0 {
							info := &GroupInfo{}
							json.Unmarshal(data, info)
							if info.Enable {
								info.Enable = !info.Enable
								noListenUsers.Set(uid, utils.JsonMarshal(info))
							}
						}
						s.Reply("ok")
					}
				}
			}
			_, ok2 := NoListenUsers.Load(uid)
			if ok2 {
				ignore = true
			}

			if uname != "" {
				CreateNickName(&Nickname{
					ID:       uid,
					Group:    false,
					Value:    uname,
					Platform: imType,
					BotsID:   []string{s.GetBotID()},
				})
			}
			if imType != "terminal" {
				if !ignore {
					logs.Info("接收到消息 %v/%v@%v：%s", imType, uid, cid, ctt)
				} else {
					logs.Info("屏蔽的消息 %v/%v@%v：%s", imType, uid, cid, ctt)
				}
			}
			if ignore {
				continue
			}
			go HandleMessage(s)
		}
	}()
}

func fmtRule(cmd *common.Function) {
	cmd.Params = make([][]string, len(cmd.Rules))
	for i := range cmd.Rules {
		cmd.Rules[i] = strings.Trim(cmd.Rules[i], "")
		cmd.Params[i] = []string{}
		if strings.HasPrefix(cmd.Rules[i], "raw") {
			cmd.Rules[i] = strings.Replace(cmd.Rules[i], "raw ", "", -1)
			continue
		}
		if strings.HasPrefix(cmd.Rules[i], "^") {
			continue
		}
		if strings.HasSuffix(cmd.Rules[i], "$") {
			continue
		}
		cmd.Rules[i] = strings.ReplaceAll(cmd.Rules[i], `\r\a\w`, "raw")
		cmd.Rules[i] = strings.Replace(cmd.Rules[i], "(", `[(]`, -1)
		cmd.Rules[i] = strings.Replace(cmd.Rules[i], ")", `[)]`, -1)
		ress := ruleParamPattern.FindAllStringSubmatch(cmd.Rules[i], -1)
		for _, res := range ress {
			var inner = res[1]
			vv := strings.SplitN(inner, ":", 2)
			name := vv[0]
			if len(vv) == 1 {
				cmd.Rules[i] = strings.ReplaceAll(cmd.Rules[i], res[0], "?")
			} else {
				cmd.Rules[i] = strings.ReplaceAll(cmd.Rules[i], res[0], fmt.Sprintf("(%s)", strings.ReplaceAll(vv[1], ",", "|")))
			}
			cmd.Params[i] = append(cmd.Params[i], name)
		}
		cmd.Rules[i] = trailingOptionalPattern.ReplaceAllString(cmd.Rules[i], `([\s\S]+)`)
		cmd.Rules[i] = strings.Replace(cmd.Rules[i], " ", `\s+`, -1)
		cmd.Rules[i] = strings.Replace(cmd.Rules[i], "?", `(\S+)`, -1)
		cmd.Rules[i] = "^" + cmd.Rules[i] + "$"
	}
	compileFunctionRules(cmd)
}

func compileFunctionRules(cmd *common.Function) {
	cmd.RulePatterns = make([]*regexp.Regexp, len(cmd.Rules))
	cmd.RuleErrors = make([]error, len(cmd.Rules))
	for i, rule := range cmd.Rules {
		reg, err := regexp.Compile(rule)
		if err != nil {
			cmd.RuleErrors[i] = err
			continue
		}
		cmd.RulePatterns[i] = reg
	}
}

func functionRulePattern(cmd *common.Function, index int) (*regexp.Regexp, error) {
	if cmd == nil || index < 0 || index >= len(cmd.Rules) {
		return nil, fmt.Errorf("规则索引不存在")
	}
	if index < len(cmd.RulePatterns) && cmd.RulePatterns[index] != nil {
		return cmd.RulePatterns[index], nil
	}
	if index < len(cmd.RuleErrors) && cmd.RuleErrors[index] != nil {
		return nil, cmd.RuleErrors[index]
	}
	return regexp.Compile(cmd.Rules[index])
}

func AddCommand(cmds []*common.Function) {
	for j := range cmds {
		if cmds[j].OnStart && pluginExecutionEnabled(cmds[j]) {
			go func(f *common.Function) {
				time.Sleep(time.Second)
				f.Handle(&CustomSender{
					F: &Factory{
						botplt: "*",
					},
				})
			}(cmds[j])
		}
		fmtRule(cmds[j])
		{
			if pluginExecutionEnabled(cmds[j]) && !cmds[j].Module {
				for plt, Cron := range cmds[j].Cron {
					if !pluginCronTaskEnabled(cmds[j].UUID, plt) {
						continue
					}
					plt := plt
					cron := strings.TrimSpace(Cron)
					if len(cronFieldPattern.FindAllString(cron, -1)) == 5 {
						Cron = "0 " + Cron
					}
					cronId, err := CRON.AddJob(Cron, skipIfStillRunningCronFunc(func() {
						if !pluginExecutionEnabled(cmds[j]) || !pluginCronTaskEnabled(cmds[j].UUID, plt) {
							return
						}
						runPluginCronFunction(cmds[j], plt)
					}))
					if err == nil {
						cmds[j].CronIds = append(cmds[j].CronIds, int(cronId))
					} else {
						console.Error("脚本%s定时器错误，%v", cmds[j].Title, err)
					}
				}
			}
		}
		{
			lf := len(Functions)
			for i := range Functions {
				f := lf - i - 1
				if Functions[f].Priority > cmds[j].Priority {
					Functions = append(Functions[:f+1], append([]*common.Function{cmds[j]}, Functions[f+1:]...)...)
					break
				}
			}
			if len(Functions) == lf {
				if lf > 0 {
					apd := false
					for i := range Functions {
						if cmds[j].Priority >= Functions[i].Priority {
							apd = true
							Functions = append(Functions[:i], append([]*common.Function{cmds[j]}, Functions[i:]...)...)
							break
						}
					}
					if !apd {
						Functions = append(Functions, cmds[j])
					}
				} else {
					Functions = append(Functions, cmds[j])
				}
			}
		}
	}
}

func HandleMessage(sender common.Sender) {
	if !debug {
		defer func() {
			err := recover()
			if err != nil {
				console.Error("HandleMessage error: %v", err)
			}
		}()
	}
	num := atomic.AddUint64(&total, 1)
	defer atomic.AddUint64(&finished, 1)
	ct := sender.GetContent()
	contents.Store(num, ct)
	defer func() {
		contents.Delete(num)
	}()
	content := utils.TrimHiddenCharacter(ct)
	defer func() {
		sender.Finish()
		if sender.IsAtLast() {
			s := sender.MessagesToSend()
			if s != "" {
				sender.Reply(s)
			}
		}
	}()
	u, g, i, a := sender.GetUserID(), sender.GetChatID(), sender.GetImType(), sender.IsAdmin()
	con := true
	mtd := false
	for _, wait := range waits {
		wait.Foreach(func(k int64, c *Carry) bool {
			if c.RequireAdmin && !a {
				return true
			}

			if len(c.AllowPlatforms) != 0 && !Contains(c.AllowPlatforms, i) {
				return true
			}

			if len(c.ProhibitPlatforms) != 0 && Contains(c.ProhibitPlatforms, i) {
				return true
			}

			if len(c.AllowUsers) != 0 && !Contains(c.AllowUsers, u) {
				return true
			}

			if len(c.ProhibitUsers) != 0 && Contains(c.ProhibitUsers, u) {
				return true
			}
			if len(c.AllowGroups) != 0 && !Contains(c.AllowGroups, g) {
				return true
			}
			if len(c.ProhibitGroups) != 0 && Contains(c.ProhibitGroups, g) {
				return true
			}

			if c.ChatID != "" { //群聊监听
				if g == "" { //私聊时
					if !c.ListenPrivate { //如果未设置允许私聊则拒绝
						return true
					}
				} else { //群聊时
					if c.UserID != "" && u != c.UserID { //群员发言
						if !c.ListenGroup { //未设置允许群员加入拒绝
							return true
						}
					}
				}
			} else { //私聊监听
				if c.UserID != "" && u != c.UserID { //其他用户
					return true
				}
			}
			for i := range c.Function.Rules {
				reg, err := functionRulePattern(&c.Function, i)
				if err != nil {
					console.Error("监听器正则错误，%v", err)
					continue
				}
				if res := reg.FindStringSubmatch(content); len(res) > 0 {
					logs.Info("匹配到%s规则：%s", c.Function.Title, c.Function.Rules[i])
					sender.SetMatch(res[1:])
					sender.SetParams(c.Function.Params[i])
					mtd = true
					c.Chan <- sender
					sender.Reply(<-c.Result)
					if !sender.IsContinue() {
						con = false
						return false
					}
					sender.ClearContinue()
					break
				}
			}
			return true
		})
	}
	if mtd && !con {
		return
	}

	for _, reply := range replies {
		if reply.Keyword == "" || reply.Value == "" {
			continue
		}
		if reply.Number != "" && reply.Number != u && reply.Number != g {
			continue
		}
		if len(reply.Platforms) != 0 && !Contains(reply.Platforms, i) {
			continue
		}
		reg, err := regexp.Compile(reply.Keyword)
		if err == nil {
			if reg.FindString(content) != "" {
				//todo 支持JS语法
				output := parseReply2(reply.Value)
				sender.Reply(output)
			}
		}
	}
	for _, function := range Functions {
		if !pluginExecutionEnabled(function) || function.Module {
			continue
		}
		for i := range function.Rules {
			reg, err := functionRulePattern(function, i)
			if err != nil {
				console.Error("脚本%s正则错误，%v", function.Title, err)
				continue
			}
			matched := false
			if res := reg.FindStringSubmatch(content); len(res) > 0 {
				if !function.Hidden {
					logs.Info("匹配到规则：%s", function.Rules[i])
				}
				sender.SetMatch(res[1:])
				sender.SetParams(function.Params[i])
				matched = true
			}
			if matched {
				if !pluginExecutionEnabled(function) {
					break
				}
				if function.Admin && !a {
					return
				}
				rt := function.Handle(sender)
				if rt != nil {
					sender.Reply(rt)
				}
				if sender.IsContinue() {
					sender.ClearContinue()
					content = utils.TrimHiddenCharacter(sender.GetContent())
					if !function.Hidden {
						logs.Info("继续去处理：%s", content)
					}
					goto next
				}
				return
			}
		}
	next:
	}

	recall := sillyGirl.GetString("recall")
	if recall != "" {
		recalled := false
		for _, v := range strings.Split(recall, "&") {
			reg, err := regexp.Compile(v)
			if err == nil {
				if reg.FindString(content) != "" {
					if !a && sender.GetImType() != "wx" {
						sender.Delete()
						recalled = true
						break
					}
				}
			}
		}
		if recalled {
			return
		}
	}
}

// pluginExecutionEnabled is the single runtime gate shared by message, cron
// and startup triggers. Status comes only from the plugin's status metadata;
// plugin.Form values do not participate in this decision. Missing status keeps
// compatibility with programmatically registered plugins and defaults to true.
func pluginExecutionEnabled(function *common.Function) bool {
	return function != nil && (function.Status == nil || *function.Status)
}

func pluginStatusValue(enabled bool) *bool {
	return &enabled
}
