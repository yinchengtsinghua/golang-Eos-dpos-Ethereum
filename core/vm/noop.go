
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

package vm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func NoopCanTransfer(db StateDB, from common.Address, balance *big.Int) bool {
	return true
}
func NoopTransfer(db StateDB, from, to common.Address, amount *big.Int) {}

type NoopEVMCallContext struct{}

func (NoopEVMCallContext) Call(caller ContractRef, addr common.Address, data []byte, gas, value *big.Int) ([]byte, error) {
	return nil, nil
}
func (NoopEVMCallContext) CallCode(caller ContractRef, addr common.Address, data []byte, gas, value *big.Int) ([]byte, error) {
	return nil, nil
}
func (NoopEVMCallContext) Create(caller ContractRef, data []byte, gas, value *big.Int) ([]byte, common.Address, error) {
	return nil, common.Address{}, nil
}
func (NoopEVMCallContext) DelegateCall(me ContractRef, addr common.Address, data []byte, gas *big.Int) ([]byte, error) {
	return nil, nil
}

type NoopStateDB struct{}

func (NoopStateDB) CreateAccount(common.Address)                                       {}
func (NoopStateDB) SubBalance(common.Address, *big.Int)                                {}
func (NoopStateDB) AddBalance(common.Address, *big.Int)                                {}
func (NoopStateDB) GetBalance(common.Address) *big.Int                                 { return nil }
func (NoopStateDB) GetNonce(common.Address) uint64                                     { return 0 }
func (NoopStateDB) SetNonce(common.Address, uint64)                                    {}
func (NoopStateDB) GetCodeHash(common.Address) common.Hash                             { return common.Hash{} }
func (NoopStateDB) GetCode(common.Address) []byte                                      { return nil }
func (NoopStateDB) SetCode(common.Address, []byte)                                     {}
func (NoopStateDB) GetCodeSize(common.Address) int                                     { return 0 }
func (NoopStateDB) AddRefund(uint64)                                                   {}
func (NoopStateDB) GetRefund() uint64                                                  { return 0 }
func (NoopStateDB) GetState(common.Address, common.Hash) common.Hash                   { return common.Hash{} }
func (NoopStateDB) SetState(common.Address, common.Hash, common.Hash)                  {}
func (NoopStateDB) Suicide(common.Address) bool                                        { return false }
func (NoopStateDB) HasSuicided(common.Address) bool                                    { return false }
func (NoopStateDB) Exist(common.Address) bool                                          { return false }
func (NoopStateDB) Empty(common.Address) bool                                          { return false }
func (NoopStateDB) RevertToSnapshot(int)                                               {}
func (NoopStateDB) Snapshot() int                                                      { return 0 }
func (NoopStateDB) AddLog(*types.Log)                                                  {}
func (NoopStateDB) AddPreimage(common.Hash, []byte)                                    {}
func (NoopStateDB) ForEachStorage(common.Address, func(common.Hash, common.Hash) bool) {}
