package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/utils"
)

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

func TestDisabledTaskIsNotRegistered(t *testing.T) {
	task := &Tasks{Schedule: "0 * * * *", Enable: false}
	RegistTasks(task)
	if task.CronID != 0 {
		t.Fatalf("disabled task cron id = %d; want 0", task.CronID)
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

func TestSetTaskEnabledUpdatesPluginConfig(t *testing.T) {
	uuid := "plugin-toggle-test-" + utils.GenUUID()
	f := &common.Function{UUID: uuid, Type: NODE, Cron: map[string]string{"task": "0 * * * *"}}
	previous := Functions
	Functions = append(Functions, f)
	t.Cleanup(func() {
		Functions = previous
		pluginConfigValues.Set(uuid, "")
	})

	if err := setTaskEnabled(pluginCronTaskID(uuid, "task"), false); err != nil {
		t.Fatal(err)
	}
	if pluginExecutionEnabled(f) {
		t.Fatal("plugin cron remains enabled")
	}
}

func TestPluginCronEnableOnlyUpdateDoesNotRequireTitle(t *testing.T) {
	uuid := "plugin-toggle-request-test-" + utils.GenUUID()
	f := &common.Function{UUID: uuid, Type: NODE, Cron: map[string]string{"task": "0 * * * *"}}
	previous := Functions
	Functions = append(Functions, f)
	t.Cleanup(func() {
		Functions = previous
		pluginConfigValues.Set(uuid, "")
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	handled := handleTaskEnabledOnlyUpdate(ctx, pluginCronTaskID(uuid, "task"), map[string]interface{}{
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
	if pluginExecutionEnabled(f) {
		t.Fatal("plugin cron remains enabled after enable-only update")
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

func TestRunTaskNowExecutesPluginCron(t *testing.T) {
	uuid := "plugin-run-test-" + utils.GenUUID()
	called := 0
	f := &common.Function{
		UUID: uuid,
		Type: NODE,
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
		pluginConfigValues.Set(uuid, "")
	})

	if err := runTaskNow(pluginCronTaskID(uuid, "task")); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("plugin cron called %d times; want 1", called)
	}
	if err := runTaskNow(pluginCronTaskID(uuid, "missing")); err == nil {
		t.Fatal("unknown plugin cron platform was executed")
	}
	if err := setTaskEnabled(pluginCronTaskID(uuid, "task"), false); err != nil {
		t.Fatal(err)
	}
	if err := runTaskNow(pluginCronTaskID(uuid, "task")); err == nil {
		t.Fatal("disabled plugin cron was executed")
	}
}
