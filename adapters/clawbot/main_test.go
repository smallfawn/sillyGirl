package clawbot

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildClientVersion(t *testing.T) {
	if got := buildClientVersion("2.4.6"); got != 132102 {
		t.Fatalf("unexpected client version: %d", got)
	}
	if got := buildClientVersion("1.0.11"); got != 65547 {
		t.Fatalf("unexpected client version: %d", got)
	}
}

func TestRandomWechatUin(t *testing.T) {
	value := randomWechatUin()
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("uin should be base64: %v", err)
	}
	if strings.TrimSpace(string(decoded)) == "" {
		t.Fatal("uin payload should not be empty")
	}
}

func TestMessageText(t *testing.T) {
	msg := weixinMessage{
		ItemList: []messageItem{
			{Type: messageItemText, TextItem: &textItem{Text: "你好"}},
			{Type: 2},
			{Type: messageItemText, TextItem: &textItem{Text: "世界"}},
		},
	}
	if got := messageText(msg); got != "你好\n世界" {
		t.Fatalf("unexpected message text: %q", got)
	}
}

func TestStripUnsupportedCQ(t *testing.T) {
	if got := stripUnsupportedCQ("文字[CQ:image,file=a.png]继续"); got != "文字继续" {
		t.Fatalf("unexpected stripped text: %q", got)
	}
}

func TestSplitReplySegments(t *testing.T) {
	segments := splitReplySegments("前文[CQ:image,url=https://example.com/a&#44;b.png&amp;x=1]后文")
	if len(segments) != 3 || segments[0].value != "前文" || !segments[1].image || segments[1].value != "https://example.com/a,b.png&x=1" || segments[2].value != "后文" {
		t.Fatalf("unexpected segments: %#v", segments)
	}
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("png-data"))
	segments = splitReplySegments("[CQ:image,url=" + dataURI + "]")
	if len(segments) != 1 || !segments[0].image || segments[0].value != dataURI {
		t.Fatalf("data URI was not preserved: %#v", segments)
	}
}

func TestSendImageUploadAndMessage(t *testing.T) {
	rawImage := []byte("synthetic-png-image")
	var uploadAESKey []byte
	var sentItem *messageItem
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/image.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(rawImage)
		case "/ilink/bot/getuploadurl":
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("missing bearer token: %q", r.Header.Get("Authorization"))
			}
			var request getUploadURLRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode upload request: %v", err)
			}
			if request.MediaType != uploadMediaImage || request.RawSize != len(rawImage) || request.FileSize != aesEcbPaddedSize(len(rawImage)) || !request.NoNeedThumb {
				t.Errorf("unexpected upload metadata: %#v", request)
			}
			var err error
			uploadAESKey, err = hex.DecodeString(request.AESKey)
			if err != nil || len(uploadAESKey) != aes.BlockSize {
				t.Errorf("invalid upload AES key: %q %v", request.AESKey, err)
			}
			_ = json.NewEncoder(w).Encode(getUploadURLResponse{UploadFullURL: server.URL + "/upload"})
		case "/upload":
			if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/octet-stream" {
				t.Errorf("unexpected CDN request: %s %s", r.Method, r.Header.Get("Content-Type"))
			}
			ciphertext, _ := io.ReadAll(r.Body)
			plaintext, err := decryptAesEcbForTest(ciphertext, uploadAESKey)
			if err != nil || string(plaintext) != string(rawImage) {
				t.Errorf("uploaded ciphertext mismatch: got=%q err=%v", plaintext, err)
			}
			w.Header().Set("x-encrypted-param", "download-param")
			w.WriteHeader(http.StatusOK)
		case "/ilink/bot/sendmessage":
			var request sendMessageRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode send request: %v", err)
			}
			if len(request.Message.ItemList) != 1 {
				t.Errorf("unexpected send items: %#v", request.Message.ItemList)
			} else {
				sentItem = &request.Message.ItemList[0]
			}
			_, _ = io.WriteString(w, `{"ret":0}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &apiClient{
		baseURL:        server.URL,
		cdnBaseURL:     server.URL,
		token:          "test-token",
		channelVersion: defaultChannelVer,
		appID:          defaultIlinkAppID,
		clientVersion:  "132102",
		client:         server.Client(),
	}
	source := server.URL + "/image.png"
	if _, err := client.sendImage(t.Context(), "user-1", "context-1", "run-1", source); err != nil {
		t.Fatalf("sendImage failed: %v", err)
	}
	if sentItem == nil || sentItem.Type != messageItemImage || sentItem.TextItem != nil || sentItem.ImageItem == nil {
		t.Fatalf("unexpected sent image item: %#v", sentItem)
	}
	if sentItem.ImageItem.Media.EncryptQueryParam != "download-param" || sentItem.ImageItem.Media.EncryptType != 1 || sentItem.ImageItem.MidSize != aesEcbPaddedSize(len(rawImage)) {
		t.Fatalf("unexpected image media: %#v", sentItem.ImageItem)
	}
	if got, err := base64.StdEncoding.DecodeString(sentItem.ImageItem.Media.AESKey); err != nil || string(got) != string(uploadAESKey) {
		t.Fatalf("message AES key mismatch: %q %v", sentItem.ImageItem.Media.AESKey, err)
	}
}

func decryptAesEcbForTest(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, io.ErrUnexpectedEOF
	}
	plaintext := make([]byte, len(ciphertext))
	for offset := 0; offset < len(ciphertext); offset += aes.BlockSize {
		block.Decrypt(plaintext[offset:offset+aes.BlockSize], ciphertext[offset:offset+aes.BlockSize])
	}
	padding := int(plaintext[len(plaintext)-1])
	if padding < 1 || padding > aes.BlockSize || padding > len(plaintext) {
		return nil, io.ErrUnexpectedEOF
	}
	return plaintext[:len(plaintext)-padding], nil
}
