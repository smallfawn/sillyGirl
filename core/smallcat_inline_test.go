package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var smallCatClientAPIEndpoints = map[string]string{
	"createQr":        "/api/qr/start",
	"checkQr":         "/api/qr/status",
	"addUser":         "/api/accounts/add",
	"rescanUser":      "/api/accounts/rescan",
	"userList":        "/api/accounts",
	"checkUsers":      "/api/accounts/status",
	"setUserRemark":   "/api/accounts/remark",
	"setUserDisabled": "/api/accounts/disable",
	"deleteUser":      "/api/accounts/delete",
	"proxyList":       "/api/proxies",
	"testProxy":       "/api/proxies/test",
	"addProxy":        "/api/proxies/add",
	"deleteProxy":     "/api/proxies/delete",
	"creditBalance":   "/credits/balance",
	"creditLedger":    "/credits/ledger",
	"getCode":         "/wx/code",
	"getSession":      "/wx/getsession",
	"refreshSession":  "/wx/refresh",
	"getUserInfo":     "/wx/getuserinfo",
	"getEncryptKey":   "/wx/encryptkey",
	"getPhoneNumber":  "/wx/getphonenumber",
	"cloud":           "/wx/cloud",
	"gateway":         "/wx/gateway",
	"qrCodeAuth":      "/wx/qrcodeauth",
	"oauth":           "/wx/oauth",
	"translateLink":   "/wx/translatelink",
	"autoAuth":        "/wx/autoauth",
	"appMsgExt":       "/wx/appmsgext",
	"appMsgLike":      "/wx/appmsglike",
}

func smallCatSourceBlock(t *testing.T, source, start, end string) string {
	t.Helper()
	from := strings.Index(source, start)
	if from < 0 {
		t.Fatalf("SmallCat start marker %q is missing", start)
	}
	block := source[from:]
	if to := strings.Index(block, end); to >= 0 {
		block = block[:to]
	}
	return block
}

func readSmallCatSource(t *testing.T, relativePath, start, end string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", relativePath))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return smallCatSourceBlock(t, string(raw), start, end)
}

func TestSmallCatInlineEndpointWrappers(t *testing.T) {
	runtimes := map[string]string{
		"node module":   readSmallCatSource(t, "proto3/sillygirl.js", "class SmallCat {", "class DaiDai {"),
		"python module": readSmallCatSource(t, "proto3/sillygirl.py", "class _SmallCat:", "class _DaiDai:"),
	}
	typings := map[string]string{
		"grpc plugin typings": smallCatSourceBlock(t, typeat, "declare class SmallCat {", "declare class DaiDai {"),
		"node typings":        readSmallCatSource(t, "proto3/sillygirl.d.ts", "declare class SmallCat {", "declare class DaiDai {"),
	}

	for method, path := range smallCatClientAPIEndpoints {
		for name, source := range runtimes {
			if !strings.Contains(source, method+"(") {
				t.Errorf("%s: SmallCat.%s is missing", name, method)
			}
			if !strings.Contains(source, `"`+path+`"`) {
				t.Errorf("%s: SmallCat.%s route %s is missing", name, method, path)
			}
		}
		for name, source := range typings {
			if !strings.Contains(source, method+"(") {
				t.Errorf("%s: SmallCat.%s is missing", name, method)
			}
		}
	}

	for _, method := range []string{"health", "validateAuth"} {
		for name, source := range runtimes {
			if strings.Contains(source, method+"(") {
				t.Errorf("%s: removed SmallCat.%s wrapper is still exposed", name, method)
			}
		}
		for name, source := range typings {
			if strings.Contains(source, method+"(") {
				t.Errorf("%s: removed SmallCat.%s typing is still exposed", name, method)
			}
		}
	}

	for name, source := range runtimes {
		if !strings.Contains(source, "authorizedUsers(") || !strings.Contains(source, pluginSmallcatRuntimeBucket) {
			t.Errorf("%s: SmallCat.userList authorization bridge is missing", name)
		}
		getCode := smallCatSourceBlock(t, source, "getCode(", "getSession(")
		if strings.Contains(getCode, "authorizedUsers") {
			t.Errorf("%s: SmallCat.getCode must not request an extra read authorization", name)
		}
	}
	for name, source := range typings {
		if !strings.Contains(source, "authorizedUsers(") {
			t.Errorf("%s: SmallCat.authorizedUsers typing is missing", name)
		}
	}
}

func TestUserNamespaceRuntimeExports(t *testing.T) {
	read := func(path string) string {
		data, err := os.ReadFile(filepath.Join("..", path))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	runtimes := map[string]string{
		"node runtime":   read("proto3/sillygirl.js"),
		"python runtime": read("proto3/sillygirl.py"),
	}
	for name, source := range runtimes {
		for _, marker := range []string{"getUserList", "getUser", pluginUserRuntimeBucket, pluginUserRuntimeListKey} {
			if !strings.Contains(source, marker) {
				t.Errorf("%s: user namespace marker %s is missing", name, marker)
			}
		}
	}
	typings := map[string]string{"grpc typings": typeat, "node typings": read("proto3/sillygirl.d.ts")}
	for name, source := range typings {
		for _, marker := range []string{"getUserList", "getUser", "authorized", "smallcat_openids"} {
			if !strings.Contains(source, marker) {
				t.Errorf("%s: user typing %s is missing", name, marker)
			}
		}
	}
	for name, source := range map[string]string{
		"node typings": read("proto3/sillygirl.d.ts"),
		"grpc typings": typeat,
	} {
		if strings.Contains(source, "userList: typeof userList") {
			t.Errorf("%s: obsolete utils.userList export remains", name)
		}
	}
}

func TestSDKExportNamingConsistency(t *testing.T) {
	read := func(path string) string {
		data, err := os.ReadFile(filepath.Join("..", path))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	nodeRuntime := read("proto3/sillygirl.js")
	pythonRuntime := read("proto3/sillygirl.py")
	typings := []string{read("proto3/sillygirl.d.ts"), typeat}

	for _, marker := range []string{"async resume()", "async count()", "oauth(options)"} {
		if !strings.Contains(nodeRuntime, marker) {
			t.Errorf("node runtime: expected %q", marker)
		}
	}
	for _, marker := range []string{"async def resume(self)", "async def count(self)", "async def oauth(self, options)"} {
		if !strings.Contains(pythonRuntime, marker) {
			t.Errorf("python runtime: expected %q", marker)
		}
	}
	for _, source := range typings {
		for _, marker := range []string{"resume():", "count():", "oauth(options:"} {
			if !strings.Contains(source, marker) {
				t.Errorf("typing: expected %q", marker)
			}
		}
		for _, obsolete := range []string{"\n    continue():", "\n    len():", "\n    oAuth(options:", "\n    Get():", "\n    Set(values"} {
			if strings.Contains(source, obsolete) {
				t.Errorf("typing: obsolete member %q remains", obsolete)
			}
		}
	}
}
