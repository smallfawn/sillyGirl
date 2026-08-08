package core

import (
	"testing"

	"github.com/smallfawn/sillyGirl/core/common"
)

func TestRunScriptTaskCommandUsesConfiguredRecipient(t *testing.T) {
	const (
		platform = "task-recipient-test-platform"
		botID    = "task-recipient-test-bot"
		userID   = "task-recipient-test-user"
		chatID   = "task-recipient-test-chat"
	)

	adapter := &Factory{}
	adapter.Init(platform, botID, nil)
	defer adapter.Destroy()

	originalFunctions := Functions
	defer func() { Functions = originalFunctions }()

	captured := make(chan common.Sender, 1)
	Functions = append(append([]*common.Function{}, Functions...), &common.Function{
		UUID:  "task-recipient-test-plugin",
		Title: "task-recipient-test.js",
		Type:  NODE,
		Path:  "task-recipient-test.js",
		Handle: func(sender common.Sender) interface{} {
			captured <- sender
			return nil
		},
	})

	ok := runScriptTaskCommand("node task-recipient-test.js", "", []Sender{{
		Platfrom: platform,
		BotID:    botID,
		UserID:   userID,
		ChatID:   chatID,
	}})
	if !ok {
		t.Fatal("script task command was not recognized")
	}

	sender := <-captured
	if got := sender.GetContent(); got != "node task-recipient-test.js" {
		t.Fatalf("content = %q, want command fallback", got)
	}
	if got := sender.GetImType(); got != platform {
		t.Fatalf("platform = %q, want %q", got, platform)
	}
	if got := sender.GetBotID(); got != botID {
		t.Fatalf("bot id = %q, want %q", got, botID)
	}
	if got := sender.GetUserID(); got != userID {
		t.Fatalf("user id = %q, want %q", got, userID)
	}
	if got := sender.GetChatID(); got != chatID {
		t.Fatalf("chat id = %q, want %q", got, chatID)
	}
}

func TestRunScriptTaskCommandUsesTriggerPhraseAndRuleParams(t *testing.T) {
	originalFunctions := Functions
	defer func() { Functions = originalFunctions }()

	captured := make(chan common.Sender, 1)
	function := &common.Function{
		UUID:   "task-trigger-test-plugin",
		Title:  "task-trigger-test.js",
		Type:   NODE,
		Path:   "task-trigger-test.js",
		Rules:  []string{`^查询 (\S+)$`, `^刷新$`},
		Params: [][]string{{"account"}, {}},
		Handle: func(sender common.Sender) interface{} {
			captured <- sender
			return nil
		},
	}
	Functions = append(append([]*common.Function{}, Functions...), function)

	if ok := runScriptTaskCommand("node task-trigger-test.js", "查询 account-a", nil); !ok {
		t.Fatal("script task command was not recognized")
	}
	sender := <-captured
	if got := sender.GetContent(); got != "查询 account-a" {
		t.Fatalf("content = %q, want trigger phrase", got)
	}
	if got := sender.GetMatch(); len(got) != 1 || got[0] != "account-a" {
		t.Fatalf("match = %#v, want account-a", got)
	}
	if err := validateTaskTrigger(function, "不存在的口令"); err == nil {
		t.Fatal("unmatched trigger phrase was accepted")
	}
}
