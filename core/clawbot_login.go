package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/utils"
)

const (
	clawbotLoginDefaultBotTyp = "3"
	clawbotLoginTTL           = 5 * time.Minute
	clawbotLoginPollTimeout   = 35 * time.Second
)

var clawbotLoginBucket = MakeBucket("clawbot")
var clawbotLoginAPIBase = "https://ilinkai.weixin.qq.com"
var clawbotLoginHTTPClient = http.DefaultClient

var clawbotLoginSessions = struct {
	sync.Mutex
	rows map[string]*clawbotLoginSession
}{
	rows: map[string]*clawbotLoginSession{},
}

type clawbotLoginSession struct {
	ID             string
	QRCode         string
	QRCodeURL      string
	StartedAt      time.Time
	CurrentAPIBase string
	VerifyCode     string
}

type clawbotQRCodeResponse struct {
	QRCode            string `json:"qrcode"`
	QRCodeImage       string `json:"qrcode_img_content"`
	QRCodeImageLegacy string `json:"qrcode_url"`
}

type clawbotStatusResponse struct {
	Status       string `json:"status"`
	BotToken     string `json:"bot_token"`
	IlinkBotID   string `json:"ilink_bot_id"`
	BaseURL      string `json:"baseurl"`
	IlinkUserID  string `json:"ilink_user_id"`
	RedirectHost string `json:"redirect_host"`
	ErrMsg       string `json:"errmsg"`
	Message      string `json:"message"`
}

type clawbotLoginStatus struct {
	Session      string `json:"session"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	NeedVerify   bool   `json:"need_verify"`
	Connected    bool   `json:"connected"`
	AlreadyBound bool   `json:"already_bound"`
	BotID        string `json:"bot_id,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	BaseURL      string `json:"base_url,omitempty"`
}

func init() {
	GinApi(POST, "/api/admin/clawbot-login-sessions", RequireAuth, func(ctx *gin.Context) {
		payload := struct {
			BotType string `json:"bot_type"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		result, err := startClawbotLogin(ctx.Request.Context(), payload.BotType)
		if err != nil {
			ApiError(ctx, http.StatusBadGateway, err.Error())
			return
		}
		session := strings.TrimSpace(fmt.Sprint(result["session"]))
		ApiCreated(ctx, "/api/admin/clawbot-login-sessions/"+session, result)
	})
	GinApi(GET, "/api/admin/clawbot-login-sessions/:session", RequireAuth, func(ctx *gin.Context) {
		respondClawbotLoginStatus(ctx, "", false)
	})
	GinApi(POST, "/api/admin/clawbot-login-sessions/:session/verification-attempts", RequireAuth, func(ctx *gin.Context) {
		payload := struct {
			VerifyCode string `json:"verify_code"`
		}{}
		if err := ctx.BindJSON(&payload); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		respondClawbotLoginStatus(ctx, payload.VerifyCode, true)
	})
}

func respondClawbotLoginStatus(ctx *gin.Context, verifyCode string, accepted bool) {
	session := strings.TrimSpace(ctx.Param("session"))
	result, err := pollClawbotLogin(ctx.Request.Context(), session, strings.TrimSpace(verifyCode))
	if err != nil {
		if strings.Contains(err.Error(), "不存在") || strings.Contains(err.Error(), "失效") {
			ApiNotFound(ctx, err.Error())
		} else {
			ApiError(ctx, http.StatusBadGateway, err.Error())
		}
		return
	}
	if accepted {
		ApiAccepted(ctx, "/api/admin/clawbot-login-sessions/"+session, result)
		return
	}
	ApiOK(ctx, result)
}

func startClawbotLogin(ctx context.Context, botType string) (map[string]interface{}, error) {
	purgeClawbotLoginSessions()
	botType = strings.TrimSpace(botType)
	if botType == "" {
		botType = clawbotLoginDefaultBotTyp
	}
	qrResp, err := fetchClawbotQRCode(ctx, botType)
	if err != nil {
		return nil, err
	}
	qrURL := strings.TrimSpace(qrResp.QRCodeImage)
	if qrURL == "" {
		qrURL = strings.TrimSpace(qrResp.QRCodeImageLegacy)
	}
	if qrURL == "" {
		return nil, errors.New("ClawBot 未返回二维码内容")
	}
	sessionID := utils.GenUUID()
	session := &clawbotLoginSession{
		ID:             sessionID,
		QRCode:         strings.TrimSpace(qrResp.QRCode),
		QRCodeURL:      qrURL,
		StartedAt:      time.Now(),
		CurrentAPIBase: clawbotLoginAPIBase,
	}
	clawbotLoginSessions.Lock()
	clawbotLoginSessions.rows[sessionID] = session
	clawbotLoginSessions.Unlock()
	return map[string]interface{}{
		"session":     sessionID,
		"qrcode":      session.QRCode,
		"qrcode_url":  session.QRCodeURL,
		"expires_at":  session.StartedAt.Add(clawbotLoginTTL).Unix(),
		"poll_after":  1000,
		"status":      "wait",
		"message":     "请使用微信扫描二维码并确认授权",
		"masked_hint": maskSecret(clawbotLoginBucket.GetString("token")),
	}, nil
}

func pollClawbotLogin(ctx context.Context, sessionID string, verifyCode string) (clawbotLoginStatus, error) {
	session, err := getClawbotLoginSession(sessionID)
	if err != nil {
		return clawbotLoginStatus{}, err
	}
	if verifyCode != "" {
		session.VerifyCode = verifyCode
	}
	statusResp, err := fetchClawbotLoginStatus(ctx, session)
	if err != nil {
		return clawbotLoginStatus{}, err
	}
	status := strings.TrimSpace(statusResp.Status)
	result := clawbotLoginStatus{
		Session: session.ID,
		Status:  status,
		Message: clawbotStatusMessage(statusResp),
	}
	switch status {
	case "wait", "":
		result.Status = "wait"
		result.Message = "等待扫码"
	case "scaned":
		result.Message = "已扫码，请在手机微信确认"
	case "need_verifycode":
		result.NeedVerify = true
		result.Message = "需要输入手机微信显示的数字验证码"
	case "verify_code_blocked":
		deleteClawbotLoginSession(session.ID)
		result.Message = "验证码错误次数过多，请重新生成二维码"
	case "expired":
		deleteClawbotLoginSession(session.ID)
		result.Message = "二维码已过期，请重新生成"
	case "scaned_but_redirect":
		if baseURL := clawbotRedirectBaseURL(statusResp.RedirectHost); baseURL != "" {
			session.CurrentAPIBase = baseURL
			result.Message = "已切换授权节点，继续等待确认"
		}
	case "binded_redirect":
		deleteClawbotLoginSession(session.ID)
		result.AlreadyBound = true
		result.Message = "该 ClawBot 已绑定，无需重复获取 token"
	case "confirmed":
		if strings.TrimSpace(statusResp.BotToken) == "" {
			deleteClawbotLoginSession(session.ID)
			return clawbotLoginStatus{}, errors.New("授权成功但未返回 bot_token")
		}
		_, _, err := clawbotLoginBucket.Set("token", strings.TrimSpace(statusResp.BotToken))
		if err != nil {
			return clawbotLoginStatus{}, err
		}
		_, _, _ = clawbotLoginBucket.Set("enable", true)
		if baseURL := normalizeClawbotHTTPBaseURL(statusResp.BaseURL); baseURL != "" {
			_, _, _ = clawbotLoginBucket.Set("api_base", baseURL)
		}
		deleteClawbotLoginSession(session.ID)
		result.Connected = true
		result.BotID = strings.TrimSpace(statusResp.IlinkBotID)
		result.UserID = strings.TrimSpace(statusResp.IlinkUserID)
		result.BaseURL = strings.TrimSpace(statusResp.BaseURL)
		result.Message = "ClawBot token 已保存"
	default:
		if result.Message == "" {
			result.Message = "未知状态：" + status
		}
	}
	return result, nil
}

func getClawbotLoginSession(sessionID string) (*clawbotLoginSession, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("缺少扫码会话")
	}
	clawbotLoginSessions.Lock()
	defer clawbotLoginSessions.Unlock()
	session := clawbotLoginSessions.rows[sessionID]
	if session == nil {
		return nil, errors.New("扫码会话不存在，请重新生成")
	}
	if time.Since(session.StartedAt) > clawbotLoginTTL {
		delete(clawbotLoginSessions.rows, sessionID)
		return nil, errors.New("扫码会话已过期，请重新生成")
	}
	return session, nil
}

func deleteClawbotLoginSession(sessionID string) {
	clawbotLoginSessions.Lock()
	delete(clawbotLoginSessions.rows, sessionID)
	clawbotLoginSessions.Unlock()
}

func purgeClawbotLoginSessions() {
	now := time.Now()
	clawbotLoginSessions.Lock()
	defer clawbotLoginSessions.Unlock()
	for id, session := range clawbotLoginSessions.rows {
		if session == nil || now.Sub(session.StartedAt) > clawbotLoginTTL {
			delete(clawbotLoginSessions.rows, id)
		}
	}
}

func fetchClawbotQRCode(ctx context.Context, botType string) (clawbotQRCodeResponse, error) {
	resp := clawbotQRCodeResponse{}
	err := clawbotLoginGet(ctx, clawbotLoginAPIBase, "ilink/bot/get_bot_qrcode?bot_type="+url.QueryEscape(botType), 15*time.Second, &resp)
	return resp, err
}

func fetchClawbotLoginStatus(ctx context.Context, session *clawbotLoginSession) (clawbotStatusResponse, error) {
	resp := clawbotStatusResponse{}
	endpoint := "ilink/bot/get_qrcode_status?qrcode=" + urlQueryEscape(session.QRCode)
	if strings.TrimSpace(session.VerifyCode) != "" {
		endpoint += "&verify_code=" + urlQueryEscape(session.VerifyCode)
	}
	err := clawbotLoginGet(ctx, session.CurrentAPIBase, endpoint, clawbotLoginPollTimeout, &resp)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return clawbotStatusResponse{Status: "wait"}, nil
		}
		return resp, err
	}
	return resp, nil
}

func clawbotLoginGet(ctx context.Context, baseURL string, endpoint string, timeout time.Duration, out interface{}) error {
	return clawbotLoginRequest(ctx, http.MethodGet, baseURL, endpoint, nil, timeout, out)
}

func clawbotLoginRequest(ctx context.Context, method string, baseURL string, endpoint string, body io.Reader, timeout time.Duration, out interface{}) error {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, method, strings.TrimRight(baseURL, "/")+"/"+endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := clawbotLoginHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := utils.ReadAllLimit(resp.Body, 1<<20)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ClawBot 登录接口 HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("ClawBot 登录接口返回非 JSON：%w", err)
	}
	return nil
}

func clawbotStatusMessage(resp clawbotStatusResponse) string {
	for _, value := range []string{resp.Message, resp.ErrMsg} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func urlQueryEscape(value string) string {
	return url.QueryEscape(value)
}

func clawbotRedirectBaseURL(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	if host == "" || strings.ContainsAny(host, "/\\?#") {
		return ""
	}
	return "https://" + host
}

func normalizeClawbotHTTPBaseURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func maskSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return strings.Repeat("*", len(value))
	}
	return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
}
