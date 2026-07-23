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

const qinglongPanelsStorageKey = "qinglong_panels"

var legacyQinglongPanels = MakeBucket("qinglong_panels")

type QinglongPanel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Address       string `json:"address"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	CreatedAt     int    `json:"created_at"`
	UpdatedAt     int    `json:"updated_at"`
	LastCheckedAt int    `json:"last_checked_at"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

type qinglongTokenResponse struct {
	Code int `json:"code"`
	Data struct {
		Token      string `json:"token"`
		TokenType  string `json:"token_type"`
		Expiration int64  `json:"expiration"`
	} `json:"data"`
	Message string `json:"message"`
}

func init() {
	GinApi(GET, "/api/qinglong/panels", RequireAuth, func(ctx *gin.Context) {
		panels := getQinglongPanels()
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data":    panels,
			"total":   len(panels),
		})
	})

	GinApi(POST, "/api/qinglong/panel/test", RequireAuth, func(ctx *gin.Context) {
		panel := QinglongPanel{}
		if err := ctx.BindJSON(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if err := validateQinglongPanelInput(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		result, err := testQinglongPanel(panel)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		ctx.JSON(200, map[string]interface{}{"success": true, "data": result})
	})

	GinApi(POST, "/api/qinglong/panel", RequireAuth, func(ctx *gin.Context) {
		panel := QinglongPanel{}
		if err := ctx.BindJSON(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if err := validateQinglongPanelInput(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		result, err := testQinglongPanel(panel)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		now := int(time.Now().Unix())
		panels := getQinglongPanels()
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
		saveQinglongPanels(panels)
		ctx.JSON(200, map[string]interface{}{"success": true, "data": panel})
	})

	GinApi(DELETE, "/api/qinglong/panel", RequireAuth, func(ctx *gin.Context) {
		panel := QinglongPanel{}
		if err := ctx.BindJSON(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if panel.ID == "" {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": "缺少青龙面板 ID"})
			return
		}
		panels := getQinglongPanels()
		next := make([]QinglongPanel, 0, len(panels))
		for _, item := range panels {
			if item.ID != panel.ID {
				next = append(next, item)
			}
		}
		saveQinglongPanels(next)
		ctx.JSON(200, map[string]interface{}{"success": true})
	})
}

func getQinglongPanels() []QinglongPanel {
	raw := strings.TrimSpace(sillyGirl.GetString(qinglongPanelsStorageKey))
	if raw != "" {
		panels := []QinglongPanel{}
		if json.Unmarshal([]byte(strings.TrimPrefix(raw, "o:")), &panels) == nil {
			return panels
		}
	}
	panels := getLegacyQinglongPanels()
	if len(panels) > 0 {
		saveQinglongPanels(panels)
	}
	return panels
}

func getLegacyQinglongPanels() []QinglongPanel {
	panels := []QinglongPanel{}
	legacyQinglongPanels.Foreach(func(_, data []byte) error {
		panel := QinglongPanel{}
		if json.Unmarshal(data, &panel) == nil && panel.ID != "" {
			panels = append(panels, panel)
		}
		return nil
	})
	return panels
}

func saveQinglongPanels(panels []QinglongPanel) {
	sillyGirl.Set(qinglongPanelsStorageKey, utils.JsonMarshal(panels))
}

func validateQinglongPanelInput(panel *QinglongPanel) error {
	panel.Name = strings.TrimSpace(panel.Name)
	panel.Address = normalizeQinglongAddress(panel.Address)
	panel.ClientID = strings.TrimSpace(panel.ClientID)
	panel.ClientSecret = strings.TrimSpace(panel.ClientSecret)
	if panel.Address == "" {
		return errors.New("青龙地址不能为空")
	}
	if panel.ClientID == "" {
		return errors.New("client_id 不能为空")
	}
	if panel.ClientSecret == "" {
		return errors.New("client_secret 不能为空")
	}
	if _, err := url.ParseRequestURI(panel.Address); err != nil {
		return fmt.Errorf("青龙地址格式错误：%v", err)
	}
	return nil
}

func normalizeQinglongAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}
	return strings.TrimRight(address, "/")
}

func qinglongURL(panel QinglongPanel, path string, values url.Values) string {
	u := panel.Address + path
	if encoded := values.Encode(); encoded != "" {
		u += "?" + encoded
	}
	return u
}

func testQinglongPanel(panel QinglongPanel) (*QinglongPanel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	values := url.Values{}
	values.Set("client_id", panel.ClientID)
	values.Set("client_secret", panel.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, qinglongURL(panel, "/open/auth/token", values), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("青龙接口连接失败：%v", err)
	}
	defer resp.Body.Close()
	tokenResp := qinglongTokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("青龙接口返回无法解析：%v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("青龙接口 HTTP %d：%s", resp.StatusCode, tokenResp.Message)
	}
	if tokenResp.Code != 200 || tokenResp.Data.Token == "" {
		if tokenResp.Message == "" {
			tokenResp.Message = "认证失败，请检查 client_id/client_secret"
		}
		return nil, errors.New(tokenResp.Message)
	}
	panel.Address = normalizeQinglongAddress(panel.Address)
	panel.Status = "online"
	panel.Message = "连接成功"
	panel.LastCheckedAt = int(time.Now().Unix())
	return &panel, nil
}
