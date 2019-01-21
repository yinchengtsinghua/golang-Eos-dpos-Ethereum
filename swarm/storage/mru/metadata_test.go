
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
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

func compareByteSliceToExpectedHex(t *testing.T, variableName string, actualValue []byte, expectedHex string) {
	if hexutil.Encode(actualValue) != expectedHex {
		t.Fatalf("%s: Expected %s to be %s, got %s", t.Name(), variableName, expectedHex, hexutil.Encode(actualValue))
	}
}

func getTestMetadata() *ResourceMetadata {
	return &ResourceMetadata{
		Name: "world news report, every hour, on the hour",
		StartTime: Timestamp{
			Time: 1528880400,
		},
		Frequency: 3600,
		Owner:     newCharlieSigner().Address(),
	}
}

func TestMetadataSerializerDeserializer(t *testing.T) {
	metadata := *getTestMetadata()

rootAddr, metaHash, chunkData, err := metadata.serializeAndHash() //
	if err != nil {
		t.Fatal(err)
	}
	const expectedRootAddr = "0xfb0ed7efa696bdb0b54cd75554cc3117ffc891454317df7dd6fefad978e2f2fb"
	const expectedMetaHash = "0xf74a10ce8f26ffc8bfaa07c3031a34b2c61f517955e7deb1592daccf96c69cf0"
	const expectedChunkData = "0x00004f0010dd205b00000000100e0000000000002a776f726c64206e657773207265706f72742c20657665727920686f75722c206f6e2074686520686f7572876a8936a7cd0b79ef0735ad0896c1afe278781c"

	compareByteSliceToExpectedHex(t, "rootAddr", rootAddr, expectedRootAddr)
	compareByteSliceToExpectedHex(t, "metaHash", metaHash, expectedMetaHash)
	compareByteSliceToExpectedHex(t, "chunkData", chunkData, expectedChunkData)

	recoveredMetadata := ResourceMetadata{}
	recoveredMetadata.binaryGet(chunkData)

	if recoveredMetadata != metadata {
		t.Fatalf("Expected that the recovered metadata equals the marshalled metadata")
	}

//
	backup := make([]byte, len(chunkData))
	copy(backup, chunkData)

	chunkData = []byte{1, 2, 3}
	if err := recoveredMetadata.binaryGet(chunkData); err == nil {
		t.Fatal("Expected binaryGet to fail since chunk is too small")
	}

//
	chunkData = make([]byte, len(backup))
	copy(chunkData, backup)

//
	chunkData[0] = 7
	chunkData[1] = 9

	if err := recoveredMetadata.binaryGet(chunkData); err == nil {
		t.Fatal("Expected binaryGet to fail since prefix bytes are not zero")
	}

//
	chunkData = make([]byte, len(backup))
	copy(chunkData, backup)

//
	chunkData[2] = 255
	chunkData[3] = 44
	if err := recoveredMetadata.binaryGet(chunkData); err == nil {
		t.Fatal("Expected binaryGet to fail since header length does not match")
	}

//
	chunkData = make([]byte, len(backup))
	copy(chunkData, backup)

//
	chunkData[20] = 255
	if err := recoveredMetadata.binaryGet(chunkData); err == nil {
		t.Fatal("Expected binaryGet to fail since name length is incorrect")
	}

//
	chunkData = make([]byte, len(backup))
	copy(chunkData, backup)

//
	chunkData[20] = 3
	if err := recoveredMetadata.binaryGet(chunkData); err == nil {
		t.Fatal("Expected binaryGet to fail since name length is too small")
	}
}

func TestMetadataSerializerLengthCheck(t *testing.T) {
	metadata := *getTestMetadata()

//
	serializedMetadata := make([]byte, 4)

	if err := metadata.binaryPut(serializedMetadata); err == nil {
		t.Fatal("Expected metadata.binaryPut to fail, since target slice is too small")
	}

}
