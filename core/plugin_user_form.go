package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/utils"
)

var pluginUserFormSchemas = MakeBucket("plugin_user_form_schemas")
var pluginUserFormRecords = MakeBucket("plugin_user_form_records")
var pluginUserFormRecordsMu sync.Mutex

const maxPluginUserFormBodyBytes = 1 << 20

type pluginUserFormDefinition struct {
	Schema     map[string]interface{}               `json:"schema"`
	Multiple   int                                  `json:"multiple"`
	KeyBy      []string                             `json:"key_by"`
	Validators map[string][]pluginUserFormValidator `json:"validators,omitempty"`
}

type pluginUserFormValidator struct {
	Runtime string `json:"runtime"`
	Source  string `json:"source"`
	Message string `json:"message,omitempty"`
}

type pluginUserFormRecord struct {
	ID        string                 `json:"id"`
	Values    map[string]interface{} `json:"values"`
	CreatedAt int64                  `json:"created_at"`
	UpdatedAt int64                  `json:"updated_at"`
}

type pluginUserFormView struct {
	UUID     string                 `json:"uuid"`
	Title    string                 `json:"title"`
	Schema   map[string]interface{} `json:"schema"`
	Multiple int                    `json:"multiple"`
	KeyBy    []string               `json:"key_by"`
	Records  []pluginUserFormRecord `json:"records"`
}

func init() {
	GinApi(GET, "/api/user/plugins/:uuid/form", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		plugin, definition, err := accessiblePluginUserForm(ctx.Param("uuid"))
		if err != nil {
			ApiNotFound(ctx, err.Error())
			return
		}
		ApiOK(ctx, pluginUserFormView{
			UUID: plugin.UUID, Title: firstNonEmpty(plugin.Title, plugin.UUID),
			Schema: definition.Schema, Multiple: definition.Multiple,
			KeyBy:   append([]string(nil), definition.KeyBy...),
			Records: pluginUserRecords(user.ID, plugin.UUID),
		})
	})

	savePluginUserFormRecord := func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		payload := struct {
			UUID     string                 `json:"uuid"`
			RecordID string                 `json:"record_id"`
			Value    map[string]interface{} `json:"value"`
		}{}
		if err := decodePluginUserFormJSON(ctx, &payload); err != nil {
			ApiFail(ctx, "请求体无效或超过 1 MiB")
			return
		}
		payload.UUID = ctx.Param("uuid")
		recordID := strings.TrimSpace(ctx.Param("record_id"))
		creating := recordID == ""
		if recordID != "" {
			payload.RecordID = recordID
		} else {
			payload.RecordID = ""
		}
		plugin, definition, err := accessiblePluginUserForm(payload.UUID)
		if err != nil {
			ApiNotFound(ctx, err.Error())
			return
		}
		values, fieldErrors := validatePluginUserForm(definition.Schema, payload.Value)
		if len(fieldErrors) != 0 {
			ApiValidationError(ctx, "表单验证失败", fieldErrors)
			return
		}
		if fieldErrors = runPluginUserFormValidators(ctx.Request.Context(), plugin, definition, user, values); len(fieldErrors) != 0 {
			ApiValidationError(ctx, "表单验证失败", fieldErrors)
			return
		}
		pluginUserFormRecordsMu.Lock()
		defer pluginUserFormRecordsMu.Unlock()
		records := pluginUserRecords(user.ID, plugin.UUID)
		now := time.Now().Unix()
		index := -1
		if payload.RecordID != "" {
			for i := range records {
				if records[i].ID == payload.RecordID {
					index = i
					break
				}
			}
			if index < 0 {
				ApiNotFound(ctx, "提交记录不存在")
				return
			}
		} else if len(definition.KeyBy) != 0 {
			key := pluginUserRecordKey(values, definition.KeyBy)
			for i := range records {
				if pluginUserRecordKey(records[i].Values, definition.KeyBy) == key {
					index = i
					break
				}
			}
		} else if definition.Multiple <= 1 && len(records) != 0 {
			index = 0
		}
		if index >= 0 {
			if creating {
				ApiConflict(ctx, "相同键值的提交记录已存在")
				return
			}
			records[index].Values = values
			records[index].UpdatedAt = now
		} else {
			limit := definition.Multiple
			if limit <= 0 {
				limit = 1
			}
			if len(records) >= limit {
				ApiConflict(ctx, fmt.Sprintf("最多提交 %d 条记录", limit))
				return
			}
			records = append(records, pluginUserFormRecord{ID: utils.GenUUID(), Values: values, CreatedAt: now, UpdatedAt: now})
			index = len(records) - 1
		}
		if err := savePluginUserRecords(user.ID, plugin.UUID, records); err != nil {
			ApiInternalError(ctx, "保存失败："+err.Error())
			return
		}
		if creating {
			location := "/api/user/plugins/" + plugin.UUID + "/form-records/" + records[index].ID
			ApiCreated(ctx, location, records[index])
			return
		}
		ApiOK(ctx, records[index])
	}
	GinApi(POST, "/api/user/plugins/:uuid/form-records", RequireUserAuth, savePluginUserFormRecord)
	GinApi(POST, "/api/user/plugins/:uuid/form-records/:record_id", RequireUserAuth, savePluginUserFormRecord)

	GinApi(POST, "/api/user/plugins/:uuid/form-records/:record_id/deletions", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		payload := struct {
			UUID     string
			RecordID string
		}{UUID: ctx.Param("uuid"), RecordID: ctx.Param("record_id")}
		plugin, _, err := accessiblePluginUserForm(payload.UUID)
		if err != nil {
			ApiNotFound(ctx, err.Error())
			return
		}
		pluginUserFormRecordsMu.Lock()
		defer pluginUserFormRecordsMu.Unlock()
		records := pluginUserRecords(user.ID, plugin.UUID)
		filtered := records[:0]
		for _, record := range records {
			if record.ID != strings.TrimSpace(payload.RecordID) {
				filtered = append(filtered, record)
			}
		}
		if len(filtered) == len(records) {
			ApiNotFound(ctx, "提交记录不存在")
			return
		}
		if err := savePluginUserRecords(user.ID, plugin.UUID, filtered); err != nil {
			ApiInternalError(ctx, "删除失败："+err.Error())
			return
		}
		ApiNoContent(ctx)
	})
}

func decodePluginUserFormJSON(ctx *gin.Context, target interface{}) error {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxPluginUserFormBodyBytes)
	decoder := json.NewDecoder(ctx.Request.Body)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("请求体只能包含一个 JSON 对象")
		}
		return err
	}
	return nil
}

func accessiblePluginUserForm(uuid string) (*common.Function, pluginUserFormDefinition, error) {
	uuid = strings.TrimSpace(uuid)
	plugin := installedPluginByUUID(uuid)
	if plugin == nil || !pluginExecutionEnabled(plugin) || !plugin.Open || !plugin.HasUserForm {
		return nil, pluginUserFormDefinition{}, errors.New("插件用户表单未开放")
	}
	definition, ok := getPluginUserFormDefinition(uuid)
	if !ok {
		return nil, pluginUserFormDefinition{}, errors.New("插件用户表单尚未注册")
	}
	return plugin, definition, nil
}

func getPluginUserFormDefinition(uuid string) (pluginUserFormDefinition, bool) {
	raw := strings.TrimPrefix(pluginUserFormSchemas.GetString(strings.TrimSpace(uuid)), "o:")
	if raw == "" {
		return pluginUserFormDefinition{}, false
	}
	definition := pluginUserFormDefinition{}
	if json.Unmarshal([]byte(raw), &definition) != nil || len(definition.Schema) == 0 {
		return pluginUserFormDefinition{}, false
	}
	if definition.Multiple < 1 {
		definition.Multiple = 1
	}
	return definition, true
}

func pluginUserRecords(userID, pluginID string) []pluginUserFormRecord {
	raw := strings.TrimPrefix(pluginUserFormRecords.GetString(pluginUserFormStorageKey(userID, pluginID)), "o:")
	result := []pluginUserFormRecord{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &result)
	}
	return result
}

func savePluginUserRecords(userID, pluginID string, records []pluginUserFormRecord) error {
	key := pluginUserFormStorageKey(userID, pluginID)
	if len(records) == 0 {
		_, _, err := pluginUserFormRecords.Set(key, nil)
		return err
	}
	_, _, err := SetBucketKeyValue(pluginUserFormRecords, key, records)
	return err
}

func deletePluginUserRecordsForUser(userID string) error {
	prefix := strings.TrimSpace(userID) + ":"
	return deletePluginUserRecordsMatching(func(key string) bool { return strings.HasPrefix(key, prefix) })
}

func deletePluginUserRecordsForPlugin(pluginID string) error {
	suffix := ":" + strings.TrimSpace(pluginID)
	return deletePluginUserRecordsMatching(func(key string) bool { return strings.HasSuffix(key, suffix) })
}

func deletePluginUserRecordsMatching(matches func(string) bool) error {
	pluginUserFormRecordsMu.Lock()
	defer pluginUserFormRecordsMu.Unlock()
	keys := []string{}
	pluginUserFormRecords.Foreach(func(key, _ []byte) error {
		if matches(string(key)) {
			keys = append(keys, string(key))
		}
		return nil
	})
	for _, key := range keys {
		if _, _, err := pluginUserFormRecords.Set(key, nil); err != nil {
			return err
		}
	}
	return nil
}

func pluginUserFormStorageKey(userID, pluginID string) string {
	return strings.TrimSpace(userID) + ":" + strings.TrimSpace(pluginID)
}

func pluginUserRecordKey(values map[string]interface{}, fields []string) string {
	parts := make([]interface{}, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, values[field])
	}
	encoded, _ := json.Marshal(parts)
	return string(encoded)
}

func validatePluginUserForm(schema map[string]interface{}, input map[string]interface{}) (map[string]interface{}, []gin.H) {
	properties, _ := schema["properties"].(map[string]interface{})
	result := map[string]interface{}{}
	errorsList := []gin.H{}
	for key := range input {
		if _, ok := properties[key]; !ok {
			errorsList = append(errorsList, gin.H{"field": key, "code": "unknown", "message": "字段未在表单中声明"})
		}
	}
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		field, _ := properties[key].(map[string]interface{})
		value, exists := input[key]
		required, _ := field["required"].(bool)
		empty := !exists || value == nil || (fmt.Sprintf("%v", value) == "")
		if empty {
			if required {
				errorsList = append(errorsList, gin.H{"field": key, "code": "required", "message": schemaRuleError(field, "required", "该字段不能为空")})
			} else if defaultValue, ok := field["default"]; ok {
				result[key] = defaultValue
			}
			continue
		}
		fieldType, _ := field["type"].(string)
		if options, ok := field["enum"].([]interface{}); ok && !userFormEnumContains(options, value) {
			errorsList = append(errorsList, gin.H{"field": key, "code": "enum", "message": "请选择有效选项"})
			continue
		}
		switch fieldType {
		case "string":
			text, ok := value.(string)
			if !ok {
				errorsList = append(errorsList, gin.H{"field": key, "code": "type", "message": "必须是字符串"})
				continue
			}
			if pattern, _ := field["pattern"].(string); pattern != "" {
				matched, err := regexp.MatchString(pattern, text)
				if err != nil || !matched {
					errorsList = append(errorsList, gin.H{"field": key, "code": "match", "message": schemaRuleError(field, "match", "格式不正确")})
					continue
				}
			}
			result[key] = text
		case "boolean":
			if _, ok := value.(bool); !ok {
				errorsList = append(errorsList, gin.H{"field": key, "code": "type", "message": "必须是布尔值"})
				continue
			}
			result[key] = value
		case "number", "integer":
			number, ok := value.(float64)
			if !ok || (fieldType == "integer" && number != float64(int64(number))) {
				errorsList = append(errorsList, gin.H{"field": key, "code": "type", "message": "必须是有效数字"})
				continue
			}
			if minimum, ok := field["minimum"].(float64); ok && number < minimum {
				errorsList = append(errorsList, gin.H{"field": key, "code": "min", "message": fmt.Sprintf("不能小于 %v", minimum)})
				continue
			}
			if maximum, ok := field["maximum"].(float64); ok && number > maximum {
				errorsList = append(errorsList, gin.H{"field": key, "code": "max", "message": fmt.Sprintf("不能大于 %v", maximum)})
				continue
			}
			result[key] = number
		default:
			result[key] = value
		}
	}
	return result, errorsList
}

func userFormEnumContains(options []interface{}, value interface{}) bool {
	for _, option := range options {
		if reflect.DeepEqual(option, value) {
			return true
		}
	}
	return false
}

func schemaRuleError(field map[string]interface{}, rule, fallback string) string {
	errorsMap, _ := field["errorMessages"].(map[string]interface{})
	if message := strings.TrimSpace(fmt.Sprintf("%v", errorsMap[rule])); message != "" && message != "<nil>" {
		return message
	}
	return fallback
}

func validateUserFormDefinition(definition pluginUserFormDefinition) error {
	properties, _ := definition.Schema["properties"].(map[string]interface{})
	if len(properties) == 0 {
		return errors.New("用户表单 schema 为空")
	}
	for key, raw := range properties {
		field, _ := raw.(map[string]interface{})
		if pattern, _ := field["pattern"].(string); pattern != "" {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("字段 %s 的 match 正则无效：%v", key, err)
			}
		}
	}
	for _, key := range definition.KeyBy {
		if _, ok := properties[key]; !ok {
			return fmt.Errorf("keyBy 字段不存在：%s", key)
		}
	}
	for key, validators := range definition.Validators {
		if _, ok := properties[key]; !ok {
			return fmt.Errorf("test 字段不存在：%s", key)
		}
		for _, validator := range validators {
			if strings.TrimSpace(validator.Source) == "" || len(validator.Source) > 256<<10 || (validator.Runtime != NODE && validator.Runtime != PYTHON) {
				return fmt.Errorf("字段 %s 的 test 校验器无效", key)
			}
		}
	}
	if definition.Multiple < 1 {
		return errors.New("multiple 必须大于 0")
	}
	return nil
}
