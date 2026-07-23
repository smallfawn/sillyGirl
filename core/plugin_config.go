package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/smallfawn/sillyGirl/core/storage"
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
}

type SillyGirlPluginConfig struct {
	UUID       string                 `json:"-"`
	JsonSchema map[string]interface{} `json:"jsonSchema"`
	UserConfig map[string]interface{} `json:"userConfig"`
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
	pluginConfigSchemas.Foreach(func(k, _ []byte) error {
		if record := getPluginConfigRecordWithIndex(string(k), nodePluginNames); record != nil {
			records = append(records, record)
		}
		return nil
	})
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
		if !file.IsDir() || strings.HasPrefix(file.Name(), ".") {
			continue
		}
		index[nameUuid(file.Name())] = file.Name()
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

func registerPluginConfig(uuid string, schema map[string]interface{}) {
	if uuid == "" || len(schema) == 0 {
		return
	}
	SetBucketKeyValue(pluginConfigSchemas, uuid, schema)
}

func makeSillyGirlPluginConfig(vm *goja.Runtime, uuid string, schemaValue goja.Value) *SillyGirlPluginConfig {
	schema := exportSchemaValue(vm, schemaValue)
	if _, ok := schema["type"]; !ok {
		schema["type"] = "object"
	}
	cfg := &SillyGirlPluginConfig{
		UUID:       uuid,
		JsonSchema: schema,
		UserConfig: getPluginUserConfig(uuid),
	}
	registerPluginConfig(uuid, schema)
	return cfg
}

func (cfg *SillyGirlPluginConfig) Get() map[string]interface{} {
	cfg.UserConfig = getPluginUserConfig(cfg.UUID)
	return cfg.UserConfig
}

func (cfg *SillyGirlPluginConfig) Set(values ...map[string]interface{}) map[string]interface{} {
	if len(values) != 0 && values[0] != nil {
		cfg.UserConfig = values[0]
	}
	SetBucketKeyValue(pluginConfigValues, cfg.UUID, cfg.UserConfig)
	return map[string]interface{}{
		"error": "",
	}
}

func exportSchemaValue(vm *goja.Runtime, value goja.Value) map[string]interface{} {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return map[string]interface{}{}
	}
	if obj := value.ToObject(vm); obj != nil {
		if fn, ok := goja.AssertFunction(obj.Get("toJSON")); ok {
			if v, err := fn(obj); err == nil {
				if schema, ok := normalizeSchema(v.Export()).(map[string]interface{}); ok {
					return schema
				}
				return map[string]interface{}{}
			}
		}
		if schema := obj.Get("schema"); schema != nil && !goja.IsUndefined(schema) {
			if schema, ok := normalizeSchema(schema.Export()).(map[string]interface{}); ok {
				return schema
			}
			return map[string]interface{}{}
		}
	}
	if schema, ok := normalizeSchema(value.Export()).(map[string]interface{}); ok {
		return schema
	}
	return map[string]interface{}{}
}

func normalizeSchema(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		if schema, ok := v["schema"]; ok {
			return normalizeSchema(schema)
		}
		rt := map[string]interface{}{}
		for key, item := range v {
			if strings.HasPrefix(key, "_") || key == "__schemaNode" {
				continue
			}
			rt[key] = normalizeSchema(item)
		}
		return rt
	case map[interface{}]interface{}:
		rt := map[string]interface{}{}
		for key, item := range v {
			rt[fmt.Sprint(key)] = normalizeSchema(item)
		}
		return rt
	case []interface{}:
		rt := make([]interface{}, 0, len(v))
		for _, item := range v {
			rt = append(rt, normalizeSchema(item))
		}
		return rt
	default:
		return v
	}
}

func installSillyGirlSchemaRuntime(vm *goja.Runtime, uuid string) {
	vm.Set("SillyGirlPluginConfig", func(call goja.ConstructorCall) *goja.Object {
		arg := goja.Undefined()
		if len(call.Arguments) != 0 {
			arg = call.Arguments[0]
		}
		cfg := makeSillyGirlPluginConfig(vm, uuid, arg)
		return vm.ToValue(cfg).ToObject(vm)
	})
	vm.Set("Form", func(schema goja.Value) *SillyGirlPluginConfig {
		return makeSillyGirlPluginConfig(vm, uuid, schema)
	})
	_, err := vm.RunString(`
(function () {
  function normalize(value) {
    if (value && value.__schemaNode && value.schema) return value.schema;
    if (value && typeof value.toJSON === 'function') return value.toJSON();
    return value;
  }
  function Node(type, extra) {
    this.__schemaNode = true;
    this.schema = Object.assign({ type: type }, extra || {});
  }
  Node.prototype.setTitle = function (value) { this.schema.title = value; return this; };
  Node.prototype.setDescription = function (value) { this.schema.description = value; return this; };
  Node.prototype.setDefault = function (value) { this.schema.default = value; return this; };
  Node.prototype.setEnum = function (value) { this.schema.enum = value; return this; };
  Node.prototype.setEnumNames = function (value) { this.schema.enumNames = value; return this; };
  Node.prototype.setRequired = function (value) { this.schema.required = value; return this; };
  Node.prototype.setFormat = function (value) { this.schema.format = value; return this; };
  Node.prototype.setMin = function (value) { this.schema.minimum = value; return this; };
  Node.prototype.setMax = function (value) { this.schema.maximum = value; return this; };
  Node.prototype.setMinLength = function (value) { this.schema.minLength = value; return this; };
  Node.prototype.setMaxLength = function (value) { this.schema.maxLength = value; return this; };
  Node.prototype.setPattern = function (value) { this.schema.pattern = value; return this; };
  Node.prototype.setWidget = function (value) { this.schema['ui:widget'] = value; return this; };
  Node.prototype.toJSON = function () { return this.schema; };
  globalThis.SillyGirlCreateSchema = {
    string: function () { return new Node('string'); },
    number: function () { return new Node('number'); },
    integer: function () { return new Node('integer'); },
    boolean: function () { return new Node('boolean'); },
    array: function (item) { return new Node('array', { items: normalize(item) || {} }); },
    object: function (props) {
      var properties = {};
      Object.keys(props || {}).forEach(function (key) { properties[key] = normalize(props[key]); });
      return new Node('object', { properties: properties });
    }
  };
})();
`)
	if err != nil {
		console.Error("SillyGirlCreateSchema 初始化失败: %v", err)
	}
}

func pluginConfigWatch(uuid string, handle func()) {
	storage.Watch(pluginConfigValues, uuid, func(_, _, _ string) *storage.Final {
		if handle != nil {
			handle()
		}
		return nil
	}, uuid)
}

func savePluginConfigFromJS(uuid string, value interface{}) map[string]interface{} {
	config, ok := normalizeSchema(value).(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "配置必须是对象"}
	}
	SetBucketKeyValue(pluginConfigValues, uuid, config)
	return map[string]interface{}{"error": ""}
}

func readPluginConfigFromJS(uuid string) map[string]interface{} {
	return getPluginUserConfig(uuid)
}

func installPluginConfigHelpers(vm *goja.Runtime, uuid string) {
	vm.Set("readPluginConfig", func() map[string]interface{} {
		return readPluginConfigFromJS(uuid)
	})
	vm.Set("savePluginConfig", func(value interface{}) map[string]interface{} {
		return savePluginConfigFromJS(uuid, value)
	})
	vm.Set("watchPluginConfig", func(handle func()) {
		pluginConfigWatch(uuid, handle)
	})
	vm.Set("pluginConfigDefaults", func(schemaValue goja.Value) map[string]interface{} {
		return collectSchemaDefaults(exportSchemaValue(vm, schemaValue))
	})
}

func collectSchemaDefaults(schema map[string]interface{}) map[string]interface{} {
	values := map[string]interface{}{}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return values
	}
	for key, raw := range props {
		prop, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if v, ok := prop["default"]; ok {
			values[key] = v
			continue
		}
		if prop["type"] == "object" {
			values[key] = collectSchemaDefaults(prop)
		}
	}
	return values
}

func pluginConfigSetDefault(uuid string, schema map[string]interface{}) {
	if pluginConfigValues.GetString(uuid) != "" {
		return
	}
	values := collectSchemaDefaults(schema)
	if len(values) != 0 {
		SetBucketKeyValue(pluginConfigValues, uuid, values)
	}
}
