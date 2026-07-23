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

type qinglongRuntimeClient struct {
	vm         *goja.Runtime
	panel      QinglongPanel
	panelIndex int
	token      string
	expiration int64
}

func installQinglongRuntime(vm *goja.Runtime) {
	constructor := func(call goja.ConstructorCall) *goja.Object {
		panel, panelIndex, err := qinglongPanelFromArgs(call.Arguments)
		if err != nil {
			panic(Error(vm, err))
		}
		client := &qinglongRuntimeClient{
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
		o.Set("deleteEnvs", client.DeleteEnvs)
		o.Set("moveEnv", client.MoveEnv)
		o.Set("disableEnvs", client.DisableEnvs)
		o.Set("enableEnvs", client.EnableEnvs)
		o.Set("updateEnvNames", client.UpdateEnvNames)
		o.Set("systemNotify", client.SystemNotify)
		return o
	}
	vm.Set("qinglong", constructor)
}

func qinglongPanelFromArgs(args []goja.Value) (QinglongPanel, int, error) {
	if len(args) == 0 || goja.IsUndefined(args[0]) || goja.IsNull(args[0]) {
		return QinglongPanel{}, 0, errors.New("缺少青龙编号，例如 new qinglong({ id: 1 })")
	}
	ref := args[0].Export()
	if obj, ok := ref.(map[string]interface{}); ok {
		if v, ok := obj["id"]; ok {
			ref = v
		} else if v, ok := obj["ID"]; ok {
			ref = v
		} else {
			return QinglongPanel{}, 0, errors.New("缺少青龙编号，例如 new qinglong({ id: 1 })")
		}
	} else {
		return QinglongPanel{}, 0, errors.New("请使用 new qinglong({ id: 1 }) 创建青龙实例")
	}
	return qinglongPanelByReference(ref)
}

func qinglongPanelByReference(ref interface{}) (QinglongPanel, int, error) {
	panels := getQinglongPanels()
	index, ok := qinglongPanelIndex(ref)
	if ok {
		if index < 1 || index > len(panels) {
			return QinglongPanel{}, 0, fmt.Errorf("青龙编号 %d 不存在", index)
		}
		return panels[index-1], index, nil
	}
	id := strings.TrimSpace(fmt.Sprint(ref))
	for i, panel := range panels {
		if panel.ID == id {
			return panel, i + 1, nil
		}
	}
	return QinglongPanel{}, 0, fmt.Errorf("青龙面板 %s 不存在", id)
}

func qinglongPanelIndex(ref interface{}) (int, bool) {
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

func (client *qinglongRuntimeClient) GetEnvs(options ...interface{}) interface{} {
	query := map[string]interface{}{}
	if len(options) > 0 {
		if opts, ok := options[0].(map[string]interface{}); ok {
			query = opts
		} else if search := strings.TrimSpace(fmt.Sprint(options[0])); search != "" {
			query["searchValue"] = search
		}
	}
	return client.requestJSON(http.MethodGet, "/open/envs", nil, query, true)
}

func (client *qinglongRuntimeClient) GetEnvById(id interface{}) interface{} {
	return client.requestJSON(http.MethodGet, "/open/envs/"+fmt.Sprint(id), nil, nil, true)
}

func (client *qinglongRuntimeClient) CreateEnv(env interface{}) interface{} {
	if _, ok := env.([]interface{}); !ok {
		env = []interface{}{env}
	}
	return client.requestJSON(http.MethodPost, "/open/envs", env, nil, true)
}

func (client *qinglongRuntimeClient) UpdateEnv(env interface{}) interface{} {
	return client.requestJSON(http.MethodPut, "/open/envs", env, nil, true)
}

func (client *qinglongRuntimeClient) DeleteEnvs(ids interface{}) interface{} {
	return client.requestJSON(http.MethodDelete, "/open/envs", normalizeQinglongIDs(ids), nil, true)
}

func (client *qinglongRuntimeClient) MoveEnv(id interface{}, args ...interface{}) interface{} {
	body := map[string]interface{}{}
	if len(args) == 1 {
		if v, ok := args[0].(map[string]interface{}); ok {
			body = v
		}
	} else if len(args) >= 2 {
		body["fromIndex"] = args[0]
		body["toIndex"] = args[1]
	}
	return client.requestJSON(http.MethodPut, "/open/envs/"+fmt.Sprint(id)+"/move", body, nil, true)
}

func (client *qinglongRuntimeClient) DisableEnvs(ids interface{}) interface{} {
	return client.requestJSON(http.MethodPut, "/open/envs/disable", normalizeQinglongIDs(ids), nil, true)
}

func (client *qinglongRuntimeClient) EnableEnvs(ids interface{}) interface{} {
	return client.requestJSON(http.MethodPut, "/open/envs/enable", normalizeQinglongIDs(ids), nil, true)
}

func (client *qinglongRuntimeClient) UpdateEnvNames(args ...interface{}) interface{} {
	body := map[string]interface{}{}
	if len(args) == 1 {
		if v, ok := args[0].(map[string]interface{}); ok {
			body = v
		}
	} else if len(args) >= 2 {
		body["ids"] = normalizeQinglongIDs(args[0])
		body["name"] = args[1]
	}
	return client.requestJSON(http.MethodPut, "/open/envs/name", body, nil, true)
}

func (client *qinglongRuntimeClient) SystemNotify(title, content string) interface{} {
	return client.requestJSON(http.MethodPut, "/open/system/notify", map[string]interface{}{
		"title":   title,
		"content": content,
	}, nil, true)
}

func (client *qinglongRuntimeClient) Request(method, path string, args ...interface{}) interface{} {
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

func normalizeQinglongIDs(ids interface{}) interface{} {
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

func (client *qinglongRuntimeClient) requestJSON(method, apiPath string, body interface{}, query map[string]interface{}, dataOnly bool) interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := client.ensureToken(ctx); err != nil {
		panic(Error(client.vm, err))
	}
	apiPath = normalizeQinglongAPIPath(apiPath)
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
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &result); err != nil {
			panic(Error(client.vm, fmt.Errorf("青龙接口返回无法解析：%v", err)))
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		panic(Error(client.vm, fmt.Errorf("青龙接口 HTTP %d：%s", resp.StatusCode, qinglongResultMessage(result))))
	}
	if code, ok := qinglongResultCode(result); ok && code != 200 {
		panic(Error(client.vm, qinglongResultMessage(result)))
	}
	if dataOnly {
		if data, ok := result["data"]; ok {
			return data
		}
	}
	return result
}

func (client *qinglongRuntimeClient) ensureToken(ctx context.Context) error {
	now := time.Now().Unix()
	if client.token != "" && client.expiration > now+60 {
		return nil
	}
	values := url.Values{}
	values.Set("client_id", client.panel.ClientID)
	values.Set("client_secret", client.panel.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, qinglongURL(client.panel, "/open/auth/token", values), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("青龙接口连接失败：%v", err)
	}
	defer resp.Body.Close()
	tokenResp := qinglongTokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("青龙接口返回无法解析：%v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("青龙接口 HTTP %d：%s", resp.StatusCode, tokenResp.Message)
	}
	if tokenResp.Code != 200 || tokenResp.Data.Token == "" {
		if tokenResp.Message == "" {
			tokenResp.Message = "认证失败，请检查 client_id/client_secret"
		}
		return errors.New(tokenResp.Message)
	}
	client.token = tokenResp.Data.Token
	client.expiration = tokenResp.Data.Expiration
	return nil
}

func normalizeQinglongAPIPath(apiPath string) string {
	apiPath = strings.TrimSpace(apiPath)
	if apiPath == "" {
		return "/open/system"
	}
	if !strings.HasPrefix(apiPath, "/") {
		apiPath = "/" + apiPath
	}
	if !strings.HasPrefix(apiPath, "/open/") {
		apiPath = "/open" + apiPath
	}
	return apiPath
}

func qinglongResultCode(result map[string]interface{}) (int, bool) {
	raw, ok := result["code"]
	if !ok {
		return 0, false
	}
	switch v := raw.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		code, err := strconv.Atoi(v)
		return code, err == nil
	default:
		return 0, false
	}
}

func qinglongResultMessage(result map[string]interface{}) string {
	if message := strings.TrimSpace(fmt.Sprint(result["message"])); message != "" && message != "<nil>" {
		return message
	}
	return "请求失败"
}
