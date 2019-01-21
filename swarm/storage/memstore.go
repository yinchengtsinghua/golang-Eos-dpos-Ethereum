
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

//

package storage

import (
	"context"
	"sync"

	lru "github.com/hashicorp/golang-lru"
)

type MemStore struct {
	cache    *lru.Cache
	requests *lru.Cache
	mu       sync.RWMutex
	disabled bool
}

//
//
//
//
//
//
func NewMemStore(params *StoreParams, _ *LDBStore) (m *MemStore) {
	if params.CacheCapacity == 0 {
		return &MemStore{
			disabled: true,
		}
	}

	onEvicted := func(key interface{}, value interface{}) {
		v := value.(*Chunk)
		<-v.dbStoredC
	}
	c, err := lru.NewWithEvict(int(params.CacheCapacity), onEvicted)
	if err != nil {
		panic(err)
	}

	requestEvicted := func(key interface{}, value interface{}) {
//
//
	}
	r, err := lru.NewWithEvict(int(params.ChunkRequestsCacheCapacity), requestEvicted)
	if err != nil {
		panic(err)
	}

	return &MemStore{
		cache:    c,
		requests: r,
	}
}

func (m *MemStore) Get(ctx context.Context, addr Address) (*Chunk, error) {
	if m.disabled {
		return nil, ErrChunkNotFound
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.requests.Get(string(addr))
//
	if ok {
		return r.(*Chunk), nil
	}

//
	c, ok := m.cache.Get(string(addr))
	if !ok {
		return nil, ErrChunkNotFound
	}
	return c.(*Chunk), nil
}

func (m *MemStore) Put(ctx context.Context, c *Chunk) {
	if m.disabled {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

//
	if c.ReqC != nil {
		select {
		case <-c.ReqC:
			if c.GetErrored() != nil {
				m.requests.Remove(string(c.Addr))
				return
			}
			m.cache.Add(string(c.Addr), c)
			m.requests.Remove(string(c.Addr))
		default:
			m.requests.Add(string(c.Addr), c)
		}
		return
	}

//
	m.cache.Add(string(c.Addr), c)
	m.requests.Remove(string(c.Addr))
}

func (m *MemStore) setCapacity(n int) {
	if n <= 0 {
		m.disabled = true
	} else {
		onEvicted := func(key interface{}, value interface{}) {
			v := value.(*Chunk)
			<-v.dbStoredC
		}
		c, err := lru.NewWithEvict(n, onEvicted)
		if err != nil {
			panic(err)
		}

		r, err := lru.New(defaultChunkRequestsCacheCapacity)
		if err != nil {
			panic(err)
		}

		m = &MemStore{
			cache:    c,
			requests: r,
		}
	}
}

func (s *MemStore) Close() {}
