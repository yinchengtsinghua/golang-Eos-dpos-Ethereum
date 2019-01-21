
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

package storage

import (
	"errors"
)

const (
	ErrInit = iota
	ErrNotFound
	ErrIO
	ErrUnauthorized
	ErrInvalidValue
	ErrDataOverflow
	ErrNothingToReturn
	ErrCorruptData
	ErrInvalidSignature
	ErrNotSynced
	ErrPeriodDepth
	ErrCnt
)

var (
	ErrChunkNotFound    = errors.New("chunk not found")
	ErrFetching         = errors.New("chunk still fetching")
	ErrChunkInvalid     = errors.New("invalid chunk")
	ErrChunkForward     = errors.New("cannot forward")
	ErrChunkUnavailable = errors.New("chunk unavailable")
	ErrChunkTimeout     = errors.New("timeout")
)
