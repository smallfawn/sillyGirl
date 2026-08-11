package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newRESTContractRouter(requests ...Req) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.HandleMethodNotAllowed = true
	registerAPIRequests(router, requests)
	router.NoMethod(func(ctx *gin.Context) {
		methods := allowedHTTPMethods(router.Routes(), ctx.Request.URL.Path)
		if len(methods) > 0 {
			ctx.Header("Allow", strings.Join(methods, ", "))
		}
		ApiError(ctx, http.StatusMethodNotAllowed, "请求方法不受支持")
	})
	router.NoRoute(func(ctx *gin.Context) {
		ApiNotFound(ctx, "资源不存在")
	})
	return router
}

func performRESTRequest(router http.Handler, method, target string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder
}

func decodeRESTBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v; body=%q", err, recorder.Body.String())
	}
	return body
}

func assertRESTEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus bool) map[string]interface{} {
	t.Helper()
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content type=%q; body=%s", got, recorder.Body.String())
	}
	body := decodeRESTBody(t, recorder)
	if len(body) != 3 {
		t.Fatalf("envelope fields=%v; want exactly status, message, data", body)
	}
	for _, key := range []string{"status", "message", "data"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("envelope is missing %q: %v", key, body)
		}
	}
	status, ok := body["status"].(bool)
	if !ok || status != wantStatus {
		t.Fatalf("envelope status=%#v; want boolean %t", body["status"], wantStatus)
	}
	if _, ok := body["message"].(string); !ok {
		t.Fatalf("envelope message=%#v; want string", body["message"])
	}
	return body
}

func TestRegisteredDynamicRoutesResolveParameters(t *testing.T) {
	router := newRESTContractRouter(
		Req{Method: GET, Path: "/api/widgets/:id", Handle: func(ctx *gin.Context) {
			ApiOK(ctx, gin.H{"id": ctx.Param("id")})
		}},
		Req{Method: GET, Path: "/api/files/*path", Handle: func(ctx *gin.Context) {
			ApiOK(ctx, gin.H{"path": ctx.Param("path")})
		}},
	)

	response := performRESTRequest(router, http.MethodGet, "/api/widgets/widget-7")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"id":"widget-7"`) {
		t.Fatalf("dynamic route mismatch: status=%d body=%s", response.Code, response.Body.String())
	}
	wildcard := performRESTRequest(router, http.MethodGet, "/api/files/a/b.js")
	if wildcard.Code != http.StatusOK || !strings.Contains(wildcard.Body.String(), `"path":"/a/b.js"`) {
		t.Fatalf("wildcard route mismatch: status=%d body=%s", wildcard.Code, wildcard.Body.String())
	}
}

func TestProductionDynamicRouteIsHandledInsteadOfFallingThrough(t *testing.T) {
	router := newRESTContractRouter(apiRouteSnapshot()...)
	response := performRESTRequest(router, http.MethodPost, "/api/admin/tasks/task-contract-probe")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("production dynamic route fell through: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRESTMethodAndNotFoundContracts(t *testing.T) {
	router := newRESTContractRouter(Req{Method: GET, Path: "/api/widgets/:id", Handle: func(ctx *gin.Context) {
		ApiOK(ctx, nil)
	}})

	methodResponse := performRESTRequest(router, http.MethodPost, "/api/widgets/widget-7")
	if methodResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method status=%d body=%s", methodResponse.Code, methodResponse.Body.String())
	}
	if methodResponse.Header().Get("Allow") != http.MethodGet+", "+http.MethodOptions {
		t.Fatalf("wrong Allow header: %q", methodResponse.Header().Get("Allow"))
	}
	problem := assertRESTEnvelope(t, methodResponse, false)
	if problem["message"] != "请求方法不受支持" || problem["data"] != nil {
		t.Fatalf("wrong method error envelope: %#v", problem)
	}

	notFound := performRESTRequest(router, http.MethodGet, "/api/missing")
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("not found status=%d body=%s", notFound.Code, notFound.Body.String())
	}
}

func TestRESTResponseStatusContracts(t *testing.T) {
	router := gin.New()
	router.POST("/api/widgets", func(ctx *gin.Context) {
		ApiCreated(ctx, "/api/widgets/widget-7", gin.H{"id": "widget-7"})
	})
	router.POST("/api/jobs", func(ctx *gin.Context) {
		ApiAccepted(ctx, "/api/jobs/job-3", gin.H{"id": "job-3"})
	})
	router.POST("/api/widgets/:id/deletions", func(ctx *gin.Context) {
		ApiOK(ctx, nil)
	})
	router.GET("/api/failure", func(ctx *gin.Context) {
		ApiFail(ctx, "参数错误")
	})

	created := performRESTRequest(router, http.MethodPost, "/api/widgets")
	if created.Code != http.StatusCreated || created.Header().Get("Location") != "/api/widgets/widget-7" {
		t.Fatalf("created contract mismatch: status=%d location=%q", created.Code, created.Header().Get("Location"))
	}
	assertRESTEnvelope(t, created, true)
	accepted := performRESTRequest(router, http.MethodPost, "/api/jobs")
	if accepted.Code != http.StatusAccepted || accepted.Header().Get("Location") != "/api/jobs/job-3" {
		t.Fatalf("accepted contract mismatch: status=%d location=%q", accepted.Code, accepted.Header().Get("Location"))
	}
	assertRESTEnvelope(t, accepted, true)
	deleted := performRESTRequest(router, http.MethodPost, "/api/widgets/widget-7/deletions")
	if deleted.Code != http.StatusOK {
		t.Fatalf("empty success contract mismatch: status=%d body=%q", deleted.Code, deleted.Body.String())
	}
	if envelope := assertRESTEnvelope(t, deleted, true); envelope["data"] != nil {
		t.Fatalf("empty success data=%#v; want null", envelope["data"])
	}
	failure := performRESTRequest(router, http.MethodGet, "/api/failure")
	if failure.Code != http.StatusBadRequest {
		t.Fatalf("failure contract mismatch: status=%d body=%s", failure.Code, failure.Body.String())
	}
	assertRESTEnvelope(t, failure, false)
}

func TestRESTErrorStatusHelpers(t *testing.T) {
	tests := []struct {
		name   string
		status int
		write  func(*gin.Context)
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, write: func(ctx *gin.Context) { ApiUnauthorized(ctx, "unauthorized") }},
		{name: "forbidden", status: http.StatusForbidden, write: func(ctx *gin.Context) { ApiForbidden(ctx, "forbidden") }},
		{name: "not-found", status: http.StatusNotFound, write: func(ctx *gin.Context) { ApiNotFound(ctx, "not found") }},
		{name: "conflict", status: http.StatusConflict, write: func(ctx *gin.Context) { ApiConflict(ctx, "conflict") }},
		{name: "unprocessable", status: http.StatusUnprocessableEntity, write: func(ctx *gin.Context) { ApiUnprocessable(ctx, "unprocessable") }},
		{name: "internal", status: http.StatusInternalServerError, write: func(ctx *gin.Context) { ApiInternalError(ctx, "internal") }},
		{name: "bad-gateway", status: http.StatusBadGateway, write: func(ctx *gin.Context) { ApiBadGateway(ctx, "bad gateway") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/problem-probe", nil)
			test.write(ctx)
			if recorder.Code != test.status {
				t.Fatalf("status=%d; want=%d; body=%s", recorder.Code, test.status, recorder.Body.String())
			}
			envelope := assertRESTEnvelope(t, recorder, false)
			if envelope["data"] != nil {
				t.Fatalf("error data=%#v; want null", envelope["data"])
			}
		})
	}
}

func TestRESTValidationErrorStoresExtensionsInData(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/widgets", nil)
	ApiValidationError(ctx, "字段校验失败", map[string]string{"name": "不能为空"})

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d; body=%s", recorder.Code, recorder.Body.String())
	}
	envelope := assertRESTEnvelope(t, recorder, false)
	data, ok := envelope["data"].(map[string]interface{})
	if !ok || data["errors"] == nil {
		t.Fatalf("validation error data=%#v", envelope["data"])
	}
}

func TestCORSAdvertisesOnlyGETAndPOSTForAPIRequests(t *testing.T) {
	router := gin.New()
	router.Use(Cors())
	router.POST("/api/settings", func(ctx *gin.Context) { ApiOK(ctx, nil) })

	request := httptest.NewRequest(http.MethodOptions, "/api/settings", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("preflight status=%d", recorder.Code)
	}
	methods := recorder.Header().Get("Access-Control-Allow-Methods")
	if methods != "GET, POST, OPTIONS" {
		t.Fatalf("unexpected CORS methods: %q", methods)
	}
}

func TestCORSRejectsUntrustedOrigins(t *testing.T) {
	router := gin.New()
	router.Use(Cors())
	router.GET("/api/settings", func(ctx *gin.Context) { ApiOK(ctx, nil) })

	request := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	request.Header.Set("Origin", "https://untrusted.invalid")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if origin := recorder.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("untrusted origin was reflected: %q", origin)
	}
}

func TestSecurityHeadersAreApplied(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
	recorder := performRESTRequest(router, http.MethodGet, "/")
	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "SAMEORIGIN",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
	}
	for name, value := range want {
		if got := recorder.Header().Get(name); got != value {
			t.Errorf("%s=%q; want %q", name, got, value)
		}
	}
}

func TestRemovedUnusedAPIRoutesReturnNotFound(t *testing.T) {
	router := newRESTContractRouter(apiRouteSnapshot()...)
	for _, route := range []string{
		"/api/admin/carry-group-names",
		"/api/admin/carry-scripts",
		"/api/admin/carry-rules",
		"/api/admin/nickname-labels",
	} {
		response := performRESTRequest(router, http.MethodGet, route)
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s status=%d; want=%d; body=%s", route, response.Code, http.StatusNotFound, response.Body.String())
		}
	}
}

func TestAPIHandlerPanicDoesNotLeakInternalDetails(t *testing.T) {
	ssMutex.Lock()
	original := append([]Req(nil), ss...)
	ssMutex.Unlock()
	defer func() {
		ssMutex.Lock()
		ss = original
		ssMutex.Unlock()
	}()

	const sensitive = "database-password=panic-secret"
	GinApi(GET, "/api/panic-probes/current", func(*gin.Context) { panic(sensitive) })
	routes := apiRouteSnapshot()
	router := newRESTContractRouter(routes[len(routes)-1])
	response := performRESTRequest(router, http.MethodGet, "/api/panic-probes/current")
	t.Logf("panic response: status=%d body=%s", response.Code, response.Body.String())
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("panic status=%d; want=%d; body=%s", response.Code, http.StatusInternalServerError, response.Body.String())
	}
	if strings.Contains(response.Body.String(), sensitive) {
		t.Fatalf("panic response leaked internal details: %s", response.Body.String())
	}
	assertRESTEnvelope(t, response, false)
}
