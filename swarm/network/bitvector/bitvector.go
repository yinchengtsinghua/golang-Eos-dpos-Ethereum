
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

package bitvector

import (
	"errors"
)

var errInvalidLength = errors.New("invalid length")

type BitVector struct {
	len int
	b   []byte
}

func New(l int) (bv *BitVector, err error) {
	return NewFromBytes(make([]byte, l/8+1), l)
}

func NewFromBytes(b []byte, l int) (bv *BitVector, err error) {
	if l <= 0 {
		return nil, errInvalidLength
	}
	if len(b)*8 < l {
		return nil, errInvalidLength
	}
	return &BitVector{
		len: l,
		b:   b,
	}, nil
}

func (bv *BitVector) Get(i int) bool {
	bi := i / 8
	return bv.b[bi]&(0x1<<uint(i%8)) != 0
}

func (bv *BitVector) Set(i int, v bool) {
	bi := i / 8
	cv := bv.Get(i)
	if cv != v {
		bv.b[bi] ^= 0x1 << uint8(i%8)
	}
}

func (bv *BitVector) Bytes() []byte {
	return bv.b
}

func (bv *BitVector) Length() int {
	return bv.len
}
