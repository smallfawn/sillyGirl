package core

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/utils"
)

const pluginSourceReposKey = "plugin_source_repos"
const pluginSourceGithubProxyKey = "plugin_source_github_proxy"
const pluginSourceGithubProxyOptionsKey = "plugin_source_github_proxy_options"
const defaultPluginSourceRepo = "https://github.com/smallfawn/sillyGirl_Plugins"
const githubNodePluginScheme = "github-node"

var builtinGithubAccelerators = []string{
	"https://gh-proxy.org",
	"https://ghproxy.net",
	"https://cdn.gh-proxy.org",
	"http://jp-proxy.gitwarp.top:3000",
	"http://kr1-proxy.gitwarp.top:8081",
	"http://kr2-proxy.gitwarp.top:9980",
	"http://jp1-proxy.gitwarp.top:8123",
}

type RequestPluginResult struct {
	Success  bool                  `json:"success,omitempty"`
	Code     int                   `json:"code,omitempty"`
	Message  string                `json:"message,omitempty"`
	Data     []*common.Function    `json:"data"`
	Page     int                   `json:"page"`
	Total    int                   `json:"total"`
	Tab1     int                   `json:"tab1"`
	Tab2     int                   `json:"tab2"`
	Tab3     int                   `json:"tab3"`
	Latest   int                   `json:"latest"`
	Private  int                   `json:"private"`
	Modules  int                   `json:"modules"`
	All      int                   `json:"all"`
	Tab      string                `json:"tab"`
	Time     time.Time             `json:"time"`
	Class    map[string]int        `json:"class"`
	Origins  map[string]string     `json:"origins"`
	Sources  []string              `json:"sources,omitempty"`
	Settings []*PluginConfigRecord `json:"settings,omitempty"`
}

var plugin_list = []*common.Function{}
var pluginMarketLock sync.RWMutex

func clonePluginFunctions(items []*common.Function) []*common.Function {
	cloned := make([]*common.Function, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		copyItem := *item
		copyItem.Rules = append([]string(nil), item.Rules...)
		copyItem.RulePatterns = append([]*regexp.Regexp(nil), item.RulePatterns...)
		copyItem.RuleErrors = append([]error(nil), item.RuleErrors...)
		copyItem.Params = make([][]string, len(item.Params))
		for index := range item.Params {
			copyItem.Params[index] = append([]string(nil), item.Params[index]...)
		}
		copyItem.CronIds = append([]int(nil), item.CronIds...)
		copyItem.Classes = append([]string(nil), item.Classes...)
		copyItem.Dependencies = append([]string(nil), item.Dependencies...)
		copyItem.ModuleDependencies = append([]string(nil), item.ModuleDependencies...)
		if item.Cron != nil {
			copyItem.Cron = make(map[string]string, len(item.Cron))
			for key, value := range item.Cron {
				copyItem.Cron[key] = value
			}
		}
		if item.Status != nil {
			status := *item.Status
			copyItem.Status = &status
		}
		cloned = append(cloned, &copyItem)
	}
	return cloned
}

func setPluginMarketItems(items []*common.Function) {
	pluginMarketLock.Lock()
	plugin_list = items
	pluginMarketLock.Unlock()
}

func pluginMarketItemsSnapshot() []*common.Function {
	pluginMarketLock.RLock()
	defer pluginMarketLock.RUnlock()
	return clonePluginFunctions(plugin_list)
}

func installedPluginSnapshot() []*common.Function {
	pluginLock.Lock()
	defer pluginLock.Unlock()
	return clonePluginFunctions(Functions)
}

const latestPluginMarketLimit = 20

func pluginPublishedAt(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func latestPluginMarketItems(items []*common.Function, limit int) []*common.Function {
	latest := make([]*common.Function, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, ok := pluginPublishedAt(item.CreateAt); ok {
			latest = append(latest, item)
		}
	}
	sort.SliceStable(latest, func(i, j int) bool {
		left, _ := pluginPublishedAt(latest[i].CreateAt)
		right, _ := pluginPublishedAt(latest[j].CreateAt)
		if left.Equal(right) {
			return firstNonEmpty(latest[i].Title, latest[i].UUID) < firstNonEmpty(latest[j].Title, latest[j].UUID)
		}
		return left.After(right)
	})
	if limit > 0 && len(latest) > limit {
		latest = latest[:limit]
	}
	return latest
}

func compactPluginSearchText(value string) string {
	var result strings.Builder
	for _, char := range strings.ToLower(value) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			result.WriteRune(char)
		}
	}
	return result.String()
}

func pluginMatchesKeyword(plugin *common.Function, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	if plugin == nil {
		return false
	}
	fields := []string{
		plugin.Title, plugin.Desc, plugin.Author, plugin.Class,
		plugin.Organization, plugin.Address, plugin.UUID, plugin.Type,
		plugin.Suffix, plugin.Rule, plugin.Version,
	}
	fields = append(fields, plugin.Classes...)
	fields = append(fields, plugin.Dependencies...)
	fields = append(fields, plugin.ModuleDependencies...)
	haystack := strings.ToLower(strings.Join(fields, " "))
	compactHaystack := compactPluginSearchText(haystack)
	for _, token := range strings.Fields(keyword) {
		if strings.Contains(haystack, token) {
			continue
		}
		compactToken := compactPluginSearchText(token)
		if compactToken == "" || !strings.Contains(compactHaystack, compactToken) {
			return false
		}
	}
	return true
}

func initPluginList() {
	list := []*common.Function{}
	for _, source := range pluginSourceAddresses() {
		items, err := pluginSourceItems(source)
		if err != nil {
			console.Error("加载插件源失败 %s: %v", source, err)
			continue
		}
		list = append(list, items...)
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].Desc < list[j].Desc
	})
	setPluginMarketItems(list)
}

var plugin_downloads = MakeBucket("plugin_downloads")

func initWebPluginList() {
	GinApi(GET, "/api/admin/plugin-market/sources", RequireAuth, func(ctx *gin.Context) {
		ApiOK(ctx, pluginSourceAddresses())
	})
	GinApi(GET, "/api/admin/plugin-market/github-proxy", RequireAuth, func(ctx *gin.Context) {
		proxy := githubAcceleratorPrefix()
		ApiOK(ctx, map[string]interface{}{
			"proxy":   proxy,
			"options": settingOptions(pluginSourceGithubProxyOptionsKey, builtinGithubAccelerators, proxy, normalizeGithubAcceleratorPrefix),
		})
	})
	GinApi(POST, "/api/admin/plugin-market/github-proxy", RequireAuth, func(ctx *gin.Context) {
		payload := map[string]string{}
		if err := ctx.BindJSON(&payload); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		proxy, err := normalizeGithubAcceleratorPrefix(payload["proxy"])
		if err != nil {
			ApiUnprocessable(ctx, err.Error())
			return
		}
		sillyGirl.Set(pluginSourceGithubProxyKey, proxy)
		ApiOK(ctx, map[string]interface{}{"proxy": proxy})
	})
	GinApi(POST, "/api/admin/plugin-market/github-proxy-options", RequireAuth, func(ctx *gin.Context) {
		payload := map[string]string{}
		if err := ctx.BindJSON(&payload); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		normalized, err := normalizeGithubAcceleratorPrefix(payload["proxy"])
		if err != nil {
			ApiUnprocessable(ctx, err.Error())
			return
		}
		if Contains(settingOptions(pluginSourceGithubProxyOptionsKey, builtinGithubAccelerators, githubAcceleratorPrefix(), normalizeGithubAcceleratorPrefix), normalized) {
			ApiConflict(ctx, "地址已存在")
			return
		}
		proxy, err := addSettingOption(pluginSourceGithubProxyOptionsKey, normalized, normalizeGithubAcceleratorPrefix)
		if err != nil {
			respondSettingOptionError(ctx, err)
			return
		}
		ApiCreated(ctx, "/api/admin/plugin-market/github-proxy-options/"+url.PathEscape(proxy), map[string]interface{}{
			"proxy":   githubAcceleratorPrefix(),
			"added":   proxy,
			"options": settingOptions(pluginSourceGithubProxyOptionsKey, builtinGithubAccelerators, githubAcceleratorPrefix(), normalizeGithubAcceleratorPrefix),
		})
	})
	GinApi(POST, "/api/admin/plugin-market/github-proxy-option-deletions/*proxy", RequireAuth, func(ctx *gin.Context) {
		proxyValue := strings.TrimPrefix(ctx.Param("proxy"), "/")
		proxy, err := removeSettingOption(pluginSourceGithubProxyOptionsKey, proxyValue, builtinGithubAccelerators, normalizeGithubAcceleratorPrefix)
		if err != nil {
			respondSettingOptionError(ctx, err)
			return
		}
		if proxy == githubAcceleratorPrefix() {
			sillyGirl.Set(pluginSourceGithubProxyKey, "")
		}
		ApiOK(ctx, map[string]interface{}{
			"proxy":   githubAcceleratorPrefix(),
			"removed": proxy,
			"options": settingOptions(pluginSourceGithubProxyOptionsKey, builtinGithubAccelerators, githubAcceleratorPrefix(), normalizeGithubAcceleratorPrefix),
		})
	})
	GinApi(POST, "/api/admin/plugin-market/sources", RequireAuth, func(ctx *gin.Context) {
		payload := map[string]string{}
		if err := ctx.BindJSON(&payload); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		address := normalizePluginSourceAddress(payload["address"])
		if address == "" {
			ApiUnprocessable(ctx, "插件源地址不能为空")
			return
		}
		items, err := pluginSourceItems(address)
		if err != nil {
			ApiUnprocessable(ctx, err.Error())
			return
		}
		sources := pluginSourceAddresses()
		if Contains(sources, address) {
			ApiConflict(ctx, "插件源已存在")
			return
		}
		sources = append(sources, address)
		savePluginSourceAddresses(sources)
		setPluginMarketItems(listPluginSources())
		ApiCreated(ctx, "/api/admin/plugin-market/sources/"+url.PathEscape(address), map[string]interface{}{"address": address, "count": len(items)})
	})
	GinApi(POST, "/api/admin/plugin-market/source-deletions/*address", RequireAuth, func(ctx *gin.Context) {
		address := normalizePluginSourceAddress(strings.TrimPrefix(ctx.Param("address"), "/"))
		next := []string{}
		found := false
		for _, source := range pluginSourceAddresses() {
			if source != address {
				next = append(next, source)
			} else {
				found = true
			}
		}
		if !found {
			ApiNotFound(ctx, "插件源不存在")
			return
		}
		savePluginSourceAddresses(next)
		setPluginMarketItems(listPluginSources())
		ApiNoContent(ctx)
	})
	GinApi(GET, "/api/plugin-market/plugins", handlePluginMarketPlugins)
}

func handlePluginMarketPlugins(ctx *gin.Context) {
	// ctx.QueryArray()
	origins := ctx.QueryArray("origin[]")
	current := utils.Int(ctx.Query("page"))
	pageSize := utils.Int(ctx.Query("page_size"))
	activeKey := ctx.Query("status")
	keyword := ctx.Query("keyword")
	class := ctx.Query("class")
	mclass := ctx.Query("mclass")
	rr := RequestPluginResult{}
	if ctx.Request.Method == http.MethodPost || ctx.Request.URL.Path == "/api/plugin-market/plugins" {
		initPluginList()
	}
	marketItems := pluginMarketItemsSnapshot()
	installedItems := installedPluginSnapshot()
	if pageSize <= 0 {
		pageSize = 10
	} else if pageSize > 200 {
		pageSize = 200
	}
	if current < 0 {
		current = 1
	}
	if class == "" {
		class = "全部"
	}
	rr.Page = current
	rr.Data = []*common.Function{}
	if current != 0 {
		privatePlugins := localPrivatePlugins(marketItems, installedItems)
		rr.Private = len(privatePlugins)
		allMarketPlugins := append(append([]*common.Function(nil), privatePlugins...), marketItems...)
		marketPlugins := allMarketPlugins
		if activeKey == "private" {
			marketPlugins = privatePlugins
		}
		var list []*common.Function
		if keyword == "" {
			if len(origins) == 0 {
				list = append(list, marketPlugins...)

			} else {
				for _, f := range marketPlugins {
					if Contains(origins, f.Organization) {
						list = append(list, f)
					}
				}
			}
		} else {
			if len(origins) == 0 {
				for _, f := range marketPlugins {
					if pluginMatchesKeyword(f, keyword) {
						list = append(list, f)
					}
				}
			} else {
				for _, f := range marketPlugins {
					if pluginMatchesKeyword(f, keyword) {
						if Contains(origins, f.Organization) {
							list = append(list, f)
						}
					}
				}
			}

		}
		rr.Total = len(list)
		tab1 := []*common.Function{}
		tab2 := []*common.Function{}
		tab3 := []*common.Function{}
		fc := installedItems
		classes := map[string][]*common.Function{}
		classesNum := map[string]int{}
		for i := range list {
			if len(list[i].Classes) == 0 {
				class := "未分类"
				if _, ok := classes[class]; !ok {
					classes[class] = []*common.Function{}
				}
				classes[class] = append(classes[class], list[i])
			} else {
				for _, class := range list[i].Classes {
					class = strings.TrimRight(class, "类")
					if _, ok := classes[class]; !ok {
						classes[class] = []*common.Function{}
					}
					classes[class] = append(classes[class], list[i])
				}
			}
		}

		for class, fs := range classes {
			classesNum[class] = len(fs)
		}
		classesNum["全部"] = len(list)
		if class != "全部" {
			list = classes[class]
		}
		latest := latestPluginMarketItems(list, latestPluginMarketLimit)
		modules := pluginModuleMarketItems(list)
		rr.Latest = len(latest)
		rr.Modules = len(modules)
		rr.Class = classesNum
		var origins = map[string]string{}
		for i := range list { //处理第二分类
			if list[i].Organization != "" {
				origins[list[i].Organization] = list[i].Organization
			}
			ded := false
			for j := range fc {
				if list[i].UUID == fc[j].UUID {
					if list[i].Version != fc[j].Version {
						tab3 = append(tab3, list[i])
					}
					ded = true
					break
				}
			}
			if ded {
				tab1 = append(tab1, list[i]) //已安装
			} else {
				tab2 = append(tab2, list[i])
			}
		}
		rr.Origins = origins
		rr.All = len(list)
		rr.Tab1 = len(tab1)
		rr.Tab2 = len(tab2)
		rr.Tab3 = len(tab3)
		if activeKey == "private" {
			rr.All = len(allMarketPlugins)
			rr.Tab1, rr.Tab2, rr.Tab3 = pluginMarketCounts(allMarketPlugins, fc)
			rr.Latest = len(latestPluginMarketItems(allMarketPlugins, latestPluginMarketLimit))
			rr.Modules = len(pluginModuleMarketItems(allMarketPlugins))
		}
		if activeKey == "tab1" {
			list = tab1
		} else if activeKey == "tab2" {
			list = tab2
		} else if activeKey == "tab3" {
			list = tab3
		} else if activeKey == "latest" {
			list = latest
		} else if activeKey == "module" {
			list = modules
		}
		tab := ""
		if mclass == "true" {
			if rr.Tab2 > rr.Tab1 {
				list = tab2
				tab = "tab2"
			} else {
				list = tab1
				tab = "tab1"
			}
		}
		rr.Tab = tab
		rr.Total = len(list)
		if len(list) == 0 {
			includePluginMarketResources(ctx, &rr)
			ApiOK(ctx, rr)
			return
		}
		if last := (rr.Total + pageSize - 1) / pageSize; current > last {
			current = last
		}
		rr.Page = current
		begin := (current - 1) * pageSize
		end := (current) * pageSize
		if end > rr.Total {
			end = rr.Total
		}
		if begin > end {
			begin = end
		}
		rr.Data = append(rr.Data, list[begin:end]...)
		publics := []string{}
		for _, f := range installedItems {
			if f.Public && f.UUID != "" {
				publics = append(publics, f.UUID)
			}
		}
		for i := range rr.Data {
			rr.Data[i].Icon = pluginIconOrDefault(rr.Data[i].Icon)
			rr.Data[i].HasForm = false
			rr.Data[i].HasUserForm = false
			rr.Data[i].ConfigRegistered = getPluginConfigRecord(rr.Data[i].UUID) != nil
			rr.Data[i].UsesSmallCat = false
			rr.Data[i].Running = false
			for j := range fc {
				if rr.Data[i].UUID == fc[j].UUID {
					rr.Data[i].Admin = fc[j].Admin
					rr.Data[i].Cron = fc[j].Cron
					rr.Data[i].Messages = GetPluginMessage(rr.Data[i].UUID)
					rr.Data[i].CurrentVersion = fc[j].Version
					rr.Data[i].LatestVersion = rr.Data[i].Version
					if rr.Data[i].Version != fc[j].Version {
						rr.Data[i].InstallStatus = 1
						if rr.Data[i].UpdateContent == "" {
							rr.Data[i].UpdateContent = firstNonEmpty(rr.Data[i].Desc, "发现新版本")
						}
					} else {
						rr.Data[i].InstallStatus = 2
					}
					if rr.Data[i].InstallStatus != 1 && Contains(publics, rr.Data[i].UUID) {
						rr.Data[i].InstallStatus = 6
					}
					if fc[j].HasForm {
						rr.Data[i].HasForm = true
					}
					rr.Data[i].HasUserForm = fc[j].HasUserForm
					rr.Data[i].UsesSmallCat = fc[j].UsesSmallCat
					if fc[j].Running {
						rr.Data[i].Running = true
					}
					rr.Data[i].Debug = plugin_debug.GetString(rr.Data[i].UUID) == "b:true"
					rr.Data[i].Status = pluginStatusValue(pluginExecutionEnabled(fc[j]))
					rr.Data[i].Open = fc[j].Open && (fc[j].UsesSmallCat || fc[j].HasUserForm)
				}
			}
			rr.Data[i].Desc = parseReply2(rr.Data[i].Desc)
		}

		includePluginMarketResources(ctx, &rr)
		ApiOK(ctx, rr)
		return
	}

	ApiOK(ctx, GetPublicResponse())
}

func pluginModuleMarketItems(items []*common.Function) []*common.Function {
	modules := make([]*common.Function, 0)
	for _, item := range items {
		if item != nil && item.Module {
			modules = append(modules, item)
		}
	}
	return modules
}

func includePluginMarketResources(ctx *gin.Context, response *RequestPluginResult) {
	if !strings.HasPrefix(ctx.Request.URL.Path, "/api/admin/") {
		return
	}
	include := map[string]bool{}
	for _, name := range strings.Split(ctx.Query("include"), ",") {
		include[strings.TrimSpace(name)] = true
	}
	if include["sources"] {
		response.Sources = pluginSourceAddresses()
	}
	if include["settings"] {
		response.Settings = getPluginConfigRecords()
	}
}

func pluginMarketCounts(market []*common.Function, installed []*common.Function) (installedCount, missingCount, updateCount int) {
	for _, item := range market {
		if item == nil || item.UUID == "" {
			continue
		}
		found := false
		for _, current := range installed {
			if current == nil || current.UUID != item.UUID {
				continue
			}
			found = true
			installedCount++
			if current.Version != item.Version {
				updateCount++
			}
			break
		}
		if !found {
			missingCount++
		}
	}
	return
}

func localPrivatePlugins(remote []*common.Function, installed []*common.Function) []*common.Function {
	remoteIDs := map[string]struct{}{}
	for _, plugin := range remote {
		if plugin != nil && plugin.UUID != "" {
			remoteIDs[plugin.UUID] = struct{}{}
		}
	}
	local := []*common.Function{}
	for _, plugin := range installed {
		if plugin == nil || plugin.UUID == "" || (plugin.Type != NODE && plugin.Type != PYTHON) {
			continue
		}
		if _, exists := remoteIDs[plugin.UUID]; exists {
			continue
		}
		item := *plugin
		item.InstallStatus = 2
		item.CurrentVersion = plugin.Version
		item.LatestVersion = plugin.Version
		item.Organization = "本地插件"
		item.Address = ""
		item.Messages = nil
		item.Dependencies = append([]string(nil), plugin.Dependencies...)
		item.ModuleDependencies = append([]string(nil), plugin.ModuleDependencies...)
		item.Classes = append([]string(nil), plugin.Classes...)
		local = append(local, &item)
	}
	sort.SliceStable(local, func(i, j int) bool {
		return firstNonEmpty(local[i].Title, local[i].UUID) < firstNonEmpty(local[j].Title, local[j].UUID)
	})
	return local
}

func listPluginSources() []*common.Function {
	list := []*common.Function{}
	for _, source := range pluginSourceAddresses() {
		items, err := pluginSourceItems(source)
		if err != nil {
			continue
		}
		list = append(list, items...)
	}
	return list
}

func pluginSourceAddresses() []string {
	raw := strings.TrimSpace(sillyGirl.GetString(pluginSourceReposKey))
	if raw == "" {
		return []string{defaultPluginSourceRepo}
	}
	sources := []string{}
	if json.Unmarshal([]byte(strings.TrimPrefix(raw, "o:")), &sources) != nil {
		sources = strings.FieldsFunc(raw, func(r rune) bool {
			return r == '\n' || r == '\r' || r == ',' || r == ';' || r == '\t'
		})
	}
	out := []string{}
	for _, source := range sources {
		address := normalizePluginSourceAddress(source)
		if address != "" && !Contains(out, address) {
			out = append(out, address)
		}
	}
	return out
}

func savePluginSourceAddresses(sources []string) {
	sillyGirl.Set(pluginSourceReposKey, string(utils.JsonMarshal(sources)))
}

func normalizePluginSourceAddress(address string) string {
	address = strings.TrimSpace(address)
	if strings.HasPrefix(strings.ToLower(address), "link://") {
		return address
	}
	address = strings.TrimSuffix(address, ".git")
	return strings.TrimRight(address, "/")
}

func pluginSourceItems(address string) ([]*common.Function, error) {
	address = normalizePluginSourceAddress(address)
	if strings.HasPrefix(strings.ToLower(address), "link://") {
		return linkPluginSourceItems(address)
	}
	return githubPluginSourceItems(address)
}

func linkPluginSourceItems(address string) ([]*common.Function, error) {
	raw := address[len("link://"):]
	data, err := DecryptByAes(raw)
	if err != nil {
		return nil, errors.New("link 插件源解析失败")
	}
	publisher := common.PluginPublisher{}
	if err := json.Unmarshal(data, &publisher); err != nil {
		return nil, err
	}
	if strings.TrimSpace(publisher.Address) == "" {
		return nil, errors.New("link 插件源地址为空")
	}
	listURL := publisher.Address
	if !strings.HasSuffix(listURL, "list.json") {
		listURL = strings.TrimRight(listURL, "/") + "/api/plugin-market/plugins"
	}
	req, err := http.NewRequest(http.MethodGet, listURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sillyGirl")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("link 插件源读取失败：HTTP %d", resp.StatusCode)
	}
	payload := map[string]json.RawMessage{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	result := RequestPluginResult{}
	if raw, ok := payload["status"]; ok {
		status := false
		_ = json.Unmarshal(raw, &status)
		if !status {
			return nil, errors.New("无效的 link 插件源")
		}
		if raw, ok := payload["data"]; ok {
			if err := json.Unmarshal(raw, &result); err != nil {
				return nil, err
			}
		}
	} else {
		if data, err := json.Marshal(payload); err == nil {
			if err := json.Unmarshal(data, &result); err != nil {
				return nil, err
			}
		}
		if !result.Success {
			return nil, errors.New("无效的 link 插件源")
		}
	}
	if len(result.Data) == 0 {
		return nil, errors.New("无效的 link 插件源")
	}
	for _, item := range result.Data {
		item.Address = publisher.Address
		item.Organization = publisher.Organization
		item.Identified = publisher.Identified
		item.Icon = pluginIconOrDefault(item.Icon)
	}
	sort.SliceStable(result.Data, func(i, j int) bool {
		return result.Data[i].CreateAt > result.Data[j].CreateAt
	})
	return result.Data, nil
}

type githubPluginSource struct {
	Owner  string
	Repo   string
	Branch string
}

type githubTreeResponse struct {
	Tree []githubTreeItem `json:"tree"`
}

type githubTreeItem struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type githubPublicFileIndexEntry struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Title              string   `json:"title"`
	Author             string   `json:"author"`
	Version            string   `json:"version"`
	Desc               string   `json:"desc"`
	Icon               string   `json:"icon"`
	Class              string   `json:"class"`
	Rule               string   `json:"rule"`
	Public             bool     `json:"public"`
	Admin              bool     `json:"admin"`
	Module             bool     `json:"module"`
	Cron               string   `json:"cron"`
	Status             *bool    `json:"status"`
	Path               string   `json:"path"`
	Raw                string   `json:"raw"`
	Dependencies       []string `json:"dependencies"`
	ModuleDependencies []string `json:"module_dependencies"`
	Type               string   `json:"type"`
	Origin             string   `json:"origin"`
	CreateAt           string   `json:"create_at"`
}

func normalizedPluginStatus(status *bool) *bool {
	if status == nil {
		return pluginStatusValue(true)
	}
	return pluginStatusValue(*status)
}

func parseGithubPluginSource(address string) (*githubPluginSource, error) {
	address = normalizePluginSourceAddress(address)
	if address == "" {
		return nil, errors.New("插件源地址不能为空")
	}
	if !strings.Contains(address, "://") {
		address = "https://" + address
	}
	parsed, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	if parsed.Host != "github.com" {
		return nil, errors.New("目前仅支持 github.com 仓库地址")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return nil, errors.New("GitHub 仓库地址格式错误")
	}
	repo := strings.TrimSuffix(parts[1], ".git")
	source := &githubPluginSource{Owner: parts[0], Repo: repo}
	if len(parts) >= 4 && parts[2] == "tree" {
		source.Branch = parts[3]
	}
	if source.Branch == "" {
		source.Branch = "main"
	}
	return source, nil
}

func githubPluginTree(source *githubPluginSource) ([]githubTreeItem, error) {
	api := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", source.Owner, source.Repo, url.QueryEscape(source.Branch))
	tree := githubTreeResponse{}
	err := httpGetJSON(api, 20*time.Second, &tree)
	if err == nil {
		return tree.Tree, nil
	}
	archiveTree, zipErr := githubArchiveTree(source)
	if zipErr == nil {
		return archiveTree, nil
	}
	return nil, fmt.Errorf("GitHub 插件源读取失败：%v，zip fallback：%v", err, zipErr)
}

func githubArchiveTree(source *githubPluginSource) ([]githubTreeItem, error) {
	archiveURL := fmt.Sprintf("https://codeload.github.com/%s/%s/zip/refs/heads/%s", source.Owner, source.Repo, url.PathEscape(source.Branch))
	data, err := httpGetBytesLimit(archiveURL, 60*time.Second, 128<<20)
	if err != nil {
		return nil, err
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	items := make([]githubTreeItem, 0, len(reader.File))
	for _, file := range reader.File {
		name := strings.Trim(file.Name, "/")
		if name == "" {
			continue
		}
		parts := strings.SplitN(name, "/", 2)
		if len(parts) != 2 {
			continue
		}
		itemPath := parts[1]
		if itemPath == "" {
			continue
		}
		itemType := "blob"
		if file.FileInfo().IsDir() {
			itemType = "tree"
		}
		items = append(items, githubTreeItem{
			Path: itemPath,
			Type: itemType,
		})
	}
	return items, nil
}

func githubPluginSourceItems(address string) ([]*common.Function, error) {
	source, err := parseGithubPluginSource(address)
	if err != nil {
		return nil, err
	}
	if items, err := githubPublicFileIndexItems(source); err == nil && len(items) != 0 {
		return items, nil
	}
	tree, err := githubPluginTree(source)
	if err != nil {
		return nil, err
	}
	items := []*common.Function{}
	organization := source.Owner + "/" + source.Repo
	for _, item := range tree {
		if item.Type != "blob" || !isGithubFlatNodePlugin(item.Path) {
			continue
		}
		pluginName := strings.TrimSuffix(path.Base(item.Path), path.Ext(item.Path))
		identity := pluginIdentity(source.Owner, pluginName)
		class := pluginClassFromExt(path.Ext(item.Path))
		rawURL := githubRawURL(source.Owner, source.Repo, source.Branch, item.Path)
		pluginAddress := makeGithubNodePluginAddress(source, item.Path, rawURL, class)
		dependencies := []string{}
		moduleDependencies := []string{}
		metadata := &common.Function{}
		if data, err := httpGetBytes(rawURL, 20*time.Second); err == nil {
			script := string(data)
			dependencies = parseDeclaredDependencies(script, class)
			moduleDependencies = parseDeclaredModuleDependencies(script, class)
			metadata, _ = pluginParse(script, nameUuid(identity))
		}
		classes := metadata.Classes
		if len(classes) == 0 {
			classes = []string{source.Owner}
		}
		items = append(items, &common.Function{
			UUID:               nameUuid(identity),
			Title:              firstNonEmpty(metadata.Title, pluginName),
			Type:               class,
			Suffix:             path.Ext(item.Path),
			Desc:               firstNonEmpty(metadata.Desc, item.Path),
			Rule:               strings.Join(metadata.Rules, " "),
			Icon:               pluginIconOrDefault(metadata.Icon),
			Version:            firstNonEmpty(metadata.Version, "v1.0.0"),
			Author:             firstNonEmpty(metadata.Author, source.Owner),
			Class:              strings.Join(classes, " "),
			Address:            pluginAddress,
			Classes:            classes,
			Public:             metadata.Public,
			Admin:              metadata.Admin,
			Cron:               metadata.Cron,
			Status:             normalizedPluginStatus(metadata.Status),
			Module:             metadata.Module,
			OnStart:            metadata.OnStart,
			Web:                metadata.Web,
			Dependencies:       dependencies,
			ModuleDependencies: moduleDependencies,
			PluginPublisher: common.PluginPublisher{
				Address:      pluginAddress,
				Organization: organization,
			},
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Desc < items[j].Desc
	})
	if len(items) == 0 {
		return nil, errors.New("该仓库 plugins 目录下没有找到 JS 或 Python 插件")
	}
	return items, nil
}

func githubPublicFileIndexItems(source *githubPluginSource) ([]*common.Function, error) {
	indexURL := githubRawURL(source.Owner, source.Repo, source.Branch, "publicFileIndex.json")
	data, err := httpGetBytes(indexURL, 20*time.Second)
	if err != nil {
		return nil, err
	}
	records, err := parseGithubPublicFileIndex(data)
	if err != nil {
		return nil, err
	}
	items := make([]*common.Function, 0, len(records))
	organization := source.Owner + "/" + source.Repo
	for _, record := range records {
		if !isGithubFlatNodePlugin(record.Path) {
			continue
		}
		pluginPath := strings.TrimSpace(record.Path)
		pluginName := strings.TrimSuffix(path.Base(pluginPath), path.Ext(pluginPath))
		identity := pluginIdentity(source.Owner, pluginName)
		class := pluginClassFromIndexType(record.Type, pluginPath)
		title := record.Title
		if title == "" {
			title = record.Name
		}
		if title == "" {
			title = pluginName
		}
		id := nameUuid(identity)
		classes := []string{}
		for _, item := range strings.FieldsFunc(record.Class, func(r rune) bool {
			return r == ',' || r == '，' || r == ' ' || r == '\t' || r == '\n'
		}) {
			if item != "" {
				classes = append(classes, item)
			}
		}
		if len(classes) == 0 && record.Author != "" {
			classes = append(classes, record.Author)
		}
		if len(classes) == 0 {
			classes = append(classes, source.Owner)
		}
		rawURL := strings.TrimSpace(record.Raw)
		if rawURL == "" {
			rawURL = githubRawURL(source.Owner, source.Repo, source.Branch, pluginPath)
		}
		pluginAddress := makeGithubNodePluginAddress(source, pluginPath, rawURL, class)
		cron := map[string]string{}
		if schedule := parseCronMetaValue(record.Cron); schedule != "" {
			cron["task"] = schedule
		}
		items = append(items, &common.Function{
			UUID:               id,
			Title:              title,
			Type:               class,
			Suffix:             path.Ext(pluginPath),
			Desc:               record.Desc,
			Rule:               strings.TrimSpace(record.Rule),
			Icon:               pluginIconOrDefault(record.Icon),
			Version:            firstNonEmpty(record.Version, "v1.0.0"),
			Author:             firstNonEmpty(record.Author, source.Owner),
			Class:              strings.Join(classes, " "),
			Address:            pluginAddress,
			Classes:            classes,
			Public:             record.Public,
			Admin:              record.Admin,
			Module:             record.Module,
			Cron:               cron,
			Status:             normalizedPluginStatus(record.Status),
			Dependencies:       record.Dependencies,
			ModuleDependencies: record.ModuleDependencies,
			CreateAt:           strings.TrimSpace(record.CreateAt),
			PluginPublisher: common.PluginPublisher{
				Address:      pluginAddress,
				Organization: organization,
			},
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Title < items[j].Title
	})
	return items, nil
}

func parseGithubPublicFileIndex(data []byte) ([]githubPublicFileIndexEntry, error) {
	records := map[string]githubPublicFileIndexEntry{}
	if err := json.Unmarshal(data, &records); err == nil {
		items := make([]githubPublicFileIndexEntry, 0, len(records))
		for key, record := range records {
			record = completeGithubPublicFileIndexEntry(record, key)
			items = append(items, record)
		}
		return items, nil
	}
	items := []githubPublicFileIndexEntry{}
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	for i := range items {
		items[i] = completeGithubPublicFileIndexEntry(items[i], "")
	}
	return items, nil
}

func completeGithubPublicFileIndexEntry(record githubPublicFileIndexEntry, key string) githubPublicFileIndexEntry {
	key = strings.TrimSpace(key)
	if record.ID == "" {
		record.ID = key
	}
	if record.Path == "" && strings.HasPrefix(key, "plugins/") {
		record.Path = key
	}
	if record.Name == "" && record.Path != "" {
		record.Name = strings.TrimSuffix(path.Base(record.Path), path.Ext(record.Path))
	}
	if record.Title == "" {
		record.Title = record.Name
	}
	runtime := pluginClassFromIndexType(record.Type, record.Path)
	record.Dependencies, record.ModuleDependencies = splitDeclaredDependencyValues(
		append(append([]string{}, record.Dependencies...), record.ModuleDependencies...),
		runtime,
	)
	return record
}

func isGithubFlatNodePlugin(itemPath string) bool {
	itemPath = strings.TrimSpace(itemPath)
	ext := strings.ToLower(path.Ext(itemPath))
	if path.Dir(itemPath) != "plugins" || (ext != ".js" && ext != ".py") {
		return false
	}
	name := strings.TrimSuffix(path.Base(itemPath), path.Ext(itemPath))
	return name != "" && !strings.Contains(name, "..")
}

func pluginClassFromIndexType(typeText, itemPath string) string {
	if runtime := normalizePluginIndexType(typeText); runtime != "" {
		return runtime
	}
	return pluginClassFromExt(path.Ext(itemPath))
}

func normalizePluginIndexType(typeText string) string {
	switch strings.ToLower(strings.TrimSpace(typeText)) {
	case PYTHON, "py":
		return PYTHON
	case NODE, "nodejs", "js", "javascript":
		return NODE
	default:
		return ""
	}
}

func githubRawURL(owner, repo, branch, itemPath string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/refs/heads/%s/%s", owner, repo, url.PathEscape(branch), itemPath)
}

func makeGithubNodePluginAddress(source *githubPluginSource, pluginPath string, rawValues ...string) string {
	values := url.Values{}
	values.Set("branch", source.Branch)
	values.Set("path", pluginPath)
	if len(rawValues) != 0 && strings.TrimSpace(rawValues[0]) != "" {
		values.Set("raw", strings.TrimSpace(rawValues[0]))
	}
	if len(rawValues) > 1 && strings.TrimSpace(rawValues[1]) != "" {
		values.Set("type", normalizePluginIndexType(rawValues[1]))
	}
	return fmt.Sprintf("%s://%s/%s?%s", githubNodePluginScheme, source.Owner, source.Repo, values.Encode())
}

func parseGithubNodePluginAddress(address string) (*githubPluginSource, string, string, string, error) {
	parsed, err := url.Parse(address)
	if err != nil {
		return nil, "", "", "", err
	}
	if parsed.Scheme != githubNodePluginScheme {
		return nil, "", "", "", errors.New("不是 GitHub Node 插件地址")
	}
	pluginPath := strings.Trim(parsed.Query().Get("path"), "/")
	if !isGithubFlatNodePlugin(pluginPath) || strings.Contains(pluginPath, "..") {
		return nil, "", "", "", errors.New("GitHub Node 插件路径不合法")
	}
	source := &githubPluginSource{
		Owner:  parsed.Host,
		Repo:   strings.Trim(strings.TrimPrefix(parsed.Path, "/"), "/"),
		Branch: parsed.Query().Get("branch"),
	}
	if source.Owner == "" || source.Repo == "" || source.Branch == "" {
		return nil, "", "", "", errors.New("GitHub Node 插件地址不完整")
	}
	rawURL := strings.TrimSpace(parsed.Query().Get("raw"))
	if rawURL != "" && !isSafeGithubRawURL(rawURL) {
		return nil, "", "", "", errors.New("GitHub Node 插件 raw 地址不合法")
	}
	class := pluginClassFromIndexType(parsed.Query().Get("type"), pluginPath)
	return source, pluginPath, rawURL, class, nil
}

func installGithubNodePlugin(address string) error {
	pluginLock.Lock()
	defer pluginLock.Unlock()

	source, pluginPath, rawURL, class, err := parseGithubNodePluginAddress(address)
	if err != nil {
		return err
	}

	pluginName := strings.TrimSuffix(path.Base(pluginPath), path.Ext(pluginPath))
	root := nodePluginsRoot()
	target := filepath.Join(root, pluginPublisherDirName(source.Owner))
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	if class == NODE {
		if err := ensureNodeSillygirlModule(root); err != nil {
			return err
		}
		if err := ensureNodePackageJSON(root, "sillygirl-plugins"); err != nil {
			return err
		}
	} else if _, err := ensurePythonSillygirlModule(); err != nil {
		return err
	}
	downloadURL := rawURL
	if downloadURL == "" {
		downloadURL = githubRawURL(source.Owner, source.Repo, source.Branch, pluginPath)
	}
	data, err := httpGetBytes(downloadURL, 30*time.Second)
	if err != nil {
		return err
	}
	fileName := filepath.Base(pluginPath)
	if strings.EqualFold(fileName, "main.js") || strings.EqualFold(fileName, "main.py") || strings.EqualFold(fileName, "demo.main.js") {
		fileName = pluginName + path.Ext(pluginPath)
	}
	if err := ensureNoPluginRuntimeNameConflict(target, fileName); err != nil {
		return err
	}
	mainFile := filepath.Join(target, fileName)
	if err := ensureChildPath(target, mainFile); err != nil {
		return err
	}
	checkedMainFile, err := checkedNodeScriptPath(mainFile)
	if err != nil {
		return err
	}
	mainFile = checkedMainFile
	previous, previousErr := os.ReadFile(mainFile)
	if err := os.WriteFile(mainFile, data, 0644); err != nil {
		return err
	}
	identity := pluginIdentity(source.Owner, pluginName)
	if err := addNodePluginLocked(strings.ReplaceAll(mainFile, "\\", "/"), identity, class); err != nil {
		if previousErr == nil {
			if restoreErr := os.WriteFile(mainFile, previous, 0644); restoreErr != nil {
				return fmt.Errorf("%w；恢复原插件源码失败：%v", err, restoreErr)
			}
			if restoreErr := addNodePluginLocked(strings.ReplaceAll(mainFile, "\\", "/"), identity, class); restoreErr != nil {
				return fmt.Errorf("%w；恢复原插件运行状态失败：%v", err, restoreErr)
			}
		} else {
			_ = os.Remove(mainFile)
		}
		return err
	}
	console.Log("已安装脚本插件 %s", pluginName)
	return nil
}

func ensureChildPath(root, child string) error {
	rel, err := filepath.Rel(root, child)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return errors.New("插件文件路径不合法")
	}
	return nil
}

func httpGetJSON(address string, timeout time.Duration, target interface{}) error {
	data, err := httpGetBytes(address, timeout)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

const maxPluginDownloadBytes int64 = 16 << 20

func httpGetBytes(address string, timeout time.Duration) ([]byte, error) {
	return httpGetBytesLimit(address, timeout, maxPluginDownloadBytes)
}

func httpGetBytesLimit(address string, timeout time.Duration, limit int64) ([]byte, error) {
	reqURL := githubAcceleratedURLFor(address)
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sillyGirl")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("下载内容超过 %d 字节限制", limit)
	}
	return data, nil
}

func githubAcceleratedURLFor(address string) string {
	parsedAddress, err := url.Parse(address)
	if err != nil || !isGithubURLHost(parsedAddress.Host) {
		return address
	}
	prefix := githubAcceleratorPrefix()
	if prefix == "" {
		return address
	}
	return strings.TrimRight(prefix, "/") + "/" + address
}

func githubAcceleratorPrefix() string {
	return strings.TrimSpace(sillyGirl.GetString(pluginSourceGithubProxyKey))
}

func normalizeGithubAcceleratorPrefix(prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", nil
	}
	prefix = strings.TrimRight(prefix, "/")
	parsed, err := url.Parse(prefix)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("GitHub 加速地址格式错误")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("GitHub 加速地址只支持 http 或 https")
	}
	return prefix, nil
}

func isGithubURLHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	parsed, err := url.Parse("//" + host)
	if err == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	}
	return host == "github.com" ||
		host == "api.github.com" ||
		host == "raw.githubusercontent.com" ||
		host == "codeload.github.com" ||
		strings.HasSuffix(host, ".githubusercontent.com")
}

func isSafeGithubRawURL(address string) bool {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	return isGithubURLHost(parsed.Host) && (ext == ".js" || ext == ".py")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func GetPublicResponse() *RequestPluginResult {
	rr := &RequestPluginResult{}
	fs := []*common.Function{}
	for _, f := range installedPluginSnapshot() {
		if f.Public {
			fs = append(fs, f)
			f.Downloads = plugin_downloads.GetInt(f.UUID)
		}
	}
	rr.Total = len(fs)
	rr.Data = fs
	rr.Page = 1
	return rr
}
