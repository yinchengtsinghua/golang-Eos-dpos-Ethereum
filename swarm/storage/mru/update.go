
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
	"errors"

	"github.com/ethereum/go-ethereum/swarm/chunk"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/multihash"
)

//
type resourceUpdate struct {
updateHeader        //
data         []byte //
}

//
//
//
//
const chunkPrefixLength = 2 + 2

//
//
//
//
//
const minimumUpdateDataLength = updateHeaderLength + 1
const maxUpdateDataLength = chunk.DefaultSize - signatureLength - updateHeaderLength - chunkPrefixLength

//
func (r *resourceUpdate) binaryPut(serializedData []byte) error {
	datalength := len(r.data)
	if datalength == 0 {
		return NewError(ErrInvalidValue, "cannot update a resource with no data")
	}

	if datalength > maxUpdateDataLength {
		return NewErrorf(ErrInvalidValue, "data is too big (length=%d). Max length=%d", datalength, maxUpdateDataLength)
	}

	if len(serializedData) != r.binaryLength() {
		return NewErrorf(ErrInvalidValue, "slice passed to putBinary must be of exact size. Expected %d bytes", r.binaryLength())
	}

	if r.multihash {
		if _, _, err := multihash.GetMultihashLength(r.data); err != nil {
			return NewError(ErrInvalidValue, "Invalid multihash")
		}
	}

//
	cursor := 0
	binary.LittleEndian.PutUint16(serializedData[cursor:], uint16(updateHeaderLength))
	cursor += 2

//
	binary.LittleEndian.PutUint16(serializedData[cursor:], uint16(datalength))
	cursor += 2

//
	if err := r.updateHeader.binaryPut(serializedData[cursor : cursor+updateHeaderLength]); err != nil {
		return err
	}
	cursor += updateHeaderLength

//
	copy(serializedData[cursor:], r.data)
	cursor += datalength

	return nil
}

//
func (r *resourceUpdate) binaryLength() int {
	return chunkPrefixLength + updateHeaderLength + len(r.data)
}

//
func (r *resourceUpdate) binaryGet(serializedData []byte) error {
	if len(serializedData) < minimumUpdateDataLength {
		return NewErrorf(ErrNothingToReturn, "chunk less than %d bytes cannot be a resource update chunk", minimumUpdateDataLength)
	}
	cursor := 0
	declaredHeaderlength := binary.LittleEndian.Uint16(serializedData[cursor : cursor+2])
	if declaredHeaderlength != updateHeaderLength {
		return NewErrorf(ErrCorruptData, "Invalid header length. Expected %d, got %d", updateHeaderLength, declaredHeaderlength)
	}

	cursor += 2
	datalength := int(binary.LittleEndian.Uint16(serializedData[cursor : cursor+2]))
	cursor += 2

	if chunkPrefixLength+updateHeaderLength+datalength+signatureLength != len(serializedData) {
		return NewError(ErrNothingToReturn, "length specified in header is different than actual chunk size")
	}

//
	if err := r.updateHeader.binaryGet(serializedData[cursor : cursor+updateHeaderLength]); err != nil {
		return err
	}
	cursor += updateHeaderLength

	data := serializedData[cursor : cursor+datalength]
	cursor += datalength

//
	if r.updateHeader.multihash {
		mhLength, mhHeaderLength, err := multihash.GetMultihashLength(data)
		if err != nil {
			log.Error("multihash parse error", "err", err)
			return err
		}
		if datalength != mhLength+mhHeaderLength {
			log.Debug("multihash error", "datalength", datalength, "mhLength", mhLength, "mhHeaderLength", mhHeaderLength)
			return errors.New("Corrupt multihash data")
		}
	}

//
	r.data = make([]byte, datalength)
	copy(r.data, data)

	return nil

}

//
func (r *resourceUpdate) Multihash() bool {
	return r.multihash
}
