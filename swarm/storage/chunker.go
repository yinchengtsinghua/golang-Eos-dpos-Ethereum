
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
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/chunk"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/spancontext"
	opentracing "github.com/opentracing/opentracing-go"
	olog "github.com/opentracing/opentracing-go/log"
)

/*














  


  
  
  

 
*/


/*





*/


var (
	errAppendOppNotSuported = errors.New("Append operation not supported")
	errOperationTimedOut    = errors.New("operation timed out")
)

type ChunkerParams struct {
	chunkSize int64
	hashSize  int64
}

type SplitterParams struct {
	ChunkerParams
	reader io.Reader
	putter Putter
	addr   Address
}

type TreeSplitterParams struct {
	SplitterParams
	size int64
}

type JoinerParams struct {
	ChunkerParams
	addr   Address
	getter Getter
//
	depth int
	ctx   context.Context
}

type TreeChunker struct {
	ctx context.Context

	branches int64
	hashFunc SwarmHasher
	dataSize int64
	data     io.Reader
//
	addr        Address
	depth       int
hashSize    int64        //
chunkSize   int64        //
workerCount int64        //
workerLock  sync.RWMutex //
	jobC        chan *hashJob
	wg          *sync.WaitGroup
	putter      Putter
	getter      Getter
	errC        chan error
	quitC       chan bool
}

/*
 
 
 
 
 
 
 
 
 
 
*/

func TreeJoin(ctx context.Context, addr Address, getter Getter, depth int) *LazyChunkReader {
	jp := &JoinerParams{
		ChunkerParams: ChunkerParams{
			chunkSize: chunk.DefaultSize,
			hashSize:  int64(len(addr)),
		},
		addr:   addr,
		getter: getter,
		depth:  depth,
		ctx:    ctx,
	}

	return NewTreeJoiner(jp).Join(ctx)
}

/*
 
 
*/

func TreeSplit(ctx context.Context, data io.Reader, size int64, putter Putter) (k Address, wait func(context.Context) error, err error) {
	tsp := &TreeSplitterParams{
		SplitterParams: SplitterParams{
			ChunkerParams: ChunkerParams{
				chunkSize: chunk.DefaultSize,
				hashSize:  putter.RefSize(),
			},
			reader: data,
			putter: putter,
		},
		size: size,
	}
	return NewTreeSplitter(tsp).Split(ctx)
}

func NewTreeJoiner(params *JoinerParams) *TreeChunker {
	tc := &TreeChunker{}
	tc.hashSize = params.hashSize
	tc.branches = params.chunkSize / params.hashSize
	tc.addr = params.addr
	tc.getter = params.getter
	tc.depth = params.depth
	tc.chunkSize = params.chunkSize
	tc.workerCount = 0
	tc.jobC = make(chan *hashJob, 2*ChunkProcessors)
	tc.wg = &sync.WaitGroup{}
	tc.errC = make(chan error)
	tc.quitC = make(chan bool)

	tc.ctx = params.ctx

	return tc
}

func NewTreeSplitter(params *TreeSplitterParams) *TreeChunker {
	tc := &TreeChunker{}
	tc.data = params.reader
	tc.dataSize = params.size
	tc.hashSize = params.hashSize
	tc.branches = params.chunkSize / params.hashSize
	tc.addr = params.addr
	tc.chunkSize = params.chunkSize
	tc.putter = params.putter
	tc.workerCount = 0
	tc.jobC = make(chan *hashJob, 2*ChunkProcessors)
	tc.wg = &sync.WaitGroup{}
	tc.errC = make(chan error)
	tc.quitC = make(chan bool)

	return tc
}

//
func (c *Chunk) String() string {
	return fmt.Sprintf("Key: %v TreeSize: %v Chunksize: %v", c.Addr.Log(), c.Size, len(c.SData))
}

type hashJob struct {
	key      Address
	chunk    []byte
	size     int64
	parentWg *sync.WaitGroup
}

func (tc *TreeChunker) incrementWorkerCount() {
	tc.workerLock.Lock()
	defer tc.workerLock.Unlock()
	tc.workerCount += 1
}

func (tc *TreeChunker) getWorkerCount() int64 {
	tc.workerLock.RLock()
	defer tc.workerLock.RUnlock()
	return tc.workerCount
}

func (tc *TreeChunker) decrementWorkerCount() {
	tc.workerLock.Lock()
	defer tc.workerLock.Unlock()
	tc.workerCount -= 1
}

func (tc *TreeChunker) Split(ctx context.Context) (k Address, wait func(context.Context) error, err error) {
	if tc.chunkSize <= 0 {
		panic("chunker must be initialised")
	}

	tc.runWorker()

	depth := 0
	treeSize := tc.chunkSize

//
//
	for ; treeSize < tc.dataSize; treeSize *= tc.branches {
		depth++
	}

	key := make([]byte, tc.hashSize)
//
	tc.wg.Add(1)
//
	go tc.split(depth, treeSize/tc.branches, key, tc.dataSize, tc.wg)

//
	go func() {
//
		tc.wg.Wait()
		close(tc.errC)
	}()

	defer close(tc.quitC)
	defer tc.putter.Close()
	select {
	case err := <-tc.errC:
		if err != nil {
			return nil, nil, err
		}
	case <-time.NewTimer(splitTimeout).C:
		return nil, nil, errOperationTimedOut
	}

	return key, tc.putter.Wait, nil
}

func (tc *TreeChunker) split(depth int, treeSize int64, addr Address, size int64, parentWg *sync.WaitGroup) {

//

	for depth > 0 && size < treeSize {
		treeSize /= tc.branches
		depth--
	}

	if depth == 0 {
//
		chunkData := make([]byte, size+8)
		binary.LittleEndian.PutUint64(chunkData[0:8], uint64(size))
		var readBytes int64
		for readBytes < size {
			n, err := tc.data.Read(chunkData[8+readBytes:])
			readBytes += int64(n)
			if err != nil && !(err == io.EOF && readBytes == size) {
				tc.errC <- err
				return
			}
		}
		select {
		case tc.jobC <- &hashJob{addr, chunkData, size, parentWg}:
		case <-tc.quitC:
		}
		return
	}
//
//
	branchCnt := (size + treeSize - 1) / treeSize

	var chunk = make([]byte, branchCnt*tc.hashSize+8)
	var pos, i int64

	binary.LittleEndian.PutUint64(chunk[0:8], uint64(size))

	childrenWg := &sync.WaitGroup{}
	var secSize int64
	for i < branchCnt {
//
		if size-pos < treeSize {
			secSize = size - pos
		} else {
			secSize = treeSize
		}
//
		subTreeKey := chunk[8+i*tc.hashSize : 8+(i+1)*tc.hashSize]

		childrenWg.Add(1)
		tc.split(depth-1, treeSize/tc.branches, subTreeKey, secSize, childrenWg)

		i++
		pos += treeSize
	}
//
//
//
	childrenWg.Wait()

	worker := tc.getWorkerCount()
	if int64(len(tc.jobC)) > worker && worker < ChunkProcessors {
		tc.runWorker()

	}
	select {
	case tc.jobC <- &hashJob{addr, chunk, size, parentWg}:
	case <-tc.quitC:
	}
}

func (tc *TreeChunker) runWorker() {
	tc.incrementWorkerCount()
	go func() {
		defer tc.decrementWorkerCount()
		for {
			select {

			case job, ok := <-tc.jobC:
				if !ok {
					return
				}

				h, err := tc.putter.Put(tc.ctx, job.chunk)
				if err != nil {
					tc.errC <- err
					return
				}
				copy(job.key, h)
				job.parentWg.Done()
			case <-tc.quitC:
				return
			}
		}
	}()
}

func (tc *TreeChunker) Append() (Address, func(), error) {
	return nil, nil, errAppendOppNotSuported
}

//
type LazyChunkReader struct {
	Ctx       context.Context
key       Address //
	chunkData ChunkData
off       int64 //
chunkSize int64 //
branches  int64 //
hashSize  int64 //
	depth     int
	getter    Getter
}

func (tc *TreeChunker) Join(ctx context.Context) *LazyChunkReader {
	return &LazyChunkReader{
		key:       tc.addr,
		chunkSize: tc.chunkSize,
		branches:  tc.branches,
		hashSize:  tc.hashSize,
		depth:     tc.depth,
		getter:    tc.getter,
		Ctx:       tc.ctx,
	}
}

func (r *LazyChunkReader) Context() context.Context {
	return r.Ctx
}

//
func (r *LazyChunkReader) Size(ctx context.Context, quitC chan bool) (n int64, err error) {
	metrics.GetOrRegisterCounter("lazychunkreader.size", nil).Inc(1)

	var sp opentracing.Span
	var cctx context.Context
	cctx, sp = spancontext.StartSpan(
		ctx,
		"lcr.size")
	defer sp.Finish()

	log.Debug("lazychunkreader.size", "key", r.key)
	if r.chunkData == nil {
		chunkData, err := r.getter.Get(cctx, Reference(r.key))
		if err != nil {
			return 0, err
		}
		if chunkData == nil {
			select {
			case <-quitC:
				return 0, errors.New("aborted")
			default:
				return 0, fmt.Errorf("root chunk not found for %v", r.key.Hex())
			}
		}
		r.chunkData = chunkData
	}
	return r.chunkData.Size(), nil
}

//
//
//
func (r *LazyChunkReader) ReadAt(b []byte, off int64) (read int, err error) {
	metrics.GetOrRegisterCounter("lazychunkreader.readat", nil).Inc(1)

	var sp opentracing.Span
	var cctx context.Context
	cctx, sp = spancontext.StartSpan(
		r.Ctx,
		"lcr.read")
	defer sp.Finish()

	defer func() {
		sp.LogFields(
			olog.Int("off", int(off)),
			olog.Int("read", read))
	}()

//
	if len(b) == 0 {
		return 0, nil
	}
	quitC := make(chan bool)
	size, err := r.Size(cctx, quitC)
	if err != nil {
		log.Error("lazychunkreader.readat.size", "size", size, "err", err)
		return 0, err
	}

	errC := make(chan error)

//
	var treeSize int64
	var depth int
//
	treeSize = r.chunkSize
	for ; treeSize < size; treeSize *= r.branches {
		depth++
	}
	wg := sync.WaitGroup{}
	length := int64(len(b))
	for d := 0; d < r.depth; d++ {
		off *= r.chunkSize
		length *= r.chunkSize
	}
	wg.Add(1)
	go r.join(cctx, b, off, off+length, depth, treeSize/r.branches, r.chunkData, &wg, errC, quitC)
	go func() {
		wg.Wait()
		close(errC)
	}()

	err = <-errC
	if err != nil {
		log.Error("lazychunkreader.readat.errc", "err", err)
		close(quitC)
		return 0, err
	}
	if off+int64(len(b)) >= size {
		return int(size - off), io.EOF
	}
	return len(b), nil
}

func (r *LazyChunkReader) join(ctx context.Context, b []byte, off int64, eoff int64, depth int, treeSize int64, chunkData ChunkData, parentWg *sync.WaitGroup, errC chan error, quitC chan bool) {
	defer parentWg.Done()
//
	for chunkData.Size() < treeSize && depth > r.depth {
		treeSize /= r.branches
		depth--
	}

//
	if depth == r.depth {
		extra := 8 + eoff - int64(len(chunkData))
		if extra > 0 {
			eoff -= extra
		}
		copy(b, chunkData[8+off:8+eoff])
return //
	}

//
	start := off / treeSize
	end := (eoff + treeSize - 1) / treeSize

//
	currentBranches := int64(len(chunkData)-8) / r.hashSize
	if end > currentBranches {
		end = currentBranches
	}

	wg := &sync.WaitGroup{}
	defer wg.Wait()
	for i := start; i < end; i++ {
		soff := i * treeSize
		roff := soff
		seoff := soff + treeSize

		if soff < off {
			soff = off
		}
		if seoff > eoff {
			seoff = eoff
		}
		if depth > 1 {
			wg.Wait()
		}
		wg.Add(1)
		go func(j int64) {
			childKey := chunkData[8+j*r.hashSize : 8+(j+1)*r.hashSize]
			chunkData, err := r.getter.Get(ctx, Reference(childKey))
			if err != nil {
				log.Error("lazychunkreader.join", "key", fmt.Sprintf("%x", childKey), "err", err)
				select {
				case errC <- fmt.Errorf("chunk %v-%v not found; key: %s", off, off+treeSize, fmt.Sprintf("%x", childKey)):
				case <-quitC:
				}
				return
			}
			if l := len(chunkData); l < 9 {
				select {
				case errC <- fmt.Errorf("chunk %v-%v incomplete; key: %s, data length %v", off, off+treeSize, fmt.Sprintf("%x", childKey), l):
				case <-quitC:
				}
				return
			}
			if soff < off {
				soff = off
			}
			r.join(ctx, b[soff-off:seoff-off], soff-roff, seoff-roff, depth-1, treeSize/r.branches, chunkData, wg, errC, quitC)
		}(i)
} //
}

//
func (r *LazyChunkReader) Read(b []byte) (read int, err error) {
	log.Debug("lazychunkreader.read", "key", r.key)
	metrics.GetOrRegisterCounter("lazychunkreader.read", nil).Inc(1)

	read, err = r.ReadAt(b, r.off)
	if err != nil && err != io.EOF {
		log.Error("lazychunkreader.readat", "read", read, "err", err)
		metrics.GetOrRegisterCounter("lazychunkreader.read.err", nil).Inc(1)
	}

	metrics.GetOrRegisterCounter("lazychunkreader.read.bytes", nil).Inc(int64(read))

	r.off += int64(read)
	return
}

//
var errWhence = errors.New("Seek: invalid whence")
var errOffset = errors.New("Seek: invalid offset")

func (r *LazyChunkReader) Seek(offset int64, whence int) (int64, error) {
	log.Debug("lazychunkreader.seek", "key", r.key, "offset", offset)
	switch whence {
	default:
		return 0, errWhence
	case 0:
		offset += 0
	case 1:
		offset += r.off
	case 2:
if r.chunkData == nil { //
			_, err := r.Size(context.TODO(), nil)
			if err != nil {
				return 0, fmt.Errorf("can't get size: %v", err)
			}
		}
		offset += r.chunkData.Size()
	}

	if offset < 0 {
		return 0, errOffset
	}
	r.off = offset
	return offset, nil
}
