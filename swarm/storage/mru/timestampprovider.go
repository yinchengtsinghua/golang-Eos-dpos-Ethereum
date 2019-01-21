
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
	"encoding/binary"
	"time"
)

//
var TimestampProvider timestampProvider = NewDefaultTimestampProvider()

//
type Timestamp struct {
Time uint64 //
}

//
const timestampLength = 8

//
type timestampProvider interface {
Now() Timestamp //
}

//
func (t *Timestamp) binaryGet(data []byte) error {
	if len(data) != timestampLength {
		return NewError(ErrCorruptData, "timestamp data has the wrong size")
	}
	t.Time = binary.LittleEndian.Uint64(data[:8])
	return nil
}

//
func (t *Timestamp) binaryPut(data []byte) error {
	if len(data) != timestampLength {
		return NewError(ErrCorruptData, "timestamp data has the wrong size")
	}
	binary.LittleEndian.PutUint64(data, t.Time)
	return nil
}

type DefaultTimestampProvider struct {
}

//
func NewDefaultTimestampProvider() *DefaultTimestampProvider {
	return &DefaultTimestampProvider{}
}

//
func (dtp *DefaultTimestampProvider) Now() Timestamp {
	return Timestamp{
		Time: uint64(time.Now().Unix()),
	}
}
