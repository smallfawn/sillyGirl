package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/utils"
)

func responseJWT(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	payload := struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}{}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode JWT response: %v; body=%s", err, response.Body.String())
	}
	if strings.Count(payload.Data.Token, ".") != 2 {
		t.Fatalf("response token is not JWT: %q; body=%s", payload.Data.Token, response.Body.String())
	}
	return payload.Data.Token
}

func TestAdminLoginRejectsMalformedJSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/sessions", strings.NewReader(`{"username":`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handleAdminLogin(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d; want=%d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	assertRESTEnvelope(t, recorder, false)
}

func requestWithAuthHeader(router http.Handler, method, target, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, nil)
	if token != "" {
		request.Header.Set("token", token)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestAuthOnlyReadsTokenRequestHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name  string
		apply func(*http.Request)
		admin string
		user  string
	}{
		{
			name: "token header",
			apply: func(request *http.Request) {
				request.Header.Set("token", " jwt-value ")
			},
			admin: "jwt-value",
			user:  "jwt-value",
		},
		{
			name: "authorization bearer is ignored",
			apply: func(request *http.Request) {
				request.Header.Set("Authorization", "Bearer legacy-value")
			},
		},
		{
			name: "cookies are ignored",
			apply: func(request *http.Request) {
				request.AddCookie(&http.Cookie{Name: "token", Value: "legacy-admin"})
				request.AddCookie(&http.Cookie{Name: "user_token", Value: "legacy-user"})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodGet, "/auth-probe", nil)
			test.apply(ctx.Request)
			if got := authTokenFromRequest(ctx); got != test.admin {
				t.Errorf("admin token=%q; want=%q", got, test.admin)
			}
			if got := userAuthTokenFromRequest(ctx); got != test.user {
				t.Errorf("user token=%q; want=%q", got, test.user)
			}
		})
	}
}

func TestAdminJWTTokenHeaderInterfaceFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalPassword := password
	originalName := sillyGirl.GetString("name")
	authsLock.Lock()
	originalAuths := auths
	auths = nil
	authsLock.Unlock()
	t.Cleanup(func() {
		password = originalPassword
		sillyGirl.Set("name", originalName)
		authsLock.Lock()
		auths = originalAuths
		authsLock.Unlock()
	})

	password = "$2a$10$header-token-test-password-hash"
	username := "header_admin_" + utils.GenUUID()
	sillyGirl.Set("name", username)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/sessions", nil)
	token, err := createAdminJWTSession(ctx, username)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(token, ".") != 2 {
		t.Fatalf("admin token is not JWT: %q", token)
	}
	if cookies := recorder.Header().Values("Set-Cookie"); len(cookies) != 0 {
		t.Fatalf("admin JWT was written to a cookie: %v", cookies)
	}

	router := newRESTContractRouter(apiRouteSnapshot()...)
	if response := requestWithAuthHeader(router, http.MethodGet, "/api/admin/sessions/current", token); response.Code != http.StatusOK {
		t.Fatalf("admin token header rejected: status=%d body=%s", response.Code, response.Body.String())
	}
	claims, err := parseAdminJWT(token)
	if err != nil {
		t.Fatal(err)
	}
	if response := requestWithAuthHeader(router, http.MethodGet, "/api/admin/sessions/current", claims.JTI); response.Code != http.StatusUnauthorized {
		t.Fatalf("raw admin session token unexpectedly authenticated: status=%d", response.Code)
	}
	if response := requestWithAuthHeader(router, http.MethodGet, "/api/user/profile", token); response.Code != http.StatusUnauthorized {
		t.Fatalf("admin JWT unexpectedly authenticated as a user: status=%d", response.Code)
	}
	legacyCookie := httptest.NewRequest(http.MethodGet, "/api/admin/sessions/current", nil)
	legacyCookie.AddCookie(&http.Cookie{Name: "token", Value: token})
	legacyResponse := httptest.NewRecorder()
	router.ServeHTTP(legacyResponse, legacyCookie)
	if legacyResponse.Code != http.StatusUnauthorized {
		t.Fatalf("admin cookie unexpectedly authenticated: status=%d", legacyResponse.Code)
	}
	legacyBearer := httptest.NewRequest(http.MethodGet, "/api/admin/sessions/current", nil)
	legacyBearer.Header.Set("Authorization", "Bearer "+token)
	legacyBearerResponse := httptest.NewRecorder()
	router.ServeHTTP(legacyBearerResponse, legacyBearer)
	if legacyBearerResponse.Code != http.StatusUnauthorized {
		t.Fatalf("admin Authorization header unexpectedly authenticated: status=%d", legacyBearerResponse.Code)
	}
	if response := requestWithAuthHeader(router, http.MethodPost, "/api/admin/sessions/current/deletions", token); response.Code != http.StatusOK {
		t.Fatalf("admin logout rejected: status=%d body=%s", response.Code, response.Body.String())
	}
	if response := requestWithAuthHeader(router, http.MethodGet, "/api/admin/sessions/current", token); response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked admin JWT remained valid: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestUserJWTTokenHeaderInterfaceFlow(t *testing.T) {
	username := "header_user_" + strings.ReplaceAll(utils.GenUUID(), "-", "")[:12]
	user, err := createNormalUser(username, "header-password", "请求头测试用户")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deleteNormalUser(username) })
	token, err := createUserJWT(user)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(token, ".") != 2 {
		t.Fatalf("user token is not JWT: %q", token)
	}

	router := newRESTContractRouter(apiRouteSnapshot()...)
	if response := requestWithAuthHeader(router, http.MethodGet, "/api/user/profile", token); response.Code != http.StatusOK {
		t.Fatalf("user token header rejected: status=%d body=%s", response.Code, response.Body.String())
	}
	originalPassword := password
	if strings.TrimSpace(password) == "" {
		password = "cross-role-test-enabled"
	}
	if response := requestWithAuthHeader(router, http.MethodGet, "/api/admin/sessions/current", token); response.Code != http.StatusUnauthorized {
		t.Fatalf("user JWT unexpectedly authenticated as an administrator: status=%d", response.Code)
	}
	password = originalPassword
	for _, test := range []struct {
		name  string
		apply func(*http.Request)
	}{
		{name: "missing token", apply: func(*http.Request) {}},
		{name: "invalid token", apply: func(request *http.Request) { request.Header.Set("token", "not-a-jwt") }},
		{name: "legacy cookie", apply: func(request *http.Request) { request.AddCookie(&http.Cookie{Name: "user_token", Value: token}) }},
		{name: "legacy bearer", apply: func(request *http.Request) { request.Header.Set("Authorization", "Bearer "+token) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/user/profile", nil)
			test.apply(request)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	if response := requestWithAuthHeader(router, http.MethodPost, "/api/user/sessions/current/deletions", token); response.Code != http.StatusOK {
		t.Fatalf("user logout rejected: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestLoginInterfacesReturnJWTWithoutCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newRESTContractRouter(apiRouteSnapshot()...)

	adminName := "login_admin_" + strings.ReplaceAll(utils.GenUUID(), "-", "")[:10]
	adminPassword := "header-login-password"
	originalName := sillyGirl.GetString("name")
	originalPassword := sillyGirl.GetString("password")
	t.Cleanup(func() {
		sillyGirl.Set("name", originalName)
		sillyGirl.Set("password", originalPassword)
	})
	sillyGirl.Set("name", adminName)
	sillyGirl.Set("password", adminPassword)
	adminBody := `{"username":"` + adminName + `","password":"` + adminPassword + `"}`
	adminRequest := httptest.NewRequest(http.MethodPost, "/api/admin/sessions", strings.NewReader(adminBody))
	adminRequest.Header.Set("Content-Type", "application/json")
	adminResponse := httptest.NewRecorder()
	router.ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusCreated {
		t.Fatalf("admin login status=%d body=%s", adminResponse.Code, adminResponse.Body.String())
	}
	responseJWT(t, adminResponse)
	if cookies := adminResponse.Header().Values("Set-Cookie"); len(cookies) != 0 {
		t.Fatalf("admin login emitted cookies: %v", cookies)
	}

	username := "login_user_" + strings.ReplaceAll(utils.GenUUID(), "-", "")[:10]
	userPassword := "header-login-password"
	if _, err := createNormalUser(username, userPassword, "JWT 登录测试"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deleteNormalUser(username) })
	userBody := `{"username":"` + username + `","password":"` + userPassword + `"}`
	userRequest := httptest.NewRequest(http.MethodPost, "/api/user/sessions", strings.NewReader(userBody))
	userRequest.Header.Set("Content-Type", "application/json")
	userResponse := httptest.NewRecorder()
	router.ServeHTTP(userResponse, userRequest)
	if userResponse.Code != http.StatusCreated {
		t.Fatalf("user login status=%d body=%s", userResponse.Code, userResponse.Body.String())
	}
	responseJWT(t, userResponse)
	if cookies := userResponse.Header().Values("Set-Cookie"); len(cookies) != 0 {
		t.Fatalf("user login emitted cookies: %v", cookies)
	}
}

func TestFrontendSendsJWTInTokenHeader(t *testing.T) {
	tests := []struct {
		path     string
		required []string
		forbid   []string
	}{
		{
			path: "../frontend/src/api.ts",
			required: []string{
				`localStorage.setItem(authTokenKey, token.trim())`,
				`headers.set("token", token)`,
			},
			forbid: []string{`credentials: "include"`},
		},
		{
			path: "../frontend/src/Home.vue",
			required: []string{
				`const userAuthTokenKey = 'sillygirl_user_jwt'`,
				`headers.set('token', token)`,
				`localStorage.setItem(userAuthTokenKey, data.token.trim())`,
			},
			forbid: []string{`credentials: 'include'`},
		},
		{
			path: "../frontend/src/User.vue",
			required: []string{
				`const userAuthTokenKey = "sillygirl_user_jwt"`,
				`headers.set("token", token)`,
			},
			forbid: []string{`credentials: "include"`},
		},
	}
	for _, test := range tests {
		raw, err := os.ReadFile(test.path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, required := range test.required {
			if !strings.Contains(text, required) {
				t.Errorf("%s lacks %q", test.path, required)
			}
		}
		for _, forbidden := range test.forbid {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s still contains %q", test.path, forbidden)
			}
		}
	}
}

func concreteAuthTestPath(pattern string) string {
	parts := strings.Split(pattern, "/")
	for index, part := range parts {
		if strings.HasPrefix(part, ":") || strings.HasPrefix(part, "*") {
			parts[index] = "fixture"
		}
	}
	return strings.Join(parts, "/")
}

func TestEveryProtectedInterfaceRejectsMissingAndMalformedHeaderJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalPassword := password
	password = "header-matrix-enabled"
	t.Cleanup(func() { password = originalPassword })
	router := newRESTContractRouter(apiRouteSnapshot()...)
	public := map[string]bool{
		"GET /api/admin/setup":     true,
		"POST /api/admin/setup":    true,
		"POST /api/admin/sessions": true,
		"POST /api/user/accounts":  true,
		"POST /api/user/sessions":  true,
	}
	checkedAdmin, checkedUser := 0, 0
	for _, route := range apiRouteSnapshot() {
		key := route.Method + " " + route.Path
		if public[key] {
			continue
		}
		kind := ""
		if strings.HasPrefix(route.Path, "/api/admin/") {
			kind = "admin"
			checkedAdmin++
		} else if strings.HasPrefix(route.Path, "/api/user/") {
			kind = "user"
			checkedUser++
		} else {
			continue
		}
		target := concreteAuthTestPath(route.Path)
		for _, token := range []string{"", "malformed.jwt"} {
			request := httptest.NewRequest(route.Method, target, strings.NewReader("{}"))
			request.Header.Set("Content-Type", "application/json")
			if token != "" {
				request.Header.Set("token", token)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Errorf("%s %s %s token status=%d body=%s", kind, route.Method, route.Path, response.Code, response.Body.String())
				continue
			}
			assertRESTEnvelope(t, response, false)
		}
	}
	if checkedAdmin < 65 || checkedUser < 16 {
		t.Fatalf("auth interface matrix too small: admin=%d user=%d", checkedAdmin, checkedUser)
	}
	t.Logf("逐项验证完成：admin=%d 个接口，user=%d 个接口；均拒绝缺失及错误的 token 请求头 JWT", checkedAdmin, checkedUser)
}

func TestJWTExpiryPolicies(t *testing.T) {
	now := time.Now().Unix()
	if jwtClaimsExpired(now, now-60, now+60, adminJWTExpireSeconds) {
		t.Fatal("valid admin JWT was treated as expired")
	}
	if jwtClaimsExpired(now, now-60, now+60, userJWTExpireSeconds) {
		t.Fatal("valid user JWT was treated as expired")
	}
	if !jwtClaimsExpired(now, now+60, now+120, userJWTExpireSeconds) {
		t.Fatal("future-issued JWT was accepted")
	}
}
