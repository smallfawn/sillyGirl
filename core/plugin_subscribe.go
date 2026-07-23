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
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/utils"
)

const pluginSourceReposKey = "plugin_source_repos"
const githubNodePluginScheme = "github-node"
const githubProxyEnv = "SILLYGIRL_GITHUB_PROXY"

var builtinGithubProxyPrefixes = []string{
	"https://ghfast.top/",
	"https://gh.llkk.cc/",
	"https://gh-proxy.com/",
	"https://gh.idayer.com/",
	"https://gh.xmly.dev/",
	"https://gh.jasonzeng.dev/",
}

type RequestPluginResult struct {
	Success bool               `json:"success"`
	Data    []*common.Function `json:"data"`
	Page    int                `json:"page"`
	Total   int                `json:"total"`
	Tab1    int                `json:"tab1"`
	Tab2    int                `json:"tab2"`
	Tab3    int                `json:"tab3"`
	All     int                `json:"all"`
	Tab     string             `json:"tab"`
	Time    time.Time          `json:"time"`
	Classes map[string]int     `json:"classes"`
	Origins map[string]string  `json:"origins"`
}

var plugin_list = []*common.Function{}

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
		return list[i].Description < list[j].Description
	})
	cyzl := "7642f5de-3300-11ed-8a79-52540066b468"
	plugin_list = list
	if sillyGirl.GetString("password") == "" && plugins.GetString(cyzl) == "" { //自动安装老版命令
		plugins.Set(cyzl, "install")
	}
	// if plugins.GetString("78b15932-334f-11ed-8b59-aaaa00117a5c") == "" { //自动安装比价文案
	// 	plugins.Set("78b15932-334f-11ed-8b59-aaaa00117a5c", "install")
	// }
}

var plugin_downloads = MakeBucket("plugin_downloads")

func initWebPluginList() {
	GinApi(GET, "/api/plugins/sources", RequireAuth, func(ctx *gin.Context) {
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data":    pluginSourceAddresses(),
		})
	})
	GinApi(POST, "/api/plugins/source", RequireAuth, func(ctx *gin.Context) {
		payload := map[string]string{}
		if err := ctx.BindJSON(&payload); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		address := normalizePluginSourceAddress(payload["address"])
		if address == "" {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": "插件源地址不能为空"})
			return
		}
		items, err := pluginSourceItems(address)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		sources := pluginSourceAddresses()
		if !Contains(sources, address) {
			sources = append(sources, address)
			savePluginSourceAddresses(sources)
		}
		plugin_list = append(plugin_list[:0], listPluginSources()...)
		ctx.JSON(200, map[string]interface{}{"success": true, "data": map[string]interface{}{"address": address, "count": len(items)}})
	})
	GinApi(DELETE, "/api/plugins/source", RequireAuth, func(ctx *gin.Context) {
		payload := map[string]string{}
		if err := ctx.BindJSON(&payload); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		address := normalizePluginSourceAddress(payload["address"])
		next := []string{}
		for _, source := range pluginSourceAddresses() {
			if source != address {
				next = append(next, source)
			}
		}
		savePluginSourceAddresses(next)
		plugin_list = append(plugin_list[:0], listPluginSources()...)
		ctx.JSON(200, map[string]interface{}{"success": true})
	})
	GinApi(GET, "/api/plugins/list.json", func(ctx *gin.Context) {
		// ctx.QueryArray()
		origins := ctx.QueryArray("origin[]")
		current := utils.Int(ctx.Query("current"))
		pageSize := utils.Int(ctx.Query("pageSize"))
		activeKey := ctx.Query("activeKey")
		init := ctx.Query("init")
		keyword := ctx.Query("keyword")
		class := ctx.Query("class")
		mclass := ctx.Query("mclass")
		rr := RequestPluginResult{
			Success: true,
		}
		if pageSize == 0 {
			pageSize = 10
		}
		if class == "" {
			class = "全部"
		}
		rr.Page = current
		rr.Data = []*common.Function{}
		if current != 0 {
			if current == 1 && init != "false" {
				initPluginList()
			}
			var list []*common.Function
			if keyword == "" {
				if len(origins) == 0 {
					list = append(list, plugin_list...)

				} else {
					for _, f := range plugin_list {
						if Contains(origins, f.Organization) {
							list = append(list, f)
						}
					}
				}
			} else {
				if len(origins) == 0 {
					for _, f := range plugin_list {
						if strings.Contains(f.Title, keyword) || strings.Contains(f.Organization, keyword) {
							list = append(list, f)
						}
					}
				} else {
					for _, f := range plugin_list {
						if strings.Contains(f.Title, keyword) || strings.Contains(f.Organization, keyword) {
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
			fc := []*common.Function{}
			fc = append(fc, Functions...)
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
				list, _ = classes[class]
			}
			rr.Classes = classesNum
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
			if activeKey == "tab1" {
				list = tab1
			} else if activeKey == "tab2" {
				list = tab2
			} else if activeKey == "tab3" {
				list = tab3
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
				ctx.JSON(200, rr)
				return
			}
			if last := (rr.Total + pageSize - 1) / pageSize; current > last {
				current = last
			}
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
			for _, f := range Functions {
				if f.Public && f.UUID != "" {
					publics = append(publics, f.UUID)
				}
			}
			for i := range rr.Data {
				rr.Data[i].HasForm = false
				rr.Data[i].Running = false
				for j := range fc {
					if rr.Data[i].UUID == fc[j].UUID {
						rr.Data[i].Messages = GetPluginMessage(rr.Data[i].UUID)
						rr.Data[i].CurrentVersion = fc[j].Version
						rr.Data[i].LatestVersion = rr.Data[i].Version
						if rr.Data[i].Version != fc[j].Version {
							rr.Data[i].Status = 1
							if rr.Data[i].UpdateContent == "" {
								rr.Data[i].UpdateContent = firstNonEmpty(rr.Data[i].Description, "发现新版本")
							}
						} else {
							rr.Data[i].Status = 2
						}
						if rr.Data[i].Status != 1 && Contains(publics, rr.Data[i].UUID) {
							rr.Data[i].Status = 6
						}
						if rr.Data[i].Icon == "" {
							rr.Data[i].Icon = "https://blog.example.com/huli.jpeg"
						}
						if fc[j].HasForm {
							rr.Data[i].HasForm = true
						}
						if fc[j].Running {
							rr.Data[i].Running = true
						}
						rr.Data[i].Debug = plugin_debug.GetString(rr.Data[i].UUID) == "b:true"
						rr.Data[i].Disable = fc[j].Disable
					}
				}
				rr.Data[i].Description = parseReply2(rr.Data[i].Description)
			}

			ctx.JSON(200, rr)
			return
		}

		ctx.JSON(200, GetPublicResponse())
	})
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
		return nil
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
		listURL = strings.TrimRight(listURL, "/") + "/api/plugins/list.json"
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
	result := RequestPluginResult{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, errors.New("无效的 link 插件源")
	}
	for _, item := range result.Data {
		item.Address = publisher.Address
		item.Organization = publisher.Organization
		item.Identified = publisher.Identified
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

type githubRepoResponse struct {
	DefaultBranch string `json:"default_branch"`
}

type githubPublicFileIndexEntry struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Title          string   `json:"title"`
	Author         string   `json:"author"`
	Version        string   `json:"version"`
	Description    string   `json:"description"`
	Classification string   `json:"classification"`
	Rule           string   `json:"rule"`
	Public         bool     `json:"public"`
	Admin          bool     `json:"admin"`
	Disable        bool     `json:"disable"`
	Path           string   `json:"path"`
	Raw            string   `json:"raw"`
	Type           string   `json:"type"`
	Origin         string   `json:"origin"`
	Dependencies   []string `json:"dependencies"`
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
		source.Branch = githubDefaultBranch(source.Owner, source.Repo)
	}
	if source.Branch == "" {
		source.Branch = "main"
	}
	return source, nil
}

func githubDefaultBranch(owner, repo string) string {
	info := githubRepoResponse{}
	if httpGetJSON(fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo), 10*time.Second, &info) != nil {
		return ""
	}
	return info.DefaultBranch
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
	data, err := httpGetBytes(archiveURL, 60*time.Second)
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
		pluginAddress := makeGithubNodePluginAddress(source, item.Path)
		dependencies := []string{}
		if data, err := httpGetBytes(githubRawURL(source.Owner, source.Repo, source.Branch, item.Path), 20*time.Second); err == nil {
			dependencies = parseNodeRequires(string(data))
		}
		items = append(items, &common.Function{
			UUID:         nameUuid(pluginName),
			Title:        pluginName,
			Type:         NODE,
			Suffix:       ".js",
			Description:  item.Path,
			Version:      "v1.0.0",
			Author:       source.Owner,
			Address:      pluginAddress,
			Classes:      []string{source.Owner},
			Dependencies: dependencies,
			PluginPublisher: common.PluginPublisher{
				Address:      pluginAddress,
				Organization: organization,
			},
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Description < items[j].Description
	})
	if len(items) == 0 {
		return nil, errors.New("该仓库 plugins 目录下没有找到 JS 插件")
	}
	return items, nil
}

func githubPublicFileIndexItems(source *githubPluginSource) ([]*common.Function, error) {
	indexURL := githubRawURL(source.Owner, source.Repo, source.Branch, "publicFileIndex.json")
	data, err := httpGetBytes(indexURL, 20*time.Second)
	if err != nil {
		return nil, err
	}
	records := map[string]githubPublicFileIndexEntry{}
	if err := json.Unmarshal(data, &records); err != nil {
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
		title := record.Title
		if title == "" {
			title = record.Name
		}
		if title == "" {
			title = pluginName
		}
		id := nameUuid(pluginName)
		classes := []string{}
		for _, item := range strings.FieldsFunc(record.Classification, func(r rune) bool {
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
		pluginAddress := makeGithubNodePluginAddress(source, pluginPath)
		dependencies := normalizeDependencyNames(record.Dependencies)
		if len(dependencies) == 0 && strings.TrimSpace(record.Raw) != "" {
			if data, err := httpGetBytes(record.Raw, 20*time.Second); err == nil {
				dependencies = parseNodeRequires(string(data))
			}
		}
		items = append(items, &common.Function{
			UUID:         id,
			Title:        title,
			Type:         NODE,
			Suffix:       ".js",
			Description:  record.Description,
			Version:      firstNonEmpty(record.Version, "v1.0.0"),
			Author:       firstNonEmpty(record.Author, source.Owner),
			Address:      pluginAddress,
			Classes:      classes,
			Public:       record.Public,
			Disable:      record.Disable,
			Dependencies: dependencies,
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

func isGithubFlatNodePlugin(itemPath string) bool {
	itemPath = strings.TrimSpace(itemPath)
	if path.Dir(itemPath) != "plugins" || !strings.HasSuffix(strings.ToLower(itemPath), ".js") {
		return false
	}
	name := strings.TrimSuffix(path.Base(itemPath), path.Ext(itemPath))
	return name != "" && !strings.Contains(name, "..")
}

func githubRawURL(owner, repo, branch, itemPath string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, url.PathEscape(branch), itemPath)
}

func makeGithubNodePluginAddress(source *githubPluginSource, pluginPath string) string {
	values := url.Values{}
	values.Set("branch", source.Branch)
	values.Set("path", pluginPath)
	return fmt.Sprintf("%s://%s/%s?%s", githubNodePluginScheme, source.Owner, source.Repo, values.Encode())
}

func parseGithubNodePluginAddress(address string) (*githubPluginSource, string, error) {
	parsed, err := url.Parse(address)
	if err != nil {
		return nil, "", err
	}
	if parsed.Scheme != githubNodePluginScheme {
		return nil, "", errors.New("不是 GitHub Node 插件地址")
	}
	pluginPath := strings.Trim(parsed.Query().Get("path"), "/")
	if !isGithubFlatNodePlugin(pluginPath) || strings.Contains(pluginPath, "..") {
		return nil, "", errors.New("GitHub Node 插件路径不合法")
	}
	source := &githubPluginSource{
		Owner:  parsed.Host,
		Repo:   strings.Trim(strings.TrimPrefix(parsed.Path, "/"), "/"),
		Branch: parsed.Query().Get("branch"),
	}
	if source.Owner == "" || source.Repo == "" || source.Branch == "" {
		return nil, "", errors.New("GitHub Node 插件地址不完整")
	}
	return source, pluginPath, nil
}

func installGithubNodePlugin(address string) error {
	source, pluginPath, err := parseGithubNodePluginAddress(address)
	if err != nil {
		return err
	}

	pluginName := strings.TrimSuffix(path.Base(pluginPath), path.Ext(pluginPath))
	target := filepath.Join(utils.ExecPath, "plugins", pluginName)
	target, err = checkedNodePluginDir(target)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	data, err := httpGetBytes(githubRawURL(source.Owner, source.Repo, source.Branch, pluginPath), 30*time.Second)
	if err != nil {
		return err
	}
	mainFile := filepath.Join(target, "main.js")
	if err := ensureChildPath(target, mainFile); err != nil {
		return err
	}
	if err := os.WriteFile(mainFile, data, 0644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(target, "node_modules"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "node_modules", "sillygirl.d.ts"), []byte(typeat), 0644); err != nil {
		return err
	}
	if err := ensureNodePackageJSON(target, pluginName); err != nil {
		return err
	}
	return AddNodePlugin(strings.ReplaceAll(mainFile, "\\", "/"), pluginName, NODE)
}

func ensureChildPath(root, child string) error {
	rel, err := filepath.Rel(root, child)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return errors.New("插件文件路径不合法")
	}
	return nil
}

func httpGetString(address string, timeout time.Duration) (string, error) {
	data, err := httpGetBytes(address, timeout)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func httpGetJSON(address string, timeout time.Duration, target interface{}) error {
	data, err := httpGetBytes(address, timeout)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func httpGetBytes(address string, timeout time.Duration) ([]byte, error) {
	var lastErr error
	candidates := githubAcceleratedURLs(address)
	for index, candidate := range candidates {
		req, err := http.NewRequest(http.MethodGet, candidate, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "sillyGirl")
		requestTimeout := timeout
		if index > 0 {
			requestTimeout = githubProxyTimeout(timeout)
		}
		resp, err := (&http.Client{Timeout: requestTimeout}).Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, readErr := func() ([]byte, error) {
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
			}
			return io.ReadAll(resp.Body)
		}()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		return data, nil
	}
	return nil, lastErr
}

func githubProxyTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 || timeout > 3*time.Second {
		return 3 * time.Second
	}
	return timeout
}

func githubAcceleratedURLs(address string) []string {
	address = strings.TrimSpace(address)
	urls := []string{address}
	parsed, err := url.Parse(address)
	if err != nil || !isGithubURLHost(parsed.Host) {
		return urls
	}
	for _, prefix := range githubProxyPrefixes() {
		candidate := strings.TrimRight(prefix, "/") + "/" + address
		if !Contains(urls, candidate) {
			urls = append(urls, candidate)
		}
	}
	return urls
}

func githubProxyPrefixes() []string {
	prefixes := []string{}
	for _, raw := range strings.FieldsFunc(os.Getenv(githubProxyEnv), func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	}) {
		if prefix := normalizeGithubProxyPrefix(raw); prefix != "" && !Contains(prefixes, prefix) {
			prefixes = append(prefixes, prefix)
		}
	}
	for _, raw := range builtinGithubProxyPrefixes {
		if prefix := normalizeGithubProxyPrefix(raw); prefix != "" && !Contains(prefixes, prefix) {
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes
}

func normalizeGithubProxyPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}
	if !strings.Contains(prefix, "://") {
		prefix = "https://" + prefix
	}
	return strings.TrimRight(prefix, "/") + "/"
}

func isGithubURLHost(host string) bool {
	host = strings.ToLower(strings.Split(host, ":")[0])
	switch host {
	case "github.com", "api.github.com", "raw.githubusercontent.com", "codeload.github.com":
		return true
	default:
		return false
	}
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
	rr := &RequestPluginResult{
		Success: true,
	}
	fs := []*common.Function{}
	for _, f := range Functions {
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
