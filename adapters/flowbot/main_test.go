package flowbot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSplitReplySegmentsWithImageURL(t *testing.T) {
	segments := splitReplySegments("hello [CQ:image,url=https://example.com/a.png] world")
	if len(segments) != 3 {
		t.Fatalf("segments length = %d, want 3: %#v", len(segments), segments)
	}
	if segments[0].image || strings.TrimSpace(segments[0].value) != "hello" {
		t.Fatalf("first segment = %#v, want hello text", segments[0])
	}
	if !segments[1].image || segments[1].value != "https://example.com/a.png" {
		t.Fatalf("second segment = %#v, want image URL", segments[1])
	}
	if segments[2].image || strings.TrimSpace(segments[2].value) != "world" {
		t.Fatalf("third segment = %#v, want world text", segments[2])
	}
}

func TestSplitReplySegmentsWithImageFileAndCQEscapes(t *testing.T) {
	segments := splitReplySegments("[CQ:image,file=https://example.com/a&#44;b.png]")
	if len(segments) != 1 {
		t.Fatalf("segments length = %d, want 1: %#v", len(segments), segments)
	}
	if !segments[0].image || segments[0].value != "https://example.com/a,b.png" {
		t.Fatalf("segment = %#v, want decoded image file", segments[0])
	}
}

func TestDecodeDataImage(t *testing.T) {
	data, err := decodeDataImage("data:image/png;base64,aGVsbG8=")
	if err != nil {
		t.Fatalf("decodeDataImage returned err=%v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("decoded data = %q, want hello", string(data))
	}
}

func TestInboundEnvelopeParsing(t *testing.T) {
	raw := `{"event":"message","data":{"message_id":"m1","session_id":"group_123","session_type":"group","sender_id":"u1","sender_name":"Alice","self_id":"bot","group_name":"测试群","type":"text","content":"你好","at_users":["u2"]}}`
	var env wsEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal err=%v", err)
	}
	if env.Event != "message" || env.Data == nil {
		t.Fatalf("unexpected envelope: %#v", env)
	}
	if env.Data.SessionID != "group_123" || env.Data.SenderID != "u1" || env.Data.Content != "你好" {
		t.Fatalf("unexpected data: %#v", env.Data)
	}
}

func TestSendRequestJSONShape(t *testing.T) {
	req := sendRequest{
		SessionID: "group_123",
		Type:      "text",
		Content:   "hi",
		AtUsers:   []string{"u2"},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal err=%v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal err=%v", err)
	}
	if m["session_id"] != "group_123" {
		t.Fatalf("session_id = %v, want group_123", m["session_id"])
	}
	if m["type"] != "text" {
		t.Fatalf("type = %v, want text", m["type"])
	}
	if _, ok := m["content"]; !ok {
		t.Fatalf("content key missing: %s", string(data))
	}
	if _, ok := m["image_base64"]; ok {
		t.Fatalf("image_base64 should be omitted for text: %s", string(data))
	}
}

func TestBackoffBounded(t *testing.T) {
	cases := map[int]struct{ want int }{
		0: {0},
		1: {int(reconnectBase / 1e6)},
		2: {int(reconnectBase / 1e6 * 2)},
		6: {int(reconnectMax / 1e6)},
	}
	for attempts, c := range cases {
		got := int(backoff(attempts) / 1e6)
		if got != c.want {
			t.Fatalf("backoff(%d) = %dms, want %dms", attempts, got, c.want)
		}
	}
}

func TestRecordMessageDedup(t *testing.T) {
	b := &bot{dedup: map[string]time.Time{}}
	if !b.recordMessage("x") {
		t.Fatalf("first occurrence should be new")
	}
	if b.recordMessage("x") {
		t.Fatalf("duplicate id should be rejected")
	}
	if !b.recordMessage("") {
		t.Fatalf("empty id should be processed")
	}
}
