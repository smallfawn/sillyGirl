package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
)

const storageBucketMarkerKey = "__sillygirl_bucket_marker__"

var protectedStorageBuckets = map[string]string{
	"plugins":   "plugins 存储桶不允许在这里删除",
	"sillyGirl": "sillyGirl 存储桶不允许删除",
	"auths":     "auths 存储桶不允许删除",
}

type storageBucketRequest struct {
	Bucket string `json:"bucket"`
}

func normalizeStorageBucketName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("存储桶名称不能为空")
	}
	if len(name) > 128 {
		return "", errors.New("存储桶名称不能超过128个字符")
	}
	if strings.ContainsAny(name, ".,\r\n\t ") {
		return "", errors.New("存储桶名称不能包含点号、逗号或空白字符")
	}
	return name, nil
}

func checkFilePlugin(key string, value *string) {
	if isNameUuid(key) {
		for _, f := range Functions {
			if f.UUID == key {
				data, _ := os.ReadFile(f.Path)
				*value = string(data)
				return
			}
		}
		// if v, ok := plugins_id.Load(key); ok {

		// } else {
		*value = "非法操作，请勿乱动。"
		// }
	}
}

func shouldHideStorageKey(bucket string, key string) bool {
	return key == storageBucketMarkerKey || isBackendVersionStorageKey(bucket, key)
}

func storageEntryMatchesSearch(key string, value string, search string) bool {
	search = strings.ToLower(strings.TrimSpace(search))
	if search == "" {
		return true
	}
	return strings.Contains(strings.ToLower(key), search) || strings.Contains(strings.ToLower(value), search)
}

func paginationBounds(page int, perPage int, total int) (int, int, int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 200 {
		perPage = 200
	}
	if total < 0 {
		total = 0
	}

	start := total
	if page <= total/perPage+1 {
		start = (page - 1) * perPage
		if start > total {
			start = total
		}
	}
	end := start + perPage
	if end > total {
		end = total
	}
	return page, perPage, start, end
}

func init() {
	var sillyGirl = MakeBucket("sillyGirl")
	GinApi(GET, "/api/admin/storage/list", RequireAuth, func(ctx *gin.Context) {
		page, _ := strconv.Atoi(ctx.DefaultQuery("current", "1"))
		perPage, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
		keys := ctx.Query("keys")
		search := ctx.Query("search")
		data := []map[string]string{}
		arr := strings.Split(keys, ",")
		if keys == "" {
			ApiList(ctx, data, len(data), map[string]interface{}{"page": page})
			return
		}
		for _, bk := range arr {
			ar := strings.SplitN(bk, ".", 2)
			if len(ar) == 2 {
				if isBackendVersionStorageKey(ar[0], ar[1]) {
					continue
				}
				if ar[0] == "plugins" && false { //todo
					// data[bk] = halfDeEct(MakeBucket(ar[0]).GetString(ar[1]))
				} else {
					value := MakeBucket(ar[0]).GetString(ar[1])
					if !storageEntryMatchesSearch(ar[1], value, search) {
						continue
					}
					data = append(data, map[string]string{
						"bucket": ar[0],
						"key":    ar[1],
						"value":  value,
					})
				}
			}
			if len(ar) == 1 {
				MakeBucket(ar[0]).Foreach(func(b1, b2 []byte) error {
					if shouldHideStorageKey(ar[0], string(b1)) {
						return nil
					}
					if !storageEntryMatchesSearch(string(b1), string(b2), search) {
						return nil
					}
					data = append(data, map[string]string{
						"bucket": bk,
						"key":    string(b1),
						"value":  string(b2),
					})
					return nil
				})
			}
		}
		sort.Slice(data, func(i, j int) bool {
			if data[i]["bucket"] == data[j]["bucket"] {
				return data[i]["key"] < data[j]["key"]
			}
			return data[i]["bucket"] < data[j]["bucket"]
		})
		page, perPage, start, end := paginationBounds(page, perPage, len(data))
		res := data[start:end]
		index := start + 1
		for i := range res {
			res[i]["index"] = fmt.Sprint(index)
			index++
		}
		ApiList(ctx, res, len(data), map[string]interface{}{"page": page, "pageSize": perPage})
	})
	GinApi(GET, "/api/admin/storage", RequireAuth, func(ctx *gin.Context) {
		keys := ctx.Query("keys")
		if keys == "" {
			buckets := sillyGirl.Buckets()
			sort.Strings(buckets)
			search := ctx.Query("search")
			res := []map[string]interface{}{}
			if search == "" {
				for _, bucket := range buckets {
					if bucket == "plugins" {
						continue
					}
					res = append(res, map[string]interface{}{
						"value": bucket,
						"text":  "[桶] " + bucket,
					})
				}
				ApiOK(ctx, res)
				return
			}
			for _, bucket := range buckets {
				if bucket == "plugins" {
					continue
				}
				if strings.Contains(bucket, search) {
					res = append(res, map[string]interface{}{
						"value": bucket,
						"text":  "[桶] " + bucket,
					})
				}
				b := MakeBucket(bucket)
				b.Foreach(func(b1, b2 []byte) error {
					key := string(b1)
					if shouldHideStorageKey(bucket, key) {
						return nil
					}
					value := string(b2)
					if strings.Contains(key, search) {
						res = append(res, map[string]interface{}{
							"value": bucket + "." + key,
							"text":  "[键] " + key,
						})
					}
					if strings.Contains(value, search) {
						res = append(res, map[string]interface{}{
							"value": bucket + "." + key,
							"text":  "[值] " + value,
						})
					}
					return nil
				})
			}

			ApiOK(ctx, res)
			return
		}
		data := map[string]interface{}{}
		arr := strings.Split(keys, ",")
		for _, bk := range arr {
			ar := strings.SplitN(bk, ".", 2)
			if len(ar) == 2 {
				if isBackendVersionStorageKey(ar[0], ar[1]) {
					continue
				}
				if ar[0] == "plugins" { //todo
					value := MakeBucket(ar[0]).GetString(ar[1])
					checkFilePlugin(ar[1], &value)
					if IsCdle {
						value = DecryptPlugin(halfDeEct(value))
					}
					data[bk] = value
				} else {
					data[bk] = TransformBucketKeyValue(MakeBucket(ar[0]).GetString(ar[1]))
				}
			}
			if len(ar) == 1 {
				MakeBucket(ar[0]).Foreach(func(b1, b2 []byte) error {
					if shouldHideStorageKey(ar[0], string(b1)) {
						return nil
					}
					data[bk+"."+string(b1)] = TransformBucketKeyValue(string(b2))
					return nil
				})
			}
		}
		ApiOK(ctx, data)
	})
	GinApi(PUT, "/api/admin/storage", RequireAuth, func(ctx *gin.Context) {
		uuid := ctx.Query("uuid")
		if uuid != "" {
			for _, f := range Functions {
				if f.UUID == uuid {
					if f.Reload != nil {
						defer f.Reload() //脚本重载
					} else {
						defer plugins.Set(uuid, "reload")
					}
					break
				}
			}
		}
		data, err := io.ReadAll(ctx.Request.Body)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		updates := map[string]interface{}{}
		err = json.Unmarshal(data, &updates)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		messages := map[string]interface{}{}
		errors := map[string]interface{}{}
		changes := map[string]bool{}
		for bk, v := range updates {
			ar := strings.SplitN(bk, ".", 2)
			if len(ar) == 2 {
				if isBackendVersionStorageKey(ar[0], ar[1]) {
					errors[bk] = "版本信息由后端维护，不允许在存储中修改"
					changes[bk] = false
					continue
				}
				bucket := MakeBucket(ar[0])
				if ar[0] == "plugins" && fmt.Sprint(v) == "install" {
					_, _, _ = SetBucketKeyValue2(bucket, ar[1], "")
				}
				msg, changed, err := SetBucketKeyValue(bucket, ar[1], v)
				if msg != "" {
					messages[bk] = msg
				}
				if err != nil {
					errors[bk] = err.Error()
				}
				changes[bk] = changed

			}
		}
		ApiOK(ctx, map[string]interface{}{
			"messages": messages,
			"errors":   errors,
			"changes":  changes,
		})
	})
	GinApi(POST, "/api/admin/storage/bucket", RequireAuth, func(ctx *gin.Context) {
		req := storageBucketRequest{}
		if err := ctx.BindJSON(&req); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		name, err := normalizeStorageBucketName(req.Bucket)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		for _, bucket := range sillyGirl.Buckets() {
			if bucket == name {
				ApiFail(ctx, "存储桶已存在")
				return
			}
		}
		if _, _, err := MakeBucket(name).Set2(storageBucketMarkerKey, "1"); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, nil)
	})
	GinApi(DELETE, "/api/admin/storage/bucket", RequireAuth, func(ctx *gin.Context) {
		req := storageBucketRequest{}
		if err := ctx.BindJSON(&req); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		name, err := normalizeStorageBucketName(req.Bucket)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		if message, ok := protectedStorageBuckets[name]; ok {
			ApiFail(ctx, message)
			return
		}
		found := false
		for _, bucket := range sillyGirl.Buckets() {
			if bucket == name {
				found = true
				break
			}
		}
		if !found {
			ApiFail(ctx, "存储桶不存在")
			return
		}
		if err := MakeBucket(name).Delete(); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, nil)
	})
}
