package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/utils"
)

const smallcatPanelsStorageKey = "smallcat_panels"

var legacySmallcatPanels = MakeBucket("smallcat_panels")

type SmallcatPanel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Address       string `json:"address"`
	APIAuth       string `json:"api_auth"`
	CreatedAt     int    `json:"created_at"`
	UpdatedAt     int    `json:"updated_at"`
	LastCheckedAt int    `json:"last_checked_at"`
	Status        string `json:"status"`
	Message       string `json:"message"`
	Group         string `json:"group"`
	Namespace     string `json:"namespace"`
	AccountLimit  string `json:"account_limit"`
	AccountUsed   string `json:"account_used"`
	CreditBalance string `json:"credit_balance"`
}

type smallcatAuthValidateResponse struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type PublicSmallcatPanel struct {
	Index   int    `json:"index"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func init() {
	GinApi(GET, "/api/admin/smallcat/panels", RequireAuth, func(ctx *gin.Context) {
		panels := getSmallcatPanels()
		refreshSmallcatPanelsStatus(panels)
		ApiList(ctx, redactSmallcatPanels(panels), len(panels))
	})

	GinApi(POST, "/api/admin/smallcat/panel/test", RequireAuth, func(ctx *gin.Context) {
		panel := SmallcatPanel{}
		if err := ctx.BindJSON(&panel); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		hydrateSmallcatPanelAPIAuth(&panel)
		if err := validateSmallcatPanelInput(&panel); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		result, err := testSmallcatPanel(panel)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		result.APIAuth = ""
		ApiOK(ctx, result)
	})

	GinApi(POST, "/api/admin/smallcat/panel/accounts", RequireAuth, func(ctx *gin.Context) {
		payload := struct {
			ID string `json:"id"`
		}{}
		if err := ctx.BindJSON(&payload); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		payload.ID = strings.TrimSpace(payload.ID)
		panel := storedSmallcatPanelByID(payload.ID)
		if panel == nil {
			ApiFail(ctx, "smallcat 不存在")
			return
		}
		openids, err := fetchSmallcatAccountOpenIDs(panel)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, gin.H{
			"openids": openids,
			"total":   len(openids),
		})
	})

	GinApi(POST, "/api/admin/smallcat/panel", RequireAuth, func(ctx *gin.Context) {
		panel := SmallcatPanel{}
		if err := ctx.BindJSON(&panel); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		hydrateSmallcatPanelAPIAuth(&panel)
		if err := validateSmallcatPanelInput(&panel); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		result, err := testSmallcatPanel(panel)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		now := int(time.Now().Unix())
		panels := getSmallcatPanels()
		index := -1
		if panel.ID != "" {
			for i := range panels {
				if panels[i].ID == panel.ID {
					index = i
					break
				}
			}
		}
		if panel.ID == "" {
			panel.ID = utils.GenUUID()
			panel.CreatedAt = now
		} else if index >= 0 {
			if panels[index].CreatedAt != 0 {
				panel.CreatedAt = panels[index].CreatedAt
			} else {
				panel.CreatedAt = now
			}
		} else {
			panel.CreatedAt = now
		}
		if panel.Name == "" {
			panel.Name = panel.Address
		}
		panel.UpdatedAt = now
		panel.LastCheckedAt = now
		panel.Status = "online"
		panel.Message = result.Message
		panel.Group = result.Group
		panel.Namespace = result.Namespace
		panel.AccountLimit = result.AccountLimit
		panel.AccountUsed = result.AccountUsed
		panel.CreditBalance = result.CreditBalance
		if index >= 0 {
			panels[index] = panel
		} else {
			panels = append(panels, panel)
		}
		saveSmallcatPanels(panels)
		panel.APIAuth = ""
		ApiOK(ctx, panel)
	})

	GinApi(DELETE, "/api/admin/smallcat/panel", RequireAuth, func(ctx *gin.Context) {
		panel := SmallcatPanel{}
		if err := ctx.BindJSON(&panel); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		if panel.ID == "" {
			ApiFail(ctx, "缺少 smallcat ID")
			return
		}
		panels := getSmallcatPanels()
		next := make([]SmallcatPanel, 0, len(panels))
		for _, item := range panels {
			if item.ID != panel.ID {
				next = append(next, item)
			}
		}
		saveSmallcatPanels(next)
		ApiOK(ctx, nil)
	})
}

func getSmallcatPanels() []SmallcatPanel {
	raw := strings.TrimSpace(sillyGirl.GetString(smallcatPanelsStorageKey))
	if raw != "" {
		panels := []SmallcatPanel{}
		if json.Unmarshal([]byte(strings.TrimPrefix(raw, "o:")), &panels) == nil {
			return panels
		}
	}
	panels := getLegacySmallcatPanels()
	if len(panels) > 0 {
		saveSmallcatPanels(panels)
	}
	return panels
}

func getLegacySmallcatPanels() []SmallcatPanel {
	panels := []SmallcatPanel{}
	legacySmallcatPanels.Foreach(func(_, data []byte) error {
		panel := SmallcatPanel{}
		if json.Unmarshal(data, &panel) == nil && panel.ID != "" {
			panels = append(panels, panel)
		}
		return nil
	})
	return panels
}

func saveSmallcatPanels(panels []SmallcatPanel) {
	sillyGirl.Set(smallcatPanelsStorageKey, utils.JsonMarshal(panels))
}

func hydrateSmallcatPanelAPIAuth(panel *SmallcatPanel) {
	if panel == nil || strings.TrimSpace(panel.APIAuth) != "" || strings.TrimSpace(panel.ID) == "" {
		return
	}
	if stored := storedSmallcatPanelByID(panel.ID); stored != nil {
		panel.APIAuth = stored.APIAuth
	}
}

func redactSmallcatPanels(panels []SmallcatPanel) []SmallcatPanel {
	result := make([]SmallcatPanel, len(panels))
	copy(result, panels)
	for index := range result {
		result[index].APIAuth = ""
	}
	return result
}

func storedSmallcatPanelByID(id string) *SmallcatPanel {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	for _, panel := range getSmallcatPanels() {
		if panel.ID == id {
			matched := panel
			return &matched
		}
	}
	return nil
}

func validateSmallcatPanelInput(panel *SmallcatPanel) error {
	panel.Name = strings.TrimSpace(panel.Name)
	panel.Address = normalizeSmallcatAddress(panel.Address)
	panel.APIAuth = strings.TrimSpace(panel.APIAuth)
	if panel.Address == "" {
		return errors.New("smallcat 地址不能为空")
	}
	if panel.APIAuth == "" {
		return errors.New("api_auth 不能为空")
	}
	parsed, err := url.ParseRequestURI(panel.Address)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("smallcat 地址格式错误：%v", err)
	}
	return nil
}

func normalizeSmallcatAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}
	return strings.TrimRight(address, "/")
}

func testSmallcatPanel(panel SmallcatPanel) (*SmallcatPanel, error) {
	raw, err := requestSmallcatJSONWithTimeout(&panel, http.MethodGet, "/api/auth/validate", nil, nil, 8*time.Second)
	if err != nil {
		return nil, err
	}
	if err := smallcatEnvelopeError(raw, "验证失败，请检查 API AUTH"); err != nil {
		return nil, err
	}
	panel.Address = normalizeSmallcatAddress(panel.Address)
	panel.Status = "online"
	panel.Message = "验证通过"
	panel.LastCheckedAt = int(time.Now().Unix())
	applySmallcatAuthStatus(&panel, raw)
	if err := refreshSmallcatAccountUsage(&panel); err != nil {
		panel.Message = "账号数量读取失败：" + err.Error()
	}
	_ = refreshSmallcatCreditBalance(&panel)
	return &panel, nil
}

func refreshSmallcatPanelsStatus(panels []SmallcatPanel) {
	var wg sync.WaitGroup
	for index := range panels {
		wg.Add(1)
		go func(panel *SmallcatPanel) {
			defer wg.Done()
			refreshSmallcatPanelStatus(panel)
		}(&panels[index])
	}
	wg.Wait()
}

func refreshSmallcatPanelStatus(panel *SmallcatPanel) {
	if panel == nil || panel.Address == "" || panel.APIAuth == "" {
		return
	}
	panel.Address = normalizeSmallcatAddress(panel.Address)
	panel.LastCheckedAt = int(time.Now().Unix())
	raw, err := requestSmallcatJSONWithTimeout(panel, http.MethodGet, "/api/auth/validate", nil, nil, 4*time.Second)
	if err != nil {
		panel.Status = "offline"
		panel.Message = err.Error()
		return
	}
	if err := smallcatEnvelopeError(raw, "验证失败，请检查 API AUTH"); err != nil {
		panel.Status = "offline"
		panel.Message = err.Error()
		return
	}
	panel.Status = "online"
	panel.Message = "验证通过"
	applySmallcatAuthStatus(panel, raw)
	if err := refreshSmallcatAccountUsage(panel); err != nil {
		panel.Message = "账号数量读取失败：" + err.Error()
	}
	if err := refreshSmallcatCreditBalance(panel); err != nil && panel.Message == "验证通过" {
		panel.Message = "积分读取失败：" + err.Error()
	}
}

// refreshSmallcatAccountUsage follows the official client implementation:
// /api/auth/validate only validates AUTH and currently reports quota.count as
// zero, while /api/accounts returns the real items and quota. The item count is
// authoritative because it is also what the SmallCat console displays.
func refreshSmallcatAccountUsage(panel *SmallcatPanel) error {
	value, err := fetchSmallcatAccounts(panel)
	if err != nil {
		return err
	}
	used, limit, ok := smallcatAccountUsage(value)
	if !ok {
		return errors.New("账号接口返回缺少 items/count")
	}
	panel.AccountUsed = used
	if limit != "" {
		panel.AccountLimit = limit
	}
	return nil
}

func fetchSmallcatAccounts(panel *SmallcatPanel) (any, error) {
	raw, err := requestSmallcatJSONWithTimeout(panel, http.MethodGet, "/api/accounts", nil, nil, 4*time.Second)
	if err != nil {
		return nil, err
	}
	if err := smallcatEnvelopeError(raw, "账号列表读取失败"); err != nil {
		return nil, err
	}
	return decodeSmallcatJSONValue(raw), nil
}

func fetchSmallcatAccountOpenIDs(panel *SmallcatPanel) ([]string, error) {
	value, err := fetchSmallcatAccounts(panel)
	if err != nil {
		return nil, err
	}
	return smallcatAccountOpenIDs(value), nil
}

func smallcatAccountOpenIDs(value any) []string {
	value = unwrapSmallcatResponseData(value)
	if object, ok := value.(map[string]any); ok {
		if items, found := smallcatMapLookup(object, "items", "accounts", "list"); found {
			value = items
		}
	}
	rows, ok := value.([]any)
	if !ok {
		return []string{}
	}
	seen := make(map[string]struct{}, len(rows))
	openids := make([]string, 0, len(rows))
	for _, row := range rows {
		object, ok := row.(map[string]any)
		if !ok {
			continue
		}
		openid := strings.TrimSpace(smallcatStringValue(smallcatMapValue(object, "openid", "open_id", "wxOpenid", "userKey")))
		if openid == "" {
			continue
		}
		if _, exists := seen[openid]; exists {
			continue
		}
		seen[openid] = struct{}{}
		openids = append(openids, openid)
	}
	return openids
}

func smallcatAccountUsage(value any) (used string, limit string, ok bool) {
	value = unwrapSmallcatResponseData(value)
	switch typed := value.(type) {
	case []any:
		return strconv.Itoa(len(typed)), "", true
	case map[string]any:
		limit = smallcatStringValue(smallcatMapValue(typed, "limit", "account_limit", "max_accounts"))
		if items, found := smallcatMapLookup(typed, "items", "accounts", "list"); found {
			switch rows := items.(type) {
			case []any:
				return strconv.Itoa(len(rows)), limit, true
			case []map[string]any:
				return strconv.Itoa(len(rows)), limit, true
			}
		}
		if count, found := smallcatMapLookup(typed, "count", "used", "account_used", "current_accounts", "accounts_count"); found {
			return smallcatStringValue(count), limit, true
		}
		if quota, found := smallcatMapLookup(typed, "quota"); found {
			quotaUsed, quotaLimit, quotaOK := smallcatAccountUsage(quota)
			if limit == "" {
				limit = quotaLimit
			}
			if quotaOK {
				return quotaUsed, limit, true
			}
		}
	}
	return "", limit, false
}

func unwrapSmallcatResponseData(value any) any {
	for depth := 0; depth < 3; depth++ {
		object, ok := value.(map[string]any)
		if !ok {
			break
		}
		data, found := smallcatMapLookup(object, "data")
		if !found || data == nil {
			break
		}
		if text, ok := data.(string); ok {
			decoded := decodeSmallcatJSONValue(json.RawMessage(strings.TrimSpace(text)))
			if decoded == nil {
				break
			}
			data = decoded
		}
		value = data
	}
	return value
}

func smallcatMapValue(value map[string]any, keys ...string) any {
	item, _ := smallcatMapLookup(value, keys...)
	return item
}

func smallcatMapLookup(value map[string]any, keys ...string) (any, bool) {
	for _, wanted := range keys {
		for key, item := range value {
			if strings.EqualFold(strings.TrimSpace(key), wanted) {
				return item, true
			}
		}
	}
	return nil, false
}

func refreshSmallcatCreditBalance(panel *SmallcatPanel) error {
	raw, err := requestSmallcatJSONWithTimeout(panel, http.MethodGet, "/credits/balance", nil, nil, 4*time.Second)
	if err != nil {
		return err
	}
	if err := smallcatEnvelopeError(raw, "积分读取失败"); err != nil {
		return err
	}
	value := decodeSmallcatJSONValue(raw)
	panel.CreditBalance = smallcatStringValue(firstSmallcatValue(value, "balance", "credits", "credit", "points"))
	return nil
}

func smallcatEnvelopeError(raw json.RawMessage, fallback string) error {
	envelope := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil
	}
	statusRaw, ok := envelope["status"]
	if !ok {
		return nil
	}
	status := false
	if err := json.Unmarshal(statusRaw, &status); err != nil || status {
		return nil
	}
	authResp := smallcatAuthValidateResponse{}
	_ = json.Unmarshal(raw, &authResp)
	if authResp.Message == "" {
		authResp.Message = fallback
	}
	return errors.New(authResp.Message)
}

func applySmallcatAuthStatus(panel *SmallcatPanel, raw json.RawMessage) {
	value := decodeSmallcatJSONValue(raw)
	panel.Group = smallcatGroupLabel(firstNonEmpty(
		smallcatStringValue(firstSmallcatValue(value, "group")),
		smallcatStringValue(firstSmallcatValue(value, "user_group")),
	))
	panel.Namespace = smallcatStringValue(firstSmallcatValue(value, "namespace"))
	panel.AccountLimit = smallcatStringValue(firstSmallcatValue(value, "limit", "account_limit", "max_accounts"))
	// Do not consume quota.count from /api/auth/validate here. The current
	// SmallCat server intentionally builds that response with count=0; the real
	// account count is refreshed from /api/accounts below.
	panel.AccountUsed = smallcatStringValue(firstSmallcatValue(value, "used", "account_used", "current_accounts", "accounts_count"))
	if quota := firstSmallcatValue(value, "quota"); quota != nil {
		panel.AccountLimit = firstNonEmpty(panel.AccountLimit, smallcatStringValue(firstSmallcatValue(quota, "limit", "max", "total")))
		panel.AccountUsed = firstNonEmpty(panel.AccountUsed, smallcatStringValue(firstSmallcatValue(quota, "used", "current")))
	}
}

func decodeSmallcatJSONValue(raw json.RawMessage) any {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil
	}
	return value
}

func firstSmallcatValue(value any, keys ...string) any {
	wanted := map[string]struct{}{}
	for _, key := range keys {
		wanted[strings.ToLower(key)] = struct{}{}
	}
	return walkSmallcatValue(value, wanted)
}

func walkSmallcatValue(value any, keys map[string]struct{}) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if _, ok := keys[strings.ToLower(key)]; ok && item != nil && item != "" {
				return item
			}
		}
		for _, item := range typed {
			if found := walkSmallcatValue(item, keys); found != nil && found != "" {
				return found
			}
		}
	case []any:
		for _, item := range typed {
			if found := walkSmallcatValue(item, keys); found != nil && found != "" {
				return found
			}
		}
	}
	return nil
}

func smallcatStringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case bool:
		return strconv.FormatBool(typed)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(data)
	}
}

func smallcatGroupLabel(group string) string {
	switch strings.ToLower(strings.TrimSpace(group)) {
	case "":
		return ""
	case "normal":
		return "普通用户组"
	case "pro":
		return "PRO"
	case "vip":
		return "VIP"
	default:
		return group
	}
}

func publicSmallcatPanels() []PublicSmallcatPanel {
	panels := getSmallcatPanels()
	result := make([]PublicSmallcatPanel, 0, len(panels))
	for index, panel := range panels {
		result = append(result, PublicSmallcatPanel{
			Index:   index + 1,
			ID:      panel.ID,
			Name:    firstNonEmpty(panel.Name, fmt.Sprintf("smallcat #%d", index+1)),
			Status:  panel.Status,
			Message: panel.Message,
		})
	}
	return result
}

func smallcatPanelByIndex(index int) (*SmallcatPanel, error) {
	panels := getSmallcatPanels()
	if len(panels) == 0 {
		return nil, errors.New("后台未绑定 smallcat")
	}
	if index <= 0 {
		index = 1
	}
	if index > len(panels) {
		return nil, fmt.Errorf("smallcat 编号 %d 不存在", index)
	}
	panel := panels[index-1]
	if panel.Address == "" || panel.APIAuth == "" {
		return nil, errors.New("smallcat 配置不完整")
	}
	return &panel, nil
}

func requestSmallcatJSON(panel *SmallcatPanel, method string, path string, body interface{}, query map[string]string) (json.RawMessage, error) {
	return requestSmallcatJSONWithTimeout(panel, method, path, body, query, 15*time.Second)
}

func requestSmallcatJSONWithTimeout(panel *SmallcatPanel, method string, path string, body interface{}, query map[string]string, timeout time.Duration) (json.RawMessage, error) {
	if panel == nil {
		return nil, errors.New("smallcat 配置不存在")
	}
	address := normalizeSmallcatAddress(panel.Address)
	values := url.Values{}
	for key, value := range query {
		values.Set(key, value)
	}
	requestURL := address + path
	if encoded := values.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("auth", panel.APIAuth)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("smallcat 请求失败：%v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(raw))
		if len(message) > 200 {
			message = message[:200]
		}
		return nil, fmt.Errorf("smallcat HTTP %d：%s", resp.StatusCode, message)
	}
	if !json.Valid(raw) {
		message := strings.TrimSpace(string(raw))
		if len(message) > 200 {
			message = message[:200]
		}
		return nil, fmt.Errorf("smallcat 返回非 JSON：%s", message)
	}
	return json.RawMessage(raw), nil
}
