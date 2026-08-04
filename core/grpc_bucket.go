package core

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/proto3/srpc"
	"github.com/smallfawn/sillyGirl/utils"
)

type SillyGirlService struct {
	srpc.UnsafeSillyGirlServiceServer
}

func (sg *SillyGirlService) BucketWatch(stream srpc.SillyGirlService_BucketWatchServer) error {
	var watcher func(old, new, key string) *storage.Final
	var echos sync.Map
	defer func() {
		watcher = nil
	}()
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		if watcher == nil {
			if isPluginRuntimeBucket(req.Name) {
				return errors.New("插件运行时视图不支持监听")
			}
			watcher = func(old, new, key string) *storage.Final {
				if watcher == nil {
					return nil
				}
				echo := utils.GenUUID()
				ch := make(chan *storage.Final)
				echos.Store(echo, ch)
				defer echos.Delete(echo)
				stream.Send(&srpc.BucketWatchResponse{
					Echo: echo,
					Old:  old,
					Now:  new,
					Key:  key,
				})
				select {
				case v := <-ch:
					return v
				case <-time.After(time.Second * 5):
				}
				return nil
			}
			storage.Watch(MakeBucket(req.Name), req.Key, watcher, req.PluginId)
		} else {
			echo := req.Echo
			v, ok := echos.Load(echo)
			var fn *storage.Final
			if req.Error != "VOID" {
				fn = &storage.Final{
					Now:     req.Now,
					Message: req.Message,
				}
				if req.Error != "" {
					fn.Error = errors.New(req.Error)
				}
			}
			if ok {
				select {
				case v.(chan *storage.Final) <- fn:
				case <-time.After(time.Millisecond):
				}
			}
		}
	}

}

// Get implements BucketServiceServer.Get.
func (sg *SillyGirlService) BucketGet(ctx context.Context, req *srpc.BucketKeyRequest) (*srpc.Default, error) {
	if req.Name == pluginSmallcatRuntimeBucket {
		return &srpc.Default{Value: pluginSmallcatRuntimeValue(pluginIDFromRuntimeContext(ctx), req.Key)}, nil
	}
	if req.Name == pluginUserRuntimeBucket {
		return &srpc.Default{Value: pluginUserRuntimeValue(pluginIDFromRuntimeContext(ctx), req.Key)}, nil
	}
	if isBackendVersionStorageKey(req.Name, req.Key) {
		return &srpc.Default{}, nil
	}
	value := MakeBucket(req.Name).GetString(req.Key)
	return &srpc.Default{Value: value}, nil
}

// Set implements BucketServiceServer.Set.
func (sg *SillyGirlService) BucketSet(ctx context.Context, req *srpc.BucketSetRequest) (*srpc.BucketSetResponse, error) {
	if isPluginRuntimeBucket(req.Name) {
		return nil, errors.New("插件运行时视图为只读")
	}
	if isBackendVersionStorageKey(req.Name, req.Key) {
		message := "版本信息由后端维护，不允许在存储中修改"
		return &srpc.BucketSetResponse{Changed: false, Message: message}, errors.New(message)
	}
	message, changed, err := MakeBucket(req.Name).Set(req.Key, req.Value)
	return &srpc.BucketSetResponse{Changed: changed, Message: message}, err
}

// Delete implements BucketServiceServer.Delete.
func (sg *SillyGirlService) BucketDelete(ctx context.Context, req *srpc.BucketRequest) (*srpc.Empty, error) {
	if isPluginRuntimeBucket(req.Name) {
		return nil, errors.New("插件运行时视图为只读")
	}
	err := MakeBucket(req.Name).Delete()
	return &srpc.Empty{}, err
}

// Keys implements BucketServiceServer.Keys.
func (sg *SillyGirlService) BucketKeys(ctx context.Context, req *srpc.BucketRequest) (*srpc.BucketKeysResponse, error) {
	if req.Name == pluginSmallcatRuntimeBucket {
		return &srpc.BucketKeysResponse{Keys: []string{pluginSmallcatRuntimeRecordKey}}, nil
	}
	if req.Name == pluginUserRuntimeBucket {
		return &srpc.BucketKeysResponse{Keys: []string{pluginUserRuntimeListKey}}, nil
	}
	keys, err := MakeBucket(req.Name).Keys()
	keys = filterBackendVersionStorageKeys(req.Name, keys)
	return &srpc.BucketKeysResponse{Keys: keys}, err
}

// Len implements BucketServiceServer.Len.
func (sg *SillyGirlService) BucketLen(ctx context.Context, req *srpc.BucketRequest) (*srpc.LenResponse, error) {
	if isPluginRuntimeBucket(req.Name) {
		return &srpc.LenResponse{Length: 1}, nil
	}
	keys, err := MakeBucket(req.Name).Keys()
	keys = filterBackendVersionStorageKeys(req.Name, keys)
	return &srpc.LenResponse{Length: int32(len(keys))}, err
}

func (sg *SillyGirlService) BucketGetAll(ctx context.Context, req *srpc.BucketRequest) (*srpc.Default, error) {
	if req.Name == pluginSmallcatRuntimeBucket {
		values := map[string]string{
			pluginSmallcatRuntimeRecordKey: pluginSmallcatRuntimeValue(pluginIDFromRuntimeContext(ctx), pluginSmallcatRuntimeRecordKey),
		}
		return &srpc.Default{Value: string(utils.JsonMarshal(values))}, nil
	}
	if req.Name == pluginUserRuntimeBucket {
		values := map[string]string{
			pluginUserRuntimeListKey: pluginUserRuntimeValue(pluginIDFromRuntimeContext(ctx), pluginUserRuntimeListKey),
		}
		return &srpc.Default{Value: string(utils.JsonMarshal(values))}, nil
	}
	var values = map[string]string{}
	MakeBucket(req.Name).Foreach(func(b1, b2 []byte) error {
		if isBackendVersionStorageKey(req.Name, string(b1)) {
			return nil
		}
		values[string(b1)] = string(b2)
		return nil
	})
	return &srpc.Default{Value: string(utils.JsonMarshal(values))}, nil
}

// Buckets implements BucketServiceServer.Buckets.
func (sg *SillyGirlService) BucketBuckets(ctx context.Context, req *srpc.Empty) (*srpc.BucketsResponse, error) {
	return &srpc.BucketsResponse{Buckets: MakeBucket("app").Buckets()}, nil
}
