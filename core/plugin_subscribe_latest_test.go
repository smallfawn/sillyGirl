package core

import (
	"testing"

	"github.com/smallfawn/sillyGirl/core/common"
)

func TestLatestPluginMarketItems(t *testing.T) {
	items := []*common.Function{
		{Title: "旧插件", CreateAt: "2025-01-01T00:00:00Z"},
		{Title: "无日期"},
		{Title: "最新插件", CreateAt: "2026-08-06T10:00:00+08:00"},
		{Title: "次新插件", CreateAt: "2026-08-05T10:00:00+08:00"},
		{Title: "坏日期", CreateAt: "today"},
	}
	got := latestPluginMarketItems(items, 2)
	if len(got) != 2 || got[0].Title != "最新插件" || got[1].Title != "次新插件" {
		t.Fatalf("latestPluginMarketItems() = %#v", got)
	}
}

func TestLatestPluginMarketItemsWithoutLimit(t *testing.T) {
	items := []*common.Function{
		{Title: "B", CreateAt: "2026-08-06T10:00:00Z"},
		{Title: "A", CreateAt: "2026-08-06T10:00:00Z"},
	}
	got := latestPluginMarketItems(items, 0)
	if len(got) != 2 || got[0].Title != "A" || got[1].Title != "B" {
		t.Fatalf("same-time ordering = %#v", got)
	}
}
