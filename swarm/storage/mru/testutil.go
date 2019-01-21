
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

package mru

import (
	"fmt"
	"path/filepath"

	"github.com/ethereum/go-ethereum/swarm/storage"
)

const (
	testDbDirName = "mru"
)

type TestHandler struct {
	*Handler
}

func (t *TestHandler) Close() {
	t.chunkStore.Close()
}

//
func NewTestHandler(datadir string, params *HandlerParams) (*TestHandler, error) {
	path := filepath.Join(datadir, testDbDirName)
	rh := NewHandler(params)
	localstoreparams := storage.NewDefaultLocalStoreParams()
	localstoreparams.Init(path)
	localStore, err := storage.NewLocalStore(localstoreparams, nil)
	if err != nil {
		return nil, fmt.Errorf("localstore create fail, path %s: %v", path, err)
	}
	localStore.Validators = append(localStore.Validators, storage.NewContentAddressValidator(storage.MakeHashFunc(resourceHashAlgorithm)))
	localStore.Validators = append(localStore.Validators, rh)
	netStore := storage.NewNetStore(localStore, nil)
	rh.SetStore(netStore)
	return &TestHandler{rh}, nil
}
