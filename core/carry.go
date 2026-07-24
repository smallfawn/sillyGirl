package core

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
)

var cgs []CarryGroup

type CarryGroupsResult struct {
	Success bool         `json:"success"`
	Data    []CarryGroup `json:"data"`
	Page    int          `json:"page"`
	Total   int          `json:"total"`
	Time    time.Time    `json:"time"`
}

var CarryGroups = MakeBucket("CarryGroups")

var carryCounter int64

func carryGroupName(cg CarryGroup) string {
	if cg.Remark != "" {
		return cg.Remark
	}
	return cg.ID
}

func syncCarryGroupListen(cg CarryGroup) {
	name := carryGroupName(cg)
	if cg.ID != "" && cg.Enable {
		AddListenOnGroup(cg.ID, fmt.Sprintf("已为搬运群(%s)开启监听模式", name), cg.Platform)
		return
	}
	RemListenOnGroup(cg.ID, fmt.Sprintf("已为搬运群(%s)关闭监听模式", name))
}

func canUseAsCarryScript(function *common.Function) bool {
	return function.UUID != "" && function.Type == NODE && function.Carry && !function.Disable && !function.Module && !function.OnStart && !function.Web
}

type QMessage struct {
	UserID    string        `json:"user_id"`
	Content   string        `json:"content"`
	MessageID string        `json:"message_id"`
	From      common.Sender `json:"-"`
	To        *Factory      `json:"-"`
}

// LOGIC
func initCarry() {
	AddCommand([]*common.Function{
		{
			Rules:    []string{`raw [\s\S]*`},
			Hidden:   true,
			Priority: 9999,
			Handle: func(s common.Sender) interface{} {
				botID := s.GetBotID()
				platform := s.GetImType()
				chatID := s.GetChatID()
				localGroups := cgs
				traceID := fmt.Sprintf("%d. ", atomic.AddInt64(&carryCounter, 1))
				var event = s.Event()
				if event != nil {
					if event["type"] == "delete_message" {
						queues.Range(func(key, value any) bool {
							q := value.(*Queue)
							for _, qm := range q.GetValues() {
								if qm.From != nil && qm.From.GetMessageID() == event["message_id"] {
									qm.To.Sender2(nil).RecallMessage(qm.MessageID)
								}
							}
							return true
						})
					}
					s.Continue()
					return nil
				}
				if chatID == "" {
					s.Continue()
					return nil
				}
				var group *CarryGroup
				for i := range localGroups {
					if localGroups[i].Enable && localGroups[i].ID == chatID && (localGroups[i].Platform == "" || localGroups[i].Platform == platform) {
						group = &localGroups[i]
						break
					}
				}
				if group == nil {
					s.Continue()
					return nil
				}
				if len(group.BotsID) != 0 && !Contains(group.BotsID, botID) {
					console.Debug("%s 忽略机器人(%s)消息，搬运群(%s)限定工作机器人%v", traceID, botID, chatID, group.BotsID)
					return nil
				}
				if len(group.Scripts) == 0 {
					console.Debug("%s 搬运群(%s)未配置处理脚本", traceID, chatID)
					s.Continue()
					return nil
				}
				console.Debug("%s 搬运群(%s)执行处理脚本%v", traceID, chatID, group.Scripts)
				executed := []string{}
				for _, scriptID := range group.Scripts {
					for _, function := range Functions {
						if function.UUID == scriptID && canUseAsCarryScript(function) && !Contains(executed, function.UUID) {
							function.Handle(s)
							executed = append(executed, function.UUID)
							break
						}
					}
				}
				s.Continue()
				return nil
			},
		},
	})

	setCgs()
	storage.Watch(CarryGroups, nil, func(old, new, key string) *storage.Final {
		console.Log("已更新搬运数据")
		ocg := CarryGroup{}
		ncg := CarryGroup{}
		json.Unmarshal([]byte(old), &ocg)
		json.Unmarshal([]byte(new), &ncg)
		tmp := cgs
		if old != "" {
			if new == "" { // 删除
				if ocg.ID != "" {
					for i, cg := range tmp {
						if cg.ID == ocg.ID {
							tmp = append(tmp[:i], tmp[i+1:]...)
							syncCarryGroupListen(CarryGroup{ID: cg.ID, Remark: carryGroupName(cg)})
							break
						}
					}
				} else {
					return nil
				}
			} else { // 修改
				if ocg.ID != "" {
					for i, cg := range tmp {
						if cg.ID == ocg.ID {
							tmp[i] = ncg
							syncCarryGroupListen(ncg)
							break
						}
					}
				} else {
					return nil
				}
			}
		} else { //创建
			if ncg.ID != "" {
				tmp = append(tmp, ncg)
				syncCarryGroupListen(ncg)
			} else {
				return nil
			}
		}
		sort.Sort(byCreatedAt(tmp))
		for i := range tmp {
			tmp[i].Index = i + 1
		}
		cgs = tmp
		return nil
	})
}

func setCgs() {
	CarryGroups.Foreach(func(b1, b2 []byte) error {
		cg := CarryGroup{}
		err := json.Unmarshal(b2, &cg)
		if err != nil {
			return nil
		}
		syncCarryGroupListen(cg)
		cgs = append(cgs, cg)
		return nil
	})
	sort.Sort(byCreatedAt(cgs))
	for i := range cgs {
		cgs[i].Index = i + 1
	}
}

type CarryGroup struct {
	Index          int      `json:"id"`             //编号 顺序编号
	In             bool     `json:"in"`             //搬进来 勾选按钮
	Out            bool     `json:"out"`            //运出去 勾选按钮
	From           []string `json:"from"`           //采集源
	Allowed        []string `json:"allowed"`        //白名单模式
	Prohibited     []string `json:"prohibited"`     //黑名单模式 Select选择器多选
	ID             string   `json:"chat_id"`        //群组ID 文字表单
	ChatName       string   `json:"chat_name"`      //群昵称 文字表单
	Remark         string   `json:"remark"`         //备注
	Platform       string   `json:"platform"`       //平台 Select选择器单选
	Enable         bool     `json:"enable"`         //启用状态 开关
	Include        []string `json:"include"`        //包含关键词 多个关键词用逗号隔开 用户复制粘贴过去后自动转换成多彩标签
	Exclude        []string `json:"exclude"`        //排除关键词 包含关键词
	CreatedAt      int      `json:"created_at"`     //创建时间戳(秒)转换成日期
	BotsID         []string `json:"bots_id"`        //工作机器人 多选
	Scripts        []string `json:"scripts"`        //处理脚本
	Deduplication  bool     `json:"deduplication"`  //文本去重
	Deduplication2 bool     `json:"deduplication2"` //图片去重
}

// CARRY API
func init() {
	GinApi(GET, "/api/carry/groups", RequireAuth, func(ctx *gin.Context) {
		current := utils.Int(ctx.Query("current"))
		pageSize := utils.Int(ctx.Query("pageSize"))
		rr := CarryGroupsResult{
			Success: true,
		}
		cgs := cgs
		rr.Total = len(cgs)
		if current == 0 {
			current = 1
		}
		if pageSize == 0 {
			pageSize = 20
		}
		begin := (current - 1) * pageSize
		end := (current) * pageSize
		if end > rr.Total {
			end = rr.Total
		}
		if begin > end {
			begin = end
		}
		rr.Data = cgs[begin:end]
		for i := range rr.Data {
			gn := &Nickname{
				ID: rr.Data[i].ID,
			}
			nickname.First(gn)
			if gn.Value != "" {
				rr.Data[i].ChatName = gn.Value
			}
		}
		ctx.JSON(200, rr)
	})
	GinApi(GET, "/api/carry/group_names", RequireAuth, func(ctx *gin.Context) {
		cgs := cgs
		var names = map[string]string{}
		for _, cg := range cgs {
			names[cg.ID] = cg.ChatName
		}
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data":    names,
		})
	})
	GinApi(GET, "/api/proxy/scripts", RequireAuth, func(ctx *gin.Context) {
		var scripts = map[string]string{}
		functions := Functions
		for _, function := range functions {
			if function.UUID != "" {
				scripts[function.UUID] = function.Title + function.Suffix
			}
		}
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data":    scripts,
		})
	})
	var isNumeric = func(keyword string) bool {
		for _, c := range keyword {
			if c != '.' && (c < '0' || c > '9') {
				return false
			}
		}
		return true
	}
	GinApi(GET, "/api/proxy/rules", RequireAuth, func(ctx *gin.Context) {
		keyword := ctx.Query("keyword")
		var scripts = map[string]string{}
		scripts[keyword] = keyword
		if strings.HasSuffix(keyword, ".") && !isNumeric(keyword) {
			for _, suffix := range []string{"com", "cn"} {
				scripts[keyword+suffix] = keyword + suffix
			}
		}
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data":    scripts,
		})
	})
	GinApi(GET, "/api/carry/group_selects", RequireAuth, func(ctx *gin.Context) {
		chat_id := ctx.Query("chat_id")
		platform := ctx.Query("platform")
		cgs := cgs
		var bots_id = []string{}
		for _, cg := range cgs {
			if cg.ID == chat_id {
				if platform == "" {
					platform = cg.Platform
				}
			}
		}
		bots_id = GetAdapterBotsID(platform)
		var scripts = map[string]string{}
		functions := Functions
		for _, function := range functions {
			if canUseAsCarryScript(function) {
				scripts[function.UUID] = function.Title + function.Suffix
			}
		}
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"bots_id":   bots_id,
				"platforms": getPltsArray(),
				"scripts":   scripts,
			},
		})
	})
	GinApi(POST, "/api/carry/group", RequireAuth, func(ctx *gin.Context) {
		// 将请求的 JSON 数据解析为一个 map[string]interface{} 类型的变量
		var updateData map[string]interface{}
		err := ctx.BindJSON(&updateData)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{
				"success":      false,
				"errorMessage": err.Error(),
			})
			return
		}
		chat_id := strings.TrimSpace(fmt.Sprint(updateData["chat_id"]))
		if chat_id == "" {
			ctx.JSON(200, map[string]interface{}{
				"success":      false,
				"errorMessage": "群号不能为空",
			})
			return
		}
		platform := strings.TrimSpace(fmt.Sprint(updateData["platform"]))
		if platform == "" {
			ctx.JSON(200, map[string]interface{}{
				"success":      false,
				"errorMessage": "平台不能为空",
			})
			return
		}
		var cg = CarryGroup{
			ID:       chat_id,
			Platform: platform,
			In:       true,
			Enable:   true,
		}
		CarryGroups.First(&cg)
		cg.ID = chat_id
		cg.Platform = platform
		cg.In = true
		cg.Enable = true
		cg.Out = false
		cg.From = nil
		cg.Allowed = nil
		cg.Prohibited = nil
		cg.ChatName = ""
		cg.Include = nil
		cg.Exclude = nil
		cg.Deduplication = false
		cg.Deduplication2 = false
		// if err != nil {
		// 	ctx.JSON(200, map[string]interface{}{
		// 		"success":      false,
		// 		"errorMessage": err.Error(),
		// 	})
		// 	return
		// }
		for key, value := range updateData {
			switch key {
			case "remark":
				if remark, ok := value.(string); ok {
					cg.Remark = remark
				}
			case "bots_id":
				if botsID, ok := value.([]interface{}); ok {
					cg.BotsID = toStringSlice(botsID)
				}
			case "scripts":
				if scripts, ok := value.([]interface{}); ok {
					cg.Scripts = toStringSlice(scripts)
				}
			}
		}
		if cg.CreatedAt == 0 {
			cg.CreatedAt = int(time.Now().Unix())
		}
		CarryGroups.Set(chat_id, utils.JsonMarshal(cg))
		if err != nil {
			ctx.JSON(200, map[string]interface{}{
				"success":      false,
				"errorMessage": err.Error(),
			})
			return
		}
		ctx.JSON(200, map[string]interface{}{
			"success": true,
		})
	})
	GinApi(DELETE, "/api/carry/group", RequireAuth, func(ctx *gin.Context) {
		cg := &CarryGroup{}
		err := ctx.BindJSON(cg)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{
				"success":      false,
				"errorMessage": err.Error(),
			})
			return
		}
		if cg.ID == "" {
			ctx.JSON(200, map[string]interface{}{
				"success":      false,
				"errorMessage": "群号不为空",
			})
			return
		}
		CarryGroups.Set(cg.ID, "")
		ctx.JSON(200, map[string]interface{}{
			"success": true,
		})
	})
}

type byCreatedAt []CarryGroup

func (s byCreatedAt) Len() int {
	return len(s)
}

func (s byCreatedAt) Less(i, j int) bool {
	return s[i].CreatedAt > s[j].CreatedAt
}

func (s byCreatedAt) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// 将 []interface{} 转为 []string 的工具函数
func toStringSlice(intfSlice []interface{}) []string {
	stringSlice := make([]string, len(intfSlice))
	for i, intf := range intfSlice {
		if str, ok := intf.(string); ok {
			stringSlice[i] = str
		}
	}
	return stringSlice
}

func Contains(strs []string, str ...string) bool {
	for _, s := range str {
		for _, str := range strs {
			if s == str {
				return true
			}
		}
	}
	return false
}

func Include(content string, includes []string) string {
	for _, include := range includes {
		if len(include) > 2 && include[0] == '/' && include[len(include)-1] == '/' {
			pattern := include[1 : len(include)-1]
			_, err := regexp.Compile(pattern)
			if err != nil {
				console.Error("包含词/排除词正则表达式 %s 错误 %s", include, err.Error())
				continue
			}
			match, err := regexp.MatchString(pattern, content)
			if err == nil && match {
				return include
			}
		} else {
			if strings.Contains(content, include) {
				return include
			}
		}
	}
	return ""
}
