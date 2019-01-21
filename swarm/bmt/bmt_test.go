
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

package bmt

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto/sha3"
)

//
const BufferSize = 4128

const (
//
//
//
	segmentCount = 128
)

var counts = []int{1, 2, 3, 4, 5, 8, 9, 15, 16, 17, 32, 37, 42, 53, 63, 64, 65, 111, 127, 128}

//
func sha3hash(data ...[]byte) []byte {
	h := sha3.NewKeccak256()
	return doSum(h, nil, data...)
}

//
//
func TestRefHasher(t *testing.T) {
//
//
	type test struct {
		from     int
		to       int
		expected func([]byte) []byte
	}

	var tests []*test
//
//
//
//
	tests = append(tests, &test{
		from: 1,
		to:   2,
		expected: func(d []byte) []byte {
			data := make([]byte, 64)
			copy(data, d)
			return sha3hash(data)
		},
	})

//
//
//
//
//
//
//
	tests = append(tests, &test{
		from: 3,
		to:   4,
		expected: func(d []byte) []byte {
			data := make([]byte, 128)
			copy(data, d)
			return sha3hash(sha3hash(data[:64]), sha3hash(data[64:]))
		},
	})

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
	tests = append(tests, &test{
		from: 5,
		to:   8,
		expected: func(d []byte) []byte {
			data := make([]byte, 256)
			copy(data, d)
			return sha3hash(sha3hash(sha3hash(data[:64]), sha3hash(data[64:128])), sha3hash(sha3hash(data[128:192]), sha3hash(data[192:])))
		},
	})

//
	for _, x := range tests {
		for segmentCount := x.from; segmentCount <= x.to; segmentCount++ {
			for length := 1; length <= segmentCount*32; length++ {
				t.Run(fmt.Sprintf("%d_segments_%d_bytes", segmentCount, length), func(t *testing.T) {
					data := make([]byte, length)
					if _, err := io.ReadFull(crand.Reader, data); err != nil && err != io.EOF {
						t.Fatal(err)
					}
					expected := x.expected(data)
					actual := NewRefHasher(sha3.NewKeccak256, segmentCount).Hash(data)
					if !bytes.Equal(actual, expected) {
						t.Fatalf("expected %x, got %x", expected, actual)
					}
				})
			}
		}
	}
}

//
func TestHasherEmptyData(t *testing.T) {
	hasher := sha3.NewKeccak256
	var data []byte
	for _, count := range counts {
		t.Run(fmt.Sprintf("%d_segments", count), func(t *testing.T) {
			pool := NewTreePool(hasher, count, PoolSize)
			defer pool.Drain(0)
			bmt := New(pool)
			rbmt := NewRefHasher(hasher, count)
			refHash := rbmt.Hash(data)
			expHash := syncHash(bmt, nil, data)
			if !bytes.Equal(expHash, refHash) {
				t.Fatalf("hash mismatch with reference. expected %x, got %x", refHash, expHash)
			}
		})
	}
}

//
func TestSyncHasherCorrectness(t *testing.T) {
	data := newData(BufferSize)
	hasher := sha3.NewKeccak256
	size := hasher().Size()

	var err error
	for _, count := range counts {
		t.Run(fmt.Sprintf("segments_%v", count), func(t *testing.T) {
			max := count * size
			var incr int
			capacity := 1
			pool := NewTreePool(hasher, count, capacity)
			defer pool.Drain(0)
			for n := 0; n <= max; n += incr {
				incr = 1 + rand.Intn(5)
				bmt := New(pool)
				err = testHasherCorrectness(bmt, hasher, data, n, count)
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

//
func TestAsyncCorrectness(t *testing.T) {
	data := newData(BufferSize)
	hasher := sha3.NewKeccak256
	size := hasher().Size()
	whs := []whenHash{first, last, random}

	for _, double := range []bool{false, true} {
		for _, wh := range whs {
			for _, count := range counts {
				t.Run(fmt.Sprintf("double_%v_hash_when_%v_segments_%v", double, wh, count), func(t *testing.T) {
					max := count * size
					var incr int
					capacity := 1
					pool := NewTreePool(hasher, count, capacity)
					defer pool.Drain(0)
					for n := 1; n <= max; n += incr {
						incr = 1 + rand.Intn(5)
						bmt := New(pool)
						d := data[:n]
						rbmt := NewRefHasher(hasher, count)
						exp := rbmt.Hash(d)
						got := syncHash(bmt, nil, d)
						if !bytes.Equal(got, exp) {
							t.Fatalf("wrong sync hash for datalength %v: expected %x (ref), got %x", n, exp, got)
						}
						sw := bmt.NewAsyncWriter(double)
						got = asyncHashRandom(sw, nil, d, wh)
						if !bytes.Equal(got, exp) {
							t.Fatalf("wrong async hash for datalength %v: expected %x, got %x", n, exp, got)
						}
					}
				})
			}
		}
	}
}

//
func TestHasherReuse(t *testing.T) {
	t.Run(fmt.Sprintf("poolsize_%d", 1), func(t *testing.T) {
		testHasherReuse(1, t)
	})
	t.Run(fmt.Sprintf("poolsize_%d", PoolSize), func(t *testing.T) {
		testHasherReuse(PoolSize, t)
	})
}

//
func testHasherReuse(poolsize int, t *testing.T) {
	hasher := sha3.NewKeccak256
	pool := NewTreePool(hasher, segmentCount, poolsize)
	defer pool.Drain(0)
	bmt := New(pool)

	for i := 0; i < 100; i++ {
		data := newData(BufferSize)
		n := rand.Intn(bmt.Size())
		err := testHasherCorrectness(bmt, hasher, data, n, segmentCount)
		if err != nil {
			t.Fatal(err)
		}
	}
}

//
func TestBMTConcurrentUse(t *testing.T) {
	hasher := sha3.NewKeccak256
	pool := NewTreePool(hasher, segmentCount, PoolSize)
	defer pool.Drain(0)
	cycles := 100
	errc := make(chan error)

	for i := 0; i < cycles; i++ {
		go func() {
			bmt := New(pool)
			data := newData(BufferSize)
			n := rand.Intn(bmt.Size())
			errc <- testHasherCorrectness(bmt, hasher, data, n, 128)
		}()
	}
LOOP:
	for {
		select {
		case <-time.NewTimer(5 * time.Second).C:
			t.Fatal("timed out")
		case err := <-errc:
			if err != nil {
				t.Fatal(err)
			}
			cycles--
			if cycles == 0 {
				break LOOP
			}
		}
	}
}

//
//
func TestBMTWriterBuffers(t *testing.T) {
	hasher := sha3.NewKeccak256

	for _, count := range counts {
		t.Run(fmt.Sprintf("%d_segments", count), func(t *testing.T) {
			errc := make(chan error)
			pool := NewTreePool(hasher, count, PoolSize)
			defer pool.Drain(0)
			n := count * 32
			bmt := New(pool)
			data := newData(n)
			rbmt := NewRefHasher(hasher, count)
			refHash := rbmt.Hash(data)
			expHash := syncHash(bmt, nil, data)
			if !bytes.Equal(expHash, refHash) {
				t.Fatalf("hash mismatch with reference. expected %x, got %x", refHash, expHash)
			}
			attempts := 10
			f := func() error {
				bmt := New(pool)
				bmt.Reset()
				var buflen int
				for offset := 0; offset < n; offset += buflen {
					buflen = rand.Intn(n-offset) + 1
					read, err := bmt.Write(data[offset : offset+buflen])
					if err != nil {
						return err
					}
					if read != buflen {
						return fmt.Errorf("incorrect read. expected %v bytes, got %v", buflen, read)
					}
				}
				hash := bmt.Sum(nil)
				if !bytes.Equal(hash, expHash) {
					return fmt.Errorf("hash mismatch. expected %x, got %x", hash, expHash)
				}
				return nil
			}

			for j := 0; j < attempts; j++ {
				go func() {
					errc <- f()
				}()
			}
			timeout := time.NewTimer(2 * time.Second)
			for {
				select {
				case err := <-errc:
					if err != nil {
						t.Fatal(err)
					}
					attempts--
					if attempts == 0 {
						return
					}
				case <-timeout.C:
					t.Fatalf("timeout")
				}
			}
		})
	}
}

//
//
func testHasherCorrectness(bmt *Hasher, hasher BaseHasherFunc, d []byte, n, count int) (err error) {
	span := make([]byte, 8)
	if len(d) < n {
		n = len(d)
	}
	binary.BigEndian.PutUint64(span, uint64(n))
	data := d[:n]
	rbmt := NewRefHasher(hasher, count)
	exp := sha3hash(span, rbmt.Hash(data))
	got := syncHash(bmt, span, data)
	if !bytes.Equal(got, exp) {
		return fmt.Errorf("wrong hash: expected %x, got %x", exp, got)
	}
	return err
}

//
func BenchmarkBMT(t *testing.B) {
	for size := 4096; size >= 128; size /= 2 {
		t.Run(fmt.Sprintf("%v_size_%v", "SHA3", size), func(t *testing.B) {
			benchmarkSHA3(t, size)
		})
		t.Run(fmt.Sprintf("%v_size_%v", "Baseline", size), func(t *testing.B) {
			benchmarkBMTBaseline(t, size)
		})
		t.Run(fmt.Sprintf("%v_size_%v", "REF", size), func(t *testing.B) {
			benchmarkRefHasher(t, size)
		})
		t.Run(fmt.Sprintf("%v_size_%v", "BMT", size), func(t *testing.B) {
			benchmarkBMT(t, size)
		})
	}
}

type whenHash = int

const (
	first whenHash = iota
	last
	random
)

func BenchmarkBMTAsync(t *testing.B) {
	whs := []whenHash{first, last, random}
	for size := 4096; size >= 128; size /= 2 {
		for _, wh := range whs {
			for _, double := range []bool{false, true} {
				t.Run(fmt.Sprintf("double_%v_hash_when_%v_size_%v", double, wh, size), func(t *testing.B) {
					benchmarkBMTAsync(t, size, wh, double)
				})
			}
		}
	}
}

func BenchmarkPool(t *testing.B) {
	caps := []int{1, PoolSize}
	for size := 4096; size >= 128; size /= 2 {
		for _, c := range caps {
			t.Run(fmt.Sprintf("poolsize_%v_size_%v", c, size), func(t *testing.B) {
				benchmarkPool(t, c, size)
			})
		}
	}
}

//
func benchmarkSHA3(t *testing.B, n int) {
	data := newData(n)
	hasher := sha3.NewKeccak256
	h := hasher()

	t.ReportAllocs()
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		doSum(h, nil, data)
	}
}

//
//
//
//
//
func benchmarkBMTBaseline(t *testing.B, n int) {
	hasher := sha3.NewKeccak256
	hashSize := hasher().Size()
	data := newData(hashSize)

	t.ReportAllocs()
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		count := int32((n-1)/hashSize + 1)
		wg := sync.WaitGroup{}
		wg.Add(PoolSize)
		var i int32
		for j := 0; j < PoolSize; j++ {
			go func() {
				defer wg.Done()
				h := hasher()
				for atomic.AddInt32(&i, 1) < count {
					doSum(h, nil, data)
				}
			}()
		}
		wg.Wait()
	}
}

//
func benchmarkBMT(t *testing.B, n int) {
	data := newData(n)
	hasher := sha3.NewKeccak256
	pool := NewTreePool(hasher, segmentCount, PoolSize)
	bmt := New(pool)

	t.ReportAllocs()
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		syncHash(bmt, nil, data)
	}
}

//
func benchmarkBMTAsync(t *testing.B, n int, wh whenHash, double bool) {
	data := newData(n)
	hasher := sha3.NewKeccak256
	pool := NewTreePool(hasher, segmentCount, PoolSize)
	bmt := New(pool).NewAsyncWriter(double)
	idxs, segments := splitAndShuffle(bmt.SectionSize(), data)
	shuffle(len(idxs), func(i int, j int) {
		idxs[i], idxs[j] = idxs[j], idxs[i]
	})

	t.ReportAllocs()
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		asyncHash(bmt, nil, n, wh, idxs, segments)
	}
}

//
func benchmarkPool(t *testing.B, poolsize, n int) {
	data := newData(n)
	hasher := sha3.NewKeccak256
	pool := NewTreePool(hasher, segmentCount, poolsize)
	cycles := 100

	t.ReportAllocs()
	t.ResetTimer()
	wg := sync.WaitGroup{}
	for i := 0; i < t.N; i++ {
		wg.Add(cycles)
		for j := 0; j < cycles; j++ {
			go func() {
				defer wg.Done()
				bmt := New(pool)
				syncHash(bmt, nil, data)
			}()
		}
		wg.Wait()
	}
}

//
func benchmarkRefHasher(t *testing.B, n int) {
	data := newData(n)
	hasher := sha3.NewKeccak256
	rbmt := NewRefHasher(hasher, 128)

	t.ReportAllocs()
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		rbmt.Hash(data)
	}
}

func newData(bufferSize int) []byte {
	data := make([]byte, bufferSize)
	_, err := io.ReadFull(crand.Reader, data)
	if err != nil {
		panic(err.Error())
	}
	return data
}

//
func syncHash(h *Hasher, span, data []byte) []byte {
	h.ResetWithLength(span)
	h.Write(data)
	return h.Sum(nil)
}

func splitAndShuffle(secsize int, data []byte) (idxs []int, segments [][]byte) {
	l := len(data)
	n := l / secsize
	if l%secsize > 0 {
		n++
	}
	for i := 0; i < n; i++ {
		idxs = append(idxs, i)
		end := (i + 1) * secsize
		if end > l {
			end = l
		}
		section := data[i*secsize : end]
		segments = append(segments, section)
	}
	shuffle(n, func(i int, j int) {
		idxs[i], idxs[j] = idxs[j], idxs[i]
	})
	return idxs, segments
}

//
func asyncHashRandom(bmt SectionWriter, span []byte, data []byte, wh whenHash) (s []byte) {
	idxs, segments := splitAndShuffle(bmt.SectionSize(), data)
	return asyncHash(bmt, span, len(data), wh, idxs, segments)
}

//
//
//
//
func asyncHash(bmt SectionWriter, span []byte, l int, wh whenHash, idxs []int, segments [][]byte) (s []byte) {
	bmt.Reset()
	if l == 0 {
		return bmt.Sum(nil, l, span)
	}
	c := make(chan []byte, 1)
	hashf := func() {
		c <- bmt.Sum(nil, l, span)
	}
	maxsize := len(idxs)
	var r int
	if wh == random {
		r = rand.Intn(maxsize)
	}
	for i, idx := range idxs {
		bmt.Write(idx, segments[idx])
		if (wh == first || wh == random) && i == r {
			go hashf()
		}
	}
	if wh == last {
		return bmt.Sum(nil, l, span)
	}
	return <-c
}

//
//
//
//
func shuffle(n int, swap func(i, j int)) {
	if n < 0 {
		panic("invalid argument to Shuffle")
	}

//
//
//
//
//
//
	i := n - 1
	for ; i > 1<<31-1-1; i-- {
		j := int(rand.Int63n(int64(i + 1)))
		swap(i, j)
	}
	for ; i > 0; i-- {
		j := int(rand.Int31n(int32(i + 1)))
		swap(i, j)
	}
}
