
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
//
//
//
//

package storage

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/swarm/chunk"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage/mock"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	gcArrayFreeRatio = 0.1
maxGCitems       = 5000 //
)

var (
	keyIndex       = byte(0)
	keyOldData     = byte(1)
	keyAccessCnt   = []byte{2}
	keyEntryCnt    = []byte{3}
	keyDataIdx     = []byte{4}
	keyData        = byte(6)
	keyDistanceCnt = byte(7)
)

type gcItem struct {
	idx    uint64
	value  uint64
	idxKey []byte
	po     uint8
}

type LDBStoreParams struct {
	*StoreParams
	Path string
	Po   func(Address) uint8
}

//
func NewLDBStoreParams(storeparams *StoreParams, path string) *LDBStoreParams {
	return &LDBStoreParams{
		StoreParams: storeparams,
		Path:        path,
		Po:          func(k Address) (ret uint8) { return uint8(Proximity(storeparams.BaseKey[:], k[:])) },
	}
}

type LDBStore struct {
	db *LDBDatabase

//
entryCnt  uint64 //
accessCnt uint64 //
dataIdx   uint64 //
	capacity  uint64
	bucketCnt []uint64

	hashfunc SwarmHasher
	po       func(Address) uint8

	batchC   chan bool
	batchesC chan struct{}
	batch    *leveldb.Batch
	lock     sync.RWMutex
	quit     chan struct{}

//
//
//
	encodeDataFunc func(chunk *Chunk) []byte
//
//
//
	getDataFunc func(addr Address) (data []byte, err error)
}

//
//
//
func NewLDBStore(params *LDBStoreParams) (s *LDBStore, err error) {
	s = new(LDBStore)
	s.hashfunc = params.Hash
	s.quit = make(chan struct{})

	s.batchC = make(chan bool)
	s.batchesC = make(chan struct{}, 1)
	go s.writeBatches()
	s.batch = new(leveldb.Batch)
//
	s.encodeDataFunc = encodeData

	s.db, err = NewLDBDatabase(params.Path)
	if err != nil {
		return nil, err
	}

	s.po = params.Po
	s.setCapacity(params.DbCapacity)

	s.bucketCnt = make([]uint64, 0x100)
	for i := 0; i < 0x100; i++ {
		k := make([]byte, 2)
		k[0] = keyDistanceCnt
		k[1] = uint8(i)
		cnt, _ := s.db.Get(k)
		s.bucketCnt[i] = BytesToU64(cnt)
		s.bucketCnt[i]++
	}
	data, _ := s.db.Get(keyEntryCnt)
	s.entryCnt = BytesToU64(data)
	s.entryCnt++
	data, _ = s.db.Get(keyAccessCnt)
	s.accessCnt = BytesToU64(data)
	s.accessCnt++
	data, _ = s.db.Get(keyDataIdx)
	s.dataIdx = BytesToU64(data)
	s.dataIdx++

	return s, nil
}

//
//
//
func NewMockDbStore(params *LDBStoreParams, mockStore *mock.NodeStore) (s *LDBStore, err error) {
	s, err = NewLDBStore(params)
	if err != nil {
		return nil, err
	}

//
	if mockStore != nil {
		s.encodeDataFunc = newMockEncodeDataFunc(mockStore)
		s.getDataFunc = newMockGetDataFunc(mockStore)
	}
	return
}

type dpaDBIndex struct {
	Idx    uint64
	Access uint64
}

func BytesToU64(data []byte) uint64 {
	if len(data) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(data)
}

func U64ToBytes(val uint64) []byte {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, val)
	return data
}

func (s *LDBStore) updateIndexAccess(index *dpaDBIndex) {
	index.Access = s.accessCnt
}

func getIndexKey(hash Address) []byte {
	hashSize := len(hash)
	key := make([]byte, hashSize+1)
	key[0] = keyIndex
	copy(key[1:], hash[:])
	return key
}

func getOldDataKey(idx uint64) []byte {
	key := make([]byte, 9)
	key[0] = keyOldData
	binary.BigEndian.PutUint64(key[1:9], idx)

	return key
}

func getDataKey(idx uint64, po uint8) []byte {
	key := make([]byte, 10)
	key[0] = keyData
	key[1] = po
	binary.BigEndian.PutUint64(key[2:], idx)

	return key
}

func encodeIndex(index *dpaDBIndex) []byte {
	data, _ := rlp.EncodeToBytes(index)
	return data
}

func encodeData(chunk *Chunk) []byte {
//
//
//
//
	return append(append([]byte{}, chunk.Addr[:]...), chunk.SData...)
}

func decodeIndex(data []byte, index *dpaDBIndex) error {
	dec := rlp.NewStream(bytes.NewReader(data), 0)
	return dec.Decode(index)
}

func decodeData(data []byte, chunk *Chunk) {
	chunk.SData = data[32:]
	chunk.Size = int64(binary.BigEndian.Uint64(data[32:40]))
}

func decodeOldData(data []byte, chunk *Chunk) {
	chunk.SData = data
	chunk.Size = int64(binary.BigEndian.Uint64(data[0:8]))
}

func (s *LDBStore) collectGarbage(ratio float32) {
	metrics.GetOrRegisterCounter("ldbstore.collectgarbage", nil).Inc(1)

	it := s.db.NewIterator()
	defer it.Release()

	garbage := []*gcItem{}
	gcnt := 0

	for ok := it.Seek([]byte{keyIndex}); ok && (gcnt < maxGCitems) && (uint64(gcnt) < s.entryCnt); ok = it.Next() {
		itkey := it.Key()

		if (itkey == nil) || (itkey[0] != keyIndex) {
			break
		}

//
		key := make([]byte, len(it.Key()))
		copy(key, it.Key())

		val := it.Value()

		var index dpaDBIndex

		hash := key[1:]
		decodeIndex(val, &index)
		po := s.po(hash)

		gci := &gcItem{
			idxKey: key,
			idx:    index.Idx,
value:  index.Access, //
			po:     po,
		}

		garbage = append(garbage, gci)
		gcnt++
	}

	sort.Slice(garbage[:gcnt], func(i, j int) bool { return garbage[i].value < garbage[j].value })

	cutoff := int(float32(gcnt) * ratio)
	metrics.GetOrRegisterCounter("ldbstore.collectgarbage.delete", nil).Inc(int64(cutoff))

	for i := 0; i < cutoff; i++ {
		s.delete(garbage[i].idx, garbage[i].idxKey, garbage[i].po)
	}
}

//
//
func (s *LDBStore) Export(out io.Writer) (int64, error) {
	tw := tar.NewWriter(out)
	defer tw.Close()

	it := s.db.NewIterator()
	defer it.Release()
	var count int64
	for ok := it.Seek([]byte{keyIndex}); ok; ok = it.Next() {
		key := it.Key()
		if (key == nil) || (key[0] != keyIndex) {
			break
		}

		var index dpaDBIndex

		hash := key[1:]
		decodeIndex(it.Value(), &index)
		po := s.po(hash)
		datakey := getDataKey(index.Idx, po)
		log.Trace("store.export", "dkey", fmt.Sprintf("%x", datakey), "dataidx", index.Idx, "po", po)
		data, err := s.db.Get(datakey)
		if err != nil {
			log.Warn(fmt.Sprintf("Chunk %x found but could not be accessed: %v", key[:], err))
			continue
		}

		hdr := &tar.Header{
			Name: hex.EncodeToString(hash),
			Mode: 0644,
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return count, err
		}
		if _, err := tw.Write(data); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

//
func (s *LDBStore) Import(in io.Reader) (int64, error) {
	tr := tar.NewReader(in)

	var count int64
	var wg sync.WaitGroup
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return count, err
		}

		if len(hdr.Name) != 64 {
			log.Warn("ignoring non-chunk file", "name", hdr.Name)
			continue
		}

		keybytes, err := hex.DecodeString(hdr.Name)
		if err != nil {
			log.Warn("ignoring invalid chunk file", "name", hdr.Name, "err", err)
			continue
		}

		data, err := ioutil.ReadAll(tr)
		if err != nil {
			return count, err
		}
		key := Address(keybytes)
		chunk := NewChunk(key, nil)
		chunk.SData = data[32:]
		s.Put(context.TODO(), chunk)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-chunk.dbStoredC
		}()
		count++
	}
	wg.Wait()
	return count, nil
}

func (s *LDBStore) Cleanup() {
//
	var errorsFound, removed, total int

	it := s.db.NewIterator()
	defer it.Release()
	for ok := it.Seek([]byte{keyIndex}); ok; ok = it.Next() {
		key := it.Key()
		if (key == nil) || (key[0] != keyIndex) {
			break
		}
		total++
		var index dpaDBIndex
		err := decodeIndex(it.Value(), &index)
		if err != nil {
			log.Warn("Cannot decode")
			errorsFound++
			continue
		}
		hash := key[1:]
		po := s.po(hash)
		datakey := getDataKey(index.Idx, po)
		data, err := s.db.Get(datakey)
		if err != nil {
			found := false

//
			for po = 1; po <= 255; po++ {
				datakey = getDataKey(index.Idx, po)
				data, err = s.db.Get(datakey)
				if err == nil {
					found = true
					break
				}
			}

			if !found {
				log.Warn(fmt.Sprintf("Chunk %x found but count not be accessed with any po", key[:]))
				errorsFound++
				continue
			}
		}

		c := &Chunk{}
		ck := data[:32]
		decodeData(data, c)

		cs := int64(binary.LittleEndian.Uint64(c.SData[:8]))
		log.Trace("chunk", "key", fmt.Sprintf("%x", key[:]), "ck", fmt.Sprintf("%x", ck), "dkey", fmt.Sprintf("%x", datakey), "dataidx", index.Idx, "po", po, "len data", len(data), "len sdata", len(c.SData), "size", cs)

		if len(c.SData) > chunk.DefaultSize+8 {
			log.Warn("chunk for cleanup", "key", fmt.Sprintf("%x", key[:]), "ck", fmt.Sprintf("%x", ck), "dkey", fmt.Sprintf("%x", datakey), "dataidx", index.Idx, "po", po, "len data", len(data), "len sdata", len(c.SData), "size", cs)
			s.delete(index.Idx, getIndexKey(key[1:]), po)
			removed++
			errorsFound++
		}
	}

	log.Warn(fmt.Sprintf("Found %v errors out of %v entries. Removed %v chunks.", errorsFound, total, removed))
}

func (s *LDBStore) ReIndex() {
//
	it := s.db.NewIterator()
	startPosition := []byte{keyOldData}
	it.Seek(startPosition)
	var key []byte
	var errorsFound, total int
	for it.Valid() {
		key = it.Key()
		if (key == nil) || (key[0] != keyOldData) {
			break
		}
		data := it.Value()
		hasher := s.hashfunc()
		hasher.Write(data)
		hash := hasher.Sum(nil)

		newKey := make([]byte, 10)
		oldCntKey := make([]byte, 2)
		newCntKey := make([]byte, 2)
		oldCntKey[0] = keyDistanceCnt
		newCntKey[0] = keyDistanceCnt
		key[0] = keyData
		key[1] = s.po(Address(key[1:]))
		oldCntKey[1] = key[1]
		newCntKey[1] = s.po(Address(newKey[1:]))
		copy(newKey[2:], key[1:])
		newValue := append(hash, data...)

		batch := new(leveldb.Batch)
		batch.Delete(key)
		s.bucketCnt[oldCntKey[1]]--
		batch.Put(oldCntKey, U64ToBytes(s.bucketCnt[oldCntKey[1]]))
		batch.Put(newKey, newValue)
		s.bucketCnt[newCntKey[1]]++
		batch.Put(newCntKey, U64ToBytes(s.bucketCnt[newCntKey[1]]))
		s.db.Write(batch)
		it.Next()
	}
	it.Release()
	log.Warn(fmt.Sprintf("Found %v errors out of %v entries", errorsFound, total))
}

func (s *LDBStore) delete(idx uint64, idxKey []byte, po uint8) {
	metrics.GetOrRegisterCounter("ldbstore.delete", nil).Inc(1)

	batch := new(leveldb.Batch)
	batch.Delete(idxKey)
	batch.Delete(getDataKey(idx, po))
	s.entryCnt--
	s.bucketCnt[po]--
	cntKey := make([]byte, 2)
	cntKey[0] = keyDistanceCnt
	cntKey[1] = po
	batch.Put(keyEntryCnt, U64ToBytes(s.entryCnt))
	batch.Put(cntKey, U64ToBytes(s.bucketCnt[po]))
	s.db.Write(batch)
}

func (s *LDBStore) CurrentBucketStorageIndex(po uint8) uint64 {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.bucketCnt[po]
}

func (s *LDBStore) Size() uint64 {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.entryCnt
}

func (s *LDBStore) CurrentStorageIndex() uint64 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.dataIdx
}

func (s *LDBStore) Put(ctx context.Context, chunk *Chunk) {
	metrics.GetOrRegisterCounter("ldbstore.put", nil).Inc(1)
	log.Trace("ldbstore.put", "key", chunk.Addr)

	ikey := getIndexKey(chunk.Addr)
	var index dpaDBIndex

	po := s.po(chunk.Addr)
	s.lock.Lock()
	defer s.lock.Unlock()

	log.Trace("ldbstore.put: s.db.Get", "key", chunk.Addr, "ikey", fmt.Sprintf("%x", ikey))
	idata, err := s.db.Get(ikey)
	if err != nil {
		s.doPut(chunk, &index, po)
		batchC := s.batchC
		go func() {
			<-batchC
			chunk.markAsStored()
		}()
	} else {
		log.Trace("ldbstore.put: chunk already exists, only update access", "key", chunk.Addr)
		decodeIndex(idata, &index)
		chunk.markAsStored()
	}
	index.Access = s.accessCnt
	s.accessCnt++
	idata = encodeIndex(&index)
	s.batch.Put(ikey, idata)
	select {
	case s.batchesC <- struct{}{}:
	default:
	}
}

//
func (s *LDBStore) doPut(chunk *Chunk, index *dpaDBIndex, po uint8) {
	data := s.encodeDataFunc(chunk)
	dkey := getDataKey(s.dataIdx, po)
	s.batch.Put(dkey, data)
	index.Idx = s.dataIdx
	s.bucketCnt[po] = s.dataIdx
	s.entryCnt++
	s.dataIdx++

	cntKey := make([]byte, 2)
	cntKey[0] = keyDistanceCnt
	cntKey[1] = po
	s.batch.Put(cntKey, U64ToBytes(s.bucketCnt[po]))
}

func (s *LDBStore) writeBatches() {
mainLoop:
	for {
		select {
		case <-s.quit:
			break mainLoop
		case <-s.batchesC:
			s.lock.Lock()
			b := s.batch
			e := s.entryCnt
			d := s.dataIdx
			a := s.accessCnt
			c := s.batchC
			s.batchC = make(chan bool)
			s.batch = new(leveldb.Batch)
			err := s.writeBatch(b, e, d, a)
//
			if err != nil {
				log.Error(fmt.Sprintf("spawn batch write (%d entries): %v", b.Len(), err))
			}
			close(c)
			for e > s.capacity {
//
//
				done := make(chan struct{})
				go func() {
					s.collectGarbage(gcArrayFreeRatio)
					close(done)
				}()

				e = s.entryCnt
				select {
				case <-s.quit:
					s.lock.Unlock()
					break mainLoop
				case <-done:
				}
			}
			s.lock.Unlock()
		}
	}
	log.Trace(fmt.Sprintf("DbStore: quit batch write loop"))
}

//
func (s *LDBStore) writeBatch(b *leveldb.Batch, entryCnt, dataIdx, accessCnt uint64) error {
	b.Put(keyEntryCnt, U64ToBytes(entryCnt))
	b.Put(keyDataIdx, U64ToBytes(dataIdx))
	b.Put(keyAccessCnt, U64ToBytes(accessCnt))
	l := b.Len()
	if err := s.db.Write(b); err != nil {
		return fmt.Errorf("unable to write batch: %v", err)
	}
	log.Trace(fmt.Sprintf("batch write (%d entries)", l))
	return nil
}

//
//
//
//
func newMockEncodeDataFunc(mockStore *mock.NodeStore) func(chunk *Chunk) []byte {
	return func(chunk *Chunk) []byte {
		if err := mockStore.Put(chunk.Addr, encodeData(chunk)); err != nil {
			log.Error(fmt.Sprintf("%T: Chunk %v put: %v", mockStore, chunk.Addr.Log(), err))
		}
		return chunk.Addr[:]
	}
}

//
func (s *LDBStore) tryAccessIdx(ikey []byte, index *dpaDBIndex) bool {
	idata, err := s.db.Get(ikey)
	if err != nil {
		return false
	}
	decodeIndex(idata, index)
	s.batch.Put(keyAccessCnt, U64ToBytes(s.accessCnt))
	s.accessCnt++
	index.Access = s.accessCnt
	idata = encodeIndex(index)
	s.batch.Put(ikey, idata)
	select {
	case s.batchesC <- struct{}{}:
	default:
	}
	return true
}

func (s *LDBStore) Get(ctx context.Context, addr Address) (chunk *Chunk, err error) {
	metrics.GetOrRegisterCounter("ldbstore.get", nil).Inc(1)
	log.Trace("ldbstore.get", "key", addr)

	s.lock.Lock()
	defer s.lock.Unlock()
	return s.get(addr)
}

func (s *LDBStore) get(addr Address) (chunk *Chunk, err error) {
	var indx dpaDBIndex

	if s.tryAccessIdx(getIndexKey(addr), &indx) {
		var data []byte
		if s.getDataFunc != nil {
//
			log.Trace("ldbstore.get retrieve with getDataFunc", "key", addr)
			data, err = s.getDataFunc(addr)
			if err != nil {
				return
			}
		} else {
//
			proximity := s.po(addr)
			datakey := getDataKey(indx.Idx, proximity)
			data, err = s.db.Get(datakey)
			log.Trace("ldbstore.get retrieve", "key", addr, "indexkey", indx.Idx, "datakey", fmt.Sprintf("%x", datakey), "proximity", proximity)
			if err != nil {
				log.Trace("ldbstore.get chunk found but could not be accessed", "key", addr, "err", err)
				s.delete(indx.Idx, getIndexKey(addr), s.po(addr))
				return
			}
		}

		chunk = NewChunk(addr, nil)
		chunk.markAsStored()
		decodeData(data, chunk)
	} else {
		err = ErrChunkNotFound
	}

	return
}

//
//
//
func newMockGetDataFunc(mockStore *mock.NodeStore) func(addr Address) (data []byte, err error) {
	return func(addr Address) (data []byte, err error) {
		data, err = mockStore.Get(addr)
		if err == mock.ErrNotFound {
//
			err = ErrChunkNotFound
		}
		return data, err
	}
}

func (s *LDBStore) updateAccessCnt(addr Address) {

	s.lock.Lock()
	defer s.lock.Unlock()

	var index dpaDBIndex
s.tryAccessIdx(getIndexKey(addr), &index) //

}

func (s *LDBStore) setCapacity(c uint64) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.capacity = c

	if s.entryCnt > c {
		ratio := float32(1.01) - float32(c)/float32(s.entryCnt)
		if ratio < gcArrayFreeRatio {
			ratio = gcArrayFreeRatio
		}
		if ratio > 1 {
			ratio = 1
		}
		for s.entryCnt > c {
			s.collectGarbage(ratio)
		}
	}
}

func (s *LDBStore) Close() {
	close(s.quit)
	s.db.Close()
}

//
func (s *LDBStore) SyncIterator(since uint64, until uint64, po uint8, f func(Address, uint64) bool) error {
	metrics.GetOrRegisterCounter("ldbstore.synciterator", nil).Inc(1)

	sincekey := getDataKey(since, po)
	untilkey := getDataKey(until, po)
	it := s.db.NewIterator()
	defer it.Release()

	for ok := it.Seek(sincekey); ok; ok = it.Next() {
		metrics.GetOrRegisterCounter("ldbstore.synciterator.seek", nil).Inc(1)

		dbkey := it.Key()
		if dbkey[0] != keyData || dbkey[1] != po || bytes.Compare(untilkey, dbkey) < 0 {
			break
		}
		key := make([]byte, 32)
		val := it.Value()
		copy(key, val[:32])
		if !f(Address(key), binary.BigEndian.Uint64(dbkey[2:])) {
			break
		}
	}
	return it.Error()
}

func databaseExists(path string) bool {
	o := &opt.Options{
		ErrorIfMissing: true,
	}
	tdb, err := leveldb.OpenFile(path, o)
	if err != nil {
		return false
	}
	defer tdb.Close()
	return true
}
