
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
	"crypto/rand"
	"encoding/binary"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/log"
)

func newTestMemStore() *MemStore {
	storeparams := NewDefaultStoreParams()
	return NewMemStore(storeparams, nil)
}

func testMemStoreRandom(n int, processors int, chunksize int64, t *testing.T) {
	m := newTestMemStore()
	defer m.Close()
	testStoreRandom(m, processors, n, chunksize, t)
}

func testMemStoreCorrect(n int, processors int, chunksize int64, t *testing.T) {
	m := newTestMemStore()
	defer m.Close()
	testStoreCorrect(m, processors, n, chunksize, t)
}

func TestMemStoreRandom_1(t *testing.T) {
	testMemStoreRandom(1, 1, 0, t)
}

func TestMemStoreCorrect_1(t *testing.T) {
	testMemStoreCorrect(1, 1, 4104, t)
}

func TestMemStoreRandom_1_1k(t *testing.T) {
	testMemStoreRandom(1, 1000, 0, t)
}

func TestMemStoreCorrect_1_1k(t *testing.T) {
	testMemStoreCorrect(1, 100, 4096, t)
}

func TestMemStoreRandom_8_1k(t *testing.T) {
	testMemStoreRandom(8, 1000, 0, t)
}

func TestMemStoreCorrect_8_1k(t *testing.T) {
	testMemStoreCorrect(8, 1000, 4096, t)
}

func TestMemStoreNotFound(t *testing.T) {
	m := newTestMemStore()
	defer m.Close()

	_, err := m.Get(context.TODO(), ZeroAddr)
	if err != ErrChunkNotFound {
		t.Errorf("Expected ErrChunkNotFound, got %v", err)
	}
}

func benchmarkMemStorePut(n int, processors int, chunksize int64, b *testing.B) {
	m := newTestMemStore()
	defer m.Close()
	benchmarkStorePut(m, processors, n, chunksize, b)
}

func benchmarkMemStoreGet(n int, processors int, chunksize int64, b *testing.B) {
	m := newTestMemStore()
	defer m.Close()
	benchmarkStoreGet(m, processors, n, chunksize, b)
}

func BenchmarkMemStorePut_1_500(b *testing.B) {
	benchmarkMemStorePut(500, 1, 4096, b)
}

func BenchmarkMemStorePut_8_500(b *testing.B) {
	benchmarkMemStorePut(500, 8, 4096, b)
}

func BenchmarkMemStoreGet_1_500(b *testing.B) {
	benchmarkMemStoreGet(500, 1, 4096, b)
}

func BenchmarkMemStoreGet_8_500(b *testing.B) {
	benchmarkMemStoreGet(500, 8, 4096, b)
}

func newLDBStore(t *testing.T) (*LDBStore, func()) {
	dir, err := ioutil.TempDir("", "bzz-storage-test")
	if err != nil {
		t.Fatal(err)
	}
	log.Trace("memstore.tempdir", "dir", dir)

	ldbparams := NewLDBStoreParams(NewDefaultStoreParams(), dir)
	db, err := NewLDBStore(ldbparams)
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		db.Close()
		err := os.RemoveAll(dir)
		if err != nil {
			t.Fatal(err)
		}
	}

	return db, cleanup
}

func TestMemStoreAndLDBStore(t *testing.T) {
	ldb, cleanup := newLDBStore(t)
	ldb.setCapacity(4000)
	defer cleanup()

	cacheCap := 200
	requestsCap := 200
	memStore := NewMemStore(NewStoreParams(4000, 200, 200, nil, nil), nil)

	tests := []struct {
n         int    //
chunkSize uint64 //
request   bool   //
	}{
		{
			n:         1,
			chunkSize: 4096,
			request:   false,
		},
		{
			n:         201,
			chunkSize: 4096,
			request:   false,
		},
		{
			n:         501,
			chunkSize: 4096,
			request:   false,
		},
		{
			n:         3100,
			chunkSize: 4096,
			request:   false,
		},
		{
			n:         100,
			chunkSize: 4096,
			request:   true,
		},
	}

	for i, tt := range tests {
		log.Info("running test", "idx", i, "tt", tt)
		var chunks []*Chunk

		for i := 0; i < tt.n; i++ {
			var c *Chunk
			if tt.request {
				c = NewRandomRequestChunk(tt.chunkSize)
			} else {
				c = NewRandomChunk(tt.chunkSize)
			}

			chunks = append(chunks, c)
		}

		for i := 0; i < tt.n; i++ {
			go ldb.Put(context.TODO(), chunks[i])
			memStore.Put(context.TODO(), chunks[i])

			if got := memStore.cache.Len(); got > cacheCap {
				t.Fatalf("expected to get cache capacity less than %v, but got %v", cacheCap, got)
			}

			if got := memStore.requests.Len(); got > requestsCap {
				t.Fatalf("expected to get requests capacity less than %v, but got %v", requestsCap, got)
			}
		}

		for i := 0; i < tt.n; i++ {
			_, err := memStore.Get(context.TODO(), chunks[i].Addr)
			if err != nil {
				if err == ErrChunkNotFound {
					_, err := ldb.Get(context.TODO(), chunks[i].Addr)
					if err != nil {
						t.Fatalf("couldn't get chunk %v from ldb, got error: %v", i, err)
					}
				} else {
					t.Fatalf("got error from memstore: %v", err)
				}
			}
		}

//
		for i := 0; i < tt.n; i++ {
			<-chunks[i].dbStoredC
		}
	}
}

func NewRandomChunk(chunkSize uint64) *Chunk {
	c := &Chunk{
		Addr:       make([]byte, 32),
		ReqC:       nil,
SData:      make([]byte, chunkSize+8), //
		dbStoredC:  make(chan bool),
		dbStoredMu: &sync.Mutex{},
	}

	rand.Read(c.SData)

	binary.LittleEndian.PutUint64(c.SData[:8], chunkSize)

	hasher := MakeHashFunc(SHA3Hash)()
	hasher.Write(c.SData)
	copy(c.Addr, hasher.Sum(nil))

	return c
}

func NewRandomRequestChunk(chunkSize uint64) *Chunk {
	c := NewRandomChunk(chunkSize)
	c.ReqC = make(chan bool)

	return c
}
