
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package mru

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

const serializedUpdateHeaderMultihashHex = "0x4f000000da070000fb0ed7efa696bdb0b54cd75554cc3117ffc891454317df7dd6fefad978e2f2fbf74a10ce8f26ffc8bfaa07c3031a34b2c61f517955e7deb1592daccf96c69cf001"

func getTestUpdateHeader(multihash bool) (header *updateHeader) {
	_, metaHash, _, _ := getTestMetadata().serializeAndHash()
	return &updateHeader{
		UpdateLookup: *getTestUpdateLookup(),
		multihash:    multihash,
		metaHash:     metaHash,
	}
}

func compareUpdateHeader(a, b *updateHeader) bool {
	return compareUpdateLookup(&a.UpdateLookup, &b.UpdateLookup) &&
		a.multihash == b.multihash &&
		bytes.Equal(a.metaHash, b.metaHash)
}

func TestUpdateHeaderSerializer(t *testing.T) {
	header := getTestUpdateHeader(true)
	serializedHeader := make([]byte, updateHeaderLength)
	if err := header.binaryPut(serializedHeader); err != nil {
		t.Fatal(err)
	}
	compareByteSliceToExpectedHex(t, "serializedHeader", serializedHeader, serializedUpdateHeaderMultihashHex)

//
	if err := header.binaryPut(make([]byte, updateHeaderLength+1)); err == nil {
		t.Fatal("Expected updateHeader.binaryPut to fail since supplied slice is of incorrect length")
	}

//
	header.metaHash = nil
	if err := header.binaryPut(serializedHeader); err == nil {
		t.Fatal("Expected updateHeader.binaryPut to fail metaHash is of incorrect length")
	}
}

func TestUpdateHeaderDeserializer(t *testing.T) {
	originalUpdate := getTestUpdateHeader(true)
	serializedData, _ := hexutil.Decode(serializedUpdateHeaderMultihashHex)
	var retrievedUpdate updateHeader
	if err := retrievedUpdate.binaryGet(serializedData); err != nil {
		t.Fatal(err)
	}
	if !compareUpdateHeader(originalUpdate, &retrievedUpdate) {
		t.Fatalf("Expected deserialized structure to equal the original")
	}

//
	serializedData = []byte{1, 2, 3}
	if err := retrievedUpdate.binaryGet(serializedData); err == nil {
		t.Fatal("Expected retrievedUpdate.binaryGet, since passed slice is too small")
	}

}
