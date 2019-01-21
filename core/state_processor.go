
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Go Ethereum作者
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

package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

//StateProcessor是一个基本的处理器，负责转换
//从一点到另一点的状态。
//
//StateProcessor实现处理器。
type StateProcessor struct {
config *params.ChainConfig //链配置选项
bc     *BlockChain         //规范区块链
engine consensus.Engine    //集体奖励的共识引擎
}

//NewStateProcessor初始化新的StateProcessor。
func NewStateProcessor(config *params.ChainConfig, bc *BlockChain, engine consensus.Engine) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}

//进程通过运行来根据以太坊规则处理状态更改
//事务消息使用statedb并对两者应用任何奖励
//处理器（coinbase）和任何包括的叔叔。
//
//流程返回流程中累积的收据和日志，以及
//返回过程中使用的气体量。如果有
//由于气体不足，无法执行事务，它将返回错误。
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.Header()
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.GasLimit())
	)
//根据任何硬分叉规范改变块和状态
	if p.config.DAOForkSupport && p.config.DAOForkBlock != nil && p.config.DAOForkBlock.Cmp(block.Number()) == 0 {
		misc.ApplyDAOHardFork(statedb)
	}
//设置块DPOS上下文
//迭代并处理单个事务
	for i, tx := range block.Transactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		receipt, _, err := ApplyTransaction(p.config, block.DposCtx(), p.bc, nil, gp, statedb, header, tx, usedGas, cfg)
		if err != nil {
			return nil, nil, 0, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}
//完成区块，应用任何共识引擎特定的额外项目（例如区块奖励）
	p.engine.Finalize(p.bc, header, statedb, block.Transactions(), block.Uncles(), receipts, block.DposCtx())

	return receipts, allLogs, *usedGas, nil
}


//ApplyTransaction尝试将事务应用到给定的状态数据库
//并为其环境使用输入参数。它返回收据
//对于交易、使用的天然气以及交易失败时的错误，
//指示块无效。
/*
func applyTransaction（config*params.chainconfig，bc chainContext，author*common.address，gp*gaspool，statedb*state.statedb，header*types.header，tx*types.transaction，usedgas*uint64，cfg vm.config）（*types.receipt，uint64，error）
 msg，err：=tx.asmessage（types.makesigner（config，header.number））。
 如果犯错！= nIL{
  返回nil，0，err
 }
 //创建要在EVM环境中使用的新上下文
 上下文：=newevmcontext（msg，header，bc，author）
 //创建一个保存所有相关信息的新环境
 //关于事务和调用机制。
 vmenv：=vm.newevm（context、statedb、config、cfg）
 //将事务应用于当前状态（包含在env中）
 _ux，gas，failed，err:=应用消息（vmenv，msg，gp）
 如果犯错！= nIL{
  返回nil，0，err
 }
 //用挂起的更改更新状态
 var根[ ]字节
 如果配置为IsByzantium（header.number）
  最终确定状态数据库（真）
 }否则{
  根=statedb.intermediateroot（config.iseip158（header.number））.bytes（））
 }
 *使用气体+气体

 //为交易创建新收据，存储Tx使用的中间根和气体
 //根据EIP阶段，我们正在传递根触式删除帐户。
 收据：=types.newreceipt（根，失败，*usedgas）
 receipt.txshash=tx.hash（）。
 receipt.gasused=气体
 //如果事务创建了合同，则将创建地址存储在收据中。
 if msg.to（）=零
  receipt.contractAddress=crypto.createAddress（vmenv.context.origin，tx.nonce（））
 }
 //设置接收日志并创建一个bloom进行过滤
 receipt.logs=statedb.getlogs（tx.hash（））
 receipt.bloom=types.createBloom（types.receipts receipt）

 退货收据，气体，错误
*/

func ApplyTransaction(config *params.ChainConfig, dposContext *types.DposContext, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, uint64, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
	if err != nil {
		return nil, 0, err
	}

	if msg.To() == nil && msg.Type() != types.Binary {
		return nil, 0, types.ErrInvalidType
	}

//创建要在EVM环境中使用的新上下文
	context := NewEVMContext(msg, header, bc, author)
//创建一个保存所有相关信息的新环境
//关于事务和调用机制。
	vmenv := vm.NewEVM(context, statedb, config, cfg)
//将事务应用于当前状态（包含在env中）
	_, gas, failed, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, 0, err
	}
	if msg.Type() != types.Binary {
		if err = applyDposMessage(dposContext, msg); err != nil {
			return nil, 0, err
		}
	}

//用挂起的更改更新状态
	var root []byte
	if config.IsByzantium(header.Number) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsEIP158(header.Number)).Bytes()
	}
	*usedGas += gas

//为交易创建新收据，存储Tx使用的中间根和气体
//基于EIP阶段，我们正在传递根触式删除帐户。
	receipt := types.NewReceipt(root, failed, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
//如果事务创建了合同，请将创建地址存储在收据中。
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
//设置收据日志并创建一个用于过滤的Bloom
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return receipt, gas, err
}
//更新包会执行所有的块内交易，如果发现交易类型不是转帐或合同调剂类型，将新的用户信息写入到候选人数据库中（候选人）
func applyDposMessage(dposContext *types.DposContext, msg types.Message) error {
	switch msg.Type() {
	case types.RegCandidate:
		dposContext.BecomeCandidate(msg.From())
	case types.UnregCandidate:
		dposContext.KickoutCandidate(msg.From())
	case types.Delegate:
		dposContext.Delegate(msg.From(), *(msg.To()))
	case types.UnDelegate:
		dposContext.UnDelegate(msg.From(), *(msg.To()))
	default:
		return types.ErrInvalidType
	}
	return nil
}

