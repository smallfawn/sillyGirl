package qq

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/smallfawn/sillyGirl/core"
	"github.com/smallfawn/sillyGirl/core/logs"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
)

var qq = core.MakeBucket("qq")

type Result struct {
	Data struct {
		MessageID interface{} `json:"message_id"`
	} `json:"data"`
	Echo string `json:"echo"`
}

type GroupList struct {
	Retcode int    `json:"retcode"`
	Status  string `json:"status"`
	Data    []struct {
		GroupID   int    `json:"group_id"`
		GroupName string `json:"group_name"`
		// MemberCount     int         `json:"member_count"`
		// MaxMemberCount  int         `json:"max_member_count"`
		// OwnerID         int         `json:"owner_id"`
		// LastJoinTime    int         `json:"last_join_time"`
		// ShutupTimeWhole int         `json:"shutup_time_whole"`
		// ShutupTimeMe    int         `json:"shutup_time_me"`
		// AdminFlag       bool        `json:"admin_flag"`
		// UpdateTime      int         `json:"update_time"`
	} `json:"data"`
	Error interface{} `json:"error"`
	Echo  string      `json:"echo"`
}

type CallApi struct {
	Action string                 `json:"action"`
	Echo   string                 `json:"echo"`
	Params map[string]interface{} `json:"params"`
}

type sender struct {
	Nickname string `json:"nickname"`
}

type Message struct {
	GroupID     interface{} `json:"group_id"`
	Message     interface{} `json:"message"`
	MessageID   interface{} `json:"message_id"`
	MessageType string      `json:"message_type"`
	PostType    string      `json:"post_type"`
	RawMessage  string      `json:"raw_message"`
	SelfID      interface{} `json:"self_id"`
	Sender      sender      `json:"sender"`
	// SubType     string      `json:"sub_type"`
	Time   int         `json:"time"`
	UserID interface{} `json:"user_id"`
}

func parseOneBotMessage(data []byte) (*Message, string, bool) {
	msg := &Message{}
	if err := json.Unmarshal(data, msg); err != nil || msg.PostType != "message" || msg.UserID == nil {
		return nil, "", false
	}
	if msg.SelfID != nil && fmt.Sprint(msg.SelfID) == fmt.Sprint(msg.UserID) {
		return nil, "", false
	}
	content := msg.RawMessage
	if strings.TrimSpace(content) == "" {
		if text, ok := msg.Message.(string); ok {
			content = text
		}
	}
	content = strings.ReplaceAll(content, "\\r", "\n")
	content = qqNewlinePattern.ReplaceAllString(content, "\n")
	content = strings.ReplaceAll(content, "amp;", "")
	content = strings.ReplaceAll(content, "&#91;", "[")
	content = strings.ReplaceAll(content, "&#93;", "]")
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, "", false
	}
	return msg, content, true
}

type GroupInfo struct {
	GroupID   string `json:"group_id"`
	GroupName string `json:"group_name"`
}

type QQ struct {
	conn *websocket.Conn
	sync.RWMutex
	id    int
	chans map[string]chan string
}

func (qq *QQ) WriteJSON(ca CallApi) (string, error) {
	var err error
	cy := make(chan string, 1)
	echo := ""
	func() {
		qq.Lock()
		defer qq.Unlock()
		qq.id++
		ca.Echo = fmt.Sprint(qq.id)
		echo = ca.Echo
		qq.chans[echo] = cy
		err = qq.conn.WriteJSON(ca)
	}()
	if err != nil {
		qq.Lock()
		delete(qq.chans, echo)
		qq.Unlock()
		close(cy)
		return "", err
	}
	defer func() {
		qq.Lock()
		delete(qq.chans, echo)
		qq.Unlock()
		close(cy)
	}()
	select {
	case v := <-cy:
		return v, nil
	case <-time.After(time.Second * 60):
	}
	return "", nil
}

var debug = qq.GetBool("debug", false)
var qqNewlinePattern = regexp.MustCompile(`[\n\r]+`)
var qqConnections sync.Map

func closeQQConnections() {
	qqConnections.Range(func(_, value interface{}) bool {
		if qqcon, ok := value.(*QQ); ok && qqcon.conn != nil {
			_ = qqcon.conn.Close()
		}
		return true
	})
}

func validOneBotAuthorization(auth string, token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return true
	}
	auth = strings.TrimSpace(auth)
	if auth == token {
		return true
	}
	const bearerPrefix = "Bearer "
	if len(auth) > len(bearerPrefix) && strings.EqualFold(auth[:len(bearerPrefix)], bearerPrefix) {
		return strings.TrimSpace(auth[len(bearerPrefix):]) == token
	}
	return false
}

func validOneBotOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	if strings.EqualFold(parsed.Host, r.Host) {
		return true
	}
	return strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://[::1]:")
}

func init() {
	storage.Watch(qq, "enable", func(old, new, key string) *storage.Final {
		if strings.EqualFold(strings.TrimSpace(new), "false") || strings.TrimSpace(new) == "0" {
			closeQQConnections()
			core.DestroyAdaptersByPlatform("qq")
		}
		return nil
	})
	storage.Watch(qq, "debug", func(old, new, key string) *storage.Final {
		now := ""
		if new == "true" {
			now = "true"
			debug = true
		} else {
			now = "false"
			debug = false
		}
		return &storage.Final{
			Now: now,
		}
	})
	go func() {
		core.GinApi(core.GET, "/qq/receive", func(c *gin.Context) {
			if !core.AdapterConfigEnabled("qq") {
				core.Logs.Warn("OneBot机器人未启动：qq.enable=false")
				c.AbortWithStatus(403)
				return
			}
			auth := c.GetHeader("Authorization")
			token := qq.GetString("token")
			if !validOneBotAuthorization(auth, token) {
				core.Logs.Warn("OneBot机器人token不正确，小心有人攻击你的傻妞！！！")
				c.AbortWithStatus(401)
				return
			}
			if token == "" {
				core.Logs.Warn(`你需要在OneBot机器人配置accessToken以及在傻妞配置对应的参数(set qq token ?)才能保证连接安全，如果不设置将会造成信息泄露和资产损失！！！`)
			}
			var upGrader = websocket.Upgrader{
				CheckOrigin: validOneBotOrigin,
			}
			ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				c.Writer.Write([]byte(err.Error()))
				return
			}
			botID := c.GetHeader("X-Self-ID")
			qqcon := &QQ{
				conn:  ws,
				chans: make(map[string]chan string),
			}
			connectionKey := botID
			if connectionKey == "" {
				connectionKey = fmt.Sprintf("%s-%d", c.ClientIP(), time.Now().UnixNano())
			}
			qqConnections.Store(connectionKey, qqcon)
			defer qqConnections.Delete(connectionKey)
			adapter := &core.Factory{}
			adapter.Init("qq", botID, nil)
			defer adapter.Destroy()
			adapter.SetReplyHandler(func(msg map[string]interface{}) string {
				if debug {
					logs.Debug("QQ发送消息：", string(utils.JsonMarshal(msg)))
				}
				if utils.IsZeroOrEmpty(msg[core.CHAT_ID].(string)) {
					params := map[string]interface{}{
						"user_id": msg[core.USER_ID],
						"message": msg[core.CONETNT],
					}
					if debug {
						logs.Debug("QQ实际私聊：", string(utils.JsonMarshal(params)))
					}
					id, err := qqcon.WriteJSON(CallApi{
						Action: "send_private_msg",
						Params: params,
					})
					if err != nil {
						core.Logs.Warn("QQ发送私聊消息错误：", err)
					}
					return id
				} else {
					// {"params":{"user_id":"798731886","message":"2023-06-29 13:44:05","group_id":"178583761"},"action":"send_group_msg"}
					params := map[string]interface{}{
						"group_id": msg[core.CHAT_ID],
						"user_id":  msg[core.USER_ID],
						"message":  msg[core.CONETNT],
					}
					if debug {
						logs.Debug("QQ实际群聊：", string(utils.JsonMarshal(params)))
					}
					id, err := qqcon.WriteJSON(CallApi{
						Action: "send_group_msg",
						Params: params,
					})
					if err != nil {
						core.Logs.Warn("QQ发送群组消息错误：", err)
					}
					return id
				}
			})

			// qqcon.WriteJSON(CallApi{
			// 	Action: "get_group_list",
			// 	Params: map[string]interface{}{},
			// })
			go func() {
				time.Sleep(time.Second * 3)
				qqcon.WriteJSON(CallApi{
					Action: "get_group_list",
					Params: map[string]interface{}{},
				})
			}()

			for {
				_, data, err := ws.ReadMessage()
				if err != nil {

					ws.Close()
					break
				}

				if debug {
					logs.Debug("QQ接收消息：", string(data))
				}

				{
					res := &GroupList{}
					json.Unmarshal(data, res)
					for _, group := range res.Data {
						core.CreateNickName(&core.Nickname{
							Group:    true,
							Value:    group.GroupName,
							ID:       strconv.Itoa(group.GroupID),
							Platform: "qq",
							BotsID:   []string{botID},
						})
					}
				}
				{
					res := &Result{}
					json.Unmarshal(data, res)
					if res.Echo != "" {
						func() {
							qqcon.RLock()
							defer qqcon.RUnlock()
							if c, ok := qqcon.chans[res.Echo]; ok {
								c <- fmt.Sprint(res.Data.MessageID)
							}
						}()
						continue
					}
				}

				msg, content, ok := parseOneBotMessage(data)
				if !ok {
					continue
				}
				_msg := map[string]interface{}{
					"user_id":    utils.Itoa(msg.UserID),
					"chat_id":    core.ChatID(msg.GroupID),
					"user_name":  msg.Sender.Nickname,
					"chat_name":  "",
					"message_id": utils.Itoa(msg.MessageID),
					"content":    content,
				}
				if debug {
					logs.Debug("QQ处理消息：", string(utils.JsonMarshal(_msg)))
				}
				adapter.Receive(_msg)

			}
		})
	}()
}
