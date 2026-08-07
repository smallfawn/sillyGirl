package core

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAdminResourceRoutesAreRESTful(t *testing.T) {
	expected := map[string]bool{
		POST + " /api/admin/sessions":                                                  false,
		GET + " /api/admin/sessions/current":                                           false,
		POST + " /api/admin/sessions/current/deletions":                                false,
		GET + " /api/admin/panels":                                                     false,
		POST + " /api/admin/panels":                                                    false,
		POST + " /api/admin/panels/:id":                                                false,
		POST + " /api/admin/panels/:id/deletions":                                      false,
		GET + " /api/admin/panels/:id/accounts":                                        false,
		POST + " /api/admin/panel-connection-tests":                                    false,
		POST + " /api/admin/panel-status-checks":                                       false,
		GET + " /api/admin/plugin-settings/:uuid":                                      false,
		POST + " /api/admin/plugin-settings/:uuid":                                     false,
		GET + " /api/admin/plugin-market/plugins":                                      false,
		GET + " /api/admin/settings":                                                   false,
		POST + " /api/admin/settings":                                                  false,
		GET + " /api/admin/storage/entries":                                            false,
		GET + " /api/admin/storage/buckets":                                            false,
		POST + " /api/admin/storage/values":                                            false,
		GET + " /api/admin/bots":                                                       false,
		GET + " /api/admin/message-rules/:kind":                                        false,
		GET + " /api/admin/dependencies":                                               false,
		GET + " /api/admin/dependency-registries/:runtime":                             false,
		POST + " /api/admin/dependency-registries/:runtime/option-deletions/*registry": false,
		GET + " /api/admin/masters":                                                    false,
		POST + " /api/admin/tasks":                                                     false,
		POST + " /api/admin/tasks/:task_id":                                            false,
		POST + " /api/admin/tasks/:task_id/deletions":                                  false,
		GET + " /api/admin/task-options":                                               false,
		POST + " /api/admin/tasks/:task_id/executions":                                 false,
		POST + " /api/admin/replies":                                                   false,
		POST + " /api/admin/replies/:id":                                               false,
		POST + " /api/admin/carry-groups":                                              false,
		POST + " /api/admin/carry-groups/:chat_id":                                     false,
		POST + " /api/user/sessions":                                                   false,
		POST + " /api/user/sessions/current/deletions":                                 false,
		GET + " /api/user/profile":                                                     false,
	}
	legacy := map[string]bool{
		"/api/admin/backup":                          false,
		"/api/admin/carry/group":                     false,
		"/api/admin/carry/group_names":               false,
		"/api/admin/carry/group_selects":             false,
		"/api/admin/carry/groups":                    false,
		"/api/admin/clawbot/login/start":             false,
		"/api/admin/clawbot/login/status":            false,
		"/api/admin/currentUser":                     false,
		"/api/admin/daidai/panel/test":               false,
		"/api/admin/login":                           false,
		"/api/admin/master":                          false,
		"/api/admin/nickname/labels":                 false,
		"/api/admin/node/dependency":                 false,
		"/api/admin/node/script":                     false,
		"/api/admin/outlogin":                        false,
		"/api/admin/plugin/dependency":               false,
		"/api/admin/plugin/dependency/registry":      false,
		"/api/admin/plugin/open":                     false,
		"/api/admin/plugins/github-proxy":            false,
		"/api/admin/plugins/local/script":            false,
		"/api/admin/plugins/source":                  false,
		"/api/admin/plugins/sources":                 false,
		"/api/admin/proxy/rules":                     false,
		"/api/admin/proxy/scripts":                   false,
		"/api/admin/qinglong/panel/test":             false,
		"/api/admin/register":                        false,
		"/api/admin/reply":                           false,
		"/api/admin/reply/list":                      false,
		"/api/admin/setup/status":                    false,
		"/api/admin/smallcat/panel/accounts":         false,
		"/api/admin/smallcat/panel/test":             false,
		"/api/admin/storage":                         false,
		"/api/admin/system/restart":                  false,
		"/api/admin/system/update":                   false,
		"/api/admin/system/update/status":            false,
		"/api/admin/task/selects":                    false,
		"/api/admin/tasks/enable":                    false,
		"/api/admin/tasks/run":                       false,
		"/api/decode/:random":                        false,
		"/api/file/*filename":                        false,
		"/api/open/plugins":                          false,
		"/api/plugins/download":                      false,
		"/api/plugins/download/:uuid":                false,
		"/api/plugins/list.json":                     false,
		"/api/user/bind":                             false,
		"/api/user/login":                            false,
		"/api/user/me":                               false,
		"/api/user/outlogin":                         false,
		"/api/user/plugin/authorization":             false,
		"/api/user/plugin/form":                      false,
		"/api/user/plugin/smallcat":                  false,
		"/api/user/register":                         false,
		"/api/user/smallcat/account/add":             false,
		"/api/user/smallcat/code":                    false,
		"/api/user/smallcat/login/confirm":           false,
		"/api/user/smallcat/panels":                  false,
		"/api/user/smallcat/qr/start":                false,
		"/api/user/smallcat/qr/status":               false,
		"/api/web_chat":                              false,
		"/api/admin/qinglong/panel":                  false,
		"/api/admin/smallcat/panel":                  false,
		"/api/admin/daidai/panel":                    false,
		"/api/admin/storage/list":                    false,
		"/api/admin/storage/bucket":                  false,
		"/api/admin/plugin/config":                   false,
		"/api/admin/plugin/configs":                  false,
		"/api/admin/plugin/dependencies":             false,
		"/api/admin/node/dependencies":               false,
		"/api/admin/node/dependency/registry":        false,
		"/api/admin/master/list":                     false,
		"/api/admin/session":                         false,
		"/api/user/session":                          false,
		"/api/admin/panels/qinglong":                 false,
		"/api/admin/panels/qinglong/:id":             false,
		"/api/admin/panels/daidai":                   false,
		"/api/admin/panels/daidai/:id":               false,
		"/api/admin/panels/smallcat":                 false,
		"/api/admin/panels/smallcat/:id":             false,
		"/api/admin/panel-connection-tests/qinglong": false,
		"/api/admin/panel-connection-tests/daidai":   false,
		"/api/admin/panel-connection-tests/smallcat": false,
		"/api/admin/tasks/:task_id/options":          false,
		"/api/admin/task-executions":                 false,
		"/api/admin/system-update-jobs/current":      false,
	}
	legacyMethods := map[string]bool{
		"PUT /api/admin/settings":                        false,
		"PUT /api/admin/storage/values":                  false,
		"PUT /api/admin/tasks/:task_id":                  false,
		"PUT /api/admin/replies/:id":                     false,
		"PUT /api/admin/clawbot-login-sessions/:session": false,
		"PUT /api/admin/tasks/:task_id/status":           false,
	}
	for _, route := range apiRouteSnapshot() {
		key := route.Method + " " + route.Path
		if _, ok := expected[key]; ok {
			expected[key] = true
		}
		if _, ok := legacy[route.Path]; ok {
			legacy[route.Path] = true
		}
		if _, ok := legacyMethods[key]; ok {
			legacyMethods[key] = true
		}
	}
	for route, registered := range expected {
		if !registered {
			t.Errorf("REST resource route is not registered: %s", route)
		}
	}
	for route, registered := range legacy {
		if registered {
			t.Errorf("legacy route is still registered: %s", route)
		}
	}
	for route, registered := range legacyMethods {
		if registered {
			t.Errorf("legacy route method is still registered: %s", route)
		}
	}
}

func TestMessageRuleKinds(t *testing.T) {
	for kind, expected := range map[string]string{
		"listening": "listenOnGroups",
		"muted":     "noReplyGroups",
		"blocked":   "noListenUsers",
	} {
		actual, ok := messageRuleBucket(kind)
		if !ok || actual != expected {
			t.Fatalf("message rule kind %q mapped to %q, %v", kind, actual, ok)
		}
	}
	if _, ok := messageRuleBucket("unknown"); ok {
		t.Fatal("unknown message rule kind was accepted")
	}
}

func TestPublicPluginMarketCannotIncludeAdminResources(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(GET, "/api/plugin-market/plugins?include=sources,settings", nil)
	response := RequestPluginResult{}
	includePluginMarketResources(ctx, &response)
	if response.Sources != nil || response.Settings != nil {
		t.Fatal("public plugin market response included admin resources")
	}
}
