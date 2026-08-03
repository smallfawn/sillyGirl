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

	ok := runScriptTaskCommand("node task-recipient-test.js", []Sender{{
		Platfrom: platform,
		BotID:    botID,
		UserID:   userID,
		ChatID:   chatID,
	}})
	if !ok {
		t.Fatal("script task command was not recognized")
	}

	sender := <-captured
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
