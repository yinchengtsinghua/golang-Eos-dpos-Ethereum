
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
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/chunk"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage/mock/mem"

	ldberrors "github.com/syndtr/goleveldb/leveldb/errors"
)

type testDbStore struct {
	*LDBStore
	dir string
}

func newTestDbStore(mock bool, trusted bool) (*testDbStore, func(), error) {
	dir, err := ioutil.TempDir("", "bzz-storage-test")
	if err != nil {
		return nil, func() {}, err
	}

	var db *LDBStore
	storeparams := NewDefaultStoreParams()
	params := NewLDBStoreParams(storeparams, dir)
	params.Po = testPoFunc

	if mock {
		globalStore := mem.NewGlobalStore()
		addr := common.HexToAddress("0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed")
		mockStore := globalStore.NewNodeStore(addr)

		db, err = NewMockDbStore(params, mockStore)
	} else {
		db, err = NewLDBStore(params)
	}

	cleanup := func() {
		if db != nil {
			db.Close()
		}
		err = os.RemoveAll(dir)
		if err != nil {
			panic(fmt.Sprintf("db cleanup failed: %v", err))
		}
	}

	return &testDbStore{db, dir}, cleanup, err
}

func testPoFunc(k Address) (ret uint8) {
	basekey := make([]byte, 32)
	return uint8(Proximity(basekey[:], k[:]))
}

func (db *testDbStore) close() {
	db.Close()
	err := os.RemoveAll(db.dir)
	if err != nil {
		panic(err)
	}
}

func testDbStoreRandom(n int, processors int, chunksize int64, mock bool, t *testing.T) {
	db, cleanup, err := newTestDbStore(mock, true)
	defer cleanup()
	if err != nil {
		t.Fatalf("init dbStore failed: %v", err)
	}
	testStoreRandom(db, processors, n, chunksize, t)
}

func testDbStoreCorrect(n int, processors int, chunksize int64, mock bool, t *testing.T) {
	db, cleanup, err := newTestDbStore(mock, false)
	defer cleanup()
	if err != nil {
		t.Fatalf("init dbStore failed: %v", err)
	}
	testStoreCorrect(db, processors, n, chunksize, t)
}

func TestDbStoreRandom_1(t *testing.T) {
	testDbStoreRandom(1, 1, 0, false, t)
}

func TestDbStoreCorrect_1(t *testing.T) {
	testDbStoreCorrect(1, 1, 4096, false, t)
}

func TestDbStoreRandom_1_5k(t *testing.T) {
	testDbStoreRandom(8, 5000, 0, false, t)
}

func TestDbStoreRandom_8_5k(t *testing.T) {
	testDbStoreRandom(8, 5000, 0, false, t)
}

func TestDbStoreCorrect_1_5k(t *testing.T) {
	testDbStoreCorrect(1, 5000, 4096, false, t)
}

func TestDbStoreCorrect_8_5k(t *testing.T) {
	testDbStoreCorrect(8, 5000, 4096, false, t)
}

func TestMockDbStoreRandom_1(t *testing.T) {
	testDbStoreRandom(1, 1, 0, true, t)
}

func TestMockDbStoreCorrect_1(t *testing.T) {
	testDbStoreCorrect(1, 1, 4096, true, t)
}

func TestMockDbStoreRandom_1_5k(t *testing.T) {
	testDbStoreRandom(8, 5000, 0, true, t)
}

func TestMockDbStoreRandom_8_5k(t *testing.T) {
	testDbStoreRandom(8, 5000, 0, true, t)
}

func TestMockDbStoreCorrect_1_5k(t *testing.T) {
	testDbStoreCorrect(1, 5000, 4096, true, t)
}

func TestMockDbStoreCorrect_8_5k(t *testing.T) {
	testDbStoreCorrect(8, 5000, 4096, true, t)
}

func testDbStoreNotFound(t *testing.T, mock bool) {
	db, cleanup, err := newTestDbStore(mock, false)
	defer cleanup()
	if err != nil {
		t.Fatalf("init dbStore failed: %v", err)
	}

	_, err = db.Get(context.TODO(), ZeroAddr)
	if err != ErrChunkNotFound {
		t.Errorf("Expected ErrChunkNotFound, got %v", err)
	}
}

func TestDbStoreNotFound(t *testing.T) {
	testDbStoreNotFound(t, false)
}
func TestMockDbStoreNotFound(t *testing.T) {
	testDbStoreNotFound(t, true)
}

func testIterator(t *testing.T, mock bool) {
	var chunkcount int = 32
	var i int
	var poc uint
	chunkkeys := NewAddressCollection(chunkcount)
	chunkkeys_results := NewAddressCollection(chunkcount)

	db, cleanup, err := newTestDbStore(mock, false)
	defer cleanup()
	if err != nil {
		t.Fatalf("init dbStore failed: %v", err)
	}

	chunks := GenerateRandomChunks(chunk.DefaultSize, chunkcount)

	wg := &sync.WaitGroup{}
	wg.Add(len(chunks))
	for i = 0; i < len(chunks); i++ {
		db.Put(context.TODO(), chunks[i])
		chunkkeys[i] = chunks[i].Addr
		j := i
		go func() {
			defer wg.Done()
			<-chunks[j].dbStoredC
		}()
	}

//

	for i = 0; i < len(chunkkeys); i++ {
		log.Trace(fmt.Sprintf("Chunk array pos %d/%d: '%v'", i, chunkcount, chunkkeys[i]))
	}
	wg.Wait()
	i = 0
	for poc = 0; poc <= 255; poc++ {
		err := db.SyncIterator(0, uint64(chunkkeys.Len()), uint8(poc), func(k Address, n uint64) bool {
			log.Trace(fmt.Sprintf("Got key %v number %d poc %d", k, n, uint8(poc)))
			chunkkeys_results[n-1] = k
			i++
			return true
		})
		if err != nil {
			t.Fatalf("Iterator call failed: %v", err)
		}
	}

	for i = 0; i < chunkcount; i++ {
		if !bytes.Equal(chunkkeys[i], chunkkeys_results[i]) {
			t.Fatalf("Chunk put #%d key '%v' does not match iterator's key '%v'", i, chunkkeys[i], chunkkeys_results[i])
		}
	}

}

func TestIterator(t *testing.T) {
	testIterator(t, false)
}
func TestMockIterator(t *testing.T) {
	testIterator(t, true)
}

func benchmarkDbStorePut(n int, processors int, chunksize int64, mock bool, b *testing.B) {
	db, cleanup, err := newTestDbStore(mock, true)
	defer cleanup()
	if err != nil {
		b.Fatalf("init dbStore failed: %v", err)
	}
	benchmarkStorePut(db, processors, n, chunksize, b)
}

func benchmarkDbStoreGet(n int, processors int, chunksize int64, mock bool, b *testing.B) {
	db, cleanup, err := newTestDbStore(mock, true)
	defer cleanup()
	if err != nil {
		b.Fatalf("init dbStore failed: %v", err)
	}
	benchmarkStoreGet(db, processors, n, chunksize, b)
}

func BenchmarkDbStorePut_1_500(b *testing.B) {
	benchmarkDbStorePut(500, 1, 4096, false, b)
}

func BenchmarkDbStorePut_8_500(b *testing.B) {
	benchmarkDbStorePut(500, 8, 4096, false, b)
}

func BenchmarkDbStoreGet_1_500(b *testing.B) {
	benchmarkDbStoreGet(500, 1, 4096, false, b)
}

func BenchmarkDbStoreGet_8_500(b *testing.B) {
	benchmarkDbStoreGet(500, 8, 4096, false, b)
}

func BenchmarkMockDbStorePut_1_500(b *testing.B) {
	benchmarkDbStorePut(500, 1, 4096, true, b)
}

func BenchmarkMockDbStorePut_8_500(b *testing.B) {
	benchmarkDbStorePut(500, 8, 4096, true, b)
}

func BenchmarkMockDbStoreGet_1_500(b *testing.B) {
	benchmarkDbStoreGet(500, 1, 4096, true, b)
}

func BenchmarkMockDbStoreGet_8_500(b *testing.B) {
	benchmarkDbStoreGet(500, 8, 4096, true, b)
}

//
//
func TestLDBStoreWithoutCollectGarbage(t *testing.T) {
	capacity := 50
	n := 10

	ldb, cleanup := newLDBStore(t)
	ldb.setCapacity(uint64(capacity))
	defer cleanup()

	chunks := []*Chunk{}
	for i := 0; i < n; i++ {
		c := GenerateRandomChunk(chunk.DefaultSize)
		chunks = append(chunks, c)
		log.Trace("generate random chunk", "idx", i, "chunk", c)
	}

	for i := 0; i < n; i++ {
		go ldb.Put(context.TODO(), chunks[i])
	}

//
	for i := 0; i < n; i++ {
		<-chunks[i].dbStoredC
	}

	log.Info("ldbstore", "entrycnt", ldb.entryCnt, "accesscnt", ldb.accessCnt)

	for i := 0; i < n; i++ {
		ret, err := ldb.Get(context.TODO(), chunks[i].Addr)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(ret.SData, chunks[i].SData) {
			t.Fatal("expected to get the same data back, but got smth else")
		}

		log.Info("got back chunk", "chunk", ret)
	}

	if ldb.entryCnt != uint64(n+1) {
		t.Fatalf("expected entryCnt to be equal to %v, but got %v", n+1, ldb.entryCnt)
	}

	if ldb.accessCnt != uint64(2*n+1) {
		t.Fatalf("expected accessCnt to be equal to %v, but got %v", n+1, ldb.accessCnt)
	}
}

//
//
func TestLDBStoreCollectGarbage(t *testing.T) {
	capacity := 500
	n := 2000

	ldb, cleanup := newLDBStore(t)
	ldb.setCapacity(uint64(capacity))
	defer cleanup()

	chunks := []*Chunk{}
	for i := 0; i < n; i++ {
		c := GenerateRandomChunk(chunk.DefaultSize)
		chunks = append(chunks, c)
		log.Trace("generate random chunk", "idx", i, "chunk", c)
	}

	for i := 0; i < n; i++ {
		ldb.Put(context.TODO(), chunks[i])
	}

//
	for i := 0; i < n; i++ {
		<-chunks[i].dbStoredC
	}

	log.Info("ldbstore", "entrycnt", ldb.entryCnt, "accesscnt", ldb.accessCnt)

//
	time.Sleep(5 * time.Second)

	var missing int
	for i := 0; i < n; i++ {
		ret, err := ldb.Get(context.TODO(), chunks[i].Addr)
		if err == ErrChunkNotFound || err == ldberrors.ErrNotFound {
			missing++
			continue
		}
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(ret.SData, chunks[i].SData) {
			t.Fatal("expected to get the same data back, but got smth else")
		}

		log.Trace("got back chunk", "chunk", ret)
	}

	if missing < n-capacity {
		t.Fatalf("gc failure: expected to miss %v chunks, but only %v are actually missing", n-capacity, missing)
	}

	log.Info("ldbstore", "total", n, "missing", missing, "entrycnt", ldb.entryCnt, "accesscnt", ldb.accessCnt)
}

//
func TestLDBStoreAddRemove(t *testing.T) {
	ldb, cleanup := newLDBStore(t)
	ldb.setCapacity(200)
	defer cleanup()

	n := 100

	chunks := []*Chunk{}
	for i := 0; i < n; i++ {
		c := GenerateRandomChunk(chunk.DefaultSize)
		chunks = append(chunks, c)
		log.Trace("generate random chunk", "idx", i, "chunk", c)
	}

	for i := 0; i < n; i++ {
		go ldb.Put(context.TODO(), chunks[i])
	}

//
	for i := 0; i < n; i++ {
		<-chunks[i].dbStoredC
	}

	for i := 0; i < n; i++ {
//
		if i%2 == 0 {

			key := chunks[i].Addr
			ikey := getIndexKey(key)

			var indx dpaDBIndex
			ldb.tryAccessIdx(ikey, &indx)

			ldb.delete(indx.Idx, ikey, ldb.po(key))
		}
	}

	log.Info("ldbstore", "entrycnt", ldb.entryCnt, "accesscnt", ldb.accessCnt)

	for i := 0; i < n; i++ {
		ret, err := ldb.Get(context.TODO(), chunks[i].Addr)

		if i%2 == 0 {
//
			if err == nil || ret != nil {
				t.Fatal("expected chunk to be missing, but got no error")
			}
		} else {
//
			if err != nil {
				t.Fatalf("expected no error, but got %s", err)
			}

			if !bytes.Equal(ret.SData, chunks[i].SData) {
				t.Fatal("expected to get the same data back, but got smth else")
			}
		}
	}
}

//
func TestLDBStoreRemoveThenCollectGarbage(t *testing.T) {
	capacity := 10

	ldb, cleanup := newLDBStore(t)
	ldb.setCapacity(uint64(capacity))

	n := 7

	chunks := []*Chunk{}
	for i := 0; i < capacity; i++ {
		c := GenerateRandomChunk(chunk.DefaultSize)
		chunks = append(chunks, c)
		log.Trace("generate random chunk", "idx", i, "chunk", c)
	}

	for i := 0; i < n; i++ {
		ldb.Put(context.TODO(), chunks[i])
	}

//
	for i := 0; i < n; i++ {
		<-chunks[i].dbStoredC
	}

//
	for i := 0; i < n; i++ {
		key := chunks[i].Addr
		ikey := getIndexKey(key)

		var indx dpaDBIndex
		ldb.tryAccessIdx(ikey, &indx)

		ldb.delete(indx.Idx, ikey, ldb.po(key))
	}

	log.Info("ldbstore", "entrycnt", ldb.entryCnt, "accesscnt", ldb.accessCnt)

	cleanup()

	ldb, cleanup = newLDBStore(t)
	ldb.setCapacity(uint64(capacity))

	n = 10

	for i := 0; i < n; i++ {
		ldb.Put(context.TODO(), chunks[i])
	}

//
	for i := 0; i < n; i++ {
		<-chunks[i].dbStoredC
	}

//
	idx := 0
	ret, err := ldb.Get(context.TODO(), chunks[idx].Addr)
	if err == nil || ret != nil {
		t.Fatal("expected first chunk to be missing, but got no error")
	}

//
	idx = 9
	ret, err = ldb.Get(context.TODO(), chunks[idx].Addr)
	if err != nil {
		t.Fatalf("expected no error, but got %s", err)
	}

	if !bytes.Equal(ret.SData, chunks[idx].SData) {
		t.Fatal("expected to get the same data back, but got smth else")
	}
}
