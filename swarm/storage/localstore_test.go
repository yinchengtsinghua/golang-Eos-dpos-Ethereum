
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
	"io/ioutil"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/chunk"
)

var (
	hashfunc = MakeHashFunc(DefaultHash)
)

//
//
//
func TestValidator(t *testing.T) {
//
	datadir, err := ioutil.TempDir("", "storage-testvalidator")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(datadir)

	params := NewDefaultLocalStoreParams()
	params.Init(datadir)
	store, err := NewLocalStore(params, nil)
	if err != nil {
		t.Fatal(err)
	}

//
	chunks := GenerateRandomChunks(259, 2)
	goodChunk := chunks[0]
	badChunk := chunks[1]
	copy(badChunk.SData, goodChunk.SData)

	PutChunks(store, goodChunk, badChunk)
	if err := goodChunk.GetErrored(); err != nil {
		t.Fatalf("expected no error on good content address chunk in spite of no validation, but got: %s", err)
	}
	if err := badChunk.GetErrored(); err != nil {
		t.Fatalf("expected no error on bad content address chunk in spite of no validation, but got: %s", err)
	}

//
//
	store.Validators = append(store.Validators, NewContentAddressValidator(hashfunc))
	chunks = GenerateRandomChunks(chunk.DefaultSize, 2)
	goodChunk = chunks[0]
	badChunk = chunks[1]
	copy(badChunk.SData, goodChunk.SData)

	PutChunks(store, goodChunk, badChunk)
	if err := goodChunk.GetErrored(); err != nil {
		t.Fatalf("expected no error on good content address chunk with content address validator only, but got: %s", err)
	}
	if err := badChunk.GetErrored(); err == nil {
		t.Fatal("expected error on bad content address chunk with content address validator only, but got nil")
	}

//
//
	var negV boolTestValidator
	store.Validators = append(store.Validators, negV)

	chunks = GenerateRandomChunks(chunk.DefaultSize, 2)
	goodChunk = chunks[0]
	badChunk = chunks[1]
	copy(badChunk.SData, goodChunk.SData)

	PutChunks(store, goodChunk, badChunk)
	if err := goodChunk.GetErrored(); err != nil {
		t.Fatalf("expected no error on good content address chunk with content address validator only, but got: %s", err)
	}
	if err := badChunk.GetErrored(); err == nil {
		t.Fatal("expected error on bad content address chunk with content address validator only, but got nil")
	}

//
//
	var posV boolTestValidator = true
	store.Validators = append(store.Validators, posV)

	chunks = GenerateRandomChunks(chunk.DefaultSize, 2)
	goodChunk = chunks[0]
	badChunk = chunks[1]
	copy(badChunk.SData, goodChunk.SData)

	PutChunks(store, goodChunk, badChunk)
	if err := goodChunk.GetErrored(); err != nil {
		t.Fatalf("expected no error on good content address chunk with content address validator only, but got: %s", err)
	}
	if err := badChunk.GetErrored(); err != nil {
		t.Fatalf("expected no error on bad content address chunk with content address validator only, but got: %s", err)
	}
}

type boolTestValidator bool

func (self boolTestValidator) Validate(addr Address, data []byte) bool {
	return bool(self)
}
