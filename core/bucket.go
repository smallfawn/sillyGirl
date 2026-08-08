package core

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/goccy/go-json"
	"github.com/smallfawn/sillyGirl/core/logs"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/core/storage/boltdb"
	"github.com/smallfawn/sillyGirl/core/storage/redis"
	"github.com/smallfawn/sillyGirl/utils"
)

var bkt storage.Bucket
var HttpPort string
var sillyGirl = MakeBucket("sillyGirl")

var MakeBucketlocker sync.Mutex

func MakeBucket(name string) storage.Bucket {
	MakeBucketlocker.Lock()
	defer MakeBucketlocker.Unlock()
	if bkt == nil {
		bkt = boltdb.InitsillyGirl()
		var app = bkt
		isredis := false
		if def := bkt.GetString("storage"); def == "redis" {
			func() {
				defer func() {
					err := recover()
					if err != nil {
						logs.Warn("Redis 初始化失败，已回退到 BoltDB：%v", err)
						bkt = app
					} else {
						isredis = true
						logs.Info("已使用redis进行数据存储")
					}
				}()
				bkt = redis.InitsillyGirl(app.GetString("redis_addr"), app.GetString("redis_password"))
			}()
		} else {
			if def != "boltdb" {
				bkt.Set("storage", "boltdb")
			}
			logs.Info("默认使用boltdb进行数据存储")
		}
		storage.Watch(bkt, "storage", func(old, new, key string) *storage.Final {
			if isredis {
				if new == "boltdb" {
					app.Set2(key, new)
					return &storage.Final{
						Message: "重启生效！",
					}
				}
			} else {
				if new != "redis" {
					return nil
				}
				message := "Redis连接成功，重启生效！"
				err := redis.Try(app.GetString("redis_addr"), app.GetString("redis_password"))
				if err != nil {
					message = "Redis连接失败，操作无效：" + err.Error()
					return &storage.Final{
						Error: errors.New(message),
					}
				} else {
					return &storage.Final{
						Message: message,
					}
				}
			}
			return nil
		})
		if !isredis {
			storage.Watch(app, "redis_addr", func(old, new, _ string) *storage.Final {
				message := "Redis连接成功，重启生效！"
				err := redis.Try(new, app.GetString("redis_password"))
				if err != nil {
					message = "Redis连接失败：" + err.Error()
				} else {
					app.Set2("storage", "redis")
				}
				return &storage.Final{
					Message: message,
				}
			})
			storage.Watch(app, "redis_password", func(old, new, _ string) *storage.Final {
				message := "Redis连接成功，重启生效！"
				err := redis.Try(app.GetString("redis_addr"), new)
				if err != nil {
					message = "Redis连接失败：" + err.Error()
				} else {
					app.Set2("storage", "redis")
				}
				return &storage.Final{
					Message: message,
				}
			})
		}
		for _, name := range bkt.Buckets() {
			b := bkt.Copy(name)
			keys, err := b.Keys()
			if len(keys) == 0 && err == nil {
				b.Delete()
			}
		}

	}
	if name == "" {
		name = "sillyGirl"
	}
	if name == "silly" || name == "app" {
		name = "sillyGirl"
	}
	return bkt.Copy(name)
}

func TransformBucketKeyValue(v string) interface{} {
	var result interface{}
	if strings.HasPrefix(v, "f:") {
		result, _ = strconv.ParseFloat(strings.Replace(v, "f:", "", 1), 64)
		return result
	}
	if strings.HasPrefix(v, "d:") {
		result = utils.Int(strings.Replace(v, "d:", "", 1))
		return result
	}
	if strings.HasPrefix(v, "b:") {
		result = strings.Replace(v, "b:", "", 1) == "true"
		return result
	}
	if strings.HasPrefix(v, "o:") {
		json.Unmarshal([]byte(strings.Replace(v, "o:", "", 1)), &result)
		return result
	}
	if v == "" {
		return nil
	}
	return v
}

func GetBucketKeyValue(bucket storage.Bucket, ps ...interface{}) interface{} {
	var key interface{}
	var value interface{}
	if len(ps) == 0 {
		return nil
	}
	if len(ps) > 0 {
		key = ps[0]
	}
	if len(ps) > 1 {
		value = ps[1]
	}
	v := bucket.GetString(key)
	var result = TransformBucketKeyValue(v)
	if result == nil {
		return value
	}
	return result
}

func SetBucketKeyValue(bucket storage.Bucket, key interface{}, value interface{}) (string, bool, error) {
	return bucket.Set(key, encodeBucketValue(value))
}

func SetBucketKeyValue2(bucket storage.Bucket, key interface{}, value interface{}) (string, bool, error) {
	return bucket.Set2(key, encodeBucketValue(value))
}

func encodeBucketValue(value interface{}) string {
	switch value := value.(type) {
	case int, int64, int32, uint:
		return fmt.Sprintf("d:%d", value)
	case float32:
		return encodeBucketFloat(float64(value))
	case float64:
		return encodeBucketFloat(value)
	case string, []byte:
		return fmt.Sprintf("%s", value)
	case bool:
		return fmt.Sprintf("b:%t", value)
	case nil:
		return ""
	default:
		return fmt.Sprintf("o:%s", utils.JsonMarshal(value))
	}
}

func encodeBucketFloat(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "f:0.000000"
	}
	if math.Trunc(value) == value && value >= math.MinInt64 && value <= math.MaxInt64 {
		return fmt.Sprintf("d:%d", int64(value))
	}
	return fmt.Sprintf("f:%f", value)
}
