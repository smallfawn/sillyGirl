package core

import (
	"bytes"
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

const daidaiPanelsStorageKey = "daidai_panels"

var legacyDaidaiPanels = MakeBucket("daidai_panels")

type DaidaiPanel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Address       string `json:"address"`
	AppKey        string `json:"app_key"`
	AppSecret     string `json:"app_secret"`
	CreatedAt     int    `json:"created_at"`
	UpdatedAt     int    `json:"updated_at"`
	LastCheckedAt int    `json:"last_checked_at"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

type daidaiTokenResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error"`
	Data    struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	} `json:"data"`
}

func init() {
	GinApi(GET, "/api/daidai/panels", RequireAuth, func(ctx *gin.Context) {
		panels := getDaidaiPanels()
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data":    panels,
			"total":   len(panels),
		})
	})

	GinApi(POST, "/api/daidai/panel/test", RequireAuth, func(ctx *gin.Context) {
		panel := DaidaiPanel{}
		if err := ctx.BindJSON(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if err := validateDaidaiPanelInput(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		result, err := testDaidaiPanel(panel)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		ctx.JSON(200, map[string]interface{}{"success": true, "data": result})
	})

	GinApi(POST, "/api/daidai/panel", RequireAuth, func(ctx *gin.Context) {
		panel := DaidaiPanel{}
		if err := ctx.BindJSON(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if err := validateDaidaiPanelInput(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		result, err := testDaidaiPanel(panel)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		now := int(time.Now().Unix())
		panels := getDaidaiPanels()
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
		saveDaidaiPanels(panels)
		ctx.JSON(200, map[string]interface{}{"success": true, "data": panel})
	})

	GinApi(DELETE, "/api/daidai/panel", RequireAuth, func(ctx *gin.Context) {
		panel := DaidaiPanel{}
		if err := ctx.BindJSON(&panel); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if panel.ID == "" {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": "缺少呆呆面板 ID"})
			return
		}
		panels := getDaidaiPanels()
		next := make([]DaidaiPanel, 0, len(panels))
		for _, item := range panels {
			if item.ID != panel.ID {
				next = append(next, item)
			}
		}
		saveDaidaiPanels(next)
		ctx.JSON(200, map[string]interface{}{"success": true})
	})
}

func getDaidaiPanels() []DaidaiPanel {
	raw := strings.TrimSpace(sillyGirl.GetString(daidaiPanelsStorageKey))
	if raw != "" {
		panels := []DaidaiPanel{}
		if json.Unmarshal([]byte(strings.TrimPrefix(raw, "o:")), &panels) == nil {
			return panels
		}
	}
	panels := getLegacyDaidaiPanels()
	if len(panels) > 0 {
		saveDaidaiPanels(panels)
	}
	return panels
}

func getLegacyDaidaiPanels() []DaidaiPanel {
	panels := []DaidaiPanel{}
	legacyDaidaiPanels.Foreach(func(_, data []byte) error {
		panel := DaidaiPanel{}
		if json.Unmarshal(data, &panel) == nil && panel.ID != "" {
			panels = append(panels, panel)
		}
		return nil
	})
	return panels
}

func saveDaidaiPanels(panels []DaidaiPanel) {
	sillyGirl.Set(daidaiPanelsStorageKey, utils.JsonMarshal(panels))
}

func validateDaidaiPanelInput(panel *DaidaiPanel) error {
	panel.Name = strings.TrimSpace(panel.Name)
	panel.Address = normalizeDaidaiAddress(panel.Address)
	panel.AppKey = strings.TrimSpace(panel.AppKey)
	panel.AppSecret = strings.TrimSpace(panel.AppSecret)
	if panel.Address == "" {
		return errors.New("呆呆面板地址不能为空")
	}
	if panel.AppKey == "" {
		return errors.New("app_key 不能为空")
	}
	if panel.AppSecret == "" {
		return errors.New("app_secret 不能为空")
	}
	parsed, err := url.ParseRequestURI(panel.Address)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("呆呆面板地址格式错误：%v", err)
	}
	return nil
}

func normalizeDaidaiAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}
	return strings.TrimRight(address, "/")
}

func testDaidaiPanel(panel DaidaiPanel) (*DaidaiPanel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	tokenResp, err := requestDaidaiToken(ctx, panel)
	if err != nil {
		return nil, err
	}
	if tokenResp.Data.AccessToken == "" {
		return nil, errors.New(daidaiTokenMessage(tokenResp, "认证失败，请检查 app_key/app_secret"))
	}
	panel.Address = normalizeDaidaiAddress(panel.Address)
	panel.Status = "online"
	panel.Message = "连接成功"
	panel.LastCheckedAt = int(time.Now().Unix())
	return &panel, nil
}

func requestDaidaiToken(ctx context.Context, panel DaidaiPanel) (*daidaiTokenResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"app_key":    panel.AppKey,
		"app_secret": panel.AppSecret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, panel.Address+"/api/open-api/token", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("呆呆面板接口连接失败：%v", err)
	}
	defer resp.Body.Close()
	tokenResp := &daidaiTokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(tokenResp); err != nil {
		return nil, fmt.Errorf("呆呆面板接口返回无法解析：%v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("呆呆面板接口 HTTP %d：%s", resp.StatusCode, daidaiTokenMessage(tokenResp, "请求失败"))
	}
	if tokenResp.Success == false && tokenResp.Data.AccessToken == "" {
		return nil, errors.New(daidaiTokenMessage(tokenResp, "认证失败，请检查 app_key/app_secret"))
	}
	return tokenResp, nil
}

func daidaiTokenMessage(tokenResp *daidaiTokenResponse, fallback string) string {
	if tokenResp == nil {
		return fallback
	}
	if strings.TrimSpace(tokenResp.Message) != "" {
		return tokenResp.Message
	}
	if strings.TrimSpace(tokenResp.Error) != "" {
		return tokenResp.Error
	}
	return fallback
}
