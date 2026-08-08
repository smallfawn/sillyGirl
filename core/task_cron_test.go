package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/utils"
)

func writeTaskTestPlugin(t *testing.T, name, content string) string {
	t.Helper()
	root := nodePluginsRoot()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(root, ".task-plugin-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseCronMetaValue(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"0 * * * *", "0 * * * *"},
		{"0 0 * * * *", "0 0 * * * *"},
		{"qq 0 * * * *", ""},
		{"telegram 0 0 * * * *", ""},
	}
	for _, tt := range tests {
		got := parseCronMetaValue(tt.value)
		if got != tt.want {
			t.Fatalf("parseCronMetaValue(%q) = %q; want %q", tt.value, got, tt.want)
		}
	}
}

func TestUpsertPluginCronAnnotation(t *testing.T) {
	script := "// [title: Demo]\n// [name: demo]\nconsole.log('ok');\n"
	updated := upsertPluginCronAnnotation(script, "0 * * * *", NODE)
	if !strings.Contains(updated, "// [cron: 0 * * * *]") {
		t.Fatalf("cron line was not inserted into header:\n%s", updated)
	}

	updated = upsertPluginCronAnnotation(updated, "*/5 * * * *", NODE)
	if strings.Count(updated, "[cron:") != 1 || !strings.Contains(updated, "// [cron: */5 * * * *]") {
		t.Fatalf("cron line was not updated cleanly:\n%s", updated)
	}

	updated = upsertPluginCronAnnotation(updated, "", NODE)
	if strings.Contains(updated, "[cron:") {
		t.Fatalf("cron line was not removed:\n%s", updated)
	}
}

func TestUpsertPluginCronAnnotationPython(t *testing.T) {
	script := "# [title: Demo]\n# [name: demo]\nprint('ok')\n"
	updated := upsertPluginCronAnnotation(script, "0 * * * *", PYTHON)
	if !strings.Contains(updated, "# [cron: 0 * * * *]") {
		t.Fatalf("cron line was not inserted into python header:\n%s", updated)
	}

	noHeader := "print('ok')\n"
	updated = upsertPluginCronAnnotation(noHeader, "0 * * * *", PYTHON)
	if !strings.HasPrefix(updated, "# [cron: 0 * * * *]\n\n") {
		t.Fatalf("python header was not created:\n%s", updated)
	}
}

func TestUpsertPluginStatusAnnotation(t *testing.T) {
	legacy := "// [title: Demo]\n// [status: true] 保留说明\nconsole.log('ok');\n"
	updated := upsertPluginStatusAnnotation(legacy, false, NODE)
	if strings.Count(updated, "[status:") != 1 || !strings.Contains(updated, "// [status: false] 保留说明") {
		t.Fatalf("legacy status annotation was not updated cleanly:\n%s", updated)
	}

	atStyle := "/**\n * @title Demo\n * @rule ^demo$\n */\n"
	updated = upsertPluginStatusAnnotation(atStyle, false, NODE)
	if !strings.Contains(updated, " * @status false\n */") {
		t.Fatalf("@status annotation was not inserted inside the metadata block:\n%s", updated)
	}

	python := "# [title: Demo]\nprint('ok')\n"
	updated = upsertPluginStatusAnnotation(python, true, PYTHON)
	if !strings.Contains(updated, "# [status: true]") {
		t.Fatalf("python status annotation was not inserted:\n%s", updated)
	}

	shebang := "#!/usr/bin/env node\n// [title: Demo]\nconsole.log('ok');\n"
	updated = upsertPluginStatusAnnotation(shebang, false, NODE)
	if !strings.HasPrefix(updated, "#!/usr/bin/env node\n") || !strings.Contains(updated, "// [title: Demo]\n// [status: false]") {
		t.Fatalf("node shebang or metadata header was damaged:\n%s", updated)
	}
}

func TestScriptCommandForPythonFunction(t *testing.T) {
	f := &common.Function{
		Type: PYTHON,
		Path: filepath.Join("plugins", "daily.py"),
	}
	if got := scriptCommandForFunction(f); got != "python daily.py" {
		t.Fatalf("scriptCommandForFunction() = %q; want %q", got, "python daily.py")
	}
	target, class := scriptTaskTarget("python daily.py")
	if target != "daily.py" || class != PYTHON {
		t.Fatalf("scriptTaskTarget() = (%q, %q); want (%q, %q)", target, class, "daily.py", PYTHON)
	}
}

func TestScriptCommandAndLookupUsePublisherIdentity(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	root := nodePluginsRoot()
	first := &common.Function{UUID: "author-a-same", Type: NODE, Path: filepath.Join(root, "author-a", "same.js"), Title: "Same A"}
	second := &common.Function{UUID: "author-b-same", Type: NODE, Path: filepath.Join(root, "author-b", "same.js"), Title: "Same B"}
	previous := Functions
	Functions = []*common.Function{first, second}
	t.Cleanup(func() { Functions = previous })

	if got := scriptCommandForFunction(first); got != "node author-a/same.js" {
		t.Fatalf("publisher command = %q", got)
	}
	if got := scriptFunctionByCommandTarget("author-b/same.js", NODE); got != second {
		t.Fatalf("publisher lookup = %#v; want author-b plugin", got)
	}
	if got := scriptFunctionByCommandTarget("same.js", NODE); got != nil {
		t.Fatalf("ambiguous bare lookup = %#v; want nil", got)
	}
}

func TestDisabledTaskIsNotRegistered(t *testing.T) {
	task := &Tasks{Schedule: "0 * * * *", Enable: false}
	RegistTasks(task)
	if task.CronID != 0 {
		t.Fatalf("disabled task cron id = %d; want 0", task.CronID)
	}
}

func TestPluginCronRegistrationUsesTaskAndPluginStatesIndependently(t *testing.T) {
	uuid := "plugin-cron-registration-test-" + utils.GenUUID()
	disabledTaskID := pluginCronTaskID(uuid+"-task-off", "task")
	pluginCronStatus.Set(disabledTaskID, false)
	previous := Functions
	functions := []*common.Function{
		{
			UUID:   uuid + "-task-off",
			Status: pluginStatusValue(true),
			Cron:   map[string]string{"task": "0 0 1 1 *"},
			Handle: func(common.Sender) interface{} { return nil },
		},
		{
			UUID:   uuid + "-plugin-off",
			Status: pluginStatusValue(false),
			Cron:   map[string]string{"task": "0 0 1 1 *"},
			Handle: func(common.Sender) interface{} { return nil },
		},
		{
			UUID:   uuid + "-active",
			Status: pluginStatusValue(true),
			Cron:   map[string]string{"task": "0 0 1 1 *"},
			Handle: func(common.Sender) interface{} { return nil },
		},
	}
	t.Cleanup(func() {
		for _, function := range functions {
			for _, id := range function.CronIds {
				CRON.Remove(cron.EntryID(id))
			}
		}
		Functions = previous
		pluginCronStatus.Set(disabledTaskID, "")
	})

	AddCommand(functions)
	if len(functions[0].CronIds) != 0 {
		t.Fatal("task-disabled plugin cron was registered")
	}
	if len(functions[1].CronIds) != 0 {
		t.Fatal("plugin-disabled cron was registered")
	}
	if len(functions[2].CronIds) != 1 {
		t.Fatalf("active plugin cron registrations = %d; want 1", len(functions[2].CronIds))
	}
}

func TestSetTaskEnabledPersistsOrdinaryTask(t *testing.T) {
	id := "task-toggle-test-" + utils.GenUUID()
	task := Tasks{ID: id, Title: "toggle", Schedule: "0 * * * *", Enable: true}
	tasks.Set(id, utils.JsonMarshal(task))
	t.Cleanup(func() { tasks.Set(id, "") })

	if err := setTaskEnabled(id, false); err != nil {
		t.Fatal(err)
	}
	var saved Tasks
	if err := json.Unmarshal([]byte(tasks.GetString(id)), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Enable {
		t.Fatal("ordinary task remains enabled")
	}
}

func TestSetTaskEnabledPersistsPluginCronStateWithoutChangingPluginStatus(t *testing.T) {
	uuid := "plugin-toggle-test-" + utils.GenUUID()
	taskID := pluginCronTaskID(uuid, "task")
	path := writeTaskTestPlugin(t, "toggle.js", "// [title: Toggle]\n// [status: true]\n")
	f := &common.Function{UUID: uuid, Type: NODE, Path: path, Cron: map[string]string{"task": "0 * * * *"}}
	previous := Functions
	Functions = append(Functions, f)
	t.Cleanup(func() {
		Functions = previous
		pluginCronStatus.Set(taskID, "")
	})

	if err := setTaskEnabled(taskID, false); err != nil {
		t.Fatal(err)
	}
	if pluginCronTaskEnabled(uuid, "task") || pluginCronTask(f, "task").Enable {
		t.Fatal("plugin cron task remains enabled")
	}
	if !pluginExecutionEnabled(f) {
		t.Fatal("task switch changed the plugin status")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "// [status: true]") || strings.Contains(string(data), "// [status: false]") {
		t.Fatalf("task switch changed the plugin status annotation:\n%s", data)
	}
}

func TestPluginCronEnableOnlyUpdateDoesNotRequireTitle(t *testing.T) {
	uuid := "plugin-toggle-request-test-" + utils.GenUUID()
	taskID := pluginCronTaskID(uuid, "task")
	path := writeTaskTestPlugin(t, "toggle-request.js", "// [status: true]\n")
	f := &common.Function{UUID: uuid, Type: NODE, Path: path, Cron: map[string]string{"task": "0 * * * *"}}
	previous := Functions
	Functions = append(Functions, f)
	t.Cleanup(func() {
		Functions = previous
		pluginCronStatus.Set(taskID, "")
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	handled := handleTaskEnabledOnlyUpdate(ctx, taskID, map[string]interface{}{
		"enable": false,
	})
	if !handled {
		t.Fatal("enable-only update was not handled")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("enable-only update status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "定时任务标题不能为空") {
		t.Fatalf("enable-only update still requires a title: %s", recorder.Body.String())
	}
	if pluginCronTaskEnabled(uuid, "task") {
		t.Fatal("plugin cron task remains enabled after enable-only update")
	}
	if !pluginExecutionEnabled(f) {
		t.Fatal("enable-only task update changed the plugin status")
	}
}

func TestTaskEnableOnlyUpdateRejectsNonBooleanState(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	handled := handleTaskEnabledOnlyUpdate(ctx, "plugin-cron:fixture:task", map[string]interface{}{
		"enable": "false",
	})
	if !handled {
		t.Fatal("invalid enable-only update was not handled")
	}
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid enable-only update status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "定时任务状态必须是布尔值") {
		t.Fatalf("invalid enable-only update returned the wrong error: %s", recorder.Body.String())
	}
}

func TestPluginCronTaskHydrationSupportsPartialUpdates(t *testing.T) {
	uuid := "plugin-partial-update-test-" + utils.GenUUID()
	f := &common.Function{
		UUID:  uuid,
		Title: "Partial Update Fixture",
		Type:  NODE,
		Cron:  map[string]string{"task": "0 * * * *"},
	}
	previous := Functions
	Functions = append(Functions, f)
	t.Cleanup(func() { Functions = previous })

	current, platform := findScriptFunctionByTask(pluginCronTaskID(uuid, "task"), "")
	if current == nil {
		t.Fatal("plugin cron task was not found by its virtual task ID")
	}
	task := pluginCronTask(current, platform)
	if task.Title != f.Title || task.Schedule != "0 * * * *" || task.ID == "" {
		t.Fatalf("plugin cron task was not hydrated: %#v", task)
	}
}

func TestPluginCronTaskPersistsConfiguredRecipient(t *testing.T) {
	uuid := "plugin-recipient-persistence-test-" + utils.GenUUID()
	taskID := pluginCronTaskID(uuid, "task")
	want := []Sender{{
		Platfrom: "qqguild",
		UserID:   "recipient-open-id",
		BotID:    "bot-app-id",
	}}
	t.Cleanup(func() { _ = setPluginCronTaskSenders(taskID, nil) })

	if err := setPluginCronTaskSenders(taskID, want); err != nil {
		t.Fatal(err)
	}
	f := &common.Function{
		UUID:  uuid,
		Title: "Recipient Fixture",
		Type:  NODE,
		Cron:  map[string]string{"task": "0 * * * *"},
	}
	got := pluginCronTask(f, "task").Senders
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("plugin cron senders = %#v; want %#v", got, want)
	}

	if err := setPluginCronTaskSenders(taskID, nil); err != nil {
		t.Fatal(err)
	}
	if got := pluginCronTaskSenders(taskID); len(got) != 0 {
		t.Fatalf("cleared plugin cron senders = %#v; want empty", got)
	}
}

func TestPluginCronTaskPersistsTriggerPhrase(t *testing.T) {
	uuid := "plugin-trigger-persistence-test-" + utils.GenUUID()
	taskID := pluginCronTaskID(uuid, "task")
	t.Cleanup(func() { _ = setPluginCronTaskTrigger(taskID, "") })

	if err := setPluginCronTaskTrigger(taskID, "  查询 account-a  "); err != nil {
		t.Fatal(err)
	}
	f := &common.Function{
		UUID:  uuid,
		Title: "Trigger Fixture",
		Type:  NODE,
		Cron:  map[string]string{"task": "0 * * * *"},
	}
	if got := pluginCronTask(f, "task").Trigger; got != "查询 account-a" {
		t.Fatalf("plugin cron trigger = %q; want trimmed phrase", got)
	}

	if err := setPluginCronTaskTrigger(taskID, ""); err != nil {
		t.Fatal(err)
	}
	if got := pluginCronTaskTrigger(taskID); got != "" {
		t.Fatalf("cleared plugin cron trigger = %q; want empty", got)
	}
}

func TestRunPluginCronFunctionUsesTriggerPhraseAndRuleParams(t *testing.T) {
	uuid := "plugin-trigger-run-test-" + utils.GenUUID()
	taskID := pluginCronTaskID(uuid, "task")
	t.Cleanup(func() { _ = setPluginCronTaskTrigger(taskID, "") })
	if err := setPluginCronTaskTrigger(taskID, "查询 account-b"); err != nil {
		t.Fatal(err)
	}

	captured := make(chan common.Sender, 1)
	f := &common.Function{
		UUID:   uuid,
		Type:   NODE,
		Path:   "author/trigger.js",
		Rules:  []string{`^查询 (\S+)$`, `^刷新$`},
		Params: [][]string{{"account"}, {}},
		Handle: func(sender common.Sender) interface{} {
			captured <- sender
			return nil
		},
	}
	runPluginCronFunction(f, "task")
	sender := <-captured
	if got := sender.GetContent(); got != "查询 account-b" {
		t.Fatalf("content = %q, want trigger phrase", got)
	}
	if got := sender.GetMatch(); len(got) != 1 || got[0] != "account-b" {
		t.Fatalf("match = %#v, want account-b", got)
	}
}

func TestRunPluginCronFunctionUsesConfiguredRecipient(t *testing.T) {
	uuid := "plugin-recipient-run-test-" + utils.GenUUID()
	platform := "plugin-recipient-platform-" + utils.GenUUID()
	taskID := pluginCronTaskID(uuid, "task")
	const (
		botID  = "recipient-bot"
		userID = "recipient-user"
		chatID = "recipient-chat"
	)

	adapter := &Factory{}
	adapter.Init(platform, botID, nil)
	t.Cleanup(func() {
		adapter.Destroy()
		_ = setPluginCronTaskSenders(taskID, nil)
	})
	if err := setPluginCronTaskSenders(taskID, []Sender{{
		Platfrom: platform,
		BotID:    botID,
		UserID:   userID,
		ChatID:   chatID,
	}}); err != nil {
		t.Fatal(err)
	}

	captured := make(chan common.Sender, 1)
	f := &common.Function{
		UUID: uuid,
		Type: NODE,
		Path: "author/recipient.js",
		Handle: func(sender common.Sender) interface{} {
			captured <- sender
			return nil
		},
	}
	runPluginCronFunction(f, "task")
	sender := <-captured
	if sender.GetImType() != platform || sender.GetBotID() != botID || sender.GetUserID() != userID || sender.GetChatID() != chatID {
		t.Fatalf("recipient sender = platform:%q bot:%q user:%q chat:%q", sender.GetImType(), sender.GetBotID(), sender.GetUserID(), sender.GetChatID())
	}
}

func TestRunTaskNowExecutesPluginCron(t *testing.T) {
	uuid := "plugin-run-test-" + utils.GenUUID()
	taskID := pluginCronTaskID(uuid, "task")
	called := 0
	path := writeTaskTestPlugin(t, "run-task.js", "// [status: true]\n")
	f := &common.Function{
		UUID: uuid,
		Type: NODE,
		Path: path,
		Cron: map[string]string{"task": "0 * * * *"},
		Handle: func(common.Sender) interface{} {
			called++
			return nil
		},
	}
	previous := Functions
	Functions = append(Functions, f)
	t.Cleanup(func() {
		Functions = previous
		pluginCronStatus.Set(taskID, "")
	})

	if err := runTaskNow(taskID); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("plugin cron called %d times; want 1", called)
	}
	if err := runTaskNow(pluginCronTaskID(uuid, "missing")); err == nil {
		t.Fatal("unknown plugin cron platform was executed")
	}
	if err := setTaskEnabled(taskID, false); err != nil {
		t.Fatal(err)
	}
	if err := runTaskNow(taskID); err == nil || !strings.Contains(err.Error(), "定时任务未启用") {
		t.Fatalf("disabled plugin cron execution error = %v", err)
	}
	if !pluginExecutionEnabled(f) {
		t.Fatal("disabling the cron task changed the plugin status")
	}
	if err := setTaskEnabled(taskID, true); err != nil {
		t.Fatal(err)
	}
	f.Status = pluginStatusValue(false)
	if err := runTaskNow(taskID); err == nil || !strings.Contains(err.Error(), "插件未启用") {
		t.Fatalf("disabled plugin execution error = %v", err)
	}
}
