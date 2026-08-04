package main

import (
	"flag"
	"fmt"

	"github.com/goccy/go-json"
	storagemigrate "github.com/smallfawn/sillyGirl/internal/storage_migrate"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "只打印将迁移的数量，不写入存储")
	flag.Parse()

	root, storageType := storagemigrate.OpenStorage()
	result := storagemigrate.Run(root, storageType, *dryRun)
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	if result.Total == 0 {
		fmt.Println("没有发现需要迁移的旧字段。")
		return
	}
	if *dryRun {
		fmt.Println("dry-run 完成；正式迁移执行：go run ./cmd/storage-migrate")
		return
	}
	fmt.Println("迁移完成；现在可以启动 sillyGirl。")
}
