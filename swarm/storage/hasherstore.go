
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
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/swarm/chunk"
	"github.com/ethereum/go-ethereum/swarm/storage/encryption"
)

type chunkEncryption struct {
	spanEncryption encryption.Encryption
	dataEncryption encryption.Encryption
}

type hasherStore struct {
	store           ChunkStore
	hashFunc        SwarmHasher
	chunkEncryption *chunkEncryption
hashSize        int   //
refSize         int64 //
	wg              *sync.WaitGroup
	closed          chan struct{}
}

func newChunkEncryption(chunkSize, refSize int64) *chunkEncryption {
	return &chunkEncryption{
		spanEncryption: encryption.New(0, uint32(chunkSize/refSize), sha3.NewKeccak256),
		dataEncryption: encryption.New(int(chunkSize), 0, sha3.NewKeccak256),
	}
}

//
//
//
func NewHasherStore(chunkStore ChunkStore, hashFunc SwarmHasher, toEncrypt bool) *hasherStore {
	var chunkEncryption *chunkEncryption

	hashSize := hashFunc().Size()
	refSize := int64(hashSize)
	if toEncrypt {
		refSize += encryption.KeyLength
		chunkEncryption = newChunkEncryption(chunk.DefaultSize, refSize)
	}

	return &hasherStore{
		store:           chunkStore,
		hashFunc:        hashFunc,
		chunkEncryption: chunkEncryption,
		hashSize:        hashSize,
		refSize:         refSize,
		wg:              &sync.WaitGroup{},
		closed:          make(chan struct{}),
	}
}

//
//
//
func (h *hasherStore) Put(ctx context.Context, chunkData ChunkData) (Reference, error) {
	c := chunkData
	size := chunkData.Size()
	var encryptionKey encryption.Key
	if h.chunkEncryption != nil {
		var err error
		c, encryptionKey, err = h.encryptChunkData(chunkData)
		if err != nil {
			return nil, err
		}
	}
	chunk := h.createChunk(c, size)

	h.storeChunk(ctx, chunk)

	return Reference(append(chunk.Addr, encryptionKey...)), nil
}

//
//
//
func (h *hasherStore) Get(ctx context.Context, ref Reference) (ChunkData, error) {
	key, encryptionKey, err := parseReference(ref, h.hashSize)
	if err != nil {
		return nil, err
	}
	toDecrypt := (encryptionKey != nil)

	chunk, err := h.store.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	chunkData := chunk.SData
	if toDecrypt {
		var err error
		chunkData, err = h.decryptChunkData(chunkData, encryptionKey)
		if err != nil {
			return nil, err
		}
	}
	return chunkData, nil
}

//
//
func (h *hasherStore) Close() {
	close(h.closed)
}

//
//
//
func (h *hasherStore) Wait(ctx context.Context) error {
	<-h.closed
	h.wg.Wait()
	return nil
}

func (h *hasherStore) createHash(chunkData ChunkData) Address {
	hasher := h.hashFunc()
hasher.ResetWithLength(chunkData[:8]) //
hasher.Write(chunkData[8:])           //
	return hasher.Sum(nil)
}

func (h *hasherStore) createChunk(chunkData ChunkData, chunkSize int64) *Chunk {
	hash := h.createHash(chunkData)
	chunk := NewChunk(hash, nil)
	chunk.SData = chunkData
	chunk.Size = chunkSize

	return chunk
}

func (h *hasherStore) encryptChunkData(chunkData ChunkData) (ChunkData, encryption.Key, error) {
	if len(chunkData) < 8 {
		return nil, nil, fmt.Errorf("Invalid ChunkData, min length 8 got %v", len(chunkData))
	}

	encryptionKey, err := encryption.GenerateRandomKey()
	if err != nil {
		return nil, nil, err
	}

	encryptedSpan, err := h.chunkEncryption.spanEncryption.Encrypt(chunkData[:8], encryptionKey)
	if err != nil {
		return nil, nil, err
	}
	encryptedData, err := h.chunkEncryption.dataEncryption.Encrypt(chunkData[8:], encryptionKey)
	if err != nil {
		return nil, nil, err
	}
	c := make(ChunkData, len(encryptedSpan)+len(encryptedData))
	copy(c[:8], encryptedSpan)
	copy(c[8:], encryptedData)
	return c, encryptionKey, nil
}

func (h *hasherStore) decryptChunkData(chunkData ChunkData, encryptionKey encryption.Key) (ChunkData, error) {
	if len(chunkData) < 8 {
		return nil, fmt.Errorf("Invalid ChunkData, min length 8 got %v", len(chunkData))
	}

	decryptedSpan, err := h.chunkEncryption.spanEncryption.Decrypt(chunkData[:8], encryptionKey)
	if err != nil {
		return nil, err
	}

	decryptedData, err := h.chunkEncryption.dataEncryption.Decrypt(chunkData[8:], encryptionKey)
	if err != nil {
		return nil, err
	}

//
	length := ChunkData(decryptedSpan).Size()
	for length > chunk.DefaultSize {
		length = length + (chunk.DefaultSize - 1)
		length = length / chunk.DefaultSize
		length *= h.refSize
	}

	c := make(ChunkData, length+8)
	copy(c[:8], decryptedSpan)
	copy(c[8:], decryptedData[:length])

	return c[:length+8], nil
}

func (h *hasherStore) RefSize() int64 {
	return h.refSize
}

func (h *hasherStore) storeChunk(ctx context.Context, chunk *Chunk) {
	h.wg.Add(1)
	go func() {
		<-chunk.dbStoredC
		h.wg.Done()
	}()
	h.store.Put(ctx, chunk)
}

func parseReference(ref Reference, hashSize int) (Address, encryption.Key, error) {
	encryptedKeyLength := hashSize + encryption.KeyLength
	switch len(ref) {
	case KeyLength:
		return Address(ref), nil, nil
	case encryptedKeyLength:
		encKeyIdx := len(ref) - encryption.KeyLength
		return Address(ref[:encKeyIdx]), encryption.Key(ref[encKeyIdx:]), nil
	default:
		return nil, nil, fmt.Errorf("Invalid reference length, expected %v or %v got %v", hashSize, encryptedKeyLength, len(ref))
	}

}
