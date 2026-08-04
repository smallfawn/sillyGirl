package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core"
)

type WebMessage struct {
	UserID  string   `json:"-"`
	Type    string   `json:"t"`
	Content string   `json:"c"`
	Images  []string `json:"m"`
}

var webUsers sync.Map

var (
	webCQImageFileURLPattern = regexp.MustCompile(`file=[^\[\]]*,url`)
	webCQImagePattern        = regexp.MustCompile(`\[CQ:image,file=([^\[\]]+)\]`)
	webRIDPattern            = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{7,127}$`)
)

const (
	webUserTTL         = 30 * time.Second
	webCleanupInterval = 15 * time.Second
	webPollTimeout     = 4 * time.Second
	webMessageMaxBytes = 32 * 1024
)

func Broadcast2WebUser(content, class string) {
	webUsers.Range(func(key, value interface{}) bool {
		wu := value.(*WebUser)
		if wu.GetActivedAt().Add(webUserTTL).After(time.Now()) {
			wu.Enqueue(WebMessage{
				UserID:  key.(string),
				Content: content,
				Type:    class,
			})
		} else {
			webUsers.Delete(key)
			webAdmins.Delete(key)
		}
		return true
	})
}

type WebUser struct {
	Carry chan WebMessage
	sync.RWMutex
	ActivedAt time.Time
}

func (wu *WebUser) GetCarry() chan WebMessage {
	wu.RLock()
	defer wu.RUnlock()
	return wu.Carry
}

func (wu *WebUser) Active() {
	wu.Lock()
	defer wu.Unlock()
	wu.ActivedAt = time.Now()
}

func (wu *WebUser) GetActivedAt() time.Time {
	wu.RLock()
	defer wu.RUnlock()
	return wu.ActivedAt
}

func (wu *WebUser) Enqueue(message WebMessage) {
	carry := wu.GetCarry()
	select {
	case carry <- message:
		return
	default:
	}
	// Keep the newest replies without blocking a bot handler or leaking a
	// goroutine when a browser stops polling.
	select {
	case <-carry:
	default:
	}
	select {
	case carry <- message:
	default:
	}
}

var webAdmins sync.Map

var (
	adapter     *core.Factory
	adapterOnce sync.Once
)

var GetUserNumber = func() int {
	i := 0
	webUsers.Range(func(key, value any) bool {
		i++
		return true
	})
	return i
}

func initWebBot() {
	adapterOnce.Do(func() {
		adapter = &core.Factory{}
		adapter.Init("web", "default", nil)
		adapter.SetIsAdmin(func(s string) bool {
			isAdmin, ok := webAdmins.Load(s)
			if ok {
				return isAdmin.(bool)
			}
			return false
		})
		adapter.SetReplyHandler(func(msg map[string]interface{}) string {
			userValue, ok := msg[core.USER_ID]
			if !ok {
				return ""
			}
			userID := strings.TrimSpace(fmt.Sprint(userValue))
			if userID == "" {
				return ""
			}
			content := ""
			if contentValue, ok := msg[core.CONETNT]; ok {
				content = fmt.Sprint(contentValue)
			}
			message := WebMessage{
				UserID:  userID,
				Images:  []string{},
				Type:    "chat",
				Content: content,
			}
			sendWebMessage(&message)
			return ""
		})
	})
}

func init() {
	core.RegistFuncs["Broadcast2WebUser"] = Broadcast2WebUser
	go func() {
		time.Sleep(time.Second)
		initWebBot()
	}()
	go cleanupWebUsers()
	core.GinApi(core.GET, "/api/web_chat", receiveWebChat)
	core.GinApi(core.POST, "/api/web_chat", receiveWebChat)
}

func receiveWebChat(ctx *gin.Context) {
	initWebBot()
	rid, content, legacySend, err := webChatRequest(ctx)
	if err != nil {
		core.ApiError(ctx, http.StatusBadRequest, err.Error())
		return
	}
	if !webRIDPattern.MatchString(rid) {
		core.ApiError(ctx, http.StatusBadRequest, "web_chat rid 格式不正确")
		return
	}
	_, authErr := core.CheckAuthRequest(ctx)
	isAdmin := authErr == nil
	if !isAdmin && !core.MakeBucket("sillyGirl").GetBool("web_chat_public") {
		core.ApiError(ctx, http.StatusUnauthorized, "web_chat 未开启匿名访问")
		return
	}
	webAdmins.Store(rid, isAdmin)
	wu := loadWebUser(rid)
	wu.Active()
	if content != "" {
		if len([]byte(content)) > webMessageMaxBytes {
			core.ApiError(ctx, http.StatusRequestEntityTooLarge, "web_chat 消息不能超过 32KB")
			return
		}
		adapter.Receive(map[string]interface{}{
			core.USER_ID: rid,
			core.CONETNT: content,
		})
		if !legacySend {
			core.ApiOK(ctx, []WebMessage{})
			return
		}
	}
	core.ApiOK(ctx, pollWebMessages(ctx, wu, content == "" || legacySend))
}

func webChatRequest(ctx *gin.Context) (rid, content string, legacySend bool, err error) {
	if ctx.Request.Method == http.MethodPost {
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, webMessageMaxBytes+4096)
		payload := struct {
			RID     string `json:"rid"`
			Content string `json:"ctt"`
		}{}
		if decodeErr := json.NewDecoder(ctx.Request.Body).Decode(&payload); decodeErr != nil {
			return "", "", false, fmt.Errorf("请求体不是有效 JSON")
		}
		payload.RID = strings.TrimSpace(payload.RID)
		payload.Content = strings.TrimSpace(payload.Content)
		if payload.Content == "" {
			return "", "", false, fmt.Errorf("web_chat 消息不能为空")
		}
		return payload.RID, payload.Content, false, nil
	}
	rid = strings.TrimSpace(ctx.Query("rid"))
	content = strings.TrimSpace(ctx.Query("ctt"))
	return rid, content, content != "", nil
}

func loadWebUser(rid string) *WebUser {
	created := &WebUser{Carry: make(chan WebMessage, 1000)}
	value, _ := webUsers.LoadOrStore(rid, created)
	return value.(*WebUser)
}

func pollWebMessages(ctx *gin.Context, wu *WebUser, wait bool) []WebMessage {
	msgs := drainWebMessages(wu.GetCarry())
	if len(msgs) != 0 || !wait {
		return msgs
	}
	timer := time.NewTimer(webPollTimeout)
	defer timer.Stop()
	select {
	case msg := <-wu.GetCarry():
		msgs = append(msgs, msg)
		msgs = append(msgs, drainWebMessages(wu.GetCarry())...)
	case <-ctx.Request.Context().Done():
	case <-timer.C:
	}
	return msgs
}

func drainWebMessages(carry <-chan WebMessage) []WebMessage {
	msgs := []WebMessage{}
	for {
		select {
		case msg := <-carry:
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}

func cleanupWebUsers() {
	ticker := time.NewTicker(webCleanupInterval)
	defer ticker.Stop()
	for now := range ticker.C {
		webUsers.Range(func(key, value interface{}) bool {
			if value.(*WebUser).GetActivedAt().Add(webUserTTL).Before(now) {
				webUsers.Delete(key)
				webAdmins.Delete(key)
			}
			return true
		})
	}
}

var sendWebMessage = func(message *WebMessage) {
	message.Content = webCQImageFileURLPattern.ReplaceAllString(message.Content, "file")
	for _, v := range webCQImagePattern.FindAllStringSubmatch(message.Content, -1) {
		message.Images = append(message.Images, v[1])
		message.Content = strings.Replace(message.Content, fmt.Sprintf(`[CQ:image,file=%s]`, v[1]), "", -1)
	}
	v, ok := webUsers.Load(message.UserID)
	var wu *WebUser
	if !ok {
		wu = &WebUser{
			Carry: make(chan WebMessage, 1000),
		}
		webUsers.Store(message.UserID, wu)
		wu.Active()
	} else {
		wu = v.(*WebUser)
	}
	wu.Enqueue(*message)
}
