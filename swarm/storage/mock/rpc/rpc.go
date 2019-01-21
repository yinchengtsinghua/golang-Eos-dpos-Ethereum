
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
//
//
package rpc

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage/mock"
)

//
//
type GlobalStore struct {
	client *rpc.Client
}

//
func NewGlobalStore(client *rpc.Client) *GlobalStore {
	return &GlobalStore{
		client: client,
	}
}

//
func (s *GlobalStore) Close() error {
	s.client.Close()
	return nil
}

//
//
func (s *GlobalStore) NewNodeStore(addr common.Address) *mock.NodeStore {
	return mock.NewNodeStore(addr, s)
}

//
func (s *GlobalStore) Get(addr common.Address, key []byte) (data []byte, err error) {
	err = s.client.Call(&data, "mockStore_get", addr, key)
	if err != nil && err.Error() == "not found" {
//
		return data, mock.ErrNotFound
	}
	return data, err
}

//
func (s *GlobalStore) Put(addr common.Address, key []byte, data []byte) error {
	err := s.client.Call(nil, "mockStore_put", addr, key, data)
	return err
}

//
func (s *GlobalStore) HasKey(addr common.Address, key []byte) bool {
	var has bool
	if err := s.client.Call(&has, "mockStore_hasKey", addr, key); err != nil {
		log.Error(fmt.Sprintf("mock store HasKey: addr %s, key %064x: %v", addr, key, err))
		return false
	}
	return has
}
