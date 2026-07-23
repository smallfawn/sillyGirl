package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/goccy/go-json"
)

type daidaiRuntimeClient struct {
	vm         *goja.Runtime
	panel      DaidaiPanel
	panelIndex int
	token      string
	expiration int64
}

func installDaidaiRuntime(vm *goja.Runtime) {
	constructor := func(call goja.ConstructorCall) *goja.Object {
		panel, panelIndex, err := daidaiPanelFromArgs(call.Arguments)
		if err != nil {
			panic(Error(vm, err))
		}
		client := &daidaiRuntimeClient{
			vm:         vm,
			panel:      panel,
			panelIndex: panelIndex,
		}
		o := call.This
		o.Set("id", panelIndex)
		o.Set("uuid", panel.ID)
		o.Set("name", panel.Name)
		o.Set("address", panel.Address)
		o.Set("request", client.Request)
		o.Set("getEnvs", client.GetEnvs)
		o.Set("getEnvById", client.GetEnvById)
		o.Set("createEnv", client.CreateEnv)
		o.Set("updateEnv", client.UpdateEnv)
		o.Set("deleteEnv", client.DeleteEnv)
		o.Set("deleteEnvs", client.DeleteEnvs)
		o.Set("enableEnv", client.EnableEnv)
		o.Set("disableEnv", client.DisableEnv)
		o.Set("enableEnvs", client.EnableEnvs)
		o.Set("disableEnvs", client.DisableEnvs)
		o.Set("getTasks", client.GetTasks)
		o.Set("getTaskById", client.GetTaskById)
		o.Set("createTask", client.CreateTask)
		o.Set("updateTask", client.UpdateTask)
		o.Set("deleteTask", client.DeleteTask)
		o.Set("runTask", client.RunTask)
		o.Set("stopTask", client.StopTask)
		o.Set("enableTask", client.EnableTask)
		o.Set("disableTask", client.DisableTask)
		o.Set("systemNotify", client.SystemNotify)
		return o
	}
	vm.Set("daidai", constructor)
}

func daidaiPanelFromArgs(args []goja.Value) (DaidaiPanel, int, error) {
	if len(args) == 0 || goja.IsUndefined(args[0]) || goja.IsNull(args[0]) {
		return DaidaiPanel{}, 0, errors.New("缺少呆呆面板编号，例如 new daidai({ id: 1 })")
	}
	ref := args[0].Export()
	if obj, ok := ref.(map[string]interface{}); ok {
		if v, ok := obj["id"]; ok {
			ref = v
		} else if v, ok := obj["ID"]; ok {
			ref = v
		} else {
			return DaidaiPanel{}, 0, errors.New("缺少呆呆面板编号，例如 new daidai({ id: 1 })")
		}
	} else {
		return DaidaiPanel{}, 0, errors.New("请使用 new daidai({ id: 1 }) 创建呆呆面板实例")
	}
	return daidaiPanelByReference(ref)
}

func daidaiPanelByReference(ref interface{}) (DaidaiPanel, int, error) {
	panels := getDaidaiPanels()
	index, ok := daidaiPanelIndex(ref)
	if ok {
		if index < 1 || index > len(panels) {
			return DaidaiPanel{}, 0, fmt.Errorf("呆呆面板编号 %d 不存在", index)
		}
		return panels[index-1], index, nil
	}
	id := strings.TrimSpace(fmt.Sprint(ref))
	for i, panel := range panels {
		if panel.ID == id {
			return panel, i + 1, nil
		}
	}
	return DaidaiPanel{}, 0, fmt.Errorf("呆呆面板 %s 不存在", id)
}

func daidaiPanelIndex(ref interface{}) (int, bool) {
	switch v := ref.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint:
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(v))
		return i, err == nil
	default:
		return 0, false
	}
}

func (client *daidaiRuntimeClient) GetEnvs(options ...interface{}) interface{} {
	query := map[string]interface{}{}
	if len(options) > 0 {
		if opts, ok := options[0].(map[string]interface{}); ok {
			query = opts
		} else if keyword := strings.TrimSpace(fmt.Sprint(options[0])); keyword != "" {
			query["keyword"] = keyword
		}
	}
	return client.requestJSON(http.MethodGet, "/api/envs", nil, query, true)
}

func (client *daidaiRuntimeClient) GetEnvById(id interface{}) interface{} {
	return client.requestJSON(http.MethodGet, "/api/envs/"+fmt.Sprint(id), nil, nil, true)
}

func (client *daidaiRuntimeClient) CreateEnv(env interface{}) interface{} {
	return client.requestJSON(http.MethodPost, "/api/envs", env, nil, true)
}

func (client *daidaiRuntimeClient) UpdateEnv(env interface{}) interface{} {
	envMap, ok := env.(map[string]interface{})
	if !ok {
		return client.requestJSON(http.MethodPut, "/api/envs", env, nil, true)
	}
	id := firstDaidaiID(envMap)
	if id == "" {
		return client.requestJSON(http.MethodPut, "/api/envs", env, nil, true)
	}
	body := map[string]interface{}{}
	for key, value := range envMap {
		if key != "id" && key != "ID" {
			body[key] = value
		}
	}
	return client.requestJSON(http.MethodPut, "/api/envs/"+id, body, nil, true)
}

func (client *daidaiRuntimeClient) DeleteEnv(id interface{}) interface{} {
	return client.requestJSON(http.MethodDelete, "/api/envs/"+fmt.Sprint(id), nil, nil, false)
}

func (client *daidaiRuntimeClient) DeleteEnvs(ids interface{}) interface{} {
	return client.requestJSON(http.MethodDelete, "/api/envs/batch", map[string]interface{}{"ids": normalizeDaidaiIDs(ids)}, nil, false)
}

func (client *daidaiRuntimeClient) EnableEnv(id interface{}) interface{} {
	return client.requestJSON(http.MethodPut, "/api/envs/"+fmt.Sprint(id)+"/enable", nil, nil, true)
}

func (client *daidaiRuntimeClient) DisableEnv(id interface{}) interface{} {
	return client.requestJSON(http.MethodPut, "/api/envs/"+fmt.Sprint(id)+"/disable", nil, nil, true)
}

func (client *daidaiRuntimeClient) EnableEnvs(ids interface{}) interface{} {
	return client.requestJSON(http.MethodPut, "/api/envs/batch/enable", map[string]interface{}{"ids": normalizeDaidaiIDs(ids)}, nil, false)
}

func (client *daidaiRuntimeClient) DisableEnvs(ids interface{}) interface{} {
	return client.requestJSON(http.MethodPut, "/api/envs/batch/disable", map[string]interface{}{"ids": normalizeDaidaiIDs(ids)}, nil, false)
}

func (client *daidaiRuntimeClient) GetTasks(options ...interface{}) interface{} {
	query := map[string]interface{}{}
	if len(options) > 0 {
		if opts, ok := options[0].(map[string]interface{}); ok {
			query = opts
		} else if keyword := strings.TrimSpace(fmt.Sprint(options[0])); keyword != "" {
			query["keyword"] = keyword
		}
	}
	return client.requestJSON(http.MethodGet, "/api/tasks", nil, query, true)
}

func (client *daidaiRuntimeClient) GetTaskById(id interface{}) interface{} {
	return client.requestJSON(http.MethodGet, "/api/tasks/"+fmt.Sprint(id), nil, nil, true)
}

func (client *daidaiRuntimeClient) CreateTask(task interface{}) interface{} {
	return client.requestJSON(http.MethodPost, "/api/tasks", task, nil, true)
}

func (client *daidaiRuntimeClient) UpdateTask(task interface{}) interface{} {
	taskMap, ok := task.(map[string]interface{})
	if !ok {
		return client.requestJSON(http.MethodPut, "/api/tasks", task, nil, true)
	}
	id := firstDaidaiID(taskMap)
	if id == "" {
		return client.requestJSON(http.MethodPut, "/api/tasks", task, nil, true)
	}
	body := map[string]interface{}{}
	for key, value := range taskMap {
		if key != "id" && key != "ID" {
			body[key] = value
		}
	}
	return client.requestJSON(http.MethodPut, "/api/tasks/"+id, body, nil, true)
}

func (client *daidaiRuntimeClient) DeleteTask(id interface{}) interface{} {
	return client.requestJSON(http.MethodDelete, "/api/tasks/"+fmt.Sprint(id), nil, nil, false)
}

func (client *daidaiRuntimeClient) RunTask(id interface{}) interface{} {
	return client.requestJSON(http.MethodPut, "/api/tasks/"+fmt.Sprint(id)+"/run", nil, nil, false)
}

func (client *daidaiRuntimeClient) StopTask(id interface{}) interface{} {
	return client.requestJSON(http.MethodPut, "/api/tasks/"+fmt.Sprint(id)+"/stop", nil, nil, false)
}

func (client *daidaiRuntimeClient) EnableTask(id interface{}) interface{} {
	return client.requestJSON(http.MethodPut, "/api/tasks/"+fmt.Sprint(id)+"/enable", nil, nil, false)
}

func (client *daidaiRuntimeClient) DisableTask(id interface{}) interface{} {
	return client.requestJSON(http.MethodPut, "/api/tasks/"+fmt.Sprint(id)+"/disable", nil, nil, false)
}

func (client *daidaiRuntimeClient) SystemNotify(title, content string) interface{} {
	return client.requestJSON(http.MethodPost, "/api/notifications/send", map[string]interface{}{
		"title":   title,
		"content": content,
	}, nil, false)
}

func (client *daidaiRuntimeClient) Request(method, path string, args ...interface{}) interface{} {
	var body interface{}
	var query map[string]interface{}
	if len(args) > 0 {
		body = args[0]
	}
	if len(args) > 1 {
		if v, ok := args[1].(map[string]interface{}); ok {
			query = v
		}
	}
	return client.requestJSON(method, path, body, query, false)
}

func normalizeDaidaiIDs(ids interface{}) []interface{} {
	switch v := ids.(type) {
	case []interface{}:
		return v
	case []int:
		out := make([]interface{}, 0, len(v))
		for _, id := range v {
			out = append(out, id)
		}
		return out
	case string:
		parts := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\n' || r == '\t'
		})
		out := make([]interface{}, 0, len(parts))
		for _, part := range parts {
			if id, err := strconv.Atoi(part); err == nil {
				out = append(out, id)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return []interface{}{ids}
}

func firstDaidaiID(values map[string]interface{}) string {
	for _, key := range []string{"id", "ID"} {
		if value, ok := values[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func (client *daidaiRuntimeClient) requestJSON(method, apiPath string, body interface{}, query map[string]interface{}, dataOnly bool) interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.ensureToken(ctx); err != nil {
		panic(Error(client.vm, err))
	}
	apiPath = normalizeDaidaiAPIPath(apiPath)
	values := url.Values{}
	for key, value := range query {
		if value != nil {
			values.Set(key, fmt.Sprint(value))
		}
	}
	u := client.panel.Address + apiPath
	if encoded := values.Encode(); encoded != "" {
		u += "?" + encoded
	}
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			panic(Error(client.vm, err))
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), u, payload)
	if err != nil {
		panic(Error(client.vm, err))
	}
	req.Header.Set("Authorization", "Bearer "+client.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(Error(client.vm, err))
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(Error(client.vm, err))
	}
	result := map[string]interface{}{}
	if len(bytes.TrimSpace(raw)) != 0 {
		if err := json.Unmarshal(raw, &result); err != nil {
			panic(Error(client.vm, fmt.Errorf("呆呆面板接口返回无法解析：%v", err)))
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		panic(Error(client.vm, fmt.Errorf("呆呆面板接口 HTTP %d：%s", resp.StatusCode, daidaiResultMessage(result))))
	}
	if success, ok := result["success"].(bool); ok && !success {
		panic(Error(client.vm, errors.New(daidaiResultMessage(result))))
	}
	if dataOnly {
		if data, ok := result["data"]; ok {
			return data
		}
	}
	return result
}

func (client *daidaiRuntimeClient) ensureToken(ctx context.Context) error {
	now := time.Now().Unix()
	if client.token != "" && client.expiration > now+60 {
		return nil
	}
	tokenResp, err := requestDaidaiToken(ctx, client.panel)
	if err != nil {
		return err
	}
	if tokenResp.Data.AccessToken == "" {
		return errors.New(daidaiTokenMessage(tokenResp, "认证失败，请检查 app_key/app_secret"))
	}
	client.token = tokenResp.Data.AccessToken
	client.expiration = now + tokenResp.Data.ExpiresIn
	return nil
}

func normalizeDaidaiAPIPath(apiPath string) string {
	apiPath = strings.TrimSpace(apiPath)
	if apiPath == "" {
		return "/api/health"
	}
	if !strings.HasPrefix(apiPath, "/") {
		apiPath = "/" + apiPath
	}
	if !strings.HasPrefix(apiPath, "/api/") && apiPath != "/api" {
		apiPath = "/api" + apiPath
	}
	return apiPath
}

func daidaiResultMessage(result map[string]interface{}) string {
	for _, key := range []string{"message", "error", "errorMessage"} {
		if message := strings.TrimSpace(fmt.Sprint(result[key])); message != "" && message != "<nil>" {
			return message
		}
	}
	return "请求失败"
}
