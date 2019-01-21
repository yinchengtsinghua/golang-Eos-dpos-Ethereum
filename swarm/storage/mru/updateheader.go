
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
	"github.com/ethereum/go-ethereum/swarm/storage"
)

//
type updateHeader struct {
UpdateLookup        //
multihash    bool   //
metaHash     []byte //
}

const metaHashLength = storage.KeyLength

//
//
//
const updateHeaderLength = updateLookupLength + 1 + metaHashLength

//
func (h *updateHeader) binaryPut(serializedData []byte) error {
	if len(serializedData) != updateHeaderLength {
		return NewErrorf(ErrInvalidValue, "Incorrect slice size to serialize updateHeaderLength. Expected %d, got %d", updateHeaderLength, len(serializedData))
	}
	if len(h.metaHash) != metaHashLength {
		return NewError(ErrInvalidValue, "updateHeader.binaryPut called without metaHash set")
	}
	if err := h.UpdateLookup.binaryPut(serializedData[:updateLookupLength]); err != nil {
		return err
	}
	cursor := updateLookupLength
	copy(serializedData[cursor:], h.metaHash[:metaHashLength])
	cursor += metaHashLength

	var flags byte
	if h.multihash {
		flags |= 0x01
	}

	serializedData[cursor] = flags
	cursor++

	return nil
}

//
func (h *updateHeader) binaryLength() int {
	return updateHeaderLength
}

//
func (h *updateHeader) binaryGet(serializedData []byte) error {
	if len(serializedData) != updateHeaderLength {
		return NewErrorf(ErrInvalidValue, "Incorrect slice size to read updateHeaderLength. Expected %d, got %d", updateHeaderLength, len(serializedData))
	}

	if err := h.UpdateLookup.binaryGet(serializedData[:updateLookupLength]); err != nil {
		return err
	}
	cursor := updateLookupLength
	h.metaHash = make([]byte, metaHashLength)
	copy(h.metaHash[:storage.KeyLength], serializedData[cursor:cursor+storage.KeyLength])
	cursor += metaHashLength

	flags := serializedData[cursor]
	cursor++

	h.multihash = flags&0x01 != 0

	return nil
}
