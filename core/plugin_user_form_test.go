package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/smallfawn/sillyGirl/core/common"
)

func TestRegisterNodePluginAndUserForms(t *testing.T) {
	if _, err := resolveNodeCommand(); err != nil {
		t.Skipf("node not available: %v", err)
	}
	uuid := "test_user_form_registration"
	pluginConfigSchemas.Set(uuid, nil)
	pluginUserFormSchemas.Set(uuid, nil)
	t.Cleanup(func() {
		pluginConfigSchemas.Set(uuid, nil)
		pluginUserFormSchemas.Set(uuid, nil)
	})

	path := filepath.Join("..", "plugins", "userFormTest.js")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := registerNodePluginConfigSchema(path, uuid); err != nil {
		t.Fatalf("register test plugin forms: %v", err)
	}
	if schema := pluginConfigSchemas.GetString(uuid); !strings.Contains(schema, `"enabled"`) {
		t.Fatalf("plugin.Form was not registered: %s", schema)
	}
	definition, ok := getPluginUserFormDefinition(uuid)
	if !ok {
		t.Fatal("user.Form was not registered")
	}
	if definition.Multiple != 3 || len(definition.KeyBy) != 1 || definition.KeyBy[0] != "phone" {
		t.Fatalf("multiple/keyBy not registered: %#v", definition)
	}
	phone, _ := definition.Schema["properties"].(map[string]interface{})["phone"].(map[string]interface{})
	if phone["pattern"] != `^1[3-9]\d{9}$` {
		t.Fatalf("match regex was not registered: %#v", phone)
	}
	order, _ := definition.Schema["propertyOrder"].([]interface{})
	if len(order) != 3 || order[0] != "phone" || order[1] != "openid" || order[2] != "remark" {
		t.Fatalf("field declaration order was not preserved: %#v", order)
	}
}

func TestRegisterNodeUserFormTestFunction(t *testing.T) {
	if _, err := resolveNodeCommand(); err != nil {
		t.Skipf("node not available: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "validator.js")
	source := `const { user } = require("sillygirl");
new user.Form({ token: user.Form.string().test(async (value, ctx) => value === ctx.config.expected).err("认证失败") });`
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	uuid := "test_node_user_form_validator"
	pluginUserFormSchemas.Set(uuid, nil)
	t.Cleanup(func() { pluginUserFormSchemas.Set(uuid, nil) })
	if err := registerNodePluginConfigSchema(path, uuid); err != nil {
		t.Fatal(err)
	}
	definition, ok := getPluginUserFormDefinition(uuid)
	if !ok || len(definition.Validators["token"]) != 1 {
		t.Fatalf("validator not registered: %#v", definition)
	}
	validator := definition.Validators["token"][0]
	if validator.Runtime != NODE || validator.Message != "认证失败" || !strings.Contains(validator.Source, "ctx.config.expected") {
		t.Fatalf("unexpected validator: %#v", validator)
	}
}

func TestRegisterPythonUserFormTestFunction(t *testing.T) {
	if _, _, err := resolvePythonCommand(); err != nil {
		t.Skipf("python not available: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "validator.py")
	source := `from sillygirl import user

async def verify(value, ctx):
    return value == ctx["config"].get("expected")

user.Form({"token": user.Form.string().test(verify).err("认证失败")})
`
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	uuid := "test_python_user_form_validator"
	pluginUserFormSchemas.Set(uuid, nil)
	t.Cleanup(func() { pluginUserFormSchemas.Set(uuid, nil) })
	if err := registerPythonPluginConfigSchema(path, uuid); err != nil {
		t.Fatal(err)
	}
	definition, ok := getPluginUserFormDefinition(uuid)
	if !ok || len(definition.Validators["token"]) != 1 {
		t.Fatalf("validator not registered: %#v", definition)
	}
	validator := definition.Validators["token"][0]
	if validator.Runtime != PYTHON || validator.Message != "认证失败" || !strings.Contains(validator.Source, "async def verify") {
		t.Fatalf("unexpected validator: %#v", validator)
	}
}

func TestExecuteNodeUserFormTestFunction(t *testing.T) {
	if _, err := resolveNodeCommand(); err != nil {
		t.Skipf("node not available: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "validator.js")
	if err := os.WriteFile(path, []byte("// test"), 0600); err != nil {
		t.Fatal(err)
	}
	plugin := &common.Function{Type: NODE, Path: path}
	payload := userFormValidatorInput{
		Values:     map[string]interface{}{"token": "bad"},
		Context:    map[string]interface{}{"config": map[string]interface{}{"expected": "good"}},
		Validators: []userFormValidatorEntry{{Field: "token", Source: `async (value, ctx) => value === ctx.config.expected`, Message: "认证失败"}},
	}
	result, err := executeUserFormValidators(context.Background(), plugin, payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0]["message"] != "认证失败" {
		t.Fatalf("unexpected result: %#v", result)
	}
	payload.Values["token"] = "good"
	result, err = executeUserFormValidators(context.Background(), plugin, payload)
	if err != nil || len(result) != 0 {
		t.Fatalf("valid token rejected: %#v, %v", result, err)
	}
}

func TestExecutePythonUserFormTestFunction(t *testing.T) {
	if _, _, err := resolvePythonCommand(); err != nil {
		t.Skipf("python not available: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "validator.py")
	if err := os.WriteFile(path, []byte("# test"), 0600); err != nil {
		t.Fatal(err)
	}
	plugin := &common.Function{Type: PYTHON, Path: path}
	payload := userFormValidatorInput{
		Values: map[string]interface{}{"token": "bad"}, Context: map[string]interface{}{},
		Validators: []userFormValidatorEntry{{Field: "token", Source: "async def verify(value, ctx):\n    return True if value == 'good' else '认证失败'"}},
	}
	result, err := executeUserFormValidators(context.Background(), plugin, payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0]["message"] != "认证失败" {
		t.Fatalf("unexpected result: %#v", result)
	}
	payload.Values["token"] = "good"
	result, err = executeUserFormValidators(context.Background(), plugin, payload)
	if err != nil || len(result) != 0 {
		t.Fatalf("valid token rejected: %#v, %v", result, err)
	}
}

func TestUserFormValidatorEnvironmentIsolation(t *testing.T) {
	t.Setenv("SILLYGIRL_VALIDATOR_SECRET", "must-not-leak")
	if _, err := resolveNodeCommand(); err == nil {
		dir := t.TempDir()
		path := filepath.Join(dir, "validator.js")
		if err := os.WriteFile(path, []byte("// test"), 0600); err != nil {
			t.Fatal(err)
		}
		payload := userFormValidatorInput{Values: map[string]interface{}{"token": "x"}, Context: map[string]interface{}{}, Validators: []userFormValidatorEntry{{Field: "token", Source: `() => process.env.SILLYGIRL_VALIDATOR_SECRET === undefined || "环境变量泄漏"`}}}
		result, err := executeUserFormValidators(context.Background(), &common.Function{Type: NODE, Path: path}, payload)
		if err != nil || len(result) != 0 {
			t.Fatalf("node validator inherited secret environment: %#v, %v", result, err)
		}
	}
	if _, _, err := resolvePythonCommand(); err == nil {
		dir := t.TempDir()
		path := filepath.Join(dir, "validator.py")
		if err := os.WriteFile(path, []byte("# test"), 0600); err != nil {
			t.Fatal(err)
		}
		source := "def verify(value, ctx):\n    import os\n    return os.environ.get('SILLYGIRL_VALIDATOR_SECRET') is None or '环境变量泄漏'"
		payload := userFormValidatorInput{Values: map[string]interface{}{"token": "x"}, Context: map[string]interface{}{}, Validators: []userFormValidatorEntry{{Field: "token", Source: source}}}
		result, err := executeUserFormValidators(context.Background(), &common.Function{Type: PYTHON, Path: path}, payload)
		if err != nil || len(result) != 0 {
			t.Fatalf("python validator inherited secret environment: %#v, %v", result, err)
		}
	}
}

func TestUserFormValidatorStopsAfterFirstFailure(t *testing.T) {
	if _, err := resolveNodeCommand(); err != nil {
		t.Skipf("node not available: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "validator.js")
	if err := os.WriteFile(path, []byte("// test"), 0600); err != nil {
		t.Fatal(err)
	}
	payload := userFormValidatorInput{
		Values: map[string]interface{}{"token": "x"}, Context: map[string]interface{}{},
		Validators: []userFormValidatorEntry{
			{Field: "token", Source: `() => "第一个错误"`},
			{Field: "token", Source: `() => "第二个错误"`},
		},
	}
	result, err := executeUserFormValidators(context.Background(), &common.Function{Type: NODE, Path: path}, payload)
	if err != nil || len(result) != 1 || result[0]["message"] != "第一个错误" {
		t.Fatalf("unexpected result: %#v, %v", result, err)
	}
}

func TestUserFormValidatorTimeout(t *testing.T) {
	if _, err := resolveNodeCommand(); err != nil {
		t.Skipf("node not available: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "validator.js")
	if err := os.WriteFile(path, []byte("// test"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	payload := userFormValidatorInput{Values: map[string]interface{}{"token": "x"}, Context: map[string]interface{}{}, Validators: []userFormValidatorEntry{{Field: "token", Source: `async () => await new Promise(() => setInterval(() => {}, 1000))`}}}
	_, err := executeUserFormValidators(ctx, &common.Function{Type: NODE, Path: path}, payload)
	if err == nil || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("validator timeout was not enforced: %v", err)
	}
}

func TestRunUserFormValidatorSkipsEmptyAndRejectsRuntimeMismatch(t *testing.T) {
	definition := pluginUserFormDefinition{Validators: map[string][]pluginUserFormValidator{"token": {{Runtime: PYTHON, Source: "lambda value, ctx: True"}}}}
	plugin := &common.Function{Type: NODE, UUID: "validator-mismatch"}
	user := &normalUser{ID: "user", Username: "user"}
	if result := runPluginUserFormValidators(context.Background(), plugin, definition, user, map[string]interface{}{}); len(result) != 0 {
		t.Fatalf("empty optional value should skip test: %#v", result)
	}
	result := runPluginUserFormValidators(context.Background(), plugin, definition, user, map[string]interface{}{"token": "x"})
	if len(result) != 1 || result[0]["message"] != "校验器运行时配置错误" {
		t.Fatalf("runtime mismatch bypassed validation: %#v", result)
	}
}

func TestValidatePluginUserFormMatchAndErr(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"phone": map[string]interface{}{
				"type":     "string",
				"required": true,
				"pattern":  `^1[3-9]\d{9}$`,
				"errorMessages": map[string]interface{}{
					"required": "请填写手机号",
					"match":    "手机号格式错误",
				},
			},
		},
	}
	values, errorsList := validatePluginUserForm(schema, map[string]interface{}{"phone": "13800138000"})
	if len(errorsList) != 0 || values["phone"] != "13800138000" {
		t.Fatalf("valid phone rejected: values=%#v errors=%#v", values, errorsList)
	}
	_, errorsList = validatePluginUserForm(schema, map[string]interface{}{"phone": "123"})
	if len(errorsList) != 1 || errorsList[0]["code"] != "match" || errorsList[0]["message"] != "手机号格式错误" {
		t.Fatalf("custom match error lost: %#v", errorsList)
	}
	_, errorsList = validatePluginUserForm(schema, map[string]interface{}{})
	if len(errorsList) != 1 || errorsList[0]["message"] != "请填写手机号" {
		t.Fatalf("custom required error lost: %#v", errorsList)
	}
}

func TestPluginUserFormRecordsArePlaintext(t *testing.T) {
	userID, pluginID := "plain-user", "plain-plugin"
	key := pluginUserFormStorageKey(userID, pluginID)
	pluginUserFormRecords.Set(key, nil)
	t.Cleanup(func() { pluginUserFormRecords.Set(key, nil) })
	records := []pluginUserFormRecord{{ID: "record-1", Values: map[string]interface{}{"phone": "13800138000"}, CreatedAt: 1, UpdatedAt: 1}}
	if err := savePluginUserRecords(userID, pluginID, records); err != nil {
		t.Fatal(err)
	}
	raw := pluginUserFormRecords.GetString(key)
	if !strings.Contains(raw, "13800138000") {
		t.Fatalf("record was unexpectedly transformed: %s", raw)
	}
	loaded := pluginUserRecords(userID, pluginID)
	if len(loaded) != 1 || loaded[0].Values["phone"] != "13800138000" {
		t.Fatalf("plaintext record round-trip failed: %#v", loaded)
	}
}

func TestDeletePluginConfigRemovesUserFormData(t *testing.T) {
	pluginID := "delete-user-form-plugin"
	keys := []string{"user-a:" + pluginID, "user-b:" + pluginID}
	pluginUserFormSchemas.Set(pluginID, `{"schema":{"type":"object"}}`)
	for _, key := range keys {
		pluginUserFormRecords.Set(key, `[{"id":"record"}]`)
	}
	t.Cleanup(func() {
		pluginUserFormSchemas.Set(pluginID, nil)
		for _, key := range keys {
			pluginUserFormRecords.Set(key, nil)
		}
	})
	deletePluginConfig(pluginID, true)
	if pluginUserFormSchemas.GetString(pluginID) != "" {
		t.Fatal("user form schema survived synchronized config deletion")
	}
	for _, key := range keys {
		if pluginUserFormRecords.GetString(key) != "" {
			t.Fatalf("user form record survived synchronized config deletion: %s", key)
		}
	}
}
