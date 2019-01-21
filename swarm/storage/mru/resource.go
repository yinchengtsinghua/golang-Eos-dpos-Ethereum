
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//

package mru

import (
	"bytes"
	"context"
	"time"

	"github.com/ethereum/go-ethereum/swarm/storage"
)

const (
	defaultStoreTimeout    = 4000 * time.Millisecond
	hasherCount            = 8
	resourceHashAlgorithm  = storage.SHA3Hash
	defaultRetrieveTimeout = 100 * time.Millisecond
)

//
type resource struct {
	resourceUpdate
	ResourceMetadata
	*bytes.Reader
	lastKey storage.Address
	updated time.Time
}

func (r *resource) Context() context.Context {
	return context.TODO()
}

//
func (r *resource) isSynced() bool {
	return !r.updated.IsZero()
}

//
func (r *resource) Size(ctx context.Context, _ chan bool) (int64, error) {
	if !r.isSynced() {
		return 0, NewError(ErrNotSynced, "Not synced")
	}
	return int64(len(r.resourceUpdate.data)), nil
}

//
func (r *resource) Name() string {
	return r.ResourceMetadata.Name
}

//
func getNextPeriod(start uint64, current uint64, frequency uint64) (uint32, error) {
	if current < start {
		return 0, NewErrorf(ErrInvalidValue, "given current time value %d < start time %d", current, start)
	}
	if frequency == 0 {
		return 0, NewError(ErrInvalidValue, "frequency is 0")
	}
	timeDiff := current - start
	period := timeDiff / frequency
	return uint32(period + 1), nil
}
