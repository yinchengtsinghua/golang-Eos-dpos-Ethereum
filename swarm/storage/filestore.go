
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
	"io"
)

/*









*/


const (
defaultLDBCapacity                = 5000000 //
defaultCacheCapacity              = 10000   //
defaultChunkRequestsCacheCapacity = 5000000 //
)

type FileStore struct {
	ChunkStore
	hashFunc SwarmHasher
}

type FileStoreParams struct {
	Hash string
}

func NewFileStoreParams() *FileStoreParams {
	return &FileStoreParams{
		Hash: DefaultHash,
	}
}

//
func NewLocalFileStore(datadir string, basekey []byte) (*FileStore, error) {
	params := NewDefaultLocalStoreParams()
	params.Init(datadir)
	localStore, err := NewLocalStore(params, nil)
	if err != nil {
		return nil, err
	}
	localStore.Validators = append(localStore.Validators, NewContentAddressValidator(MakeHashFunc(DefaultHash)))
	return NewFileStore(localStore, NewFileStoreParams()), nil
}

func NewFileStore(store ChunkStore, params *FileStoreParams) *FileStore {
	hashFunc := MakeHashFunc(params.Hash)
	return &FileStore{
		ChunkStore: store,
		hashFunc:   hashFunc,
	}
}

//
//
//
//
//
func (f *FileStore) Retrieve(ctx context.Context, addr Address) (reader *LazyChunkReader, isEncrypted bool) {
	isEncrypted = len(addr) > f.hashFunc().Size()
	getter := NewHasherStore(f.ChunkStore, f.hashFunc, isEncrypted)
	reader = TreeJoin(ctx, addr, getter, 0)
	return
}

//
//
func (f *FileStore) Store(ctx context.Context, data io.Reader, size int64, toEncrypt bool) (addr Address, wait func(context.Context) error, err error) {
	putter := NewHasherStore(f.ChunkStore, f.hashFunc, toEncrypt)
	return PyramidSplit(ctx, data, putter, putter)
}

func (f *FileStore) HashSize() int {
	return f.hashFunc().Size()
}
