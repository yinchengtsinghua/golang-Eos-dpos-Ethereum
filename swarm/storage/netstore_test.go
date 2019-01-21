
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
	"encoding/hex"
	"errors"
	"io/ioutil"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/swarm/network"
)

var (
	errUnknown = errors.New("unknown error")
)

type mockRetrieve struct {
	requests map[string]int
}

func NewMockRetrieve() *mockRetrieve {
	return &mockRetrieve{requests: make(map[string]int)}
}

func newDummyChunk(addr Address) *Chunk {
	chunk := NewChunk(addr, make(chan bool))
	chunk.SData = []byte{3, 4, 5}
	chunk.Size = 3

	return chunk
}

func (m *mockRetrieve) retrieve(ctx context.Context, chunk *Chunk) error {
	hkey := hex.EncodeToString(chunk.Addr)
	m.requests[hkey] += 1

//
	if m.requests[hkey] == 2 {
		return errUnknown
	}

//
	if m.requests[hkey] == 3 {
		*chunk = *newDummyChunk(chunk.Addr)
		go func() {
			time.Sleep(100 * time.Millisecond)
			close(chunk.ReqC)
		}()

		return nil
	}

	return nil
}

func TestNetstoreFailedRequest(t *testing.T) {
	searchTimeout = 300 * time.Millisecond

//
addr := network.RandomAddr() //

//
	datadir, err := ioutil.TempDir("", "netstore")
	if err != nil {
		t.Fatal(err)
	}
	params := NewDefaultLocalStoreParams()
	params.Init(datadir)
	params.BaseKey = addr.Over()
	localStore, err := NewTestLocalStoreForAddr(params)
	if err != nil {
		t.Fatal(err)
	}

	r := NewMockRetrieve()
	netStore := NewNetStore(localStore, r.retrieve)

	key := Address{}

//
//
//
//
//

//
	_, err = netStore.Get(context.TODO(), key)
	if got := r.requests[hex.EncodeToString(key)]; got != 2 {
		t.Fatalf("expected to have called retrieve two times, but got: %v", got)
	}
	if err != errUnknown {
		t.Fatalf("expected to get an unknown error, but got: %s", err)
	}

//
	chunk, err := netStore.Get(context.TODO(), key)
	if got := r.requests[hex.EncodeToString(key)]; got != 3 {
		t.Fatalf("expected to have called retrieve three times, but got: %v", got)
	}
	if err != nil || chunk == nil {
		t.Fatalf("expected to get a chunk but got: %v, %s", chunk, err)
	}
	if len(chunk.SData) != 3 {
		t.Fatalf("expected to get a chunk with size 3, but got: %v", chunk.SData)
	}
}
