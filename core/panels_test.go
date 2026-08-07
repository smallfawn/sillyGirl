package core

import "testing"

func TestAdminPanelsRouteReplacesLegacyListRoutes(t *testing.T) {
	legacy := map[string]bool{
		"/api/admin/smallcat/panels": false,
		"/api/admin/qinglong/panels": false,
		"/api/admin/daidai/panels":   false,
	}
	foundUnified := false
	for _, route := range apiRouteSnapshot() {
		if route.Method != GET {
			continue
		}
		if route.Path == "/api/admin/panels" {
			foundUnified = true
		}
		if _, exists := legacy[route.Path]; exists {
			legacy[route.Path] = true
		}
	}
	if !foundUnified {
		t.Fatal("unified admin panels route is not registered")
	}
	for path, registered := range legacy {
		if registered {
			t.Fatalf("legacy panel list route is still registered: %s", path)
		}
	}
}

func TestBuildAdminPanelsResponse(t *testing.T) {
	smallcatPanels := []SmallcatPanel{{ID: "smallcat-1", APIAuth: "secret"}}
	qinglongPanels := []QinglongPanel{{ID: "qinglong-1"}, {ID: "qinglong-2"}}
	daidaiPanels := []DaidaiPanel{{ID: "daidai-1"}}

	result := buildAdminPanelsResponse(smallcatPanels, qinglongPanels, daidaiPanels)
	if result.Smallcat.Total != 1 || len(result.Smallcat.List) != 1 {
		t.Fatalf("unexpected smallcat result: %#v", result.Smallcat)
	}
	if result.Smallcat.List[0].APIAuth != "" {
		t.Fatal("aggregated response leaked smallcat api_auth")
	}
	if smallcatPanels[0].APIAuth != "secret" {
		t.Fatal("redaction mutated the stored smallcat panel")
	}
	if result.Qinglong.Total != 2 || len(result.Qinglong.List) != 2 {
		t.Fatalf("unexpected qinglong result: %#v", result.Qinglong)
	}
	if result.Daidai.Total != 1 || len(result.Daidai.List) != 1 {
		t.Fatalf("unexpected daidai result: %#v", result.Daidai)
	}
}
