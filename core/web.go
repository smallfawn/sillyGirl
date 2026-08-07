package core

import (
	"archive/zip"
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/logs"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
)

//go:embed all:admin
var static embed.FS

var Handle = make(map[string]func(c *gin.Context))

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if origin := allowedCORSOrigin(c); origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Headers", "Content-Type,AccessToken,X-CSRF-Token, Authorization, Token")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		// 处理请求
		c.Next()
	}
}

func allowedCORSOrigin(c *gin.Context) string {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" {
		return ""
	}
	configured := strings.TrimSpace(firstNonEmpty(os.Getenv("SILLYGIRL_CORS_ORIGINS"), sillyGirl.GetString("cors_origins")))
	if configured != "" {
		for _, item := range strings.Split(configured, ",") {
			if strings.TrimSpace(item) == origin {
				return origin
			}
		}
		return ""
	}
	if strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://[::1]:") {
		return origin
	}
	return ""
}

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}

func trustedHTTPProxies() []string {
	raw := strings.TrimSpace(firstNonEmpty(os.Getenv("SILLYGIRL_TRUSTED_PROXIES"), sillyGirl.GetString("trusted_proxies")))
	if raw == "" || strings.EqualFold(raw, "none") {
		return nil
	}
	result := []string{}
	for _, value := range strings.Split(raw, ",") {
		if value = strings.TrimSpace(value); value != "" && !Contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}

func configureTrustedHTTPProxies(engine *gin.Engine) error {
	if err := engine.SetTrustedProxies(trustedHTTPProxies()); err != nil {
		_ = engine.SetTrustedProxies(nil)
		return err
	}
	return nil
}

var Server *gin.Engine

func httpServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           Server,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
}

func initWeb() {
	for _, arg := range os.Args { //处理升级
		if arg == "-r" { //准备程序->原程序
			rfix := ".ready.exe"
			ofix := ".exe"
			if strings.Contains(os.Args[0], rfix) {
				err := utils.CopyFile(utils.ProcessName, strings.Replace(utils.ProcessName, rfix, ofix, -1))
				if err == nil {
					utils.Daemon("reset")
				}
			} else {
				os.Remove(strings.ReplaceAll(os.Args[0], ofix, rfix))
			}
			continue
		}
	}
	gin.SetMode(gin.ReleaseMode)
	Server = gin.New()
	Server.HandleMethodNotAllowed = true
	if err := configureTrustedHTTPProxies(Server); err != nil {
		logs.Warn("可信代理配置无效，已忽略：%v", err)
	}
	Server.Use(Cors())
	Server.Use(SecurityHeaders())
	Server.Use(gzip.Gzip(gzip.DefaultCompression))
	registerAPIRequests(Server, apiRouteSnapshot())
	Server.GET("/api/files/*filename", FindFile)
	Server.GET("/api/binary-content/:random", Base642Binary)

	Server.GET("/api/plugin-downloads/:uuid", func(c *gin.Context) {
		uuid := c.Param("uuid")
		for _, f := range Functions {
			if f.UUID == uuid && f.Public {
				plugin_downloads.Set(f.UUID, plugin_downloads.GetInt(f.UUID)+1)
				if !isNameUuid(f.UUID) {
					c.String(200, publicScript(plugins.GetString(f.UUID)))
					return
				} else {
					dir := filepath.Dir(f.Path)
					if _, err := os.Stat(dir); err != nil { //执行压缩
						ApiNotFound(c, "插件包不存在")
						return
					}
					dir = strings.ReplaceAll(dir, "\\", "/")
					ss := strings.Split(dir, "/")
					name := ss[len(ss)-1]
					buf := new(bytes.Buffer)
					w := zip.NewWriter(buf)
					err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}

						if info.Mode()&os.ModeSymlink != 0 {
							if info.IsDir() {
								return filepath.SkipDir
							}
							return nil
						}

						if info.IsDir() && info.Name() == "node_modules" {
							return filepath.SkipDir
						}

						if info.IsDir() {
							return nil
						}

						// 将路径转换为相对路径
						relPath, err := filepath.Rel(dir, path)
						if err != nil || strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
							return fmt.Errorf("插件文件路径不合法：%s", path)
						}
						is_index := relPath == "main.js"

						relPath = name + "/" + relPath

						file, err := os.Open(path)
						if err != nil {
							return err
						}
						defer file.Close()

						fh, err := zip.FileInfoHeader(info)
						if err != nil {
							return err
						}

						// 使用相对路径作为文件名
						fh.Name = relPath

						wr, err := w.CreateHeader(fh)
						if err != nil {
							return err
						}
						if is_index {
							var data []byte
							data, err = io.ReadAll(file)
							if err != nil {
								return err
							}
							su := &ScriptUtils{
								script: string(data),
							}
							if su.GetValue("public") == "true" {
								su.SetValue("public", "false")
							}
							_, err = wr.Write([]byte(su.script))
						} else {
							_, err = io.Copy(wr, file)
						}
						return err
					})
					if err != nil {
						ApiInternalError(c, fmt.Sprintf("ZIP creation failed: %s", err))
						return
					}
					err = w.Close()
					if err != nil {
						ApiInternalError(c, fmt.Sprintf("ZIP creation failed: %s", err))
						return
					}
					c.Data(http.StatusOK, "application/zip", buf.Bytes())
					return
				}
			}
		}
		ApiNotFound(c, "公开插件不存在")
	})
	Server.NoMethod(func(c *gin.Context) {
		methods := allowedHTTPMethods(Server.Routes(), c.Request.URL.Path)
		if len(methods) != 0 {
			c.Header("Allow", strings.Join(methods, ", "))
		}
		ApiError(c, http.StatusMethodNotAllowed, "请求方法不受支持")
	})
	Server.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodGet && c.Request.URL.Path == "/" {
			serveHome(c)
			return
		}
		if c.Request.Method == http.MethodGet && c.Request.URL.Path == "/user" {
			serveUser(c)
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			if serveEmbeddedFile(c, "admin"+c.Request.URL.Path) {
				return
			}
		}
		if c.Request.Method == http.MethodGet && strings.HasPrefix(c.Request.URL.Path, "/admin") {
			if serveEmbeddedFile(c, strings.Trim(c.Request.URL.Path, "/")) {
				return
			}
			if serveAdminSPA(c) {
				return
			}
		}
		matchedPath := false
		allowed := []string{}
		for _, req := range apiRouteSnapshot() {
			params, matched := matchHTTPRoutePath(req.Path, c.Request.URL.Path)
			if !matched {
				continue
			}
			matchedPath = true
			if req.Method == c.Request.Method || req.Method == ANY {
				c.Params = params
				req.Handle(c)
				return
			}
			allowed = append(allowed, req.Method)
		}
		if matchedPath {
			allowed = append(allowed, http.MethodOptions)
			allowed = uniqueSortedHTTPMethods(allowed)
			if len(allowed) != 0 {
				c.Header("Allow", strings.Join(allowed, ", "))
			}
			ApiError(c, http.StatusMethodNotAllowed, "请求方法不受支持")
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			ApiError(c, http.StatusNotFound, "资源不存在")
			return
		}
		c.String(http.StatusNotFound, "页面被喵咪劫走了")
	})

	port, normalizedPort := canonicalHTTPPortValue(sillyGirl.GetString("port"))
	if port == 8080 && strings.TrimSpace(sillyGirl.GetString("port")) == "" {
		sillyGirl.Set("port", 8080)
	} else if stored := strings.TrimSpace(sillyGirl.GetString("port")); stored != normalizedPort && stored != fmt.Sprintf("d:%d", port) {
		sillyGirl.Set("port", port)
	}
	srvs := []*http.Server{httpServer(":" + normalizedPort)}

	storage.Watch(sillyGirl, "port", func(old, new, key string) *storage.Final {
		port, normalized := canonicalHTTPPortValue(new)
		if normalizeHTTPPort(old) == port {
			if strings.TrimSpace(new) != normalized {
				return &storage.Final{Now: normalized}
			}
			return nil
		}
		srv := httpServer(":" + normalized)
		var ch = make(chan error, 1)
		srvs = append(srvs, srv)

		go func() {
			logs.Info("Http服务(%v)重新运行", normalized)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logs.Error("Http服务(%v)运行失败：%s", normalized, err.Error())
				ch <- err
			}
		}()
		select {
		case err := <-ch:
			srvs = srvs[:len(srvs)-1]
			return &storage.Final{
				Error: err,
			}
		case <-time.After(1 * time.Millisecond * 100):
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srvs[0].Shutdown(ctx); err == nil {
				logs.Info("Http服务(%v)关闭", normalizeHTTPPort(old))
			}
			srvs = srvs[1:]
		}
		return &storage.Final{
			Now: normalized,
		}
	})

	// logs.Info("Http服务(%s)开始运行", port)

	logs.Info("管理员面板:")
	logs.Info("  > 本机: http://localhost:%d/admin", port)
	local_ip := getLocalIP()
	logs.Info("  > 局域网: http://%v:%d/admin", local_ip, port)
	ip := sillyGirl.GetString("ip")
	if ip != "" {
		logs.Info("  > 广域网: http://%v:%d/admin", ip, port)
	}
	sillyGirl.Set("local_ip", local_ip)
	go func() {
		if err := srvs[0].ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logs.Error("Http服务运行失败：%s", err.Error())
		}
	}()
}

func serveAdminSPA(c *gin.Context) bool {
	data, err := static.ReadFile("admin/index.html")
	if err != nil {
		return false
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(data)
	return true
}

func serveHome(c *gin.Context) {
	data, err := static.ReadFile("admin/home.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "首页资源不存在")
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)
	c.Writer.Write(data)
}

func serveUser(c *gin.Context) {
	data, err := static.ReadFile("admin/user.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "用户中心资源不存在")
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)
	c.Writer.Write(data)
}

func serveEmbeddedFile(c *gin.Context, name string) bool {
	name, err := safeEmbeddedFileName(name)
	if err != nil {
		return false
	}
	file, err := static.Open(name)
	if err != nil {
		return false
	}
	defer file.Close()
	fs, _ := file.Stat()
	if fs.IsDir() {
		return false
	}
	c.Header("cache-control", "max-age=864000")
	if contentType := mime.TypeByExtension(filepath.Ext(name)); contentType != "" {
		c.Header("Content-Type", contentType)
	}
	c.Status(http.StatusOK)
	io.Copy(c.Writer, file)
	return true
}

func safeEmbeddedFileName(name string) (string, error) {
	normalized := strings.Trim(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"), "/")
	if normalized == "" || strings.Contains(normalized, "\x00") || strings.Contains(normalized, ":") {
		return "", fmt.Errorf("静态资源路径不合法")
	}
	if strings.HasPrefix(normalized, "/") {
		return "", fmt.Errorf("静态资源路径不合法")
	}
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return "", fmt.Errorf("静态资源路径不合法")
		}
	}
	clean := path.Clean(normalized)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("静态资源路径不合法")
	}
	return clean, nil
}

func normalizeHTTPPort(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 8080
	}
	if strings.HasPrefix(value, "d:") || strings.HasPrefix(value, "f:") {
		value = strings.TrimSpace(value[2:])
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		port := int(f)
		if port >= 1 && port <= 65535 {
			return port
		}
	}
	return 8080
}

func canonicalHTTPPortValue(value string) (int, string) {
	port := normalizeHTTPPort(value)
	return port, fmt.Sprint(port)
}

type Req struct {
	Method string
	Path   string
	Handle func(c *gin.Context)
}

var (
	ss      = []Req{}
	ssMutex sync.RWMutex
)

type Auth struct {
	ID        int
	IP        string
	UserAgent string
	Token     string
	CreatedAt int
	ExpiredAt int
}

const (
	GET  = "GET"
	POST = "POST"
	ANY  = "ANY"
)

func GinApi(method string, path string, fs ...func(c *gin.Context)) {
	req := Req{
		Method: method,
		Path:   path,
		Handle: func(c *gin.Context) {
			defer func() {
				if err := recover(); err != nil {
					logs.Error("API handler panic %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
					if !c.Writer.Written() {
						ApiInternalError(c, "服务器内部错误")
					}
				}
			}()
			for _, f := range fs {
				f(c)
				if c.IsAborted() {
					return
				}
			}
		},
	}
	ssMutex.Lock()
	ss = append(ss, req)
	ssMutex.Unlock()
}

func apiRouteSnapshot() []Req {
	ssMutex.RLock()
	defer ssMutex.RUnlock()
	return append([]Req(nil), ss...)
}

func registerAPIRequests(router *gin.Engine, requests []Req) {
	for _, req := range requests {
		if req.Method == ANY {
			router.Any(req.Path, req.Handle)
			continue
		}
		router.Handle(req.Method, req.Path, req.Handle)
	}
}

func matchHTTPRoutePath(pattern, actual string) (gin.Params, bool) {
	patternSegments := strings.Split(strings.Trim(pattern, "/"), "/")
	actualSegments := strings.Split(strings.Trim(actual, "/"), "/")
	params := gin.Params{}
	for index, segment := range patternSegments {
		if strings.HasPrefix(segment, "*") {
			if index != len(patternSegments)-1 || index >= len(actualSegments) {
				return nil, false
			}
			params = append(params, gin.Param{Key: strings.TrimPrefix(segment, "*"), Value: "/" + strings.Join(actualSegments[index:], "/")})
			return params, true
		}
		if index >= len(actualSegments) {
			return nil, false
		}
		if strings.HasPrefix(segment, ":") {
			if actualSegments[index] == "" {
				return nil, false
			}
			params = append(params, gin.Param{Key: strings.TrimPrefix(segment, ":"), Value: actualSegments[index]})
			continue
		}
		if segment != actualSegments[index] {
			return nil, false
		}
	}
	return params, len(patternSegments) == len(actualSegments)
}

func allowedHTTPMethods(routes gin.RoutesInfo, path string) []string {
	methods := []string{}
	for _, route := range routes {
		if _, matched := matchHTTPRoutePath(route.Path, path); matched {
			methods = append(methods, route.Method)
		}
	}
	if len(methods) > 0 {
		methods = append(methods, http.MethodOptions)
	}
	return uniqueSortedHTTPMethods(methods)
}

func uniqueSortedHTTPMethods(methods []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(methods))
	for _, method := range methods {
		method = strings.TrimSpace(method)
		if method == "" || seen[method] || method == ANY {
			continue
		}
		seen[method] = true
		result = append(result, method)
	}
	sort.Strings(result)
	return result
}

func getLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip.To4() != nil {
				return ip.String()
			}
		}
	}
	return "127.0.0.1"
}
