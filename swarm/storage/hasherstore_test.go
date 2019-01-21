
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
	"bytes"
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/storage/encryption"

	"github.com/ethereum/go-ethereum/common"
)

func TestHasherStore(t *testing.T) {
	var tests = []struct {
		chunkLength int
		toEncrypt   bool
	}{
		{10, false},
		{100, false},
		{1000, false},
		{4096, false},
		{10, true},
		{100, true},
		{1000, true},
		{4096, true},
	}

	for _, tt := range tests {
		chunkStore := NewMapChunkStore()
		hasherStore := NewHasherStore(chunkStore, MakeHashFunc(DefaultHash), tt.toEncrypt)

//
		chunkData1 := GenerateRandomChunk(int64(tt.chunkLength)).SData
		key1, err := hasherStore.Put(context.TODO(), chunkData1)
		if err != nil {
			t.Fatalf("Expected no error got \"%v\"", err)
		}

		chunkData2 := GenerateRandomChunk(int64(tt.chunkLength)).SData
		key2, err := hasherStore.Put(context.TODO(), chunkData2)
		if err != nil {
			t.Fatalf("Expected no error got \"%v\"", err)
		}

		hasherStore.Close()

//
		err = hasherStore.Wait(context.TODO())
		if err != nil {
			t.Fatalf("Expected no error got \"%v\"", err)
		}

//
		retrievedChunkData1, err := hasherStore.Get(context.TODO(), key1)
		if err != nil {
			t.Fatalf("Expected no error, got \"%v\"", err)
		}

//
		if !bytes.Equal(chunkData1, retrievedChunkData1) {
			t.Fatalf("Expected retrieved chunk data %v, got %v", common.Bytes2Hex(chunkData1), common.Bytes2Hex(retrievedChunkData1))
		}

//
		retrievedChunkData2, err := hasherStore.Get(context.TODO(), key2)
		if err != nil {
			t.Fatalf("Expected no error, got \"%v\"", err)
		}

//
		if !bytes.Equal(chunkData2, retrievedChunkData2) {
			t.Fatalf("Expected retrieved chunk data %v, got %v", common.Bytes2Hex(chunkData2), common.Bytes2Hex(retrievedChunkData2))
		}

		hash1, encryptionKey1, err := parseReference(key1, hasherStore.hashSize)
		if err != nil {
			t.Fatalf("Expected no error, got \"%v\"", err)
		}

		if tt.toEncrypt {
			if encryptionKey1 == nil {
				t.Fatal("Expected non-nil encryption key, got nil")
			} else if len(encryptionKey1) != encryption.KeyLength {
				t.Fatalf("Expected encryption key length %v, got %v", encryption.KeyLength, len(encryptionKey1))
			}
		}
		if !tt.toEncrypt && encryptionKey1 != nil {
			t.Fatalf("Expected nil encryption key, got key with length %v", len(encryptionKey1))
		}

//
		chunkInStore, err := chunkStore.Get(context.TODO(), hash1)
		if err != nil {
			t.Fatalf("Expected no error got \"%v\"", err)
		}

		chunkDataInStore := chunkInStore.SData

		if tt.toEncrypt && bytes.Equal(chunkData1, chunkDataInStore) {
			t.Fatalf("Chunk expected to be encrypted but it is stored without encryption")
		}
		if !tt.toEncrypt && !bytes.Equal(chunkData1, chunkDataInStore) {
			t.Fatalf("Chunk expected to be not encrypted but stored content is different. Expected %v got %v", common.Bytes2Hex(chunkData1), common.Bytes2Hex(chunkDataInStore))
		}
	}
}
