package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/smallfawn/sillyGirl/utils"
)

const smallcatPanelsStorageKey = "smallcat_panels"

var legacySmallcatPanels = MakeBucket("smallcat_panels")

type SmallcatPanel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Address       string `json:"address"`
	APIAuth       string `json:"api_auth"`
	CreatedAt     int    `json:"created_at"`
	UpdatedAt     int    `json:"updated_at"`
	LastCheckedAt int    `json:"last_checked_at"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

type smallcatAuthValidateResponse struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func init() {
	GinApi(GET, "/api/smallcat/panels", RequireAuth, func(ctx *gin.Context) {
		panels := getSmallcatPanels()
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data":    panels,
			"total":   len(panels),
		})
	})

	GinApi(POST, "/api/smallcat/panel/test", RequireAuth, func(ctx *gin.Context) {
		panel := SmallcatPanel{}
		if err := ctx.BindJSON(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if err := validateSmallcatPanelInput(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		result, err := testSmallcatPanel(panel)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		ctx.JSON(200, map[string]interface{}{"success": true, "data": result})
	})

	GinApi(POST, "/api/smallcat/panel", RequireAuth, func(ctx *gin.Context) {
		panel := SmallcatPanel{}
		if err := ctx.BindJSON(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if err := validateSmallcatPanelInput(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		result, err := testSmallcatPanel(panel)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		now := int(time.Now().Unix())
		panels := getSmallcatPanels()
		index := -1
		if panel.ID != "" {
			for i := range panels {
				if panels[i].ID == panel.ID {
					index = i
					break
				}
			}
		}
		if panel.ID == "" {
			panel.ID = utils.GenUUID()
			panel.CreatedAt = now
		} else if index >= 0 {
			if panels[index].CreatedAt != 0 {
				panel.CreatedAt = panels[index].CreatedAt
			} else {
				panel.CreatedAt = now
			}
		} else {
			panel.CreatedAt = now
		}
		if panel.Name == "" {
			panel.Name = panel.Address
		}
		panel.UpdatedAt = now
		panel.LastCheckedAt = now
		panel.Status = "online"
		panel.Message = result.Message
		if index >= 0 {
			panels[index] = panel
		} else {
			panels = append(panels, panel)
		}
		saveSmallcatPanels(panels)
		ctx.JSON(200, map[string]interface{}{"success": true, "data": panel})
	})

	GinApi(DELETE, "/api/smallcat/panel", RequireAuth, func(ctx *gin.Context) {
		panel := SmallcatPanel{}
		if err := ctx.BindJSON(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if panel.ID == "" {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": "缺少 smallcat ID"})
			return
		}
		panels := getSmallcatPanels()
		next := make([]SmallcatPanel, 0, len(panels))
		for _, item := range panels {
			if item.ID != panel.ID {
				next = append(next, item)
			}
		}
		saveSmallcatPanels(next)
		ctx.JSON(200, map[string]interface{}{"success": true})
	})
}

func getSmallcatPanels() []SmallcatPanel {
	raw := strings.TrimSpace(sillyGirl.GetString(smallcatPanelsStorageKey))
	if raw != "" {
		panels := []SmallcatPanel{}
		if json.Unmarshal([]byte(strings.TrimPrefix(raw, "o:")), &panels) == nil {
			return panels
		}
	}
	panels := getLegacySmallcatPanels()
	if len(panels) > 0 {
		saveSmallcatPanels(panels)
	}
	return panels
}

func getLegacySmallcatPanels() []SmallcatPanel {
	panels := []SmallcatPanel{}
	legacySmallcatPanels.Foreach(func(_, data []byte) error {
		panel := SmallcatPanel{}
		if json.Unmarshal(data, &panel) == nil && panel.ID != "" {
			panels = append(panels, panel)
		}
		return nil
	})
	return panels
}

func saveSmallcatPanels(panels []SmallcatPanel) {
	sillyGirl.Set(smallcatPanelsStorageKey, utils.JsonMarshal(panels))
}

func validateSmallcatPanelInput(panel *SmallcatPanel) error {
	panel.Name = strings.TrimSpace(panel.Name)
	panel.Address = normalizeSmallcatAddress(panel.Address)
	panel.APIAuth = strings.TrimSpace(panel.APIAuth)
	if panel.Address == "" {
		return errors.New("smallcat 地址不能为空")
	}
	if panel.APIAuth == "" {
		return errors.New("api_auth 不能为空")
	}
	parsed, err := url.ParseRequestURI(panel.Address)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("smallcat 地址格式错误：%v", err)
	}
	return nil
}

func normalizeSmallcatAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}
	return strings.TrimRight(address, "/")
}

func testSmallcatPanel(panel SmallcatPanel) (*SmallcatPanel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, panel.Address+"/api/auth/validate", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("auth", panel.APIAuth)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("smallcat 接口连接失败：%v", err)
	}
	defer resp.Body.Close()
	authResp := smallcatAuthValidateResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, fmt.Errorf("smallcat 接口返回无法解析：%v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("smallcat 接口 HTTP %d：%s", resp.StatusCode, authResp.Message)
	}
	if !authResp.Status {
		if authResp.Message == "" {
			authResp.Message = "验证失败，请检查 API AUTH"
		}
		return nil, errors.New(authResp.Message)
	}
	panel.Address = normalizeSmallcatAddress(panel.Address)
	panel.Status = "online"
	panel.Message = "验证通过"
	panel.LastCheckedAt = int(time.Now().Unix())
	return &panel, nil
}
