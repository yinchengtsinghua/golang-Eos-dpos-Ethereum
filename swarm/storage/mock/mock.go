
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
//
//
//
//
//
//
//
//
//
package mock

import (
	"errors"
	"io"

	"github.com/ethereum/go-ethereum/common"
)

//
var ErrNotFound = errors.New("not found")

//
//
type NodeStore struct {
	store GlobalStorer
	addr  common.Address
}

//
//
func NewNodeStore(addr common.Address, store GlobalStorer) *NodeStore {
	return &NodeStore{
		store: store,
		addr:  addr,
	}
}

//
//
func (n *NodeStore) Get(key []byte) (data []byte, err error) {
	return n.store.Get(n.addr, key)
}

//
//
func (n *NodeStore) Put(key []byte, data []byte) error {
	return n.store.Put(n.addr, key, data)
}

//
//
//
//
type GlobalStorer interface {
	Get(addr common.Address, key []byte) (data []byte, err error)
	Put(addr common.Address, key []byte, data []byte) error
	HasKey(addr common.Address, key []byte) bool
//
//
//
	NewNodeStore(addr common.Address) *NodeStore
}

//
//
type Importer interface {
	Import(r io.Reader) (n int, err error)
}

//
//
type Exporter interface {
	Export(w io.Writer) (n int, err error)
}

//
//
type ImportExporter interface {
	Importer
	Exporter
}

//
//
type ExportedChunk struct {
	Data  []byte           `json:"d"`
	Addrs []common.Address `json:"a"`
}
