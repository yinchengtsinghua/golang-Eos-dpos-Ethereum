
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2014 Go Ethereum作者
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

//软件包核心实现以太坊共识协议。
package core

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	mrand "math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/dpos"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/hashicorp/golang-lru"
	"gopkg.in/karalabe/cookiejar.v2/collections/prque"
)

var (
	blockInsertTimer = metrics.NewRegisteredTimer("chain/inserts", nil)

	ErrNoGenesis = errors.New("Genesis not found in chain")
)

const (
	bodyCacheLimit      = 256
	blockCacheLimit     = 256
	maxFutureBlocks     = 256
	maxTimeFutureBlocks = 30
	badBlockLimit       = 10
	triesInMemory       = 128

//blockchainversion确保不兼容的数据库强制从头开始重新同步。
	BlockChainVersion = 3
)

//cacheconfig包含trie缓存/修剪的配置值
//它位于区块链中。
type CacheConfig struct {
Disabled      bool          //是否禁用trie写缓存（存档节点）
TrieNodeLimit int           //内存限制（MB），在该限制下刷新内存中的当前trie到磁盘
TrieTimeLimit time.Duration //刷新内存中当前磁盘的时间限制
}

//区块链表示给定数据库的标准链，其中包含一个Genesis
//块。区块链管理链导入、恢复、链重组。
//
//将块导入到块链中是根据规则集进行的
//由两阶段验证程序定义。块的处理是使用
//处理所包含事务的处理器。国家的确认
//在验证器的第二部分完成。失败导致中止
//进口。
//
//区块链也有助于从包含的**任何**链返回区块。
//以及表示规范链的块。它是
//需要注意的是，getBlock可以返回任何块，不需要
//
//
type BlockChain struct {
chainConfig *params.ChainConfig //Chain & network configuration
cacheConfig *CacheConfig        //用于修剪的高速缓存配置

db     ethdb.Database //Low level persistent database to store final content in
triegc *prque.Prque   //Priority queue mapping block numbers to tries to gc
gcproc time.Duration  //Accumulates canonical block processing for trie dumping

	hc            *HeaderChain
	rmLogsFeed    event.Feed
	chainFeed     event.Feed
	chainSideFeed event.Feed
	chainHeadFeed event.Feed
	logsFeed      event.Feed
	scope         event.SubscriptionScope
	genesisBlock  *types.Block

mu      sync.RWMutex //global mutex for locking chain operations
chainmu sync.RWMutex //blockchain insertion lock
procmu  sync.RWMutex //block processor lock

checkpoint       int          //checkpoint counts towards the new checkpoint
currentBlock     atomic.Value //Current head of the block chain
currentFastBlock atomic.Value //快速同步链的当前磁头（可能在区块链上方！）

stateCache   state.Database //要在导入之间重用的状态数据库（包含状态缓存）
bodyCache    *lru.Cache     //缓存最新的块体
bodyRLPCache *lru.Cache     //以rlp编码格式缓存最新的块体
blockCache   *lru.Cache     //缓存最近的整个块
futureBlocks *lru.Cache     //future blocks are blocks added for later processing

quit    chan struct{} //blockchain quit channel
running int32         //运行必须以原子方式调用
//必须原子地调用ProcInterrupt
procInterrupt int32          //用于块处理的中断信号器
wg            sync.WaitGroup //正在关闭的链处理等待组

	engine    consensus.Engine
processor Processor //块处理器接口
validator Validator //block and state validator interface
	vmConfig  vm.Config

badBlocks *lru.Cache //坏块高速缓存
triedb *trie.Database //TIE数据库
}

//newblockchain使用信息返回完全初始化的块链
//在数据库中可用。它初始化默认的以太坊验证器并
//处理器。
func NewBlockChain(db ethdb.Database, cacheConfig *CacheConfig, chainConfig *params.ChainConfig, engine consensus.Engine, vmConfig vm.Config) (*BlockChain, error) {
	if cacheConfig == nil {
		cacheConfig = &CacheConfig{
			TrieNodeLimit: 256 * 1024 * 1024,
			TrieTimeLimit: 5 * time.Minute,
		}
	}
	bodyCache, _ := lru.New(bodyCacheLimit)
	bodyRLPCache, _ := lru.New(bodyCacheLimit)
	blockCache, _ := lru.New(blockCacheLimit)
	futureBlocks, _ := lru.New(maxFutureBlocks)
	badBlocks, _ := lru.New(badBlockLimit)

	bc := &BlockChain{
		chainConfig:  chainConfig,
		cacheConfig:  cacheConfig,
		db:           db,
		triegc:       prque.New(),
		stateCache:   state.NewDatabase(db),
		quit:         make(chan struct{}),
		bodyCache:    bodyCache,
		bodyRLPCache: bodyRLPCache,
		blockCache:   blockCache,
		futureBlocks: futureBlocks,
		engine:       engine,
		vmConfig:     vmConfig,
		badBlocks:    badBlocks,
	}
	bc.SetValidator(NewBlockValidator(chainConfig, bc, engine))
	bc.SetProcessor(NewStateProcessor(chainConfig, bc, engine))

	var err error
	bc.hc, err = NewHeaderChain(db, chainConfig, engine, bc.getProcInterrupt)
	if err != nil {
		return nil, err
	}
	bc.genesisBlock = bc.GetBlockByNumber(0)
	if bc.genesisBlock == nil {
		return nil, ErrNoGenesis
	}
	if err := bc.loadLastState(); err != nil {
		return nil, err
	}
//检查块哈希的当前状态，确保链中没有任何坏块
	for hash := range BadHashes {
		if header := bc.GetHeaderByHash(hash); header != nil {
//获取与有问题的头的编号相对应的规范块
			headerByNumber := bc.GetHeaderByNumber(header.Number.Uint64())
//确保headerByNumber（如果存在）位于当前的规范链中。
			if headerByNumber != nil && headerByNumber.Hash() == header.Hash() {
				log.Error("Found bad hash, rewinding chain", "number", header.Number, "hash", header.ParentHash)
				bc.SetHead(header.Number.Uint64() - 1)
				log.Error("Chain rewind was successful, resuming normal operation")
			}
		}
	}
//取得这个国家的所有权
	blockinterval := bc.GetBlockByNumber(0).Header().BlockInterval
	fmt.Println(blockinterval)
	ft := time.Duration(int64(blockinterval) / 2 )
	go bc.update(ft)
	return bc, nil
}

func (bc *BlockChain) getProcInterrupt() bool {
	return atomic.LoadInt32(&bc.procInterrupt) == 1
}

//loadLastState从数据库加载最后一个已知的链状态。这种方法
//假定保持链管理器互斥锁。
func (bc *BlockChain) loadLastState() error {
//恢复上一个已知的头块
	head := rawdb.ReadHeadBlockHash(bc.db)
	if head == (common.Hash{}) {
//数据库已损坏或为空，从头开始初始化
		log.Warn("Empty database, resetting chain")
		return bc.Reset()
	}
//确保整个头块可用
	currentBlock := bc.GetBlockByHash(head)
	if currentBlock == nil {
//数据库已损坏或为空，从头开始初始化
		log.Warn("Head block missing, resetting chain", "hash", head)
		return bc.Reset()
	}
//确保与块关联的状态可用
	if _, err := state.New(currentBlock.Root(), bc.stateCache); err != nil {
//没有关联状态的挂起块，从头开始初始化
		log.Warn("Head state missing, repairing chain", "number", currentBlock.Number(), "hash", currentBlock.Hash())
		if err := bc.repair(&currentBlock); err != nil {
			return err
		}
	}
//一切似乎都很好，设为头挡
	bc.currentBlock.Store(currentBlock)

//恢复上一个已知的头段
	currentHeader := currentBlock.Header()
	if head := rawdb.ReadHeadHeaderHash(bc.db); head != (common.Hash{}) {
		if header := bc.GetHeaderByHash(head); header != nil {
			currentHeader = header
		}
	}
	bc.hc.SetCurrentHeader(currentHeader)

//恢复上一个已知的头快速块
	bc.currentFastBlock.Store(currentBlock)
	if head := rawdb.ReadHeadFastBlockHash(bc.db); head != (common.Hash{}) {
		if block := bc.GetBlockByHash(head); block != nil {
			bc.currentFastBlock.Store(block)
		}
	}

//为用户发出状态日志
	currentFastBlock := bc.CurrentFastBlock()

	headerTd := bc.GetTd(currentHeader.Hash(), currentHeader.Number.Uint64())
	blockTd := bc.GetTd(currentBlock.Hash(), currentBlock.NumberU64())
	fastTd := bc.GetTd(currentFastBlock.Hash(), currentFastBlock.NumberU64())

	log.Info("Loaded most recent local header", "number", currentHeader.Number, "hash", currentHeader.Hash(), "td", headerTd)
	log.Info("Loaded most recent local full block", "number", currentBlock.Number(), "hash", currentBlock.Hash(), "td", blockTd)
	log.Info("Loaded most recent local fast block", "number", currentFastBlock.Number(), "hash", currentFastBlock.Hash(), "td", fastTd)

	return nil
}

//sethead将本地链重绕到新的head。在头的情况下，一切
//上面的新头部将被删除和新的一套。如果是积木
//但是，如果块体丢失（非存档），头部可能会被进一步重绕
//快速同步后的节点）。
func (bc *BlockChain) SetHead(head uint64) error {
	log.Warn("Rewinding blockchain", "target", head)

	bc.mu.Lock()
	defer bc.mu.Unlock()

//倒带标题链，删除所有块体，直到
	delFn := func(db rawdb.DatabaseDeleter, hash common.Hash, num uint64) {
		rawdb.DeleteBody(db, hash, num)
	}
	bc.hc.SetHead(head, delFn)
	currentHeader := bc.hc.CurrentHeader()

//从缓存中清除所有过时的内容
	bc.bodyCache.Purge()
	bc.bodyRLPCache.Purge()
	bc.blockCache.Purge()
	bc.futureBlocks.Purge()

//倒带区块链，确保不会以无状态头区块结束。
	if currentBlock := bc.CurrentBlock(); currentBlock != nil && currentHeader.Number.Uint64() < currentBlock.NumberU64() {
		bc.currentBlock.Store(bc.GetBlock(currentHeader.Hash(), currentHeader.Number.Uint64()))
	}
	if currentBlock := bc.CurrentBlock(); currentBlock != nil {
		if _, err := state.New(currentBlock.Root(), bc.stateCache); err != nil {
//重绕状态丢失，回滚到轴之前，重置为Genesis
			bc.currentBlock.Store(bc.genesisBlock)
		}
	}
//以简单的方式将快速块倒回目标头
	if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock != nil && currentHeader.Number.Uint64() < currentFastBlock.NumberU64() {
		bc.currentFastBlock.Store(bc.GetBlock(currentHeader.Hash(), currentHeader.Number.Uint64()))
	}
//如果任一块达到零，则重置为“创世”状态。
	if currentBlock := bc.CurrentBlock(); currentBlock == nil {
		bc.currentBlock.Store(bc.genesisBlock)
	}
	if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock == nil {
		bc.currentFastBlock.Store(bc.genesisBlock)
	}
	currentBlock := bc.CurrentBlock()
	currentFastBlock := bc.CurrentFastBlock()

	rawdb.WriteHeadBlockHash(bc.db, currentBlock.Hash())
	rawdb.WriteHeadFastBlockHash(bc.db, currentFastBlock.Hash())

	return bc.loadLastState()
}

//fastsynccommithead将当前头块设置为哈希定义的头块
//与之前的链内容无关。
func (bc *BlockChain) FastSyncCommitHead(hash common.Hash) error {
//确保块及其状态trie都存在
	block := bc.GetBlockByHash(hash)
	if block == nil {
		return fmt.Errorf("non existent block [%x…]", hash[:4])
	}
	if _, err := trie.NewSecure(block.Root(), bc.stateCache.TrieDB(), 0); err != nil {
		return err
	}
//如果全部签出，手动设置头块
	bc.mu.Lock()
	bc.currentBlock.Store(block)
	bc.mu.Unlock()

	log.Info("Committed new head block", "number", block.Number(), "hash", hash)
	return nil
}

//gas limit返回当前头块的气体限制。
func (bc *BlockChain) GasLimit() uint64 {
	return bc.CurrentBlock().GasLimit()
}

//currentBlock检索规范链的当前头块。这个
//块从区块链的内部缓存中检索。
func (bc *BlockChain) CurrentBlock() *types.Block {
	return bc.currentBlock.Load().(*types.Block)
}

//获取genesBlock头文件
/*******添加genesBlock**********
func（bc*区块链）genesBlock（）*types.block_
 返回BC.genesBlock
}

//currentFastBlock检索规范的当前快速同步头块
/ /链。块从区块链的内部缓存中检索。
func（bc*区块链）currentFastBlock（）*types.block_
 返回bc.currentFastBlock.load（）（*types.block）
}

//setprocessor设置进行状态修改所需的处理器。
func（bc*区块链）setprocessor（处理器处理器）
 bc.procmu.lock（）。
 延迟bc.procmu.unlock（）
 bc.processor=处理器
}

//setvalidator设置用于验证传入块的验证程序。
func（bc*区块链）setvalidator（验证器验证器）
 bc.procmu.lock（）。
 延迟bc.procmu.unlock（）
 bc.validator=验证程序
}

//validator返回当前的validator。
func（bc*区块链）验证器（）验证器
 bc.procmu.rlock（）。
 推迟bc.procmu.runlock（）
 返回BC.validator
}

//处理器返回当前处理器。

 
 
 


//state返回基于当前头块的新可变状态。
func（bc*区块链）state（）（*state.statedb，error）
 返回bc.stateat（bc.currentBlock（）.root（））
}

//stateat返回基于特定时间点的新可变状态。
func（bc*区块链）stateat（root common.hash）（*state.statedb，error）
 返回state.new（根目录，bc.statecache）
}

//重置清除整个区块链，将其恢复到其创始状态。
func（bc*区块链）reset（）错误
 返回bc.resetwithgenesblock（bc.genesblock）
}

//resetwithgenesBlock清除整个区块链，将其恢复到
//指定的Genesis状态。
func（bc*区块链）resetwithgenesBlock（genesis*types.block）错误
 //转储整个块链并清除缓存
 如果错误：=bc.sethead（0）；错误！= nIL{
  返回错误
 }
 B.MULK（）
 延迟bc.mu.unlock（）

 //准备Genesis块并重新初始化链
 如果错误：=bc.hc.writetd（genesis.hash（），genesis.numberu64（），genesis.difficulty（））；错误！= nIL{
  log.crit（“未能写入Genesis块td”，“err”，err）
 }
 rawdb.writeblock（bc.db，Genesis）

 bc.genesisblock=起源
 BC.插入（BC.GenerisBlock）
 bc.currentBlock.store（bc.genesBlock）
 bc.hc.setGenesis（bc.genesBlock.header（））
 bc.hc.setCurrentHeader（bc.genesBlock.header（））
 bc.currentFastBlock.store（bc.genesBlock）

 返回零
}

//修复试图通过回滚当前块来修复当前区块链
//直到找到一个具有关联状态的。这需要修复不完整的数据库
//由崩溃/断电或简单的未提交尝试引起的写入。
/ /
//此方法只回滚当前块。当前标题和当前
//快速块保持完整。
func（bc*区块链）修复（head**types.block）错误
 对于{
  //如果重绕到具有关联状态的头块，则中止
  if u，err：=state.new（（*head.root（），bc.statecache）；err==nil
   log.info（“将区块链重新返回到过去状态”，“数字”，（*head.number（），“哈希”，（*head.hash（））
   返回零
  }
  //否则，倒带一个块并在那里重新检查状态可用性
  （*head）=bc.getBlock（（*head）.parentHash（），（*head）.numberU64（）-1）
 }
}

//export将活动链写入给定的编写器。
func（bc*区块链）导出（w io.writer）错误
 返回bc.exportn（w，uint64（0），bc.currentBlock（）.numberU64（））
}

//exportn将活动链的子集写入给定的编写器。
func（bc*区块链）exportn（w io.writer，first uint64，last uint64）错误
 B.MU. RCOLD（）
 推迟bc.mu.runlock（）

 如果第一个>最后一个
  返回fmt.errorf（“导出失败：第一个（%d）大于最后一个（%d）”，第一个，最后一个）
 }
 log.info（“导出块批”，“count”，last first+1）

 开始，报告：=time.now（），time.now（））
 对于nr：=第一个；nr<=最后一个；nr++
  块：=BC.GetBlockByNumber（nr）
  如果block==nil
   返回fmt.errorf（“在%d上导出失败：找不到”，nr）
  }
  如果错误：=block.encoderlp（w）；错误！= nIL{
   返回错误
  }
  if time.since（reported）>=statsreportlimit_
   log.info（“exporting blocks”，“exported”，block.numberU64（）-first，“elapsed”，common.prettyDuration（time.since（start）））
   reported=time.now（）。
  }
 }

 返回零
}

//insert向当前块链中注入新的头块。这种方法
//假设块确实是一个真正的头。它还将重置头部
//头和头快速同步块与此非常相同的块（如果它们较旧）
//或者如果它们位于不同的侧链上。
/ /
//注意，此函数假定保留了'mu'互斥体！
func（bc*区块链）insert（block*types.block）
 //如果块在侧链或未知的侧链上，也将其他头强制到它上面
 更新头：=rawdb.readCanonicalHash（bc.db，block.numberU64（））！=块HASH（）

 //将块添加到规范链编号方案中并标记为头
 rawdb.writeCanonicalHash（bc.db，block.hash（），block.numberU64（））
 rawdb.writeHeadBlockHash（bc.db，block.hash（））

 当前块存储（块）

 //如果块优于头部或位于不同的链上，则强制更新头部
 如果更新头
  bc.hc.setCurrentHeader（block.header（））
  rawdb.writeHeadFastBlockHash（bc.db，block.hash（））

  bc.currentFastBlock.store（块）
 }
}

//Genesis检索链的Genesis块。
func（bc*区块链）genesis（）*types.block_
 返回BC.genesBlock
}

//getbody通过以下方式从数据库中检索块体（事务和uncles）
//哈希，如果找到，则缓存它。
func（bc*区块链）getbody（hash common.hash）*types.body_
 //如果主体已在缓存中，则短路，否则检索
 如果缓存，确定：=bc.bodycache.get（hash）；确定
  正文：=缓存。（*types.body）
  返回体
 }
 数字：=bc.hc.getBlockNumber（哈希）
 如果数字=零
  返回零
 }
 正文：=rawdb.readbody（bc.db，hash，*数字）
 如果body==nil
  返回零
 }
 //缓存下一次找到的主体并返回
 bc.bodycache.add（哈希，body）
 返回体
}

//getBodyrlp通过哈希从数据库中检索rlp编码的块体，
//如果找到缓存。
func（bc*区块链）getBodyrlp（hash common.hash）rlp.rawvalue_
 //如果主体已在缓存中，则短路，否则检索
 如果缓存，确定：=bc.bodyrlpcache.get（hash）；确定
  返回缓存的。（rlp.rawvalue）
 }
 数字：=bc.hc.getBlockNumber（哈希）
 如果数字=零
  返回零
 }
 正文：=rawdb.readbodyrlp（bc.db，hash，*数字）
 如果len（body）==0
  返回零
 }
 //缓存下一次找到的主体并返回
 bc.bodyrlpcache.add（哈希，body）
 返回体
}

//hasblock检查数据库中是否完全存在块。
func（bc*区块链）hasblock（hash common.hash，number uint64）bool_
 如果bc.blockcache.contains（hash）
  返回真
 }
 返回rawdb.hasbody（bc.db，hash，number）
}

//hasstate检查数据库中是否完全存在状态trie。
func（bc*区块链）hasstate（hash common.hash）bool_
 _uuErr：=bc.statecache.opentrie（哈希）
 返回错误=零
}

//hasblockandstate检查块和关联状态trie是否完全存在
//是否在数据库中，如果存在，则缓存它。
func（bc*区块链）hasblockandstate（hash common.hash，number uint64）bool_
 //首先检查块本身是否已知
 
 如果block==nil
  返回假
 }
 返回bc.hasstate（block.root（））
}

//getblock通过哈希和数字从数据库中检索一个块，
//如果找到缓存。
func（bc*区块链）getblock（hash common.hash，number uint64）*types.block_
 //如果块已经在缓存中，则短路，否则检索
 如果是块，OK：= BC.BaskCase.GET（hash）；ok {
  返回块。（*types.block）
 }
 块：=rawdb.readblock（bc.db，hash，number）
 如果block==nil
  返回零
 }
 //缓存下一次找到的块并返回
 bc.blockcache.add（block.hash（），块）
 返回块
}

//getBlockByHash通过哈希从数据库中检索块，如果找到块，则将其缓存。
func（bc*区块链）getBlockByHash（hash common.hash）*types.block_
 数字：=bc.hc.getBlockNumber（哈希）
 如果数字=零
  返回零
 }
 返回bc.getblock（哈希，*数字）
}

//getBlockByNumber按编号从数据库中检索块，并将其缓存
//（与哈希关联）如果找到。
func（bc*区块链）getBlockByNumber（数字uint64）*types.block_
 哈希：=rawdb.readCanonicalHash（bc.db，number）
 如果hash==（common.hash）
  返回零
 }
 返回bc.getblock（哈希，数字）
}

//getReceiptsByHash检索给定块中所有事务的收据。
func（bc*区块链）getreceiptsbyhash（hash common.hash）类型.receipts_
 数字：=rawdb.readheadernumber（bc.db，hash）
 如果数字=零
  返回零
 }
 返回rawdb.readReceipts（bc.db，hash，*数字）
}

//getblocksfromhash返回哈希对应的块以及最多n-1个祖先。
//[被ETH/62否决]
func（bc*区块链）getblocksfromhash（hash common.hash，n int）（blocks[]*types.block）
 数字：=bc.hc.getBlockNumber（哈希）
 如果数字=零
  返回零
 }
 对于i：=0；i<n；i++
  块：=bc.getblock（哈希，*数字）
  如果block==nil
   打破
  }
  块=附加（块，块）
  hash=block.parenthash（）。
  *数字——
 }
 返回
}

//getUnbenshinchain向后检索给定块中的所有unbes，直到
//达到特定距离。
func（bc*区块链）getUnconsinchain（block*types.block，length int）[]*types.header_
 叔叔：=[]*类型.表头
 对于i：=0；块！=nil&&i<length；i++
  uncles=追加（uncles，block.uncles（）…）
  block=bc.getblock（block.parenthash（），block.numberU64（）-1）
 }
 归来叔叔
}

//trie node检索与trie节点（或代码哈希）关联的数据块
//要么来自短暂的内存缓存，要么来自持久存储。
func（bc*区块链）三节点（hash common.hash）（[]字节，错误）
 返回bc.statecache.triedb（）.node（哈希）
}

//停止停止区块链服务。如果任何导入当前正在进行中
//它将使用procInterrupt中止它们。
func（bc*区块链）stop（）
 如果！原子.compareandswapint32（&bc.running，0，1）
  返回
 }
 //取消订阅从区块链注册的所有订阅
 bc.scope.close（）。
 关闭（BC.退出）
 atomic.storeInt32（&bc.procInterrupt，1）

 BC.WG.WAIT（）

 //在退出前，确保最近块的状态也存储到磁盘。
 //我们正在编写三种不同的状态来捕获不同的重新启动方案：
 //-头：所以在一般情况下，我们不需要重新处理任何块。
 //head-1：所以如果我们的头变成叔叔，我们就不会进行大的重组。
 //HEAD-127：所以我们对重新执行的块数有一个硬限制
 如果！bc.cacheconfig.disabled_
  triedb:=bc.statecache.triedb（）。

  对于u，偏移量：=范围[]uint64 0，1，triesinmemory-1
   如果数字：=bc.currentBlock（）.numberU64（）；数字>偏移量
    最近：=bc.getBlockByNumber（数字-偏移量）

    log.info（“正在将缓存状态写入磁盘”，“block”，recent.number（），“hash”，recent.hash（），“root”，recent.root（））
    如果错误：=triedb.commit（recent.root（），true）；错误！= nIL{
     log.error（“提交最近状态trie失败”，“err”，err）
    }
   }
  }
  为了！bc.triegc.empty（）
   triedb.dereference（bc.triegc.popItem（）（common.hash））。
  }
  如果大小，：=triedb.size（）；大小！= 0 {
   log.error（“完全清理后悬空trie节点”）
  }
 }
 日志信息（“区块链管理器已停止”）
}

func（bc*区块链）procFutureBlocks（）
 块：=make（[]*types.block，0，bc.futureBlocks.len（））
 对于uu，哈希：=range bc.futureBlocks.keys（）
  如果是块，则存在：=bc.futureBlocks.peek（hash）；exist
   blocks=附加（blocks，block.（*types.block））。
  }
 }
 如果长度（块）>0
  types.blockby（types.number）.sort（块）

  //逐个插入，因为链插入需要块之间的连续祖先
  对于i：=范围块
   BC.插入链（块[I:I+1]）
  }
 }
}

//写入状态
类型写入状态字节

康斯特
 非atty writestatus=iota
 规范性
 副作用
）

//回滚的目的是从数据库中删除一个链接链，而该链接链不是
//足够确定有效。
func（bc*区块链）回滚（chain[]common.hash）
 B.MULK（）
 延迟bc.mu.unlock（）

 对于i：=len（chain）-1；i>=0；i--
  哈希：=链[i]

  当前标题：=bc.hc.currentHeader（）
  if currentHeader.hash（）==哈希
   bc.hc.setCurrentHeader（bc.getHeader（currentHeader.parentHash，currentHeader.number.uint64（）-1））。
  }
  如果currentFastBlock：=bc.currentFastBlock（）；currentFastBlock.hash（）==hash
   newFastBlock：=bc.getBlock（currentFastBlock.parentHash（），currentFastBlock.numberU64（）-1）
   bc.currentFastBlock.store（newFastBlock）
   rawdb.writeHeadFastBlockHash（bc.db，newFastBlock.Hash（））
  }
  如果当前块：=bc.currentBlock（）；currentBlock.hash（）==hash
   newblock：=bc.getblock（currentblock.parenthash（），currentblock.numberU64（）-1）
   当前块存储（newblock）
   rawdb.writeHeadBlockHash（bc.db，newblock.hash（））
  }
 }
}

//setReceiptsData计算收据的所有非共识字段
func setreceiptsdata（config*params.chainconfig，block*types.block，receipts types.receipts）错误
 签名者：=types.makesigner（config，block.number（））

 事务，logindex：=block.transactions（），uint（0）
 如果len（交易）！=len（收据）
  返回errors.new（“交易和收据计数不匹配”）
 }

 对于j：=0；j<len（收据）；j++
  //可以从事务本身检索事务哈希
  收据[J].txshash=交易[J].hash（））

  //合同地址可以从事务本身派生
  如果事务[j].to（）==nil
   //派生签名者很昂贵，只有在实际需要时才这样做
   发件人，：=types.sender（签名人，事务[j]）
   receipts[j].contractAddress=crypto.createAddress（发件人，事务[j].nonce（））
  }
  //用气可根据以前的收货量计算
  如果j＝0 {
   receipts[j].gasused=收据[j].累计使用的气体
  }否则{
   Receipts[J].GasUsed=收据[J].Accumulative GasUsed-收据[J-1].Accumulative GasUsed
  }
  //派生的日志字段可以简单地从块和事务中设置
  对于k：=0；k<len（receipts[j].logs）；k++
   收据[J].logs[K].blockNumber=block.numberU64（）
   收据[J].logs[K].blockHash=block.hash（）。
   receipts[j].logs[k].txshash=收据[j].txshash
   收据[J].Logs[K].TxIndex=uint（J）
   收据[J].logs[K].index=logindex
   日志索引+
  }
 }
 返回零
}

//InsertReceiptChain尝试使用
//交易和接收数据。
func（bc*区块链）insertreceiptchain（区块链类型.blocks，receiptchain[]类型.receipts）（int，error）
 添加（1）
 推迟bc.wg.done（）

 //进行健全性检查，确保提供的链实际上是有序的和链接的
 对于i：=1；i<len（区块链）；i++
  如果区块链[i].numberU64（）！=区块链[I-1].numberU64（）+1区块链[I].parentHash（）！=区块链[I-1].hash（）
   log.error（“非连续收据插入”，“数字”，区块链[I].number（），“哈希”，区块链[I].hash（），“父”，区块链[I].parent hash（），
    “prevNumber”，区块链[I-1].number（），“prevHash”，区块链[I-1].hash（））
   返回0，fmt.errorf（“非连续插入：项%d是%d[%x…]，项%d是%d[%x…]（父项[%x…））”，I-1，区块链[i-1].numberU64（），
    区块链[I-1].hash（）.bytes（）[：4]，i，区块链[I].numberU64（），区块链[I].hash（）.bytes（）[：4]，区块链[I].parenthash（）.bytes（）[：4]）
  }
 }

 var
  stats=struct已处理，忽略int32
  开始=时间。现在（））
  字节＝0
  批处理=bc.db.newbatch（））
 ）
 对于i，块：=范围区块链
  收据：=收据链[I]
  //关闭或处理失败时短路插入
  如果atomic.loadint32（&bc.procInterrupt）==1
   返回0，零
  }
  //所有者头未知时短路
  如果！bc.hasheader（block.hash（），block.numberU64（））
   返回i，fmt.errorf（“包含头%d[%x…]未知”，block.number（），block.hash（）.bytes（）[：4]）
  }
  //如果整个数据都已知，则跳过
  if bc.hasblock（block.hash（），block.numberu64（））
   忽略+
   持续
  }
  //计算所有收货的非共识字段
  如果错误：=setreceiptsdata（bc.chainconfig，block，receipts）；错误！= nIL{
   返回i，fmt.errorf（“设置收据数据失败：%v”，err）
  }
  //将所有数据写入数据库
  rawdb.writebody（批处理，block.hash（），block.numberU64（），block.body（））
  rawdb.writeReceipts（批处理、block.hash（）、block.numberU64（）、收据）
  rawdb.writetxLookupEntries（批处理、块）

  状态。已处理++

  如果batch.valuesize（）>=ethdb.idealbatchisize
   如果错误：=batch.write（）；错误！= nIL{
    返回0
   }
   bytes+=批处理。valuesize（）
   批处理
  }
 }
 如果batch.valueSize（）>0
  bytes+=批处理。valuesize（）
  如果错误：=batch.write（）；错误！= nIL{
   返回0
  }
 }

 //如果更好，请更新head fast sync块
 B.MULK（）
 头：=区块链[len（区块链）-1]
 如果td：=bc.gettd（head.hash（），head.numberu64（））；td！=nil//可能发生倒带，在这种情况下跳过
  当前快速块：=bc.currentFastBlock（）
  if bc.gettd（currentFastBlock.hash（），currentFastBlock.numberU64（））.cmp（td）<0_
   rawdb.writeHeadFastBlockHash（bc.db，head.hash（））
   bc.currentFastBlock.store（头）
  }
 }
 BU.MU.解锁（）

 log.info（“导入的新块收据”，
  “计数”，统计处理，
  “已用”，common.prettyDuration（time.since（start）），
  “数字”，head.number（），
  “hash”，head.hash（），
  “大小”，公用。存储大小（字节），
  “忽略”，stats.ignored）
 返回0，零
}

var lastwrite uint64

//WriteBlockWithOutState只将块及其元数据写入数据库，
//但不写入任何状态。这是用来构建竞争侧叉
//直至超过标准总难度。
func（bc*区块链）writeblockwithoutstate（block*types.block，td*big.int）（err错误）
 添加（1）
 推迟bc.wg.done（）

 如果错误：=bc.hc.writetd（block.hash（），block.numberu64（），td）；错误！= nIL{
  返回错误
 }
 rawdb.writeblock（bc.db，块）

 返回零
}

//WriteBlockWithState将块和所有关联状态写入数据库。
func（bc*区块链）writeblockwithstate（block*types.block，receipts[]*types.receipt，state*state.statedb）（status writestatus，err error）
 添加（1）
 推迟bc.wg.done（）

 //计算块的总难度
 ptd：=bc.gettd（block.parenthash（），block.numberU64（）-1）
 如果PTD＝nIL {
  返回非atty，converse.errunknownancestor
 }
 //确保插入期间没有不一致的状态泄漏
 B.MULK（）
 延迟bc.mu.unlock（）

 当前块：=bc.currentBlock（））
 localtd:=bc.gettd（currentBlock.hash（），currentBlock.numberU64（））
 externtd：=new（big.int）.add（block.difficulty（），ptd）

 //与规范状态无关，将块本身写入数据库
 如果错误：=bc.hc.writetd（block.hash（），block.numberu64（），externtd）；错误！= nIL{
  返回非atty，err
 }
 rawdb.writeblock（bc.db，块）

 根，错误：=state.commit（bc.chainconfig.iseip158（block.number（））
 如果犯错！= nIL{
  返回非atty，err
 }

 triedb:=bc.statecache.triedb（）。

 //如果运行的是存档节点，则始终刷新
 如果bc.cacheconfig.disabled_
  如果错误：=triedb.commit（root，false）；错误！= nIL{
   返回非atty，err
  }
 }否则{
  //已满但不是存档节点，请执行正确的垃圾收集
  triedb.reference（root，common.hash）//保持trie活动的元数据引用
  bc.triegc.push（根，-float32（block.numberU64（））

  如果当前：=block.numberU64（）；当前>triesInMemory
   //如果超出内存限制，将成熟的单例节点刷新到磁盘
   var
    节点，imgs=triedb.size（）
    limit=common.storagesize（bc.cacheconfig.trienodelimit）*1024*1024
   ）
   如果节点>限制imgs>4*1024*1024
    Triedb.Cap（限值-ethdb.idealbatchisize）
   }
   //找到我们需要提交的下一个状态trie
   头：=bc.getHeaderByNumber（当前-triesInMemory）
   选择：=header.number.uint64（）

   //如果超出超时限制，则将整个trie刷新到磁盘
   如果bc.gcproc>bc.cacheconfig.trietimelimit
    //如果我们超出了限制，但没有达到足够大的内存间隙，
    //警告用户系统不稳定。
    如果选择<lastwrite+triesinmemory&&bc.gcproc>=2*bc.cacheconfig.trietimelimit
     log.info（“内存中的状态太长，正在提交”，“时间”，bc.gcproc，“允许”，bc.cacheconfig.trietimelimit，“最佳”，float64（selected lastwrite）/triesinmemory）
    }
    //刷新整个trie并重新启动计数器
    triedb.commit（header.root，真）
    lastwrite=已选择
    BC.GCPROC＝0
   }
   //垃圾收集低于所需写保持期的任何内容
   为了！bc.triegc.empty（）
    根，编号：=bc.triegc.pop（）
    如果选择了uint64（-number）>
     triegc.push（根，数字）
     打破
    }
    triedb.dereference（根。（common.hash））
   }
  }
 }

 //使用批处理写入其他块数据。
 批次：=bc.db.newbatch（））
 rawdb.writeReceipts（批处理、block.hash（）、block.numberU64（）、收据）

 如果uuErr：=block.dposContext.commit（）；err！= nIL{
  返回非atty，err
 }

 //如果总难度大于已知值，则将其添加到规范链中
 //if语句中的第二个子句减少了自私挖掘的脆弱性。
 //请参阅http://www.cs.cornell.edu/~ie53/publications/btcprocfc.pdf
 REORG：=externtd.cmp（localtd）>0
 currentBlock=bc.currentBlock（））
 如果！reorg和externtd.cmp（localtd）==0_
  //按数字拆分相同的难度块，然后随机
  reorg=block.numberU64（）<currentBlock.numberU64（）（block.numberU64（）==currentBlock.numberU64（）&&mrand.float64（）<0.5）
 }
 如果ReRG {
  //如果父级不是头块，则重新组织链
  如果block.parenthash（）！=currentBlock.hash（）
   如果错误：=bc.reorg（currentblock，block）；错误！= nIL{
    返回非atty，err
   }
  }
  //写入事务/收据查找和预映像的位置元数据
  rawdb.writetxLookupEntries（批处理、块）
  rawdb.writePreimages（批处理，block.numberU64（），state.preimages（））

  状态=CanonStatty
 }否则{
  状态=侧滑
 }
 如果错误：=batch.write（）；错误！= nIL{
  返回非atty，err
 }

 //设置新的头。
 如果状态=CanonStatty
  插入（块）
 }
 bc.futureBlocks.remove（block.hash（））
 返回状态，无
}

//insertchain尝试将给定批块插入规范
//链或创建一个分叉。如果返回错误，它将返回
//失败块的索引号以及描述所执行操作的错误
/错了。
/ /
//插入完成后，将激发所有累积的事件。
func（bc*区块链）insertchain（chain types.blocks）（int，error）
 n，事件，日志，错误：=bc.insertchain（链）
 BC.后置事件（事件、日志）
 返回n
}

//insertchain将执行实际的链插入和事件聚合。这个
//此方法作为独立方法存在的唯一原因是为了使锁定更清晰
//带有延迟语句。
func（bc*区块链）insertchain（chain types.blocks）（int，[]interface，[]*types.log，error）
 //健全性检查我们有什么有意义的东西要导入
 如果len（链）==0
  返回0，nil，nil，nil
 }
 //从Genesis块获取Genesis头段
 genesisblock := bc.genesisBlock
 //进行健全性检查，确保提供的链实际上是有序的和链接的
 对于i：=1；i<len（chain）；i++
  如果链[i].numberU64（）！=chain[i-1].numberU64（）+1 chain[i].parentHash（）！=chain[i-1].hash（）
   //断链祖先，记录消息（编程错误）并跳过插入
   log.error（“非连续块插入”，“数字”，chain[i].number（），“哈希”，chain[i].hash（），
    “parent”，链[i].parent hash（），“prevnumber”，链[i-1].number（），“prevhash”，链[i-1].hash（））

   返回0，nil，nil，fmt.errorf（“非连续插入：项%d是%d[%x…]，项%d是%d[%x…]（父项[%x…））”，I-1，链[i-1].numberU64（），
    chain[i-1].hash（）.bytes（）[：4]，i，chain[i].numberU64（），chain[i].hash（）.bytes（）[：4]，chain[i].parenthash（）.bytes（）[：4]）
  }
 }
 //预检查通过，开始全块导入
 添加（1）
 推迟bc.wg.done（）

 bc.chainmu.lock（）。
 延迟bc.chainmu.unlock（）

 //传递事件的排队方法。这通常是
 //比直接传递更快，需要的互斥量更少
 /获取。
 var
  stats=insertstats开始时间：mclock.now（）
  事件=生成（[]接口，0，len（chain））
  lastcanon*类型.block
  合并日志[]*types.log
 ）
 //启动并行头验证程序
 标题：=make（[]*types.header，len（chain））
 密封件：=制造（[]动臂、长度（链条））

 对于i，块：=范围链
  头文件[i]=block.header（））
  海豹[I] =真
 }
 中止，结果：=bc.engine.verifyheaders（bc，headers，seals）
 延迟关闭（中止）

 //启动并行签名恢复（签名者将在fork转换时出错，性能损失最小）
 senderCacher.recoverFromBlocks（types.makeSigner（bc.chainconfig，chain[0].number（）），chain）

 //遍历块，并在验证器允许时插入
 对于i，块：=范围链
  //如果链终止，则停止处理块
  如果atomic.loadint32（&bc.procInterrupt）==1
   log.debug（“块处理期间过早中止”）
   打破
  }
  //如果头是禁止的，则直接中止
  if badhashes[block.hash（）]
   BC.报告块（block、nil、errBlackListedHash）
   返回i、事件、合并日志、errBlackListedHash
  }
  //等待块的验证完成
  bstart：=time.now（）。

  错误：=<-结果
  如果Err＝nIL {
   err=bc.validator（）.validatebody（块）
  }
  交换机{
  case err==errknownBlock:
   //块和状态都已知。但是，如果当前块低于
   //这个数字我们进行了回滚，但是我们应该重新导入它。
   如果bc.currentBlock（）.numberU64（）>=block.numberU64（）
    忽略+
    持续
   }

  case err==共识。errFutureBlock:
   //在未来块中最多允许MaxFuture秒。如果超过此限制
   //如果给定，链将被丢弃并在稍后进行处理。
   最大值：=big.newint（time.now（）.unix（）+maxTimeFutureBlocks）
   if block.time（）.cmp（max）>0_
    返回i，事件，合并日志，fmt.errorf（“未来块：%v>%v”，block.time（），max）
   }
   bc.futureBlocks.add（block.hash（），块）
   排队+
   持续

  case err==communication.errUnknownanceStor&&bc.futureBlocks.contains（block.parentHash（））：
   bc.futureBlocks.add（block.hash（），块）
   排队+
   持续

  案例错误=共识。错误删除取消者：
   //块与规范链竞争，存储在数据库中，但不处理
   //直到竞争对手td超过标准td
   当前块：=bc.currentBlock（））
   localtd:=bc.gettd（currentBlock.hash（），currentBlock.numberU64（））
   externtd：=new（big.int）.add（bc.gettd（block.parenthash（），block.numberu64（）-1），block.difficulty（））
   如果localtd.cmp（externtd）>0
    如果err=bc.writeBlockWithOutState（block，externtd）；err！= nIL{
     返回i，事件，合并日志，错误
    }
    持续
   }
   //竞争对手chain beat canonical，收集来自共同祖先的所有块
   var winner[]*类型.block

   父级：=bc.getBlock（block.parentHash（），block.numberU64（）-1）
   为了！bc.hasstate（parent.root（））
    winner=附加（winner，parent）
    parent=bc.getBlock（parent.parentHash（），parent.numberU64（）-1）
   }
   对于j：=0；j<len（winner）/2；j++
    winner[j]，winner[len（winner）-1-j]=winner[len（winner）-1-j]，winner[j]
   }
   //导入所有修剪的块以使状态可用
   bc.chainmu.unlock（）。
   _uu，evs，logs，err：=bc.insertchain（获胜者）
   bc.chainmu.lock（）。
   events，coalescedlogs=evs，日志

   如果犯错！= nIL{
    返回i，事件，合并日志，错误
   }

  案例错误！= nIL:
   BC.报告块（block、nil、err）
   返回i，事件，合并日志，错误
  }
  //使用父块创建新的statedb并报告
  //如果失败则出错。
  var父级*类型.block
  如果i＝0 {
   parent=bc.getBlock（block.parentHash（），block.numberU64（）-1）
  }否则{
   父级=链[I-1]
  }

  block.dposContext，err=types.newdposContextFromProto（bc.statecache.triedb（），parent.header（）.dposContext）

  如果犯错！= nIL{
   返回i，事件，合并日志，错误
  }
  状态，错误：=state.new（parent.root（），bc.statecache）
  如果犯错！= nIL{
   返回i，事件，合并日志，错误
  }
  //使用父状态作为参考点处理块。
  receipts，logs，usedgas，err：=bc.processor.process（块，状态，bc.vmconfig）
  如果犯错！= nIL{
   BC.报告块（块、收据、错误）
   返回i，事件，合并日志，错误
  }
  //使用默认验证器验证状态
  err=bc.validator（）.validateState（块、父级、状态、收据、usedgas）
  如果犯错！= nIL{
   BC.报告块（块、收据、错误）
   返回i，事件，合并日志，错误
  }
  proctime:=开始时间（bstart）
  //使用默认验证器验证DPOS状态
  err=bc.validator（）.validatedpostate（块）
  如果犯错！= nIL{
   BC.报告块（块、收据、错误）
   返回i，事件，合并日志，错误
  }
  //验证验证器
  dpos engine，isdpos:=bc.引擎。（*dpos.dpos）
  如果ISDPOS {
   err=dposengine.verifyseal（bc，block.header（），genesisblock.header（））
   如果犯错！= nIL{
    BC.报告块（块、收据、错误）
    返回i，事件，合并日志，错误
   }
  }

  //使用默认验证器验证DPOS状态

  //将块写入链并获取状态。
  状态，错误：=bc.WriteBlockWithState（块、收据、状态）
  如果犯错！= nIL{
   返回i，事件，合并日志，错误
  }
  开关状态{
  案例规范：
   log.debug（“inserted new block”，“number”，block.number（），“hash”，block.hash（），“uncles”，len（block.uncles（）），
    “txs”，len（block.transactions（）），“gas”，block.gasused（），“elapsed”，common.prettyDuration（time.since（bstart）））

   coalescedlogs=append（coalescedlogs，logs…）
   blockInsertTimer.updateSince（bstart）
   events=append（events，chainevent block，block.hash（），logs）
   lastcanon=阻止

   //只计算GC处理时间的规范块
   bc.gcproc+=程序时间

  箱侧状态：
   log.debug（“inserted forked block”，“number”，block.number（），“hash”，block.hash（），“diff”，block.diffly（），“elapsed”，
    common.prettyDuration（time.since（bstart）），“txs”，len（block.transactions（）），“gas”，block.gasused（），“uncles”，len（block.uncles（）））

   blockInsertTimer.updateSince（bstart）
  }
  状态。已处理++
  stats.usedgas+=已使用dgas

  缓存，：=bc.statecache.triedb（）.size（））
  stats.report（链、i、缓存）
 }
 //如果我们已经进行了链，则附加一个单链头事件
 如果拉斯卡农！=nil&&bc.currentBlock（）.hash（）==lastcanon.hash（）
  事件=附加（事件，chainHeadEvent lastCanon）
 }
 返回0，事件，合并日志，零
}

//insertstats跟踪和报告块插入。
类型insertstats结构
 排队，已处理，忽略int
 使用的GAS UInt64
 最后一个索引int
 开始时间mclock.abstime
}

//statsreportlimit是导入和导出期间的时间限制，在此之后
//始终打印出进度。这避免了用户想知道发生了什么。
const statsreportlimit=8*时间。秒

//如果处理了一些块，则报告将打印统计信息
//或自上一条消息后超过几秒钟。
func（st*insertstats）报告（chain[]*types.block，index int，cache common.storagesize）
 //获取批的计时
 var
  现在=mclock.now（））
  已用=时间。持续时间（现在）-时间。持续时间（st.starttime）
 ）
 //如果我们在到达的批或报告期的最后一个块，则记录
 如果index==len（chain）-1 elapsed>=statsreportlimit
  var
   结束=链[索引]
   txs=countTransactions（链[st.lastindex:index+1]）
  ）
  上下文：=[]接口
   “块”，st.processed，“txs”，txs，“mgas”，float64（st.usedgas）/1000000，
   “elaped”，common.prettyDuration（elaped），“mgasps”，float64（st.usedgas）*1000/float64（elaped），
   “number”，end.number（），“hash”，end.hash（），“cache”，缓存，
  }
  如果st.queued>0
   context=append（context，[]接口“排队”，st.queued…）
  }
  如果st.ignored>0
   context=append（context，[]interface“忽略”，st.ignored…）
  }
  log.info（“导入的新链段”，上下文…）

  *st=insertstats_starttime:now，lastindex:index+1_
 }
}

func counttransactions（chain[]*types.block）（c int）
 对于，b：=范围链
  C+=len（b.transactions（））
 }
 返回C
}

//REORGS接受两个块，一个旧链和一个新链，并将重新构造块并插入它们
//作为新规范链的一部分，累积潜在的丢失事务并发布
//关于它们的事件
func（bc*区块链）reorg（oldblock，newblock*types.block）错误
 var
  newchain类型.blocks
  旧链类型.块
  commonblock*类型.block
  DeletedTxs类型.Transactions
  deletedlogs[]*类型.log
  //CollectLogs收集在
  //处理与给定哈希对应的块。
  //这些日志稍后被宣布为已删除。
  collectlogs=func（hash common.hash）
   //合并日志并设置“removed”。
   数字：=bc.hc.getBlockNumber（哈希）
   如果数字=零
    返回
   }
   收据：=rawdb.readreceipts（bc.db，hash，*个）
   对于uu，收据：=范围收据
    对于u，日志：=范围接收。日志
     del:**日志
     del.removed=真
     deletedlogs=附加（deletedlogs&del）
    }
   }
  }
 ）

 //先减少谁是上界
 如果oldblock.numberU64（）>newblock.numberU64（）
  //减少旧链
  老街区！=nil和oldblock.numberU64（）！=newblock.numberU64（）；oldblock=bc.getblock（oldblock.parenthash（），oldblock.numberU64（）-1）
   oldchain=append（oldchain，oldblock）
   deletedtxs=append（deletedtxs，oldblock.transactions（）…）

   collectlogs（oldblock.hash（））
  }
 }否则{
  //减少新的链并附加新的链块以便以后插入
  新来的！=nil&&newblock.numberU64（）！=oldblock.numberU64（）；newblock=bc.getblock（newblock.parenthash（），newblock.numberU64（）-1）
   newchain=append（newchain，newblock）
  }
 }
 如果oldblock==nil
  返回fmt.errorf（“旧链无效”）
 }
 如果newblock==nil
  返回fmt.errorf（“新链无效”）
 }

 对于{
  if oldblock.hash（）==newblock.hash（）
   CommonBlock=旧块
   打破
  }

  oldchain=append（oldchain，oldblock）
  newchain=append（newchain，newblock）
  deletedtxs=append（deletedtxs，oldblock.transactions（）…）
  collectlogs（oldblock.hash（））

  oldblock，newblock=bc.getblock（oldblock.parenthash（），oldblock.numberu64（）-1），bc.getblock（newblock.parenthash（），newblock.numberu64（）-1）
  如果oldblock==nil
   返回fmt.errorf（“旧链无效”）
  }
  如果newblock==nil
   返回fmt.errorf（“新链无效”）
  }
 }
 //确保用户看到大量的REORG
 如果len（oldchain）>0&&len（newchain）>0
  logfn：=log.debug
  如果len（oldchain）>63
   logfn=日志。警告
  }
  logfn（“检测到链拆分”，“number”，“commonblock.number（），“hash”，“commonblock.hash（），
   “drop”，len（oldchain），“dropfrom”，oldchain[0].hash（），“add”，len（newchain），“addfrom”，newchain[0].hash（））
 }否则{
  log.error（“不可能的reorg，请提交问题”，“oldnum”，“oldblock.number（），“oldhash”，“oldblock.hash（），“newnum”，“newblock.number（），“newhash”，newblock.hash（））
 }
 //插入新链，注意正确的增量顺序
 var addedtxs类型.事务
 对于i：=len（newchain）-1；i>=0；i--
  //按规范方式插入块，重新写入历史记录
  插入（新链[I]）
  //为基于哈希的事务/收据搜索写入查找条目
  rawdb.writetxlookupentries（bc.db，newchain[i]）
  addedtxs=append（addedtxs，newchain[i].transactions（）…）
 }
 //计算已删除和已添加事务之间的差异
 diff：=类型.txdifference（deletedtxs，addedtxs）
 //从数据库中删除事务时，这意味着
 //在fork中创建的收据也必须删除
 批次：=bc.db.newbatch（））
 对于uux：=范围差异
  rawdb.deletetxlookupentry（批处理，tx.hash（））
 }
 批处理（）

 如果len（deletedlogs）>0
  go bc.rmlogsfeed.send（删除日志事件删除日志）
 }

 返回零
}

//Postchainevents迭代链插入生成的事件，并
//将它们发布到事件提要中。
//TODO:不应公开后置事件。应在WriteBlock中发布链事件。
func（bc*区块链）后链事件（events[]interface，logs[]*types.log）
 //发布事件日志以进行进一步处理
 如果日志！= nIL{
  bc.logsfeed.send（日志）
 }
 对于uu，事件：=范围事件
  开关ev：=事件（类型）
  案例链接事件：
   BC.链式送料发送（EV）

  案例链头事件：
   bc.链头馈送.发送（ev）

  }
 }
}

func（bc*区块链）更新（ft-time.duration）
 //默认值为5s
 英尺＝英尺
 未来时间：=time.newticker（ft*time.second）
 推迟FutureTimer.Stop（）。
 对于{
  选择{
  案例<-FutureTimer.c：
   bc.procFutureBlocks（）。
  案例<B.CUT:
   返回
  }
 }
}

//bad blocks返回客户端在网络上看到的最后一个“坏块”的列表
func（bc*区块链）badblocks（）[]*types.block_
 块：=make（[]*types.block，0，bc.badblocks.len（））
 对于u，哈希：=range bc.badblocks.keys（）
  如果BLK，则存在：=bc.badblocks.peek（hash）；exist
   块：=blk。（*types.block）
   块=附加（块，块）
  }
 }
 返回块
}

//addbadblock将坏块添加到坏块lru缓存中
func（bc*区块链）addbadblock（block*types.block）
 bc.badblocks.add（block.hash（），块）
}

//ReportBlock记录了一个错误的块错误。
func（bc*区块链）reportblock（block*types.block，receipts types.receipts，err error）
 添加块（块）

 var receiptString字符串
 对于uu，收据：=范围收据
  receiptString+=fmt.sprintf（“\t%v\n”，收据）
 }
 日志错误（fmt.sprintf（`）
______________
链配置：%v

数量：%V
哈希：0x%x
%V

错误：%V

`，bc.chainconfig，block.number（），block.hash（），receiptString，err））。
}

//InsertHeaderChain尝试将给定的头链插入本地
//链，可能正在创建REORG。如果返回错误，它将返回
//失败头的索引号以及描述错误的错误。
/ /
//verify参数可用于微调非ce验证
//是否应该完成。可选检查背后的原因是
//其中的头检索机制已经需要验证nonce，以及
//因为nonce可以被稀疏地验证，不需要检查每个nonce。
func（bc*区块链）insertheaderchain（chain[]*types.header，checkfreq int）（int，error）
 开始：=时间。现在（））
 如果i，err：=bc.hc.validateheaderchain（chain，checkfreq）；err！= nIL{
  返回我
 }

 //确保一次只有一个线程操作链
 bc.chainmu.lock（）。
 延迟bc.chainmu.unlock（）

 添加（1）
 推迟bc.wg.done（）

 whfunc：=func（header*types.header）错误
  B.MULK（）
  延迟bc.mu.unlock（）

  _uuErr：=bc.hc.writeHeader（头）
  返回错误
 }

 返回bc.hc.insertheaderchain（chain，whfunc，start）
}

//WriteHeader将一个头写入本地链，因为它的父级是
//已经知道了。如果新插入的标题的总难度变为
//大于当前已知td，则重新路由规范链。
/ /
//注意：此方法与同时插入块不同时安全
//到链中，因为无法模拟由重组引起的副作用
//没有真正的块。因此，只应直接编写头文件
// in two scenarios: pure-header mode of operation (light clients), or properly
//单独的头/块阶段（非存档客户端）。
func（bc*区块链）writeheader（header*types.header）错误
 添加（1）
 推迟bc.wg.done（）

 B.MULK（）
 延迟bc.mu.unlock（）

 _uuErr：=bc.hc.writeHeader（头）
 返回错误
}

//currentHeader检索规范链的当前头。这个
//从HeaderChain的内部缓存中检索头。
func（bc*区块链）currentHeader（）*types.header_
 返回bc.hc.currentHeader（）。
}

//gettd从
//按哈希和数字排列的数据库，如果找到，则将其缓存。
func（bc*区块链）gettd（hash common.hash，number uint64）*big.int
 返回bc.hc.gettd（哈希，数字）
}

//getDByHash从
//数据库按哈希，如果找到则将其缓存。
func（bc*区块链）gettdbyhash（hash common.hash）*big.int_
 返回bc.hc.getDByHash（哈希）
}

//getheader按哈希和数字从数据库中检索块头，
//如果找到缓存。
func（bc*区块链）getheader（hash common.hash，number uint64）*types.header_
 返回bc.hc.getheader（哈希，数字）
}

//getHeaderByHash通过哈希从数据库中检索块头，如果
发现。
func（bc*区块链）getheaderbyhash（hash common.hash）*types.header_
 返回bc.hc.getheaderbyhash（哈希）
}

//哈希表检查数据库中是否存在块头，缓存
//如果有。
func（bc*区块链）hasheader（hash common.hash，number uint64）bool_
 返回bc.hc.hasheader（哈希，数字）
}

//GetBlockHashesFromHash从给定的
//hash，向Genesis块提取。
func（bc*区块链）getblockhashesfromhash（hash common.hash，max uint64）[]common.hash_
 返回bc.hc.getBlockHashesFromHash（哈希，最大值）
}

//getAncestor检索给定块的第n个祖先。它假定给定的块或
//它的近亲是规范的。maxnoncanonical指向向下计数器，限制
//到达规范链之前要单独检查的块数。
/ /
//注意：ancestor==0返回同一块，1返回其父块，依此类推。
func（bc*区块链）getAncestor（hash common.hash，number，祖先uint64，maxnoncanonical*uint64）（common.hash，uint64）
 bc.chainmu.lock（）。
 延迟bc.chainmu.unlock（）

 返回bc.hc.getAncestor（哈希、数字、祖先、maxnoncanonical）
}

//GetHeaderByNumber按数字从数据库中检索块头，
//如果找到，则缓存它（与其哈希关联）。
func（bc*区块链）getheaderbynumber（数字uint64）*types.header_
 返回bc.hc.getHeaderByNumber（数字）
}

//config检索区块链的链配置。
func（bc*区块链）config（）*params.chainconfig返回bc.chainconfig

//引擎检索区块链的共识引擎。
func（bc*区块链）引擎（）共识。引擎返回bc.engine

/导出数据库
func（bc*区块链）chaindb（）ethdb.database返回bc.db

//subscripreMovedLogSevent注册removedLogSevent的订阅。
func（bc*区块链）subscripreMovedLogSevent（ch chan<-removedLogSevent）事件。订阅
 返回bc.scope.track（bc.rmlogsfeed.subscribe（ch））
}

//subscribeBechainEvent注册一个chainEvent的订阅。
func（bc*区块链）subscripbechainevent（ch chan<-chainevent）event.subscription_
 返回bc.scope.track（bc.chainfeed.subscribe（ch））
}

//subscribeChainHeadEvent注册chainHeadEvent的订阅。
func（bc*区块链）subscripbechainheadevent（ch chan<-chainheadeevent）事件。订阅
 返回bc.scope.track（bc.chainheadfeed.subscribe（ch））
}

//subscriptLogSevent注册了一个订阅[]*types.log。
func（bc*区块链）subscriptLogSevent（ch chan<-[]*types.log）event.subscription_
 返回bc.scope.track（bc.logsfeed.subscribe（ch））
}

