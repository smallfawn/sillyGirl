package core

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
)

type adminPanelList[T any] struct {
	List  []T `json:"list"`
	Total int `json:"total"`
}

type adminPanelsResponse struct {
	Smallcat adminPanelList[SmallcatPanel] `json:"smallcat"`
	Qinglong adminPanelList[QinglongPanel] `json:"qinglong"`
	Daidai   adminPanelList[DaidaiPanel]   `json:"daidai"`
}

func init() {
	GinApi(GET, "/api/admin/panels", RequireAuth, func(ctx *gin.Context) {
		ApiOK(ctx, getAdminPanels(false))
	})
	GinApi(POST, "/api/admin/panels", RequireAuth, handleSaveAdminPanel)
	GinApi(POST, "/api/admin/panels/:id", RequireAuth, handleSaveAdminPanel)
	GinApi(POST, "/api/admin/panels/:id/deletions", RequireAuth, handleDeleteAdminPanel)
	GinApi(GET, "/api/admin/panels/:id/accounts", RequireAuth, handleSmallcatPanelAccounts)
	GinApi(POST, "/api/admin/panel-connection-tests", RequireAuth, handleAdminPanelConnectionTest)
	GinApi(POST, "/api/admin/panel-status-checks", RequireAuth, func(ctx *gin.Context) {
		ApiOK(ctx, getAdminPanels(true))
	})
}

func handleSaveAdminPanel(ctx *gin.Context) {
	kind, err := adminPanelKindFromRequest(ctx, strings.TrimSpace(ctx.Param("id")))
	if err != nil {
		respondAdminPanelKindError(ctx, err)
		return
	}
	switch kind {
	case "qinglong":
		handleSaveQinglongPanel(ctx)
	case "daidai":
		handleSaveDaidaiPanel(ctx)
	case "smallcat":
		handleSaveSmallcatPanel(ctx)
	default:
		ApiUnprocessable(ctx, "面板类型必须是 qinglong、daidai 或 smallcat")
	}
}

func handleAdminPanelConnectionTest(ctx *gin.Context) {
	kind, err := adminPanelKindFromRequest(ctx, "")
	if err != nil {
		respondAdminPanelKindError(ctx, err)
		return
	}
	switch kind {
	case "qinglong":
		handleQinglongPanelConnectionTest(ctx)
	case "daidai":
		handleDaidaiPanelConnectionTest(ctx)
	case "smallcat":
		handleSmallcatPanelConnectionTest(ctx)
	default:
		ApiUnprocessable(ctx, "面板类型必须是 qinglong、daidai 或 smallcat")
	}
}

func respondAdminPanelKindError(ctx *gin.Context, err error) {
	if strings.Contains(err.Error(), "JSON") || strings.Contains(err.Error(), "1 MiB") {
		ApiFail(ctx, err.Error())
		return
	}
	ApiUnprocessable(ctx, err.Error())
}

func handleDeleteAdminPanel(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" {
		ApiFail(ctx, "缺少面板 ID")
		return
	}
	deleted := deleteQinglongPanel(id) || deleteDaidaiPanel(id) || deleteSmallcatPanel(id)
	if !deleted {
		ApiNotFound(ctx, "面板不存在")
		return
	}
	ApiOK(ctx, nil)
}

func adminPanelKindFromRequest(ctx *gin.Context, id string) (string, error) {
	data, err := io.ReadAll(http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("请求体无效或超过 1 MiB")
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(data))
	payload := struct {
		Type string `json:"type"`
		Kind string `json:"kind"`
	}{}
	if len(bytes.TrimSpace(data)) != 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			return "", fmt.Errorf("请求体不是有效 JSON")
		}
	}
	kind := strings.ToLower(strings.TrimSpace(firstNonEmpty(payload.Type, payload.Kind)))
	if kind == "" && id != "" {
		kind = adminPanelKindByID(id)
	}
	if kind == "" {
		return "", fmt.Errorf("缺少面板类型 type")
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(data))
	return kind, nil
}

func adminPanelKindByID(id string) string {
	for _, panel := range getQinglongPanels() {
		if panel.ID == id {
			return "qinglong"
		}
	}
	for _, panel := range getDaidaiPanels() {
		if panel.ID == id {
			return "daidai"
		}
	}
	if storedSmallcatPanelByID(id) != nil {
		return "smallcat"
	}
	return ""
}

func getAdminPanels(refreshSmallcat bool) adminPanelsResponse {
	smallcatPanels := getSmallcatPanels()
	if refreshSmallcat {
		refreshSmallcatPanelsStatus(smallcatPanels)
	}
	qinglongPanels := getQinglongPanels()
	daidaiPanels := getDaidaiPanels()
	return buildAdminPanelsResponse(smallcatPanels, qinglongPanels, daidaiPanels)
}

func buildAdminPanelsResponse(
	smallcatPanels []SmallcatPanel,
	qinglongPanels []QinglongPanel,
	daidaiPanels []DaidaiPanel,
) adminPanelsResponse {
	return adminPanelsResponse{
		Smallcat: adminPanelList[SmallcatPanel]{
			List:  redactSmallcatPanels(smallcatPanels),
			Total: len(smallcatPanels),
		},
		Qinglong: adminPanelList[QinglongPanel]{
			List:  qinglongPanels,
			Total: len(qinglongPanels),
		},
		Daidai: adminPanelList[DaidaiPanel]{
			List:  daidaiPanels,
			Total: len(daidaiPanels),
		},
	}
}
