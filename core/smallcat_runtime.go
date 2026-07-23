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

type smallcatRuntimeClient struct {
	vm         *goja.Runtime
	panel      SmallcatPanel
	panelIndex int
}

func installSmallcatRuntime(vm *goja.Runtime) {
	constructor := func(call goja.ConstructorCall) *goja.Object {
		panel, panelIndex, err := smallcatPanelFromArgs(call.Arguments)
		if err != nil {
			panic(Error(vm, err))
		}
		client := &smallcatRuntimeClient{
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
		o.Set("createQr", client.CreateQr)
		o.Set("checkQr", client.CheckQr)
		o.Set("addUser", client.AddUser)
		o.Set("userList", client.UserList)
		return o
	}
	vm.Set("smallcat", constructor)
	vm.Set("Smallcat", constructor)
}

func smallcatPanelFromArgs(args []goja.Value) (SmallcatPanel, int, error) {
	if len(args) == 0 || goja.IsUndefined(args[0]) || goja.IsNull(args[0]) {
		return SmallcatPanel{}, 0, errors.New("缺少 smallcat 编号，例如 new smallcat({ id: 1 })")
	}
	ref := args[0].Export()
	if obj, ok := ref.(map[string]interface{}); ok {
		if v, ok := obj["id"]; ok {
			ref = v
		} else if v, ok := obj["ID"]; ok {
			ref = v
		} else {
			return SmallcatPanel{}, 0, errors.New("缺少 smallcat 编号，例如 new smallcat({ id: 1 })")
		}
	} else {
		return SmallcatPanel{}, 0, errors.New("请使用 new smallcat({ id: 1 }) 创建 smallcat 实例")
	}
	return smallcatPanelByReference(ref)
}

func smallcatPanelByReference(ref interface{}) (SmallcatPanel, int, error) {
	panels := getSmallcatPanels()
	index, ok := smallcatPanelIndex(ref)
	if ok {
		if index < 1 || index > len(panels) {
			return SmallcatPanel{}, 0, fmt.Errorf("smallcat 编号 %d 不存在", index)
		}
		return panels[index-1], index, nil
	}
	id := strings.TrimSpace(fmt.Sprint(ref))
	for i, panel := range panels {
		if panel.ID == id {
			return panel, i + 1, nil
		}
	}
	return SmallcatPanel{}, 0, fmt.Errorf("smallcat %s 不存在", id)
}

func smallcatPanelIndex(ref interface{}) (int, bool) {
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

func (client *smallcatRuntimeClient) CreateQr(clientType interface{}) interface{} {
	body := map[string]interface{}{}
	if opts, ok := clientType.(map[string]interface{}); ok {
		for key, value := range opts {
			body[key] = value
		}
	} else if clientType != nil {
		body["type"] = clientType
	}
	return client.requestJSON(http.MethodPost, "/api/qr/start", body, nil)
}

func (client *smallcatRuntimeClient) CheckQr(uuid string) interface{} {
	return client.requestJSON(http.MethodGet, "/api/qr/status", nil, map[string]interface{}{
		"uuid": uuid,
	})
}

func (client *smallcatRuntimeClient) AddUser(options map[string]interface{}) interface{} {
	body := map[string]interface{}{}
	for key, value := range options {
		body[key] = value
	}
	return client.requestJSON(http.MethodPost, "/api/accounts/add", body, nil)
}

func (client *smallcatRuntimeClient) UserList() interface{} {
	return client.requestJSON(http.MethodGet, "/api/accounts", nil, nil)
}

func (client *smallcatRuntimeClient) Request(method, path string, args ...interface{}) interface{} {
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
	return client.requestJSON(method, path, body, query)
}

func (client *smallcatRuntimeClient) requestJSON(method, apiPath string, body interface{}, query map[string]interface{}) interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	apiPath = normalizeSmallcatAPIPath(apiPath)
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
			return smallcatFailure("请求体编码失败：" + err.Error())
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), u, payload)
	if err != nil {
		return smallcatFailure(err.Error())
	}
	req.Header.Set("auth", client.panel.APIAuth)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return smallcatFailure("smallcat 接口连接失败：" + err.Error())
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return smallcatFailure("smallcat 接口响应读取失败：" + err.Error())
	}
	result := map[string]interface{}{}
	if len(bytes.TrimSpace(raw)) != 0 {
		if err := json.Unmarshal(raw, &result); err != nil {
			return smallcatFailure("smallcat 接口返回无法解析：" + err.Error())
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(result) != 0 {
			return result
		}
		return smallcatFailure(fmt.Sprintf("smallcat 接口 HTTP %d", resp.StatusCode))
	}
	return result
}

func normalizeSmallcatAPIPath(apiPath string) string {
	apiPath = strings.TrimSpace(apiPath)
	if apiPath == "" {
		return "/api/auth/validate"
	}
	if !strings.HasPrefix(apiPath, "/") {
		apiPath = "/" + apiPath
	}
	return apiPath
}

func smallcatFailure(message string) map[string]interface{} {
	if strings.TrimSpace(message) == "" {
		message = "请求失败"
	}
	return map[string]interface{}{
		"status":  false,
		"message": message,
	}
}
