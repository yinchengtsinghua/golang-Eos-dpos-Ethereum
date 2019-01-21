
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
	"encoding/binary"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage/mock"
)

type LocalStoreParams struct {
	*StoreParams
	ChunkDbPath string
	Validators  []ChunkValidator `toml:"-"`
}

func NewDefaultLocalStoreParams() *LocalStoreParams {
	return &LocalStoreParams{
		StoreParams: NewDefaultStoreParams(),
	}
}

//
//
func (p *LocalStoreParams) Init(path string) {
	if p.ChunkDbPath == "" {
		p.ChunkDbPath = filepath.Join(path, "chunks")
	}
}

//
//
type LocalStore struct {
	Validators []ChunkValidator
	memStore   *MemStore
	DbStore    *LDBStore
	mu         sync.Mutex
}

//
func NewLocalStore(params *LocalStoreParams, mockStore *mock.NodeStore) (*LocalStore, error) {
	ldbparams := NewLDBStoreParams(params.StoreParams, params.ChunkDbPath)
	dbStore, err := NewMockDbStore(ldbparams, mockStore)
	if err != nil {
		return nil, err
	}
	return &LocalStore{
		memStore:   NewMemStore(params.StoreParams, dbStore),
		DbStore:    dbStore,
		Validators: params.Validators,
	}, nil
}

func NewTestLocalStoreForAddr(params *LocalStoreParams) (*LocalStore, error) {
	ldbparams := NewLDBStoreParams(params.StoreParams, params.ChunkDbPath)
	dbStore, err := NewLDBStore(ldbparams)
	if err != nil {
		return nil, err
	}
	localStore := &LocalStore{
		memStore:   NewMemStore(params.StoreParams, dbStore),
		DbStore:    dbStore,
		Validators: params.Validators,
	}
	return localStore, nil
}

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
func (ls *LocalStore) Put(ctx context.Context, chunk *Chunk) {
	valid := true
//
//
	for _, v := range ls.Validators {
		if valid = v.Validate(chunk.Addr, chunk.SData); valid {
			break
		}
	}
	if !valid {
		log.Trace("invalid chunk", "addr", chunk.Addr, "len", len(chunk.SData))
		chunk.SetErrored(ErrChunkInvalid)
		chunk.markAsStored()
		return
	}

	log.Trace("localstore.put", "addr", chunk.Addr)

	ls.mu.Lock()
	defer ls.mu.Unlock()

	chunk.Size = int64(binary.LittleEndian.Uint64(chunk.SData[0:8]))

	memChunk, err := ls.memStore.Get(ctx, chunk.Addr)
	switch err {
	case nil:
		if memChunk.ReqC == nil {
			chunk.markAsStored()
			return
		}
	case ErrChunkNotFound:
	default:
		chunk.SetErrored(err)
		return
	}

	ls.DbStore.Put(ctx, chunk)

//
	newc := NewChunk(chunk.Addr, nil)
	newc.SData = chunk.SData
	newc.Size = chunk.Size
	newc.dbStoredC = chunk.dbStoredC

	ls.memStore.Put(ctx, newc)

	if memChunk != nil && memChunk.ReqC != nil {
		close(memChunk.ReqC)
	}
}

//
//
//
//
func (ls *LocalStore) Get(ctx context.Context, addr Address) (chunk *Chunk, err error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	return ls.get(ctx, addr)
}

func (ls *LocalStore) get(ctx context.Context, addr Address) (chunk *Chunk, err error) {
	chunk, err = ls.memStore.Get(ctx, addr)
	if err == nil {
		if chunk.ReqC != nil {
			select {
			case <-chunk.ReqC:
			default:
				metrics.GetOrRegisterCounter("localstore.get.errfetching", nil).Inc(1)
				return chunk, ErrFetching
			}
		}
		metrics.GetOrRegisterCounter("localstore.get.cachehit", nil).Inc(1)
		return
	}
	metrics.GetOrRegisterCounter("localstore.get.cachemiss", nil).Inc(1)
	chunk, err = ls.DbStore.Get(ctx, addr)
	if err != nil {
		metrics.GetOrRegisterCounter("localstore.get.error", nil).Inc(1)
		return
	}
	chunk.Size = int64(binary.LittleEndian.Uint64(chunk.SData[0:8]))
	ls.memStore.Put(ctx, chunk)
	return
}

//
func (ls *LocalStore) GetOrCreateRequest(ctx context.Context, addr Address) (chunk *Chunk, created bool) {
	metrics.GetOrRegisterCounter("localstore.getorcreaterequest", nil).Inc(1)

	ls.mu.Lock()
	defer ls.mu.Unlock()

	var err error
	chunk, err = ls.get(ctx, addr)
	if err == nil && chunk.GetErrored() == nil {
		metrics.GetOrRegisterCounter("localstore.getorcreaterequest.hit", nil).Inc(1)
		log.Trace(fmt.Sprintf("LocalStore.GetOrRetrieve: %v found locally", addr))
		return chunk, false
	}
	if err == ErrFetching && chunk.GetErrored() == nil {
		metrics.GetOrRegisterCounter("localstore.getorcreaterequest.errfetching", nil).Inc(1)
		log.Trace(fmt.Sprintf("LocalStore.GetOrRetrieve: %v hit on an existing request %v", addr, chunk.ReqC))
		return chunk, false
	}
//
	metrics.GetOrRegisterCounter("localstore.getorcreaterequest.miss", nil).Inc(1)
	log.Trace(fmt.Sprintf("LocalStore.GetOrRetrieve: %v not found locally. open new request", addr))
	chunk = NewChunk(addr, make(chan bool))
	ls.memStore.Put(ctx, chunk)
	return chunk, true
}

//
func (ls *LocalStore) RequestsCacheLen() int {
	return ls.memStore.requests.Len()
}

//
func (ls *LocalStore) Close() {
	ls.DbStore.Close()
}
