package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
)

var tasks = MakeBucket("tasks")

const pluginCronTaskPrefix = "plugin-cron:"

type TasksResult struct {
	Data  []*Tasks  `json:"data"`
	Page  int       `json:"page"`
	Total int       `json:"total"`
	Time  time.Time `json:"time"`
}

type Sender struct {
	ChatID   string `json:"chat_id"`
	UserID   string `json:"user_id"`
	Platfrom string `json:"platform"`
	BotID    string `json:"bot_id"`
}

type Tasks struct {
	Index     int           `json:"id"`       //编号 顺序编号
	ID        string        `json:"task_id"`  //任务ID
	Title     string        `json:"title"`    //任务名
	Schedule  string        `json:"schedule"` //计划时间
	Senders   []Sender      `json:"senders"`  //发送人
	Command   string        `json:"command"`  //消息指令
	Scripts   []string      `json:"scripts"`  //兼容旧任务的脚本列表
	CronID    int           `json:"cron_id"`
	CreatedAt int           `json:"created_at"` //创建时间戳(秒)转换成日期
	Remark    string        `json:"remark"`
	Enable    bool          `json:"enable"`
	Handle    func()        `json:"-"`
	Icons     []interface{} `json:"icons"`
}

var pts = []*Tasks{}

func pluginCronTaskID(uuid, platform string) string {
	return pluginCronTaskPrefix + uuid + ":" + platform
}

func parsePluginCronTaskID(taskID string) (string, string, bool) {
	if !strings.HasPrefix(taskID, pluginCronTaskPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(taskID, pluginCronTaskPrefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func scriptCommandForFunction(f *common.Function) string {
	if f == nil {
		return ""
	}
	name := nodePluginNameFromPath(f.Path)
	if name == "" {
		name = strings.TrimSuffix(strings.TrimSuffix(f.Title, ".js"), ".py")
	}
	if name == "" {
		return ""
	}
	switch f.Type {
	case NODE:
		return "node " + name + ".js"
	case PYTHON:
		return "python " + name + ".py"
	default:
		return ""
	}
}

func pluginCronTasks() []*Tasks {
	rows := []*Tasks{}
	for _, f := range Functions {
		if f == nil || (f.Type != NODE && f.Type != PYTHON) || len(f.Cron) == 0 {
			continue
		}
		platforms := make([]string, 0, len(f.Cron))
		for platform := range f.Cron {
			platforms = append(platforms, platform)
		}
		sort.Strings(platforms)
		for _, platform := range platforms {
			title := f.Title
			if title == "" {
				title = scriptTaskTitle(f)
			}
			rows = append(rows, &Tasks{
				ID:       pluginCronTaskID(f.UUID, platform),
				Title:    title,
				Schedule: f.Cron[platform],
				Command:  scriptCommandForFunction(f),
				Scripts:  []string{f.UUID},
				Remark:   "来自脚本注释 @cron",
				Enable:   pluginExecutionEnabled(f),
			})
		}
	}
	return rows
}

// setTaskEnabled keeps the task list switch as the single place for changing
// scheduled execution state. Plugin @cron rows reuse the plugin's enable field;
// ordinary tasks keep their own persisted enable flag.
func setTaskEnabled(taskID string, enabled bool) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("缺少任务ID")
	}
	if _, _, ok := parsePluginCronTaskID(taskID); ok {
		f, _ := findScriptFunctionByTask(taskID, "")
		if f == nil {
			return fmt.Errorf("定时任务脚本不存在")
		}
		config := getPluginUserConfig(f.UUID)
		if config == nil {
			config = map[string]interface{}{}
		}
		config["enable"] = enabled
		if _, _, err := SetBucketKeyValue(pluginConfigValues, f.UUID, config); err != nil {
			return err
		}
		return nil
	}
	raw := tasks.GetString(taskID)
	if raw == "" {
		return fmt.Errorf("定时任务不存在")
	}
	var task Tasks
	if err := json.Unmarshal([]byte(raw), &task); err != nil {
		return err
	}
	task.Enable = enabled
	tasks.Set(taskID, utils.JsonMarshal(task))
	return nil
}

func runTaskNow(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("缺少任务ID")
	}
	if _, platform, ok := parsePluginCronTaskID(taskID); ok {
		f, _ := findScriptFunctionByTask(taskID, "")
		if f == nil || f.Handle == nil {
			return fmt.Errorf("定时任务脚本不存在")
		}
		if !pluginExecutionEnabled(f) {
			return fmt.Errorf("插件未启用")
		}
		f.Handle(&CustomSender{F: &Factory{botplt: platform, uuid: f.UUID}})
		return nil
	}
	for _, task := range pts {
		if task.ID == taskID {
			if task.Handle == nil {
				return fmt.Errorf("定时任务不可执行")
			}
			task.Handle()
			return nil
		}
	}
	return fmt.Errorf("定时任务不存在")
}

func scriptTaskTitle(f *common.Function) string {
	command := scriptCommandForFunction(f)
	if strings.HasPrefix(command, "node ") {
		return strings.TrimPrefix(command, "node ")
	}
	if strings.HasPrefix(command, "python ") {
		return strings.TrimPrefix(command, "python ")
	}
	return command
}

func findScriptFunctionByTask(taskID, command string) (*common.Function, string) {
	if uuid, platform, ok := parsePluginCronTaskID(taskID); ok {
		for _, f := range Functions {
			if f != nil && f.UUID == uuid && (f.Type == NODE || f.Type == PYTHON) {
				if _, exists := f.Cron[platform]; !exists {
					return nil, platform
				}
				return f, platform
			}
		}
	}
	if target, class := scriptTaskTarget(command); target != "" {
		return scriptFunctionByCommandTarget(target, class), "task"
	}
	return nil, ""
}

func RegistTasks(pt *Tasks) {
	pt.Handle = func() {
		content := pt.Command
		if runScriptTaskCommand(content, pt.Senders) {
			return
		}
		for _, meta := range pt.Senders {
			adapter, _ := GetAdapter(meta.Platfrom, meta.BotID)
			if adapter != nil {
				sender := adapter.Sender2(nil)
				sender.SetFsps(&common.FakerSenderParams{
					Content: content,
					ChatID:  meta.ChatID,
					UserID:  meta.UserID,
				})
				for _, script := range pt.Scripts {
					for _, function := range Functions {
						if function.UUID == script {
							if !pluginExecutionEnabled(function) {
								break
							}
							for i := range function.Rules {
								reg, err := functionRulePattern(function, i)
								if err == nil {
									if res := reg.FindStringSubmatch(content); len(res) > 0 {
										sender.SetMatch(res[1:])
										sender.SetParams(function.Params[i])
									}
								}
							}
							function.Handle(sender)
							break
						}
					}
				}
			}
		}
	}
	if !pt.Enable {
		pt.CronID = 0
		return
	}
	cid, _ := CRON.AddFunc(pt.Schedule, pt.Handle)
	pt.CronID = int(cid)
	// console.Debug("已添加计划任务：%s(%v)", pt.Title, pt.CronID)
}

func init() {
	tasks.Foreach(func(b1, b2 []byte) error {
		pt := Tasks{}
		err := json.Unmarshal(b2, &pt)
		if err != nil {
			return nil
		}
		RegistTasks(&pt)
		pts = append(pts, &pt)
		return nil
	})
	sort.Sort(byCreatedAt2(pts))
	for i := range pts {
		pts[i].Index = i + 1
	}
	storage.Watch(tasks, nil, func(old, new, key string) *storage.Final {
		console.Log("已更新计划任务")
		ocg := Tasks{}
		ncg := Tasks{}
		json.Unmarshal([]byte(old), &ocg)
		json.Unmarshal([]byte(new), &ncg)
		tmp := pts
		if old != "" {
			if new == "" { // 删除
				if ocg.ID != "" {
					for i, cg := range tmp {
						if cg.ID == ocg.ID {
							CRON.Remove(cron.EntryID(tmp[i].CronID))
							tmp = append(tmp[:i], tmp[i+1:]...)
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
							CRON.Remove(cron.EntryID(tmp[i].CronID))
							tmp[i] = &ncg
							RegistTasks(&ncg)
							//todo 增
							break
						}
					}
				} else {
					return nil
				}
			}
		} else { //创建
			if ncg.ID != "" {
				tmp = append(tmp, &ncg)
				RegistTasks(&ncg)
				//todo 增
			} else {
				return nil
			}
		}
		sort.Sort(byCreatedAt2(pts))
		for i := range tmp {
			tmp[i].Index = i + 1
		}
		pts = tmp
		return nil
	})
	GinApi(GET, "/api/admin/tasks", RequireAuth, func(ctx *gin.Context) {
		current := utils.Int(ctx.Query("page"))
		pageSize := utils.Int(ctx.Query("page_size"))
		rr := TasksResult{}
		rows := append([]*Tasks{}, pts...)
		rows = append(rows, pluginCronTasks()...)
		for i := range rows {
			rows[i].Index = i + 1
		}
		rr.Total = len(rows)
		rr.Time = time.Now()
		current, _, begin, end := paginationBounds(current, pageSize, rr.Total)
		rr.Page = current
		rr.Data = rows[begin:end]
		for i := range rr.Data {
			rr.Data[i].Icons = []interface{}{}
			for _, script := range rr.Data[i].Scripts {
				for _, f := range Functions {
					if f.UUID == script {
						if f.Icon != "" {
							rr.Data[i].Icons = append(rr.Data[i].Icons, map[string]interface{}{
								"link":  f.Icon,
								"title": f.Title,
							})
							break
						}
					}
				}
			}
		}
		ApiList(ctx, rr.Data, rr.Total, map[string]interface{}{"page": rr.Page, "time": rr.Time})
	})
	saveTask := func(ctx *gin.Context) {
		// 将请求的 JSON 数据解析为一个 map[string]interface{} 类型的变量
		var updateData map[string]interface{}
		err := ctx.BindJSON(&updateData)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		task_id := strings.TrimSpace(ctx.Param("task_id"))
		creating := task_id == ""
		if creating {
			task_id = "task-" + utils.GenUUID()
		} else if _, _, pluginTask := parsePluginCronTaskID(task_id); !pluginTask && strings.TrimSpace(tasks.GetString(task_id)) == "" {
			ApiNotFound(ctx, "定时任务不存在")
			return
		}
		var tp = Tasks{
			ID: task_id,
		}
		tasks.First(&tp)
		for key, value := range updateData {
			switch key {
			case "title":
				if v, ok := value.(string); ok {
					tp.Title = strings.TrimSpace(v)
				}
			case "remark":
				if v, ok := value.(string); ok {
					tp.Remark = v
				}
			case "schedule":
				if v, ok := value.(string); ok {
					tp.Schedule = strings.TrimSpace(v)
					if err := validateTaskSchedule(tp.Schedule); err != nil {
						ApiUnprocessable(ctx, err.Error())
						return
					}
				}
			case "senders":
				ss := []Sender{}
				err := json.Unmarshal(utils.JsonMarshal(value), &ss)
				if err != nil {
					ApiUnprocessable(ctx, "Senders错误："+err.Error())
					return
				}
				tp.Senders = ss
			case "command":
				if v, ok := value.(string); ok {
					tp.Command = v
					if isScriptTaskCommand(v) {
						tp.Scripts = nil
						tp.Remark = ""
					}
				}
			case "scripts":
				if v, ok := value.([]interface{}); ok {
					tp.Scripts = toStringSlice(v)
				}
			case "enable":
				if v, ok := value.(bool); ok {
					tp.Enable = v
				}
			}
		}
		if tp.CreatedAt == 0 {
			tp.CreatedAt = int(time.Now().Unix())
		}
		if tp.Title == "" {
			ApiUnprocessable(ctx, "定时任务标题不能为空")
			return
		}
		if err := validateTaskSchedule(tp.Schedule); err != nil {
			ApiUnprocessable(ctx, err.Error())
			return
		}
		if _, _, pluginCronTask := parsePluginCronTaskID(task_id); pluginCronTask {
			f, platform := findScriptFunctionByTask(task_id, tp.Command)
			if f == nil {
				ApiNotFound(ctx, "定时任务脚本不存在")
				return
			}
			if err := updatePluginCronAnnotation(f, platform, tp.Schedule); err != nil {
				ApiInternalError(ctx, err.Error())
				return
			}
			if task_id != "" {
				if _, _, err := tasks.Set(task_id, ""); err != nil {
					ApiInternalError(ctx, err.Error())
					return
				}
			}
			ApiOK(ctx, tp)
			return
		}
		_, _, err = tasks.Set(task_id, utils.JsonMarshal(tp))
		if err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		if creating {
			ApiCreated(ctx, "/api/admin/tasks/"+task_id, tp)
			return
		}
		ApiOK(ctx, tp)
	}
	GinApi(POST, "/api/admin/tasks", RequireAuth, saveTask)
	GinApi(POST, "/api/admin/tasks/:task_id", RequireAuth, saveTask)
	GinApi(POST, "/api/admin/tasks/:task_id/deletions", RequireAuth, func(ctx *gin.Context) {
		pt := &Tasks{ID: strings.TrimSpace(ctx.Param("task_id"))}
		if pt.ID == "" {
			ApiUnprocessable(ctx, "任务 ID 不能为空")
			return
		}
		if _, _, pluginCronTask := parsePluginCronTaskID(pt.ID); pluginCronTask {
			f, platform := findScriptFunctionByTask(pt.ID, pt.Command)
			if f == nil {
				ApiNotFound(ctx, "定时任务脚本不存在")
				return
			}
			if err := updatePluginCronAnnotation(f, platform, ""); err != nil {
				ApiInternalError(ctx, err.Error())
				return
			}
			if _, _, err := tasks.Set(pt.ID, ""); err != nil {
				ApiInternalError(ctx, err.Error())
				return
			}
			ApiNoContent(ctx)
			return
		}
		if strings.TrimSpace(tasks.GetString(pt.ID)) == "" {
			ApiNotFound(ctx, "定时任务不存在")
			return
		}
		if _, _, err := tasks.Set(pt.ID, ""); err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		ApiNoContent(ctx)
	})
	GinApi(GET, "/api/admin/task-options", RequireAuth, func(ctx *gin.Context) {
		var scripts = map[string]string{}
		var task_id = strings.TrimSpace(ctx.Query("task_id"))
		var pts = pts
		var chat_ids = []string{}
		var user_ids = []string{}
		for _, pt := range pts {
			if pt.ID == task_id {
				for _, s := range pt.Senders {
					if s.ChatID != "" {
						chat_ids = append(chat_ids, s.ChatID)
					}
					if s.UserID != "" {
						user_ids = append(user_ids, s.UserID)
					}
				}
				break
			}
		}
		functions := Functions
		for _, function := range functions {
			if function.UUID != "" && (function.Type == NODE || function.Type == PYTHON) {
				name := nodePluginNameFromPath(function.Path)
				if name == "" {
					name = strings.TrimSuffix(strings.TrimSuffix(function.Title, ".js"), ".py")
				}
				switch function.Type {
				case NODE:
					scripts[function.UUID] = name + ".js"
				case PYTHON:
					scripts[function.UUID] = name + ".py"
				}
			}
		}
		var user_names = []NicklabeL{}
		var group_names = []NicklabeL{{
			Label: "私聊",
			Value: "",
		}}
		nickname.Foreach(func(b1, b2 []byte) error {
			v := &Nickname{}
			code := string(b1)
			err := json.Unmarshal(b2, v)
			if err == nil {
				if v.Group {
					if Contains(chat_ids, code) {
						group_names = append(group_names, NicklabeL{
							Label: fmt.Sprintf("%s(%s)", v.Value, code),
							Value: code,
						})
					}
				} else {
					if Contains(user_ids, code) {
						user_names = append(user_names, NicklabeL{
							Label: fmt.Sprintf("%s(%s)", v.Value, code),
							Value: code,
						})
					}

				}
			}
			return nil
		})
		platforms := map[string][]string{}
		for _, plt := range getPltsArray() {
			platforms[plt] = GetAdapterBotsID(plt)
		}
		ApiOK(ctx, map[string]interface{}{
			"scripts":     scripts,
			"platforms":   platforms,
			"user_names":  user_names,
			"group_names": group_names,
		})
	})
	GinApi(POST, "/api/admin/tasks/:task_id/executions", RequireAuth, func(ctx *gin.Context) {
		taskID := strings.TrimSpace(ctx.Param("task_id"))
		if err := runTaskNow(taskID); err != nil {
			if strings.Contains(err.Error(), "不存在") {
				ApiNotFound(ctx, err.Error())
			} else {
				ApiUnprocessable(ctx, err.Error())
			}
			return
		}
		ApiOK(ctx, gin.H{"task_id": taskID, "status": "completed"})
	})

}

func validateTaskSchedule(schedule string) error {
	schedule = strings.TrimSpace(schedule)
	if schedule == "" {
		return fmt.Errorf("Cron表达式不能为空，例如：0 * * * *")
	}
	id, err := CRON.AddFunc(schedule, func() {})
	if err != nil {
		return fmt.Errorf("Cron表达式格式错误，例如：0 * * * *。错误：%v", err)
	}
	CRON.Remove(id)
	return nil
}

func runScriptTaskCommand(command string, targets []Sender) bool {
	target, class := scriptTaskTarget(command)
	if target == "" {
		return false
	}
	f := scriptFunctionByCommandTarget(target, class)
	if f == nil || f.Handle == nil {
		console.Error("定时任务脚本不存在：%s", target)
		return true
	}
	if !pluginExecutionEnabled(f) {
		console.Log("定时任务已跳过，插件未启用：%s", f.Title)
		return true
	}
	if len(targets) == 0 {
		sender := &CustomSender{
			F: &Factory{
				botplt: "task",
				uuid:   f.UUID,
			},
		}
		sender.SetFsps(&common.FakerSenderParams{Content: command})
		sender.SetMatch([]string{})
		sender.SetParams([]string{})
		f.Handle(sender)
		return true
	}
	for _, target := range targets {
		adapter, err := GetAdapter(target.Platfrom, target.BotID)
		if err != nil || adapter == nil {
			console.Error("定时任务接收平台不可用：%s(%s)", target.Platfrom, target.BotID)
			continue
		}
		sender := adapter.Sender2(nil)
		sender.SetFsps(&common.FakerSenderParams{
			Content: command,
			ChatID:  target.ChatID,
			UserID:  target.UserID,
		})
		sender.SetMatch([]string{})
		sender.SetParams([]string{})
		f.Handle(sender)
	}
	return true
}

func isScriptTaskCommand(command string) bool {
	target, _ := scriptTaskTarget(command)
	return target != ""
}

func scriptTaskTarget(command string) (string, string) {
	command = strings.TrimSpace(command)
	class := ""
	switch {
	case strings.HasPrefix(command, "node "):
		class = NODE
		command = strings.TrimSpace(strings.TrimPrefix(command, "node "))
	case strings.HasPrefix(command, "python "):
		class = PYTHON
		command = strings.TrimSpace(strings.TrimPrefix(command, "python "))
	default:
		return "", ""
	}
	target := strings.Trim(command, `"'`)
	if target == "" {
		return "", ""
	}
	return target, class
}

func scriptFunctionByCommandTarget(target string, class string) *common.Function {
	cleanTarget := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(filepath.ToSlash(target)), ".js"), ".py")
	for _, f := range Functions {
		if f == nil || f.Type != class {
			continue
		}
		pluginName := nodePluginNameFromPath(f.Path)
		title := strings.TrimSuffix(strings.TrimSuffix(f.Title, ".js"), ".py")
		if pluginName == cleanTarget || title == cleanTarget {
			return f
		}
	}
	return nil
}

func updatePluginCronAnnotation(f *common.Function, _ string, schedule string) error {
	if f == nil || f.Path == "" {
		return fmt.Errorf("脚本不存在")
	}
	schedule = strings.TrimSpace(schedule)
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return err
	}
	next := upsertPluginCronAnnotation(string(data), schedule, f.Type)
	if err := os.WriteFile(f.Path, []byte(next), 0644); err != nil {
		return err
	}
	if f.Reload != nil {
		f.Reload()
	}
	return nil
}

func upsertPluginCronAnnotation(script, schedule string, scriptType ...string) string {
	newline := "\n"
	if strings.Contains(script, "\r\n") {
		newline = "\r\n"
	}
	kind := ""
	if len(scriptType) > 0 {
		kind = scriptType[0]
	}
	lines := strings.Split(strings.ReplaceAll(script, "\r\n", "\n"), "\n")
	cronLine := regexp.MustCompile(`^(\s*(?://|#+)\s*\[\s*cron\s*:\s*)(.*?)(\s*\]\s*)$|^(\s*\*\s*@cron\s+)(.+?)\s*$`)
	updated := false
	out := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		if match := cronLine.FindStringSubmatch(line); len(match) != 0 {
			if !updated && schedule != "" {
				if match[1] != "" {
					out = append(out, match[1]+formatCronMetaValue(schedule)+match[3])
				} else {
					out = append(out, match[4]+formatCronMetaValue(schedule))
				}
			}
			updated = true
			continue
		}
		out = append(out, line)
	}
	if !updated && schedule != "" {
		prefix := "//"
		if kind == PYTHON {
			prefix = "#"
		}
		insert := prefix + " [cron: " + formatCronMetaValue(schedule) + "]"
		inserted := false
		lastMeta := -1
		for i, line := range out {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" && lastMeta < 0 {
				continue
			}
			if pluginLegacyMetaLinePattern.MatchString(line) {
				lastMeta = i
				continue
			}
			break
		}
		if lastMeta >= 0 {
			out = append(out[:lastMeta+1], append([]string{insert}, out[lastMeta+1:]...)...)
			inserted = true
		}
		if !inserted {
			out = append([]string{insert, ""}, out...)
		}
	}
	return strings.Join(out, newline)
}

func formatCronMetaValue(schedule string) string {
	return strings.TrimSpace(schedule)
}

type byCreatedAt2 []*Tasks

func (s byCreatedAt2) Len() int {
	return len(s)
}

func (s byCreatedAt2) Less(i, j int) bool {
	return s[i].CreatedAt > s[j].CreatedAt
}

func (s byCreatedAt2) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
