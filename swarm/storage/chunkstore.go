
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
	"sync"
)

/*







*/

type ChunkStore interface {
Put(context.Context, *Chunk) //
	Get(context.Context, Address) (*Chunk, error)
	Close()
}

//
type MapChunkStore struct {
	chunks map[string]*Chunk
	mu     sync.RWMutex
}

func NewMapChunkStore() *MapChunkStore {
	return &MapChunkStore{
		chunks: make(map[string]*Chunk),
	}
}

func (m *MapChunkStore) Put(ctx context.Context, chunk *Chunk) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chunks[chunk.Addr.Hex()] = chunk
	chunk.markAsStored()
}

func (m *MapChunkStore) Get(ctx context.Context, addr Address) (*Chunk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	chunk := m.chunks[addr.Hex()]
	if chunk == nil {
		return nil, ErrChunkNotFound
	}
	return chunk, nil
}

func (m *MapChunkStore) Close() {
}
