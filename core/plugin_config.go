package core

import (
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
	GinApi(GET, "/api/plugin/configs", RequireAuth, func(ctx *gin.Context) {
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data":    getPluginConfigRecords(),
		})
	})
	GinApi(GET, "/api/plugin/config", RequireAuth, func(ctx *gin.Context) {
		uuid := ctx.Query("uuid")
		record := getPluginConfigRecord(uuid)
		if record == nil {
			ctx.JSON(200, map[string]interface{}{
				"success":      false,
				"errorMessage": "配置不存在，请先运行一次插件或声明 SillyGirlPluginConfig",
			})
			return
		}
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data":    record,
		})
	})
	GinApi(PUT, "/api/plugin/config", RequireAuth, func(ctx *gin.Context) {
		var req struct {
			UUID  string                 `json:"uuid"`
			Value map[string]interface{} `json:"value"`
		}
		if err := ctx.BindJSON(&req); err != nil {
			ctx.JSON(200, map[string]interface{}{
				"success":      false,
				"errorMessage": err.Error(),
			})
			return
		}
		if req.UUID == "" {
			ctx.JSON(200, map[string]interface{}{
				"success":      false,
				"errorMessage": "缺少插件 UUID",
			})
			return
		}
		SetBucketKeyValue(pluginConfigValues, req.UUID, req.Value)
		ctx.JSON(200, map[string]interface{}{
			"success": true,
		})
	})
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
	if strings.HasPrefix(data, "o:") {
		data = strings.TrimPrefix(data, "o:")
	}
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
		if strings.EqualFold(filepath.Ext(file.Name()), ".js") && file.Name() != "demo.main.js" {
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
	if strings.HasPrefix(data, "o:") {
		data = strings.TrimPrefix(data, "o:")
	}
	json.Unmarshal([]byte(data), &config)
	return config
}
