package storagemigrate

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/core/storage/boltdb"
	"github.com/smallfawn/sillyGirl/core/storage/redis"
	"github.com/smallfawn/sillyGirl/utils"
)

const (
	smallcatPanelsKey = "smallcat_panels"
	qinglongPanelsKey = "qinglong_panels"
	daidaiPanelsKey   = "daidai_panels"
	smallcatReadScope = "smallcat:read"
)

type Summary struct {
	Storage            string `json:"storage"`
	PanelBuckets       int    `json:"panel_buckets"`
	AdapterKeys        int    `json:"adapter_keys"`
	PluginConfigValues int    `json:"plugin_config_values"`
	Users              int    `json:"users"`
	Authorizations     int    `json:"authorizations"`
	Total              int    `json:"total"`
	DryRun             bool   `json:"dry_run"`
	UpdatedAt          int    `json:"updated_at"`
}

type adapterKeyMigration struct {
	OldKeys []string
	Bucket  string
	Key     string
}

type normalUser struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	PasswordHash string `json:"password_hash"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	Disabled     bool   `json:"disabled"`
}

type normalUserBindings struct {
	QQ              string   `json:"qq"`
	Telegram        string   `json:"telegram"`
	SmallcatOpenID  string   `json:"smallcat_openid"`
	SmallcatOpenIDs []string `json:"smallcat_openids"`
	UpdatedAt       int64    `json:"updated_at"`
}

func OpenStorage() (storage.Bucket, string) {
	app := boltdb.InitsillyGirl()
	if strings.TrimSpace(app.GetString("storage")) == "redis" {
		return redis.InitsillyGirl(app.GetString("redis_addr"), app.GetString("redis_password")), "redis"
	}
	return app, "boltdb"
}

func Run(root storage.Bucket, storageType string, dryRun bool) Summary {
	result := Summary{Storage: storageType, DryRun: dryRun, UpdatedAt: int(time.Now().Unix())}
	result.PanelBuckets += migratePanelBucket(root, "smallcat", smallcatPanelsKey, dryRun)
	result.PanelBuckets += migratePanelBucket(root, "qinglong", qinglongPanelsKey, dryRun)
	result.PanelBuckets += migratePanelBucket(root, "daidai", daidaiPanelsKey, dryRun)
	result.AdapterKeys = migrateAdapterKeys(root, dryRun)
	result.PluginConfigValues = migratePluginConfigs(root, dryRun)
	result.Users = migrateUsers(root, dryRun)
	result.Authorizations = migrateAuthorizations(root, dryRun)
	result.Total = result.PanelBuckets + result.AdapterKeys + result.PluginConfigValues + result.Users + result.Authorizations
	if result.Total > 0 && !dryRun {
		setEncoded(root.Copy("sillyGirl"), "manual_storage_migration_v1_0_5", result)
	}
	return result
}

func migratePanelBucket(root storage.Bucket, kind, targetKey string, dryRun bool) int {
	app := root.Copy("sillyGirl")
	panels := parseJSONList(app.GetString(targetKey))
	seen := map[string]bool{}
	for _, panel := range panels {
		markSeen(seen, panel)
	}
	changed := 0
	root.Copy(targetKey).Foreach(func(keyBytes, valueBytes []byte) error {
		panel, ok := normalizePanel(kind, string(keyBytes), string(valueBytes))
		if !ok || isSeen(seen, panel) {
			return nil
		}
		panels = append(panels, panel)
		markSeen(seen, panel)
		changed++
		return nil
	})
	if changed > 0 && !dryRun {
		sort.SliceStable(panels, func(i, j int) bool { return panelSortKey(panels[i]) < panelSortKey(panels[j]) })
		setEncoded(app, targetKey, panels)
	}
	return changed
}

func normalizePanel(kind, key, raw string) (map[string]interface{}, bool) {
	item := parseJSONObject(raw)
	if len(item) == 0 {
		return nil, false
	}
	now := int(time.Now().Unix())
	id := firstString(item, "id", "uuid", "_id")
	if id == "" {
		id = strings.TrimSpace(key)
	}
	if id == "" {
		id = utils.GenUUID()
	}
	address := firstString(item, "address", "url", "base_url", "baseURL", "host", "server")
	if address == "" {
		return nil, false
	}
	name := firstStringDefault(item, address, "name", "title", "remark", "remarks", "label")
	panel := map[string]interface{}{
		"id":              id,
		"name":            name,
		"address":         address,
		"created_at":      firstInt(item, now, "created_at", "createdAt", "create_time", "createTime"),
		"updated_at":      firstInt(item, now, "updated_at", "updatedAt", "update_time", "updateTime"),
		"last_checked_at": firstInt(item, 0, "last_checked_at", "lastCheckedAt", "checked_at", "checkedAt"),
		"status":          firstStringDefault(item, "unknown", "status", "state"),
		"message":         firstString(item, "message", "msg", "error"),
	}
	switch kind {
	case "smallcat":
		panel["api_auth"] = firstString(item, "api_auth", "apiAuth", "auth", "token", "api_key", "apiKey")
		panel["group"] = firstString(item, "group")
		panel["namespace"] = firstString(item, "namespace")
		panel["account_limit"] = firstString(item, "account_limit", "accountLimit", "limit")
		panel["account_used"] = firstString(item, "account_used", "accountUsed", "used", "count")
		panel["credit_balance"] = firstString(item, "credit_balance", "creditBalance", "balance")
	case "qinglong":
		panel["client_id"] = firstString(item, "client_id", "clientID", "clientId", "app_id", "appId")
		panel["client_secret"] = firstString(item, "client_secret", "clientSecret", "secret", "app_secret", "appSecret")
	case "daidai":
		panel["app_key"] = firstString(item, "app_key", "appKey", "client_id", "clientId")
		panel["app_secret"] = firstString(item, "app_secret", "appSecret", "client_secret", "clientSecret", "secret")
	}
	return panel, true
}

func migrateAdapterKeys(root storage.Bucket, dryRun bool) int {
	items := []adapterKeyMigration{
		{[]string{"clawbot_enable", "clawbot.enable"}, "clawbot", "enable"},
		{[]string{"clawbot_token", "clawbot.token"}, "clawbot", "token"},
		{[]string{"clawbot_api_base", "clawbot.api_base"}, "clawbot", "api_base"},
		{[]string{"clawbot_debug", "clawbot.debug"}, "clawbot", "debug"},
		{[]string{"qq_enable", "qq.enable"}, "qq", "enable"},
		{[]string{"qq_token", "qq.token"}, "qq", "token"},
		{[]string{"qq_debug", "qq.debug"}, "qq", "debug"},
		{[]string{"telegram_enable", "telegram.enable"}, "telegram", "enable"},
		{[]string{"telegram_token", "telegram.token"}, "telegram", "token"},
		{[]string{"telegram_api_base", "telegram.api_base"}, "telegram", "api_base"},
		{[]string{"telegram_debug", "telegram.debug"}, "telegram", "debug"},
		{[]string{"dingtalk_enable", "dingtalk.enable"}, "dingtalk", "enable"},
		{[]string{"dingtalk_client_id", "dingtalk.client_id", "dingtalk_app_key", "dingtalk.app_key"}, "dingtalk", "client_id"},
		{[]string{"dingtalk_client_secret", "dingtalk.client_secret", "dingtalk_app_secret", "dingtalk.app_secret"}, "dingtalk", "client_secret"},
		{[]string{"dingtalk_debug", "dingtalk.debug"}, "dingtalk", "debug"},
		{[]string{"qqguild_enable", "qqguild.enable"}, "qqguild", "enable"},
		{[]string{"qqguild_mode", "qqguild.mode"}, "qqguild", "mode"},
		{[]string{"qqguild_app_id", "qqguild.app_id", "qqguild_appid", "qqguild.appid"}, "qqguild", "app_id"},
		{[]string{"qqguild_app_secret", "qqguild.app_secret", "qqguild_secret", "qqguild.secret"}, "qqguild", "app_secret"},
		{[]string{"qqguild_sandbox", "qqguild.sandbox"}, "qqguild", "sandbox"},
		{[]string{"qqguild_debug", "qqguild.debug"}, "qqguild", "debug"},
		{[]string{"pagermaid_enable", "pagermaid.enable"}, "pagermaid", "enable"},
		{[]string{"pagermaid_token", "pagermaid.token"}, "pagermaid", "token"},
		{[]string{"pagermaid_debug", "pagermaid.debug"}, "pagermaid", "debug"},
		{[]string{"web_chat_public", "web.web_chat_public"}, "sillyGirl", "web_chat_public"},
	}
	changed := 0
	for _, item := range items {
		target := root.Copy(item.Bucket)
		if strings.TrimSpace(target.GetString(item.Key)) != "" {
			continue
		}
		value := firstLegacyValue(root, item.OldKeys...)
		if strings.TrimSpace(value) == "" {
			continue
		}
		changed++
		if !dryRun {
			target.Set2(item.Key, value)
		}
	}
	return changed
}

func migratePluginConfigs(root storage.Bucket, dryRun bool) int {
	bucket := root.Copy("plugin_config_values")
	changed := 0
	bucket.Foreach(func(keyBytes, valueBytes []byte) error {
		config := parseJSONObject(string(valueBytes))
		if len(config) == 0 || !migratePluginConfig(config) {
			return nil
		}
		changed++
		if !dryRun {
			setEncoded(bucket, string(keyBytes), config)
		}
		return nil
	})
	return changed
}

func migratePluginConfig(config map[string]interface{}) bool {
	changed := false
	if _, exists := config["sync_panel"]; !exists {
		if asBool(config["sync_qinglong"]) {
			config["sync_panel"] = "qinglong"
			changed = true
		} else if _, ok := config["sync_qinglong"]; ok {
			config["sync_panel"] = "none"
			changed = true
		}
	}
	if _, exists := config["daidai_id"]; !exists {
		if _, has := config["sync_panel"]; has {
			config["daidai_id"] = 1
			changed = true
		}
	}
	if _, exists := config["account_mode"]; !exists {
		if _, has := config["manual_openids"]; has {
			config["account_mode"] = "manual"
			changed = true
		} else if v, has := config["openid"]; has {
			config["manual_openids"] = v
			config["account_mode"] = "manual"
			changed = true
		} else if v, has := config["openids"]; has {
			config["manual_openids"] = v
			config["account_mode"] = "manual"
			changed = true
		}
	}
	for _, key := range []string{"smallcat_auth", "smallcatAuth", "auth"} {
		if _, ok := config[key]; ok {
			delete(config, key)
			changed = true
		}
	}
	return changed
}

func migrateUsers(root storage.Bucket, dryRun bool) int {
	bucket := root.Copy("users")
	changed := 0
	bucket.Foreach(func(keyBytes, valueBytes []byte) error {
		key := string(keyBytes)
		if strings.HasPrefix(key, "bindings:") || strings.HasPrefix(key, "user:") {
			if strings.HasPrefix(key, "bindings:") {
				bindings := normalizeBindings(loadBindings(string(valueBytes)))
				if !dryRun {
					bucket.Set2(key, string(marshal(bindings)))
				}
			}
			return nil
		}
		value := parseJSONObject(string(valueBytes))
		username := firstString(value, "username", "name", "account")
		if username == "" {
			return nil
		}
		user := normalUser{
			ID:           firstStringDefault(value, utils.GenUUID(), "id", "uid", "user_id", "userId"),
			Username:     strings.TrimSpace(username),
			Nickname:     firstStringDefault(value, username, "nickname", "nick", "display_name", "displayName"),
			PasswordHash: firstString(value, "password_hash", "passwordHash", "hash"),
			CreatedAt:    int64(firstInt(value, int(time.Now().Unix()), "created_at", "createdAt", "create_time", "createTime")),
			UpdatedAt:    int64(firstInt(value, int(time.Now().Unix()), "updated_at", "updatedAt", "update_time", "updateTime")),
			Disabled:     asBool(value["disabled"]),
		}
		bindings := normalizeBindings(normalUserBindings{
			QQ:              firstString(value, "qq", "qq_id", "qqid"),
			Telegram:        firstString(value, "telegram", "tg", "tgid", "telegram_id", "telegramId"),
			SmallcatOpenID:  firstString(value, "smallcat_openid", "smallcatOpenID", "smallcatOpenId", "openid", "openId"),
			SmallcatOpenIDs: mergedSlices(value["smallcat_openids"], value["openids"]),
			UpdatedAt:       user.UpdatedAt,
		})
		changed++
		if !dryRun {
			bucket.Set2("user:"+strings.ToLower(user.Username), string(marshal(user)))
			bucket.Set2("bindings:"+strings.ToLower(user.Username), string(marshal(bindings)))
		}
		return nil
	})
	return changed
}

func migrateAuthorizations(root storage.Bucket, dryRun bool) int {
	users := map[string]string{}
	root.Copy("users").Foreach(func(keyBytes, valueBytes []byte) error {
		if !strings.HasPrefix(string(keyBytes), "user:") {
			return nil
		}
		user := normalUser{}
		if json.Unmarshal([]byte(strings.TrimPrefix(string(valueBytes), "o:")), &user) == nil && user.Username != "" && user.ID != "" {
			users[strings.ToLower(user.Username)] = user.ID
		}
		return nil
	})
	bucket := root.Copy("plugin_user_authorizations")
	changed := 0
	bucket.Foreach(func(keyBytes, valueBytes []byte) error {
		key := string(keyBytes)
		if strings.TrimSpace(string(valueBytes)) == "" || !strings.HasSuffix(key, ":"+smallcatReadScope) {
			return nil
		}
		rest := strings.TrimSuffix(key, ":"+smallcatReadScope)
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) != 2 {
			return nil
		}
		userID := users[strings.ToLower(parts[0])]
		if userID == "" {
			return nil
		}
		newKey := userID + ":" + parts[1] + ":" + smallcatReadScope
		if newKey == key || bucket.GetString(newKey) != "" {
			return nil
		}
		changed++
		if !dryRun {
			bucket.Set2(newKey, string(valueBytes))
		}
		return nil
	})
	return changed
}

func firstLegacyValue(root storage.Bucket, keys ...string) string {
	for _, bucketName := range []string{"sillyGirl", "app"} {
		bucket := root.Copy(bucketName)
		for _, key := range keys {
			if value := strings.TrimSpace(bucket.GetString(key)); value != "" {
				return value
			}
		}
	}
	return ""
}

func setEncoded(bucket storage.Bucket, key string, value interface{}) {
	bucket.Set2(key, encode(value))
}

func encode(value interface{}) string {
	switch value := value.(type) {
	case nil:
		return ""
	case string:
		return value
	case []byte:
		return string(value)
	case bool:
		return fmt.Sprintf("b:%t", value)
	case int:
		return fmt.Sprintf("d:%d", value)
	case int64:
		return fmt.Sprintf("d:%d", value)
	default:
		return "o:" + string(marshal(value))
	}
}

func marshal(value interface{}) []byte {
	data, _ := json.Marshal(value)
	return data
}

func parseJSONList(raw string) []map[string]interface{} {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "o:"))
	list := []map[string]interface{}{}
	if raw == "" {
		return list
	}
	if json.Unmarshal([]byte(raw), &list) == nil {
		return list
	}
	return list
}

func parseJSONObject(raw string) map[string]interface{} {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "o:"))
	if raw == "" {
		return nil
	}
	value := map[string]interface{}{}
	if json.Unmarshal([]byte(raw), &value) == nil {
		return value
	}
	return nil
}

func loadBindings(raw string) normalUserBindings {
	bindings := normalUserBindings{}
	_ = json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(raw), "o:")), &bindings)
	return bindings
}

func normalizeBindings(bindings normalUserBindings) normalUserBindings {
	bindings.QQ = strings.TrimSpace(bindings.QQ)
	bindings.Telegram = strings.TrimSpace(bindings.Telegram)
	openids := []string{}
	if bindings.SmallcatOpenID != "" {
		openids = appendUnique(openids, bindings.SmallcatOpenID)
	}
	for _, openid := range bindings.SmallcatOpenIDs {
		openids = appendUnique(openids, openid)
	}
	bindings.SmallcatOpenIDs = openids
	if len(openids) > 0 {
		bindings.SmallcatOpenID = openids[0]
	} else {
		bindings.SmallcatOpenID = ""
	}
	return bindings
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, item := range values {
		if item == value {
			return values
		}
	}
	return append(values, value)
}

func mergedSlices(values ...interface{}) []string {
	out := []string{}
	for _, value := range values {
		if rows, ok := stringSlice(value); ok {
			for _, item := range rows {
				out = appendUnique(out, item)
			}
		}
	}
	return out
}

func stringSlice(value interface{}) ([]string, bool) {
	switch typed := value.(type) {
	case []interface{}:
		out := []string{}
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
		return out, true
	case string:
		parts := strings.FieldsFunc(typed, func(r rune) bool {
			return r == ',' || r == '，' || r == '\n' || r == '\t' || r == ' '
		})
		out := []string{}
		for _, item := range parts {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
		return out, len(out) > 0
	default:
		return nil, false
	}
}

func firstString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func firstStringDefault(values map[string]interface{}, fallback string, keys ...string) string {
	if value := firstString(values, keys...); value != "" {
		return value
	}
	return fallback
}

func firstInt(values map[string]interface{}, fallback int, keys ...string) int {
	for _, key := range keys {
		switch value := values[key].(type) {
		case int:
			return value
		case int64:
			return int(value)
		case float64:
			return int(value)
		case string:
			if n := utils.Int(strings.TrimSpace(value)); n != 0 {
				return n
			}
		}
	}
	return fallback
}

func asBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "on":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func markSeen(seen map[string]bool, panel map[string]interface{}) {
	if key := panelIdentity(panel); key != "" {
		seen[key] = true
	}
}

func isSeen(seen map[string]bool, panel map[string]interface{}) bool {
	key := panelIdentity(panel)
	return key != "" && seen[key]
}

func panelIdentity(panel map[string]interface{}) string {
	if id := firstString(panel, "id", "uuid", "_id"); id != "" {
		return "id:" + id
	}
	if address := firstString(panel, "address", "url", "base_url", "host"); address != "" {
		return "address:" + strings.TrimRight(address, "/")
	}
	return ""
}

func panelSortKey(panel map[string]interface{}) string {
	if value := firstString(panel, "created_at", "createdAt"); value != "" {
		return value
	}
	if value := firstString(panel, "name"); value != "" {
		return value
	}
	return panelIdentity(panel)
}
