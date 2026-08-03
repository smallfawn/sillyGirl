package core

import "testing"

func TestDestroyReplacedAdapterKeepsCurrentFactory(t *testing.T) {
	const (
		platform = "adapter-replace-test"
		botID    = "same-id"
	)
	first := &Factory{}
	first.Init(platform, botID, nil)
	second := &Factory{}
	second.Init(platform, botID, nil)
	first.Destroy()

	BotsLocker.RLock()
	current := Bots[[2]string{platform, botID}]
	BotsLocker.RUnlock()
	if current != second {
		t.Fatalf("current adapter = %p, want replacement %p", current, second)
	}
	second.Destroy()
}
