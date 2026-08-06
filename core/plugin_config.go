package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
)

var pluginConfigSchemas = MakeBucket("plugin_config_schemas")
var pluginConfigValues = MakeBucket("plugin_config_values")

type PluginConfigRecord struct {
	UUID       string                 `json:"uuid"`
	Title      string                 `json:"title"`
	Plugin     string                 `json:"plugin"`
	File       string                 `json:"file"`
	Schema     map[string]interface{} `json:"schema"`
	UserConfig map[string]interface{} `json:"user_config"`
	Registered bool                   `json:"registered"`
}

func init() {
	GinApi(GET, "/api/admin/plugin/configs", RequireAuth, func(ctx *gin.Context) {
		ApiOK(ctx, getPluginConfigRecords())
	})
	GinApi(GET, "/api/admin/plugin/config", RequireAuth, func(ctx *gin.Context) {
		uuid := ctx.Query("uuid")
		record := getPluginConfigRecord(uuid)
		if record == nil {
			ApiFail(ctx, "配置不存在，请先运行一次插件或在脚本顶层声明 new form({...})")
			return
		}
		ApiOK(ctx, record)
	})
	GinApi(PUT, "/api/admin/plugin/config", RequireAuth, func(ctx *gin.Context) {
		var req struct {
			UUID  string                 `json:"uuid"`
			Value map[string]interface{} `json:"value"`
		}
		if err := ctx.BindJSON(&req); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		if req.UUID == "" {
			ApiFail(ctx, "缺少插件 UUID")
			return
		}
		if _, _, err := SetBucketKeyValue(pluginConfigValues, req.UUID, req.Value); err != nil {
			ApiFail(ctx, "保存插件配置失败："+err.Error())
			return
		}
		ApiOK(ctx, nil)
	})
	GinApi(DELETE, "/api/admin/plugin/config", RequireAuth, func(ctx *gin.Context) {
		var req struct {
			UUID         string `json:"uuid"`
			DeleteSchema bool   `json:"delete_schema"`
		}
		if err := ctx.BindJSON(&req); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		req.UUID = strings.TrimSpace(req.UUID)
		if req.UUID == "" {
			ApiFail(ctx, "缺少插件 UUID")
			return
		}
		deletePluginConfig(req.UUID, req.DeleteSchema)
		ApiOK(ctx, gin.H{"uuid": req.UUID, "delete_schema": req.DeleteSchema})
	})
}

func deletePluginConfig(uuid string, deleteSchema bool) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return
	}
	_, _, _ = SetBucketKeyValue2(pluginConfigValues, uuid, nil)
	if deleteSchema {
		_, _, _ = SetBucketKeyValue2(pluginConfigSchemas, uuid, nil)
		_, _, _ = SetBucketKeyValue2(pluginUserFormSchemas, uuid, nil)
		_ = deletePluginUserRecordsForPlugin(uuid)
	}
}

func getPluginConfigRecords() []*PluginConfigRecord {
	records := []*PluginConfigRecord{}
	nodePluginNames := nodePluginNameIndexByUUID()
	seen := map[string]bool{}
	pluginConfigSchemas.Foreach(func(k, _ []byte) error {
		uuid := string(k)
		if record := getPluginConfigRecordWithIndex(uuid, nodePluginNames); record != nil {
			records = append(records, record)
			seen[record.UUID] = true
		}
		return nil
	})
	for _, f := range Functions {
		if !f.HasForm || f.UUID == "" || seen[f.UUID] {
			continue
		}
		records = append(records, &PluginConfigRecord{
			UUID:       f.UUID,
			Title:      getPluginTitle(f.UUID),
			Plugin:     getPluginConfigPluginName(f.UUID, nodePluginNames),
			File:       getPluginConfigFileName(f.UUID, nodePluginNames),
			Schema:     map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
			UserConfig: getPluginUserConfig(f.UUID),
			Registered: false,
		})
		seen[f.UUID] = true
	}
	return records
}

func getPluginConfigRecord(uuid string) *PluginConfigRecord {
	return getPluginConfigRecordWithIndex(uuid, nil)
}

func getPluginConfigRecordWithIndex(uuid string, nodePluginNames map[string]string) *PluginConfigRecord {
	if uuid == "" {
		return nil
	}
	if !isLocalPluginConfigUUID(uuid, nodePluginNames) {
		return nil
	}
	schema := map[string]interface{}{}
	data := pluginConfigSchemas.GetString(uuid)
	if data == "" {
		return nil
	}
	data = strings.TrimPrefix(data, "o:")
	if err := json.Unmarshal([]byte(data), &schema); err != nil {
		return nil
	}
	return &PluginConfigRecord{
		UUID:       uuid,
		Title:      getPluginTitle(uuid),
		Plugin:     getPluginConfigPluginName(uuid, nodePluginNames),
		File:       getPluginConfigFileName(uuid, nodePluginNames),
		Schema:     schema,
		UserConfig: getPluginUserConfig(uuid),
		Registered: true,
	}
}

func getPluginTitle(uuid string) string {
	for _, f := range Functions {
		if f.UUID == uuid {
			if f.Title != "" {
				return f.Title
			}
			if f.Path != "" {
				return nodePluginNameFromPath(f.Path)
			}
		}
	}
	if name := getPluginConfigPluginName(uuid); name != "" {
		return name
	}
	return uuid
}

func getPluginConfigPluginName(uuid string, nodePluginNames ...map[string]string) string {
	for _, f := range Functions {
		if f.UUID == uuid {
			if plugin := nodePluginNameFromPath(f.Path); plugin != "" {
				return plugin
			}
			if f.Title != "" {
				return f.Title
			}
		}
	}
	return findNodePluginNameByUUID(uuid, nodePluginNames...)
}

func getPluginConfigFileName(uuid string, nodePluginNames ...map[string]string) string {
	for _, f := range Functions {
		if f.UUID == uuid && f.Path != "" {
			return filepath.Base(filepath.Clean(f.Path))
		}
		if f.UUID == uuid && f.Title != "" {
			return f.Title + f.Suffix
		}
	}
	if plugin := findNodePluginNameByUUID(uuid, nodePluginNames...); plugin != "" {
		return "main.js"
	}
	return ""
}

func isLocalPluginConfigUUID(uuid string, nodePluginNames map[string]string) bool {
	for _, f := range Functions {
		if f.UUID == uuid {
			return true
		}
	}
	return findNodePluginNameByUUID(uuid, nodePluginNames) != ""
}

func findNodePluginNameByUUID(uuid string, indexes ...map[string]string) string {
	if len(indexes) != 0 && indexes[0] != nil {
		return indexes[0][uuid]
	}
	return nodePluginNameIndexByUUID()[uuid]
}

func nodePluginNameIndexByUUID() map[string]string {
	index := map[string]string{}
	root := nodePluginsRoot()
	files, err := os.ReadDir(root)
	if err != nil {
		return index
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".") {
			continue
		}
		if file.IsDir() {
			index[nameUuid(file.Name())] = file.Name()
			continue
		}
		if (strings.EqualFold(filepath.Ext(file.Name()), ".js") && file.Name() != "demo.main.js") || strings.EqualFold(filepath.Ext(file.Name()), ".py") {
			name := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
			index[nameUuid(name)] = name
		}
	}
	return index
}

func getPluginUserConfig(uuid string) map[string]interface{} {
	config := map[string]interface{}{}
	data := pluginConfigValues.GetString(uuid)
	if data == "" {
		return config
	}
	data = strings.TrimPrefix(data, "o:")
	json.Unmarshal([]byte(data), &config)
	return config
}

// pluginConfigBool reads a boolean field from the saved plugin configuration.
// When the user has not saved the field yet, its form default is used. The
// second return value is false when the plugin form does not declare the field.
func pluginConfigBool(uuid, key string) (bool, bool) {
	if value, ok := getPluginUserConfig(uuid)[key]; ok {
		return pluginConfigBoolValue(value), true
	}
	data := strings.TrimPrefix(pluginConfigSchemas.GetString(uuid), "o:")
	if data == "" {
		return false, false
	}
	var schema map[string]interface{}
	if json.Unmarshal([]byte(data), &schema) != nil {
		return false, false
	}
	properties, _ := schema["properties"].(map[string]interface{})
	property, _ := properties[key].(map[string]interface{})
	if property == nil || property["type"] != "boolean" {
		return false, false
	}
	value, exists := property["default"]
	if !exists {
		return false, true
	}
	return pluginConfigBoolValue(value), true
}

func pluginConfigBoolValue(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		value := strings.ToLower(strings.TrimSpace(typed))
		return value == "true" || value == "1" || value == "yes" || value == "on"
	case float64:
		return typed != 0
	default:
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(value)), "true")
	}
}
