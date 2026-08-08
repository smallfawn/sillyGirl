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

func TestDestroyAdapterByUUIDRemovesOnlyMatchingFactories(t *testing.T) {
	const (
		pluginID = "adapter-plugin-destroy-test"
		platform = "adapter-plugin-destroy-platform"
	)
	first := &Factory{uuid: pluginID}
	first.Init(platform, "first", nil)
	second := &Factory{uuid: pluginID}
	second.Init(platform, "second", nil)
	other := &Factory{uuid: "another-plugin"}
	other.Init(platform, "other", nil)
	defer other.Destroy()

	DestroyAdapterByUUID(pluginID)

	BotsLocker.RLock()
	_, firstExists := Bots[[2]string{platform, "first"}]
	_, secondExists := Bots[[2]string{platform, "second"}]
	currentOther := Bots[[2]string{platform, "other"}]
	BotsLocker.RUnlock()
	if firstExists || secondExists {
		t.Fatal("plugin adapters remained registered after UUID cleanup")
	}
	if currentOther != other {
		t.Fatalf("unrelated adapter = %p, want %p", currentOther, other)
	}
}
