
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

package downloader

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
)

var (
	testKey, _  = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	testAddress = crypto.PubkeyToAddress(testKey.PublicKey)
)

//减少一些参数以使测试仪更快。
func init() {
	MaxForkAncestry = uint64(10000)
	blockCacheItems = 1024
	fsHeaderContCheck = 500 * time.Millisecond
}

//下载测试仪是模拟本地区块链的测试模拟器。
type downloadTester struct {
	downloader *Downloader

genesis *types.Block   //测试人员和同行使用的Genesis块
stateDb ethdb.Database //测试人员用于从对等机同步的数据库
peerDb  ethdb.Database //包含所有数据的对等数据库

ownHashes   []common.Hash                  //属于测试人员的哈希链
ownHeaders  map[common.Hash]*types.Header  //属于检测仪的收割台
ownBlocks   map[common.Hash]*types.Block   //属于测试仪的块
ownReceipts map[common.Hash]types.Receipts //属于测试人员的收据
ownChainTd  map[common.Hash]*big.Int       //本地链中块的总困难

peerHashes   map[string][]common.Hash                  //属于不同测试对等方的哈希链
peerHeaders  map[string]map[common.Hash]*types.Header  //属于不同测试对等的头
peerBlocks   map[string]map[common.Hash]*types.Block   //属于不同测试对等体的块
peerReceipts map[string]map[common.Hash]types.Receipts //Receipts belonging to different test peers
peerChainTds map[string]map[common.Hash]*big.Int       //对等链中块的总困难

peerMissingStates map[string]map[common.Hash]bool //快速同步不应返回的状态项

	lock sync.RWMutex
}

//NewTester创建了一个新的下载程序测试mocker。
func newTester() *downloadTester {
	testdb := ethdb.NewMemDatabase()
	genesis := core.GenesisBlockForTesting(testdb, testAddress, big.NewInt(1000000000))

	tester := &downloadTester{
		genesis:           genesis,
		peerDb:            testdb,
		ownHashes:         []common.Hash{genesis.Hash()},
		ownHeaders:        map[common.Hash]*types.Header{genesis.Hash(): genesis.Header()},
		ownBlocks:         map[common.Hash]*types.Block{genesis.Hash(): genesis},
		ownReceipts:       map[common.Hash]types.Receipts{genesis.Hash(): nil},
		ownChainTd:        map[common.Hash]*big.Int{genesis.Hash(): genesis.Difficulty()},
		peerHashes:        make(map[string][]common.Hash),
		peerHeaders:       make(map[string]map[common.Hash]*types.Header),
		peerBlocks:        make(map[string]map[common.Hash]*types.Block),
		peerReceipts:      make(map[string]map[common.Hash]types.Receipts),
		peerChainTds:      make(map[string]map[common.Hash]*big.Int),
		peerMissingStates: make(map[string]map[common.Hash]bool),
	}
	tester.stateDb = ethdb.NewMemDatabase()
	tester.stateDb.Put(genesis.Root().Bytes(), []byte{0x00})

	tester.downloader = New(FullSync, tester.stateDb, new(event.TypeMux), tester, nil, tester.dropPeer)

	return tester
}

//makechain创建一个由n个块组成的链，从父块开始并包含父块。
//返回的哈希链是有序的head->parent。此外，每三个街区
//包含一个事务，每隔5分钟一个叔叔以允许测试正确的块
//重新组装。
func (dl *downloadTester) makeChain(n int, seed byte, parent *types.Block, parentReceipts types.Receipts, heavy bool) ([]common.Hash, map[common.Hash]*types.Header, map[common.Hash]*types.Block, map[common.Hash]types.Receipts) {
//生成区块链
	blocks, receipts := core.GenerateChain(params.TestChainConfig, parent, ethash.NewFaker(), dl.peerDb, n, func(i int, block *core.BlockGen) {
		block.SetCoinbase(common.Address{seed})

//如果要求重链，则延迟阻止以增加难度。
		if heavy {
			block.OffsetTime(-1)
		}
//If the block number is multiple of 3, send a bonus transaction to the miner
		if parent == dl.genesis && i%3 == 0 {
			signer := types.MakeSigner(params.TestChainConfig, block.Number())
			tx, err := types.SignTx(types.NewTransaction(block.TxNonce(testAddress), common.Address{seed}, big.NewInt(1000), params.TxGas, nil, nil), signer, testKey)
			if err != nil {
				panic(err)
			}
			block.AddTx(tx)
		}
//如果区块编号是5的倍数，请在区块中添加一个奖金叔叔。
		if i > 0 && i%5 == 0 {
			block.AddUncle(&types.Header{
				ParentHash: block.PrevBlock(i - 1).Hash(),
				Number:     big.NewInt(block.Number().Int64() - 1),
			})
		}
	})
//将块链转换为哈希链和头/块映射
	hashes := make([]common.Hash, n+1)
	hashes[len(hashes)-1] = parent.Hash()

	headerm := make(map[common.Hash]*types.Header, n+1)
	headerm[parent.Hash()] = parent.Header()

	blockm := make(map[common.Hash]*types.Block, n+1)
	blockm[parent.Hash()] = parent

	receiptm := make(map[common.Hash]types.Receipts, n+1)
	receiptm[parent.Hash()] = parentReceipts

	for i, b := range blocks {
		hashes[len(hashes)-i-2] = b.Hash()
		headerm[b.Hash()] = b.Header()
		blockm[b.Hash()] = b
		receiptm[b.Hash()] = receipts[i]
	}
	return hashes, headerm, blockm, receiptm
}

//makechainfork创建两个长度为n的链，这样h1[：f]和
//h2[：f]不同，但有一个长度为n-f的公共后缀。
func (dl *downloadTester) makeChainFork(n, f int, parent *types.Block, parentReceipts types.Receipts, balanced bool) ([]common.Hash, []common.Hash, map[common.Hash]*types.Header, map[common.Hash]*types.Header, map[common.Hash]*types.Block, map[common.Hash]*types.Block, map[common.Hash]types.Receipts, map[common.Hash]types.Receipts) {
//创建公共后缀
	hashes, headers, blocks, receipts := dl.makeChain(n-f, 0, parent, parentReceipts, false)

//创建叉，如果请求非平衡叉，则使第二个更重
	hashes1, headers1, blocks1, receipts1 := dl.makeChain(f, 1, blocks[hashes[0]], receipts[hashes[0]], false)
	hashes1 = append(hashes1, hashes[1:]...)

	heavy := false
	if !balanced {
		heavy = true
	}
	hashes2, headers2, blocks2, receipts2 := dl.makeChain(f, 2, blocks[hashes[0]], receipts[hashes[0]], heavy)
	hashes2 = append(hashes2, hashes[1:]...)

	for hash, header := range headers {
		headers1[hash] = header
		headers2[hash] = header
	}
	for hash, block := range blocks {
		blocks1[hash] = block
		blocks2[hash] = block
	}
	for hash, receipt := range receipts {
		receipts1[hash] = receipt
		receipts2[hash] = receipt
	}
	return hashes1, hashes2, headers1, headers2, blocks1, blocks2, receipts1, receipts2
}

//终止中止嵌入式下载程序上的任何操作并释放所有
//持有资源。
func (dl *downloadTester) terminate() {
	dl.downloader.Terminate()
}

//同步开始与远程对等机同步，直到同步完成。
func (dl *downloadTester) sync(id string, td *big.Int, mode SyncMode) error {
	dl.lock.RLock()
	hash := dl.peerHashes[id][0]
//如果没有请求特定的TD，则从对等区块链加载
	if td == nil {
		td = big.NewInt(1)
		if diff, ok := dl.peerChainTds[id][hash]; ok {
			td = diff
		}
	}
	dl.lock.RUnlock()

//与选定的对等机同步，并确保随后正确清理
	err := dl.downloader.synchronise(id, hash, td, mode)
	select {
	case <-dl.downloader.cancelCh:
//好的，下载程序在同步循环后完全取消
	default:
//下载程序仍在接受数据包，可能会阻止对等机
panic("downloader active post sync cycle") //测试人员会发现恐慌
	}
	return err
}

//hasheader检查测试仪规范链中是否存在头。
func (dl *downloadTester) HasHeader(hash common.Hash, number uint64) bool {
	return dl.GetHeaderByHash(hash) != nil
}

//hasblock检查测试仪规范链中是否存在块。
func (dl *downloadTester) HasBlock(hash common.Hash, number uint64) bool {
	return dl.GetBlockByHash(hash) != nil
}

//getheader从测试人员规范链中检索一个头。
func (dl *downloadTester) GetHeaderByHash(hash common.Hash) *types.Header {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	return dl.ownHeaders[hash]
}

//GetBlock从测试程序规范链中检索一个块。
func (dl *downloadTester) GetBlockByHash(hash common.Hash) *types.Block {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	return dl.ownBlocks[hash]
}

//当前头从规范链中检索当前头。
func (dl *downloadTester) CurrentHeader() *types.Header {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	for i := len(dl.ownHashes) - 1; i >= 0; i-- {
		if header := dl.ownHeaders[dl.ownHashes[i]]; header != nil {
			return header
		}
	}
	return dl.genesis.Header()
}

//currentBlock从规范链中检索当前头块。
func (dl *downloadTester) CurrentBlock() *types.Block {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	for i := len(dl.ownHashes) - 1; i >= 0; i-- {
		if block := dl.ownBlocks[dl.ownHashes[i]]; block != nil {
			if _, err := dl.stateDb.Get(block.Root().Bytes()); err == nil {
				return block
			}
		}
	}
	return dl.genesis
}

//CurrentFastBlock从规范链中检索当前磁头快速同步块。
func (dl *downloadTester) CurrentFastBlock() *types.Block {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	for i := len(dl.ownHashes) - 1; i >= 0; i-- {
		if block := dl.ownBlocks[dl.ownHashes[i]]; block != nil {
			return block
		}
	}
	return dl.genesis
}

//fastsynccommithead手动将头块设置为给定哈希。
func (dl *downloadTester) FastSyncCommitHead(hash common.Hash) error {
//现在只检查状态trie是否正确。
	if block := dl.GetBlockByHash(hash); block != nil {
		_, err := trie.NewSecure(block.Root(), trie.NewDatabase(dl.stateDb), 0)
		return err
	}
	return fmt.Errorf("non existent block: %x", hash[:4])
}

//gettd从规范链中检索块的总难度。
func (dl *downloadTester) GetTd(hash common.Hash, number uint64) *big.Int {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	return dl.ownChainTd[hash]
}

//InsertHeaderChain将新的一批头注入到模拟链中。
func (dl *downloadTester) InsertHeaderChain(headers []*types.Header, checkFreq int) (int, error) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

//快速检查，因为区块链。插入头链不会插入任何内容以防出错。
	if _, ok := dl.ownHeaders[headers[0].ParentHash]; !ok {
		return 0, errors.New("unknown parent")
	}
	for i := 1; i < len(headers); i++ {
		if headers[i].ParentHash != headers[i-1].Hash() {
			return i, errors.New("unknown parent")
		}
	}
//如果预检查通过，则执行完全插入
	for i, header := range headers {
		if _, ok := dl.ownHeaders[header.Hash()]; ok {
			continue
		}
		if _, ok := dl.ownHeaders[header.ParentHash]; !ok {
			return i, errors.New("unknown parent")
		}
		dl.ownHashes = append(dl.ownHashes, header.Hash())
		dl.ownHeaders[header.Hash()] = header
		dl.ownChainTd[header.Hash()] = new(big.Int).Add(dl.ownChainTd[header.ParentHash], header.Difficulty)
	}
	return len(headers), nil
}

//insertchain向模拟链中注入一批新的块。
func (dl *downloadTester) InsertChain(blocks types.Blocks) (int, error) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	for i, block := range blocks {
		if parent, ok := dl.ownBlocks[block.ParentHash()]; !ok {
			return i, errors.New("unknown parent")
		} else if _, err := dl.stateDb.Get(parent.Root().Bytes()); err != nil {
			return i, fmt.Errorf("unknown parent state %x: %v", parent.Root(), err)
		}
		if _, ok := dl.ownHeaders[block.Hash()]; !ok {
			dl.ownHashes = append(dl.ownHashes, block.Hash())
			dl.ownHeaders[block.Hash()] = block.Header()
		}
		dl.ownBlocks[block.Hash()] = block
		dl.stateDb.Put(block.Root().Bytes(), []byte{0x00})
		dl.ownChainTd[block.Hash()] = new(big.Int).Add(dl.ownChainTd[block.ParentHash()], block.Difficulty())
	}
	return len(blocks), nil
}

//InsertReceiptChain将新的一批收据注入到模拟链中。
func (dl *downloadTester) InsertReceiptChain(blocks types.Blocks, receipts []types.Receipts) (int, error) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	for i := 0; i < len(blocks) && i < len(receipts); i++ {
		if _, ok := dl.ownHeaders[blocks[i].Hash()]; !ok {
			return i, errors.New("unknown owner")
		}
		if _, ok := dl.ownBlocks[blocks[i].ParentHash()]; !ok {
			return i, errors.New("unknown parent")
		}
		dl.ownBlocks[blocks[i].Hash()] = blocks[i]
		dl.ownReceipts[blocks[i].Hash()] = receipts[i]
	}
	return len(blocks), nil
}

//回滚从链中删除一些最近添加的元素。
func (dl *downloadTester) Rollback(hashes []common.Hash) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	for i := len(hashes) - 1; i >= 0; i-- {
		if dl.ownHashes[len(dl.ownHashes)-1] == hashes[i] {
			dl.ownHashes = dl.ownHashes[:len(dl.ownHashes)-1]
		}
		delete(dl.ownChainTd, hashes[i])
		delete(dl.ownHeaders, hashes[i])
		delete(dl.ownReceipts, hashes[i])
		delete(dl.ownBlocks, hashes[i])
	}
}

//newpeer向下载程序注册一个新的块下载源。
func (dl *downloadTester) newPeer(id string, version int, hashes []common.Hash, headers map[common.Hash]*types.Header, blocks map[common.Hash]*types.Block, receipts map[common.Hash]types.Receipts) error {
	return dl.newSlowPeer(id, version, hashes, headers, blocks, receipts, 0)
}

//NexStand将一个新的块下载源注册到下载器中
//处理发送给它的网络包时的特定延迟时间，模拟
//网络IO可能较慢。
func (dl *downloadTester) newSlowPeer(id string, version int, hashes []common.Hash, headers map[common.Hash]*types.Header, blocks map[common.Hash]*types.Block, receipts map[common.Hash]types.Receipts, delay time.Duration) error {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	var err = dl.downloader.RegisterPeer(id, version, &downloadTesterPeer{dl: dl, id: id, delay: delay})
	if err == nil {
//将拥有的哈希、头和块分配给对等端（深度复制）
		dl.peerHashes[id] = make([]common.Hash, len(hashes))
		copy(dl.peerHashes[id], hashes)

		dl.peerHeaders[id] = make(map[common.Hash]*types.Header)
		dl.peerBlocks[id] = make(map[common.Hash]*types.Block)
		dl.peerReceipts[id] = make(map[common.Hash]types.Receipts)
		dl.peerChainTds[id] = make(map[common.Hash]*big.Int)
		dl.peerMissingStates[id] = make(map[common.Hash]bool)

		genesis := hashes[len(hashes)-1]
		if header := headers[genesis]; header != nil {
			dl.peerHeaders[id][genesis] = header
			dl.peerChainTds[id][genesis] = header.Difficulty
		}
		if block := blocks[genesis]; block != nil {
			dl.peerBlocks[id][genesis] = block
			dl.peerChainTds[id][genesis] = block.Difficulty()
		}

		for i := len(hashes) - 2; i >= 0; i-- {
			hash := hashes[i]

			if header, ok := headers[hash]; ok {
				dl.peerHeaders[id][hash] = header
				if _, ok := dl.peerHeaders[id][header.ParentHash]; ok {
					dl.peerChainTds[id][hash] = new(big.Int).Add(header.Difficulty, dl.peerChainTds[id][header.ParentHash])
				}
			}
			if block, ok := blocks[hash]; ok {
				dl.peerBlocks[id][hash] = block
				if _, ok := dl.peerBlocks[id][block.ParentHash()]; ok {
					dl.peerChainTds[id][hash] = new(big.Int).Add(block.Difficulty(), dl.peerChainTds[id][block.ParentHash()])
				}
			}
			if receipt, ok := receipts[hash]; ok {
				dl.peerReceipts[id][hash] = receipt
			}
		}
	}
	return err
}

//DropPeer模拟从连接池中删除硬对等。
func (dl *downloadTester) dropPeer(id string) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	delete(dl.peerHashes, id)
	delete(dl.peerHeaders, id)
	delete(dl.peerBlocks, id)
	delete(dl.peerChainTds, id)

	dl.downloader.UnregisterPeer(id)
}

type downloadTesterPeer struct {
	dl    *downloadTester
	id    string
	delay time.Duration
	lock  sync.RWMutex
}

//setdelay是网络延迟值的线程安全设置器。
func (dlp *downloadTesterPeer) setDelay(delay time.Duration) {
	dlp.lock.Lock()
	defer dlp.lock.Unlock()

	dlp.delay = delay
}

//waitdelay是一种线程安全的方式，可以在配置的时间内睡眠。
func (dlp *downloadTesterPeer) waitDelay() {
	dlp.lock.RLock()
	delay := dlp.delay
	dlp.lock.RUnlock()

	time.Sleep(delay)
}

//head构造一个函数来检索对等端的当前head哈希
//以及全部的困难。
func (dlp *downloadTesterPeer) Head() (common.Hash, *big.Int) {
	dlp.dl.lock.RLock()
	defer dlp.dl.lock.RUnlock()

	return dlp.dl.peerHashes[dlp.id][0], nil
}

//RequestHeadersByHash基于哈希构造GetBlockHeaders函数
//源站；与下载测试仪中的特定对等点关联。归还的人
//函数可用于从特定的对等端检索成批的头。
func (dlp *downloadTesterPeer) RequestHeadersByHash(origin common.Hash, amount int, skip int, reverse bool) error {
//查找哈希的规范编号
	dlp.dl.lock.RLock()
	number := uint64(0)
	for num, hash := range dlp.dl.peerHashes[dlp.id] {
		if hash == origin {
			number = uint64(len(dlp.dl.peerHashes[dlp.id]) - num - 1)
			break
		}
	}
	dlp.dl.lock.RUnlock()

//使用绝对头获取器来满足查询
	return dlp.RequestHeadersByNumber(number, amount, skip, reverse)
}

//RequestHeadersByNumber基于编号的
//源站；与下载测试仪中的特定对等点关联。归还的人
//函数可用于从特定的对等端检索成批的头。
func (dlp *downloadTesterPeer) RequestHeadersByNumber(origin uint64, amount int, skip int, reverse bool) error {
	dlp.waitDelay()

	dlp.dl.lock.RLock()
	defer dlp.dl.lock.RUnlock()

//收集下一批报头
	hashes := dlp.dl.peerHashes[dlp.id]
	headers := dlp.dl.peerHeaders[dlp.id]
	result := make([]*types.Header, 0, amount)
	for i := 0; i < amount && len(hashes)-int(origin)-1-i*(skip+1) >= 0; i++ {
		if header, ok := headers[hashes[len(hashes)-int(origin)-1-i*(skip+1)]]; ok {
			result = append(result, header)
		}
	}
//延迟交付一点以允许攻击展开
	go func() {
		time.Sleep(time.Millisecond)
		dlp.dl.downloader.DeliverHeaders(dlp.id, result)
	}()
	return nil
}

//RequestBodies构造与特定
//在下载测试仪中查找。返回的函数可用于检索
//来自特定请求对等端的一批块体。
func (dlp *downloadTesterPeer) RequestBodies(hashes []common.Hash) error {
	dlp.waitDelay()

	dlp.dl.lock.RLock()
	defer dlp.dl.lock.RUnlock()

	blocks := dlp.dl.peerBlocks[dlp.id]

	transactions := make([][]*types.Transaction, 0, len(hashes))
	uncles := make([][]*types.Header, 0, len(hashes))

	for _, hash := range hashes {
		if block, ok := blocks[hash]; ok {
			transactions = append(transactions, block.Transactions())
			uncles = append(uncles, block.Uncles())
		}
	}
	go dlp.dl.downloader.DeliverBodies(dlp.id, transactions, uncles)

	return nil
}

//RequestReceipts构造与特定
//在下载测试仪中查找。返回的函数可用于检索
//来自特定请求对等方的批量块接收。
func (dlp *downloadTesterPeer) RequestReceipts(hashes []common.Hash) error {
	dlp.waitDelay()

	dlp.dl.lock.RLock()
	defer dlp.dl.lock.RUnlock()

	receipts := dlp.dl.peerReceipts[dlp.id]

	results := make([][]*types.Receipt, 0, len(hashes))
	for _, hash := range hashes {
		if receipt, ok := receipts[hash]; ok {
			results = append(results, receipt)
		}
	}
	go dlp.dl.downloader.DeliverReceipts(dlp.id, results)

	return nil
}

//RequestNodeData构造与特定
//在下载测试仪中查找。返回的函数可用于检索
//来自特定请求对等端的节点状态数据批。
func (dlp *downloadTesterPeer) RequestNodeData(hashes []common.Hash) error {
	dlp.waitDelay()

	dlp.dl.lock.RLock()
	defer dlp.dl.lock.RUnlock()

	results := make([][]byte, 0, len(hashes))
	for _, hash := range hashes {
		if data, err := dlp.dl.peerDb.Get(hash.Bytes()); err == nil {
			if !dlp.dl.peerMissingStates[dlp.id][hash] {
				results = append(results, data)
			}
		}
	}
	go dlp.dl.downloader.DeliverNodeData(dlp.id, results)

	return nil
}

//AssertownChain检查本地链是否包含正确数量的项
//各种链条组件。
func assertOwnChain(t *testing.T, tester *downloadTester, length int) {
	assertOwnForkedChain(t, tester, 1, []int{length})
}

//AssertOwnMarkedChain检查本地分叉链是否包含正确的
//各种链组件的项数。
func assertOwnForkedChain(t *testing.T, tester *downloadTester, common int, lengths []int) {
//初始化第一个分叉的计数器
	headers, blocks, receipts := lengths[0], lengths[0], lengths[0]-fsMinFullBlocks

	if receipts < 0 {
		receipts = 1
	}
//更新每个后续分叉的计数器
	for _, length := range lengths[1:] {
		headers += length - common
		blocks += length - common
		receipts += length - common - fsMinFullBlocks
	}
	switch tester.downloader.mode {
	case FullSync:
		receipts = 1
	case LightSync:
		blocks, receipts = 1, 1
	}
	if hs := len(tester.ownHeaders); hs != headers {
		t.Fatalf("synchronised headers mismatch: have %v, want %v", hs, headers)
	}
	if bs := len(tester.ownBlocks); bs != blocks {
		t.Fatalf("synchronised blocks mismatch: have %v, want %v", bs, blocks)
	}
	if rs := len(tester.ownReceipts); rs != receipts {
		t.Fatalf("synchronised receipts mismatch: have %v, want %v", rs, receipts)
	}
//Verify the state trie too for fast syncs
 /*f tester.downloader.mode==fastsync_
  透视：=uint64（0）
  var索引int
  如果pivot：=int（tester.downloader.queue.fastsyncpivot）；pivot<common_
   索引=枢轴
  }否则{
   index=len（tester.ownhahes）-长度[len（lengs）-1]+int（tester.downloader.queue.fastsyncpivot）
  }
  如果索引＞0 {
   如果statedb，则错误：=state.new（tester.ownheaders[tester.ownhahes[index]].root，state.newdatabase（trie.newdatabase（tester.statedb）））；statedb==nil err！= nIL{
    T.FATALF（“状态重建失败：%V”，Err）
   }
  }
 */

}

//测试针对规范链的简单同步是否正常工作。
//在此测试中，公共祖先查找应该是短路的，不需要
//二进制搜索。
func TestCanonicalSynchronisation62(t *testing.T)      { testCanonicalSynchronisation(t, 62, FullSync) }
func TestCanonicalSynchronisation63Full(t *testing.T)  { testCanonicalSynchronisation(t, 63, FullSync) }
func TestCanonicalSynchronisation63Fast(t *testing.T)  { testCanonicalSynchronisation(t, 63, FastSync) }
func TestCanonicalSynchronisation64Full(t *testing.T)  { testCanonicalSynchronisation(t, 64, FullSync) }
func TestCanonicalSynchronisation64Fast(t *testing.T)  { testCanonicalSynchronisation(t, 64, FastSync) }
func TestCanonicalSynchronisation64Light(t *testing.T) { testCanonicalSynchronisation(t, 64, LightSync) }

func testCanonicalSynchronisation(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建一个足够小的区块链来下载
	targetBlocks := blockCacheItems - 15
	hashes, headers, blocks, receipts := tester.makeChain(targetBlocks, 0, tester.genesis, nil, false)

	tester.newPeer("peer", protocol, hashes, headers, blocks, receipts)

//与对等机同步并确保检索到所有相关数据
	if err := tester.sync("peer", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, targetBlocks+1)
}

//测试如果下载了大量块，则会阻止
//直到检索到缓存块。
func TestThrottling62(t *testing.T)     { testThrottling(t, 62, FullSync) }
func TestThrottling63Full(t *testing.T) { testThrottling(t, 63, FullSync) }
func TestThrottling63Fast(t *testing.T) { testThrottling(t, 63, FastSync) }
func TestThrottling64Full(t *testing.T) { testThrottling(t, 64, FullSync) }
func TestThrottling64Fast(t *testing.T) { testThrottling(t, 64, FastSync) }

func testThrottling(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()
	tester := newTester()
	defer tester.terminate()

//创建要下载的长区块链和测试仪
	targetBlocks := 8 * blockCacheItems
	hashes, headers, blocks, receipts := tester.makeChain(targetBlocks, 0, tester.genesis, nil, false)

	tester.newPeer("peer", protocol, hashes, headers, blocks, receipts)

//包裹导入器以允许步进
	blocked, proceed := uint32(0), make(chan struct{})
	tester.downloader.chainInsertHook = func(results []*fetchResult) {
		atomic.StoreUint32(&blocked, uint32(len(results)))
		<-proceed
	}
//同时启动同步
	errc := make(chan error)
	go func() {
		errc <- tester.sync("peer", nil, mode)
	}()
//迭代获取一些块，始终检查检索计数
	for {
//同步检查检索计数（！这个丑陋街区的原因）
		tester.lock.RLock()
		retrieved := len(tester.ownBlocks)
		tester.lock.RUnlock()
		if retrieved >= targetBlocks+1 {
			break
		}
//等一下，让同步自行调节。
		var cached, frozen int
		for start := time.Now(); time.Since(start) < 3*time.Second; {
			time.Sleep(25 * time.Millisecond)

			tester.lock.Lock()
			tester.downloader.queue.lock.Lock()
			cached = len(tester.downloader.queue.blockDonePool)
			if mode == FastSync {
				if receipts := len(tester.downloader.queue.receiptDonePool); receipts < cached {
//if tester.downloader.queue.resultcache[receipts].header.number.uint64（）<tester.downloader.queue.fastsyncpivot_
					cached = receipts
//}
				}
			}
			frozen = int(atomic.LoadUint32(&blocked))
			retrieved = len(tester.ownBlocks)
			tester.downloader.queue.lock.Unlock()
			tester.lock.Unlock()

			if cached == blockCacheItems || retrieved+cached+frozen == targetBlocks+1 {
				break
			}
		}
//确保我们填满了缓存，然后耗尽它
time.Sleep(25 * time.Millisecond) //给它一个搞砸的机会

		tester.lock.RLock()
		retrieved = len(tester.ownBlocks)
		tester.lock.RUnlock()
		if cached != blockCacheItems && retrieved+cached+frozen != targetBlocks+1 {
			t.Fatalf("block count mismatch: have %v, want %v (owned %v, blocked %v, target %v)", cached, blockCacheItems, retrieved, frozen, targetBlocks+1)
		}
//允许导入被阻止的块
		if atomic.LoadUint32(&blocked) > 0 {
			atomic.StoreUint32(&blocked, uint32(0))
			proceed <- struct{}{}
		}
	}
//检查我们没有拖过多的街区
	assertOwnChain(t, tester, targetBlocks+1)
	if err := <-errc; err != nil {
		t.Fatalf("block synchronization failed: %v", err)
	}
}

//测试针对分叉链的简单同步是否正确工作。在
//此测试公共祖先查找应*不*短路，并且
//应执行二进制搜索。
func TestForkedSync62(t *testing.T)      { testForkedSync(t, 62, FullSync) }
func TestForkedSync63Full(t *testing.T)  { testForkedSync(t, 63, FullSync) }
func TestForkedSync63Fast(t *testing.T)  { testForkedSync(t, 63, FastSync) }
func TestForkedSync64Full(t *testing.T)  { testForkedSync(t, 64, FullSync) }
func TestForkedSync64Fast(t *testing.T)  { testForkedSync(t, 64, FastSync) }
func TestForkedSync64Light(t *testing.T) { testForkedSync(t, 64, LightSync) }

func testForkedSync(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建足够长的分叉链
	common, fork := MaxHashFetch, 2*MaxHashFetch
	hashesA, hashesB, headersA, headersB, blocksA, blocksB, receiptsA, receiptsB := tester.makeChainFork(common+fork, fork, tester.genesis, nil, true)

	tester.newPeer("fork A", protocol, hashesA, headersA, blocksA, receiptsA)
	tester.newPeer("fork B", protocol, hashesB, headersB, blocksB, receiptsB)

//与对等机同步并确保检索到所有块
	if err := tester.sync("fork A", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, common+fork+1)

//与第二个对等机同步，并确保拨叉也被拉动
	if err := tester.sync("fork B", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnForkedChain(t, tester, common+1, []int{common + fork + 1, common + fork + 1})
}

//与较短但较重的叉子同步的测试
//正确无误，不会掉落。
func TestHeavyForkedSync62(t *testing.T)      { testHeavyForkedSync(t, 62, FullSync) }
func TestHeavyForkedSync63Full(t *testing.T)  { testHeavyForkedSync(t, 63, FullSync) }
func TestHeavyForkedSync63Fast(t *testing.T)  { testHeavyForkedSync(t, 63, FastSync) }
func TestHeavyForkedSync64Full(t *testing.T)  { testHeavyForkedSync(t, 64, FullSync) }
func TestHeavyForkedSync64Fast(t *testing.T)  { testHeavyForkedSync(t, 64, FastSync) }
func TestHeavyForkedSync64Light(t *testing.T) { testHeavyForkedSync(t, 64, LightSync) }

func testHeavyForkedSync(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建足够长的分叉链
	common, fork := MaxHashFetch, 4*MaxHashFetch
	hashesA, hashesB, headersA, headersB, blocksA, blocksB, receiptsA, receiptsB := tester.makeChainFork(common+fork, fork, tester.genesis, nil, false)

	tester.newPeer("light", protocol, hashesA, headersA, blocksA, receiptsA)
	tester.newPeer("heavy", protocol, hashesB[fork/2:], headersB, blocksB, receiptsB)

//与对等机同步并确保检索到所有块
	if err := tester.sync("light", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, common+fork+1)

//与第二个对等机同步，并确保拨叉也被拉动
	if err := tester.sync("heavy", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnForkedChain(t, tester, common+1, []int{common + fork + 1, common + fork/2 + 1})
}

//测试链叉是否包含在电流的某个间隔内
//链头，确保恶意同行不会通过喂养浪费资源
//死链长。
func TestBoundedForkedSync62(t *testing.T)      { testBoundedForkedSync(t, 62, FullSync) }
func TestBoundedForkedSync63Full(t *testing.T)  { testBoundedForkedSync(t, 63, FullSync) }
func TestBoundedForkedSync63Fast(t *testing.T)  { testBoundedForkedSync(t, 63, FastSync) }
func TestBoundedForkedSync64Full(t *testing.T)  { testBoundedForkedSync(t, 64, FullSync) }
func TestBoundedForkedSync64Fast(t *testing.T)  { testBoundedForkedSync(t, 64, FastSync) }
func TestBoundedForkedSync64Light(t *testing.T) { testBoundedForkedSync(t, 64, LightSync) }

func testBoundedForkedSync(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建足够长的分叉链
	common, fork := 13, int(MaxForkAncestry+17)
	hashesA, hashesB, headersA, headersB, blocksA, blocksB, receiptsA, receiptsB := tester.makeChainFork(common+fork, fork, tester.genesis, nil, true)

	tester.newPeer("original", protocol, hashesA, headersA, blocksA, receiptsA)
	tester.newPeer("rewriter", protocol, hashesB, headersB, blocksB, receiptsB)

//与对等机同步并确保检索到所有块
	if err := tester.sync("original", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, common+fork+1)

//与第二个对等机同步，并确保叉被拒绝太旧
	if err := tester.sync("rewriter", nil, mode); err != errInvalidAncestor {
		t.Fatalf("sync failure mismatch: have %v, want %v", err, errInvalidAncestor)
	}
}

//测试链叉是否包含在电流的某个间隔内
//链头也适用于短而重的叉子。这些有点特别，因为它们
//采用不同的祖先查找路径。
func TestBoundedHeavyForkedSync62(t *testing.T)      { testBoundedHeavyForkedSync(t, 62, FullSync) }
func TestBoundedHeavyForkedSync63Full(t *testing.T)  { testBoundedHeavyForkedSync(t, 63, FullSync) }
func TestBoundedHeavyForkedSync63Fast(t *testing.T)  { testBoundedHeavyForkedSync(t, 63, FastSync) }
func TestBoundedHeavyForkedSync64Full(t *testing.T)  { testBoundedHeavyForkedSync(t, 64, FullSync) }
func TestBoundedHeavyForkedSync64Fast(t *testing.T)  { testBoundedHeavyForkedSync(t, 64, FastSync) }
func TestBoundedHeavyForkedSync64Light(t *testing.T) { testBoundedHeavyForkedSync(t, 64, LightSync) }

func testBoundedHeavyForkedSync(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建足够长的分叉链
	common, fork := 13, int(MaxForkAncestry+17)
	hashesA, hashesB, headersA, headersB, blocksA, blocksB, receiptsA, receiptsB := tester.makeChainFork(common+fork, fork, tester.genesis, nil, false)

	tester.newPeer("original", protocol, hashesA, headersA, blocksA, receiptsA)
tester.newPeer("heavy-rewriter", protocol, hashesB[MaxForkAncestry-17:], headersB, blocksB, receiptsB) //把叉子根在祖先限制以下

//与对等机同步并确保检索到所有块
	if err := tester.sync("original", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, common+fork+1)

//与第二个对等机同步，并确保叉被拒绝太旧
	if err := tester.sync("heavy-rewriter", nil, mode); err != errInvalidAncestor {
		t.Fatalf("sync failure mismatch: have %v, want %v", err, errInvalidAncestor)
	}
}

//测试非活动下载程序是否不接受传入的块头，以及
//身体。
func TestInactiveDownloader62(t *testing.T) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//检查是否不接受块头和块体
	if err := tester.downloader.DeliverHeaders("bad peer", []*types.Header{}); err != errNoSyncActive {
		t.Errorf("error mismatch: have %v, want %v", err, errNoSyncActive)
	}
	if err := tester.downloader.DeliverBodies("bad peer", [][]*types.Transaction{}, [][]*types.Header{}); err != errNoSyncActive {
		t.Errorf("error mismatch: have %v, want %v", err, errNoSyncActive)
	}
}

//测试非活动下载程序是否不接受传入的块头，
//机构和收据。
func TestInactiveDownloader63(t *testing.T) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//检查是否不接受块头和块体
	if err := tester.downloader.DeliverHeaders("bad peer", []*types.Header{}); err != errNoSyncActive {
		t.Errorf("error mismatch: have %v, want %v", err, errNoSyncActive)
	}
	if err := tester.downloader.DeliverBodies("bad peer", [][]*types.Transaction{}, [][]*types.Header{}); err != errNoSyncActive {
		t.Errorf("error mismatch: have %v, want %v", err, errNoSyncActive)
	}
	if err := tester.downloader.DeliverReceipts("bad peer", [][]*types.Receipt{}); err != errNoSyncActive {
		t.Errorf("error mismatch: have %v, want %v", err, errNoSyncActive)
	}
}

//测试取消的下载是否会清除所有以前累积的状态。
func TestCancel62(t *testing.T)      { testCancel(t, 62, FullSync) }
func TestCancel63Full(t *testing.T)  { testCancel(t, 63, FullSync) }
func TestCancel63Fast(t *testing.T)  { testCancel(t, 63, FastSync) }
func TestCancel64Full(t *testing.T)  { testCancel(t, 64, FullSync) }
func TestCancel64Fast(t *testing.T)  { testCancel(t, 64, FastSync) }
func TestCancel64Light(t *testing.T) { testCancel(t, 64, LightSync) }

func testCancel(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建一个足够小的区块链来下载和测试
	targetBlocks := blockCacheItems - 15
	if targetBlocks >= MaxHashFetch {
		targetBlocks = MaxHashFetch - 15
	}
	if targetBlocks >= MaxHeaderFetch {
		targetBlocks = MaxHeaderFetch - 15
	}
	hashes, headers, blocks, receipts := tester.makeChain(targetBlocks, 0, tester.genesis, nil, false)

	tester.newPeer("peer", protocol, hashes, headers, blocks, receipts)

//确保取消与原始下载程序一起工作
	tester.downloader.Cancel()
	if !tester.downloader.queue.Idle() {
		t.Errorf("download queue not idle")
	}
//与同级同步，但随后取消
	if err := tester.sync("peer", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	tester.downloader.Cancel()
	if !tester.downloader.queue.Idle() {
		t.Errorf("download queue not idle")
	}
}

//多个对等机同步按预期工作的测试（多线程健全性测试）。
func TestMultiSynchronisation62(t *testing.T)      { testMultiSynchronisation(t, 62, FullSync) }
func TestMultiSynchronisation63Full(t *testing.T)  { testMultiSynchronisation(t, 63, FullSync) }
func TestMultiSynchronisation63Fast(t *testing.T)  { testMultiSynchronisation(t, 63, FastSync) }
func TestMultiSynchronisation64Full(t *testing.T)  { testMultiSynchronisation(t, 64, FullSync) }
func TestMultiSynchronisation64Fast(t *testing.T)  { testMultiSynchronisation(t, 64, FastSync) }
func TestMultiSynchronisation64Light(t *testing.T) { testMultiSynchronisation(t, 64, LightSync) }

func testMultiSynchronisation(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//用链的不同部分创建不同的对等点
	targetPeers := 8
	targetBlocks := targetPeers*blockCacheItems - 15
	hashes, headers, blocks, receipts := tester.makeChain(targetBlocks, 0, tester.genesis, nil, false)

	for i := 0; i < targetPeers; i++ {
		id := fmt.Sprintf("peer #%d", i)
		tester.newPeer(id, protocol, hashes[i*blockCacheItems:], headers, blocks, receipts)
	}
	if err := tester.sync("peer #0", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, targetBlocks+1)
}

//在多版本协议环境中同步运行良好的测试
//不会对网络中的其他节点造成破坏。
func TestMultiProtoSynchronisation62(t *testing.T)      { testMultiProtoSync(t, 62, FullSync) }
func TestMultiProtoSynchronisation63Full(t *testing.T)  { testMultiProtoSync(t, 63, FullSync) }
func TestMultiProtoSynchronisation63Fast(t *testing.T)  { testMultiProtoSync(t, 63, FastSync) }
func TestMultiProtoSynchronisation64Full(t *testing.T)  { testMultiProtoSync(t, 64, FullSync) }
func TestMultiProtoSynchronisation64Fast(t *testing.T)  { testMultiProtoSync(t, 64, FastSync) }
func TestMultiProtoSynchronisation64Light(t *testing.T) { testMultiProtoSync(t, 64, LightSync) }

func testMultiProtoSync(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建一个足够小的区块链来下载
	targetBlocks := blockCacheItems - 15
	hashes, headers, blocks, receipts := tester.makeChain(targetBlocks, 0, tester.genesis, nil, false)

//创建每种类型的对等点
	tester.newPeer("peer 62", 62, hashes, headers, blocks, nil)
	tester.newPeer("peer 63", 63, hashes, headers, blocks, receipts)
	tester.newPeer("peer 64", 64, hashes, headers, blocks, receipts)

//与请求的对等机同步，并确保检索到所有块
	if err := tester.sync(fmt.Sprintf("peer %d", protocol), nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, targetBlocks+1)

//检查是否没有同伴掉线
	for _, version := range []int{62, 63, 64} {
		peer := fmt.Sprintf("peer %d", version)
		if _, ok := tester.peerHashes[peer]; !ok {
			t.Errorf("%s dropped", peer)
		}
	}
}

//测试如果一个块为空（例如，仅限头），则不应执行任何Body请求
//制作，而不是集管本身组装成一个整体。
func TestEmptyShortCircuit62(t *testing.T)      { testEmptyShortCircuit(t, 62, FullSync) }
func TestEmptyShortCircuit63Full(t *testing.T)  { testEmptyShortCircuit(t, 63, FullSync) }
func TestEmptyShortCircuit63Fast(t *testing.T)  { testEmptyShortCircuit(t, 63, FastSync) }
func TestEmptyShortCircuit64Full(t *testing.T)  { testEmptyShortCircuit(t, 64, FullSync) }
func TestEmptyShortCircuit64Fast(t *testing.T)  { testEmptyShortCircuit(t, 64, FastSync) }
func TestEmptyShortCircuit64Light(t *testing.T) { testEmptyShortCircuit(t, 64, LightSync) }

func testEmptyShortCircuit(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建要下载的区块链
	targetBlocks := 2*blockCacheItems - 15
	hashes, headers, blocks, receipts := tester.makeChain(targetBlocks, 0, tester.genesis, nil, false)

	tester.newPeer("peer", protocol, hashes, headers, blocks, receipts)

//向下载器发送信号，以发出Body请求
	bodiesHave, receiptsHave := int32(0), int32(0)
	tester.downloader.bodyFetchHook = func(headers []*types.Header) {
		atomic.AddInt32(&bodiesHave, int32(len(headers)))
	}
	tester.downloader.receiptFetchHook = func(headers []*types.Header) {
		atomic.AddInt32(&receiptsHave, int32(len(headers)))
	}
//与对等机同步并确保检索到所有块
	if err := tester.sync("peer", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, targetBlocks+1)

//验证应请求的块体数
	bodiesNeeded, receiptsNeeded := 0, 0
	for _, block := range blocks {
		if mode != LightSync && block != tester.genesis && (len(block.Transactions()) > 0 || len(block.Uncles()) > 0) {
			bodiesNeeded++
		}
	}
	for _, receipt := range receipts {
		if mode == FastSync && len(receipt) > 0 {
			receiptsNeeded++
		}
	}
	if int(bodiesHave) != bodiesNeeded {
		t.Errorf("body retrieval count mismatch: have %v, want %v", bodiesHave, bodiesNeeded)
	}
	if int(receiptsHave) != receiptsNeeded {
		t.Errorf("receipt retrieval count mismatch: have %v, want %v", receiptsHave, receiptsNeeded)
	}
}

//测试头是否连续排队，以防止恶意节点
//通过喂入有间隙的收割台链条来安装下载器。
func TestMissingHeaderAttack62(t *testing.T)      { testMissingHeaderAttack(t, 62, FullSync) }
func TestMissingHeaderAttack63Full(t *testing.T)  { testMissingHeaderAttack(t, 63, FullSync) }
func TestMissingHeaderAttack63Fast(t *testing.T)  { testMissingHeaderAttack(t, 63, FastSync) }
func TestMissingHeaderAttack64Full(t *testing.T)  { testMissingHeaderAttack(t, 64, FullSync) }
func TestMissingHeaderAttack64Fast(t *testing.T)  { testMissingHeaderAttack(t, 64, FastSync) }
func TestMissingHeaderAttack64Light(t *testing.T) { testMissingHeaderAttack(t, 64, LightSync) }

func testMissingHeaderAttack(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建一个足够小的区块链来下载
	targetBlocks := blockCacheItems - 15
	hashes, headers, blocks, receipts := tester.makeChain(targetBlocks, 0, tester.genesis, nil, false)

//尝试与攻击者进行完全同步，以提供有间隙的头文件
	tester.newPeer("attack", protocol, hashes, headers, blocks, receipts)
	missing := targetBlocks / 2
	delete(tester.peerHeaders["attack"], hashes[missing])

	if err := tester.sync("attack", nil, mode); err == nil {
		t.Fatalf("succeeded attacker synchronisation")
	}
//与有效的对等机同步并确保同步成功
	tester.newPeer("valid", protocol, hashes, headers, blocks, receipts)
	if err := tester.sync("valid", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, targetBlocks+1)
}

//测试如果请求的头被移动（即第一个头丢失），队列
//检测无效编号。
func TestShiftedHeaderAttack62(t *testing.T)      { testShiftedHeaderAttack(t, 62, FullSync) }
func TestShiftedHeaderAttack63Full(t *testing.T)  { testShiftedHeaderAttack(t, 63, FullSync) }
func TestShiftedHeaderAttack63Fast(t *testing.T)  { testShiftedHeaderAttack(t, 63, FastSync) }
func TestShiftedHeaderAttack64Full(t *testing.T)  { testShiftedHeaderAttack(t, 64, FullSync) }
func TestShiftedHeaderAttack64Fast(t *testing.T)  { testShiftedHeaderAttack(t, 64, FastSync) }
func TestShiftedHeaderAttack64Light(t *testing.T) { testShiftedHeaderAttack(t, 64, LightSync) }

func testShiftedHeaderAttack(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建一个足够小的区块链来下载
	targetBlocks := blockCacheItems - 15
	hashes, headers, blocks, receipts := tester.makeChain(targetBlocks, 0, tester.genesis, nil, false)

//尝试与攻击者进行完全同步以输入移位的头文件
	tester.newPeer("attack", protocol, hashes, headers, blocks, receipts)
	delete(tester.peerHeaders["attack"], hashes[len(hashes)-2])
	delete(tester.peerBlocks["attack"], hashes[len(hashes)-2])
	delete(tester.peerReceipts["attack"], hashes[len(hashes)-2])

	if err := tester.sync("attack", nil, mode); err == nil {
		t.Fatalf("succeeded attacker synchronisation")
	}
//与有效的对等机同步并确保同步成功
	tester.newPeer("valid", protocol, hashes, headers, blocks, receipts)
	if err := tester.sync("valid", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, targetBlocks+1)
}

//测试在检测到无效的头时，将回滚最近的头
//对于各种故障情况。之后，尝试进行完全同步
//确保没有状态损坏。
func TestInvalidHeaderRollback63Fast(t *testing.T)  { testInvalidHeaderRollback(t, 63, FastSync) }
func TestInvalidHeaderRollback64Fast(t *testing.T)  { testInvalidHeaderRollback(t, 64, FastSync) }
func TestInvalidHeaderRollback64Light(t *testing.T) { testInvalidHeaderRollback(t, 64, LightSync) }

func testInvalidHeaderRollback(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建一个足够小的区块链来下载
	targetBlocks := 3*fsHeaderSafetyNet + 256 + fsMinFullBlocks
	hashes, headers, blocks, receipts := tester.makeChain(targetBlocks, 0, tester.genesis, nil, false)

//尝试与在快速同步阶段提供垃圾的攻击者同步。
//这将导致最后一个fsheadersafetynet头回滚。
	tester.newPeer("fast-attack", protocol, hashes, headers, blocks, receipts)
	missing := fsHeaderSafetyNet + MaxHeaderFetch + 1
	delete(tester.peerHeaders["fast-attack"], hashes[len(hashes)-missing])

	if err := tester.sync("fast-attack", nil, mode); err == nil {
		t.Fatalf("succeeded fast attacker synchronisation")
	}
	if head := tester.CurrentHeader().Number.Int64(); int(head) > MaxHeaderFetch {
		t.Errorf("rollback head mismatch: have %v, want at most %v", head, MaxHeaderFetch)
	}
//尝试与在块导入阶段提供垃圾的攻击者同步。
//这将导致最后一个fsheadersafetynet头的数量
//回滚，同时将轴点还原为非块状态。
	tester.newPeer("block-attack", protocol, hashes, headers, blocks, receipts)
	missing = 3*fsHeaderSafetyNet + MaxHeaderFetch + 1
delete(tester.peerHeaders["fast-attack"], hashes[len(hashes)-missing]) //确保快速攻击者没有填写
	delete(tester.peerHeaders["block-attack"], hashes[len(hashes)-missing])

	if err := tester.sync("block-attack", nil, mode); err == nil {
		t.Fatalf("succeeded block attacker synchronisation")
	}
	if head := tester.CurrentHeader().Number.Int64(); int(head) > 2*fsHeaderSafetyNet+MaxHeaderFetch {
		t.Errorf("rollback head mismatch: have %v, want at most %v", head, 2*fsHeaderSafetyNet+MaxHeaderFetch)
	}
	if mode == FastSync {
		if head := tester.CurrentBlock().NumberU64(); head != 0 {
			t.Errorf("fast sync pivot block #%d not rolled back", head)
		}
	}
//尝试与在
//快速同步轴点。这可能是让节点
//但已经导入了透视图块。
	tester.newPeer("withhold-attack", protocol, hashes, headers, blocks, receipts)
	missing = 3*fsHeaderSafetyNet + MaxHeaderFetch + 1

	tester.downloader.syncInitHook = func(uint64, uint64) {
		for i := missing; i <= len(hashes); i++ {
			delete(tester.peerHeaders["withhold-attack"], hashes[len(hashes)-i])
		}
		tester.downloader.syncInitHook = nil
	}

	if err := tester.sync("withhold-attack", nil, mode); err == nil {
		t.Fatalf("succeeded withholding attacker synchronisation")
	}
	if head := tester.CurrentHeader().Number.Int64(); int(head) > 2*fsHeaderSafetyNet+MaxHeaderFetch {
		t.Errorf("rollback head mismatch: have %v, want at most %v", head, 2*fsHeaderSafetyNet+MaxHeaderFetch)
	}
	if mode == FastSync {
		if head := tester.CurrentBlock().NumberU64(); head != 0 {
			t.Errorf("fast sync pivot block #%d not rolled back", head)
		}
	}
//与有效的对等机同步，并确保同步成功。自上次以来
//回滚还应禁用此进程的快速同步，请验证
//进行了全新的完全同步。注意，我们不能断言任何有关收据的内容
//因为我们不会清除它们的数据库，所以我们不能使用assertownchain。
	tester.newPeer("valid", protocol, hashes, headers, blocks, receipts)
	if err := tester.sync("valid", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	if hs := len(tester.ownHeaders); hs != len(headers) {
		t.Fatalf("synchronised headers mismatch: have %v, want %v", hs, len(headers))
	}
	if mode != LightSync {
		if bs := len(tester.ownBlocks); bs != len(blocks) {
			t.Fatalf("synchronised blocks mismatch: have %v, want %v", bs, len(blocks))
		}
	}
}

//测试一个发布高TD的同龄人是否能够阻止下载者
//之后不发送任何有用的哈希。
func TestHighTDStarvationAttack62(t *testing.T)      { testHighTDStarvationAttack(t, 62, FullSync) }
func TestHighTDStarvationAttack63Full(t *testing.T)  { testHighTDStarvationAttack(t, 63, FullSync) }
func TestHighTDStarvationAttack63Fast(t *testing.T)  { testHighTDStarvationAttack(t, 63, FastSync) }
func TestHighTDStarvationAttack64Full(t *testing.T)  { testHighTDStarvationAttack(t, 64, FullSync) }
func TestHighTDStarvationAttack64Fast(t *testing.T)  { testHighTDStarvationAttack(t, 64, FastSync) }
func TestHighTDStarvationAttack64Light(t *testing.T) { testHighTDStarvationAttack(t, 64, LightSync) }

func testHighTDStarvationAttack(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

	hashes, headers, blocks, receipts := tester.makeChain(0, 0, tester.genesis, nil, false)
	tester.newPeer("attack", protocol, []common.Hash{hashes[0]}, headers, blocks, receipts)

	if err := tester.sync("attack", big.NewInt(1000000), mode); err != errStallingPeer {
		t.Fatalf("synchronisation error mismatch: have %v, want %v", err, errStallingPeer)
	}
}

//行为不端的测试是断开连接的，而行为不正确的测试则是断开连接的。
func TestBlockHeaderAttackerDropping62(t *testing.T) { testBlockHeaderAttackerDropping(t, 62) }
func TestBlockHeaderAttackerDropping63(t *testing.T) { testBlockHeaderAttackerDropping(t, 63) }
func TestBlockHeaderAttackerDropping64(t *testing.T) { testBlockHeaderAttackerDropping(t, 64) }

func testBlockHeaderAttackerDropping(t *testing.T, protocol int) {
	t.Parallel()

//定义单个哈希获取错误的断开连接要求
	tests := []struct {
		result error
		drop   bool
	}{
{nil, false},                        //同步成功，一切正常
{errBusy, false},                    //同步已在进行中，没问题
{errUnknownPeer, false},             //对等机未知，已被删除，不要重复删除
{errBadPeer, true},                  //因为某种原因，同伴被认为是坏的，放下它。
{errStallingPeer, true},             //检测到对等机正在停止，请删除它
{errNoPeers, false},                 //无需下载同龄人，软件竞赛，无问题
{errTimeout, true},                  //在适当的时间内没有收到哈希，请删除对等机
{errEmptyHeaderSet, true},           //没有返回任何头作为响应，因为它是死胡同而丢弃
{errPeersUnavailable, true},         //没有人有广告块，把广告丢了
{errInvalidAncestor, true},          //同意祖先是不可接受的，请删除链重写器
{errInvalidChain, true},             //哈希链被检测为无效，肯定会丢失
{errInvalidBlock, false},            //检测到一个错误的对等机，但不是同步源
{errInvalidBody, false},             //检测到一个错误的对等机，但不是同步源
{errInvalidReceipt, false},          //检测到一个错误的对等机，但不是同步源
{errCancelBlockFetch, false},        //同步被取消，原点可能是无辜的，不要掉下来
{errCancelHeaderFetch, false},       //同步被取消，原点可能是无辜的，不要掉下来
{errCancelBodyFetch, false},         //同步被取消，原点可能是无辜的，不要掉下来
{errCancelReceiptFetch, false},      //同步被取消，原点可能是无辜的，不要掉下来
{errCancelHeaderProcessing, false},  //同步被取消，原点可能是无辜的，不要掉下来
{errCancelContentProcessing, false}, //同步被取消，原点可能是无辜的，不要掉下来
	}
//运行测试并检查断开状态
	tester := newTester()
	defer tester.terminate()

	for i, tt := range tests {
//注册新的对等点并确保其存在
		id := fmt.Sprintf("test %d", i)
		if err := tester.newPeer(id, protocol, []common.Hash{tester.genesis.Hash()}, nil, nil, nil); err != nil {
			t.Fatalf("test %d: failed to register new peer: %v", i, err)
		}
		if _, ok := tester.peerHashes[id]; !ok {
			t.Fatalf("test %d: registered peer not found", i)
		}
//模拟同步并检查所需结果
		tester.downloader.synchroniseMock = func(string, common.Hash) error { return tt.result }

		tester.downloader.Synchronise(id, tester.genesis.Hash(), big.NewInt(1000), FullSync)
		if _, ok := tester.peerHashes[id]; !ok != tt.drop {
			t.Errorf("test %d: peer drop mismatch for %v: have %v, want %v", i, tt.result, !ok, tt.drop)
		}
	}
}

//Tests that synchronisation progress (origin block number, current block number
//以及最高的块号）被正确地跟踪和更新。
func TestSyncProgress62(t *testing.T)      { testSyncProgress(t, 62, FullSync) }
func TestSyncProgress63Full(t *testing.T)  { testSyncProgress(t, 63, FullSync) }
func TestSyncProgress63Fast(t *testing.T)  { testSyncProgress(t, 63, FastSync) }
func TestSyncProgress64Full(t *testing.T)  { testSyncProgress(t, 64, FullSync) }
func TestSyncProgress64Fast(t *testing.T)  { testSyncProgress(t, 64, FastSync) }
func TestSyncProgress64Light(t *testing.T) { testSyncProgress(t, 64, LightSync) }

func testSyncProgress(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建一个足够小的区块链来下载
	targetBlocks := blockCacheItems - 15
	hashes, headers, blocks, receipts := tester.makeChain(targetBlocks, 0, tester.genesis, nil, false)

//设置同步初始化挂钩以捕获进度更改
	starting := make(chan struct{})
	progress := make(chan struct{})

	tester.downloader.syncInitHook = func(origin, latest uint64) {
		starting <- struct{}{}
		<-progress
	}
//检索同步进度并确保它们为零（原始同步）
	if progress := tester.downloader.Progress(); progress.StartingBlock != 0 || progress.CurrentBlock != 0 || progress.HighestBlock != 0 {
		t.Fatalf("Pristine progress mismatch: have %v/%v/%v, want %v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, 0, 0, 0)
	}
//同步一半块并检查初始进度
	tester.newPeer("peer-half", protocol, hashes[targetBlocks/2:], headers, blocks, receipts)
	pending := new(sync.WaitGroup)
	pending.Add(1)

	go func() {
		defer pending.Done()
		if err := tester.sync("peer-half", nil, mode); err != nil {
			panic(fmt.Sprintf("failed to synchronise blocks: %v", err))
		}
	}()
	<-starting
	if progress := tester.downloader.Progress(); progress.StartingBlock != 0 || progress.CurrentBlock != 0 || progress.HighestBlock != uint64(targetBlocks/2+1) {
		t.Fatalf("Initial progress mismatch: have %v/%v/%v, want %v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, 0, 0, targetBlocks/2+1)
	}
	progress <- struct{}{}
	pending.Wait()

//同步所有块并检查继续进度
	tester.newPeer("peer-full", protocol, hashes, headers, blocks, receipts)
	pending.Add(1)

	go func() {
		defer pending.Done()
		if err := tester.sync("peer-full", nil, mode); err != nil {
			panic(fmt.Sprintf("failed to synchronise blocks: %v", err))
		}
	}()
	<-starting
	if progress := tester.downloader.Progress(); progress.StartingBlock != uint64(targetBlocks/2+1) || progress.CurrentBlock != uint64(targetBlocks/2+1) || progress.HighestBlock != uint64(targetBlocks) {
		t.Fatalf("Completing progress mismatch: have %v/%v/%v, want %v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, targetBlocks/2+1, targetBlocks/2+1, targetBlocks)
	}
	progress <- struct{}{}
	pending.Wait()

//同步成功后检查最终进度
	if progress := tester.downloader.Progress(); progress.StartingBlock != uint64(targetBlocks/2+1) || progress.CurrentBlock != uint64(targetBlocks) || progress.HighestBlock != uint64(targetBlocks) {
		t.Fatalf("Final progress mismatch: have %v/%v/%v, want %v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, targetBlocks/2+1, targetBlocks, targetBlocks)
	}
}

//测试同步进程（起始块编号和最高块
//如果是叉子（或手动头），则正确跟踪和更新数字。
//回复）
func TestForkedSyncProgress62(t *testing.T)      { testForkedSyncProgress(t, 62, FullSync) }
func TestForkedSyncProgress63Full(t *testing.T)  { testForkedSyncProgress(t, 63, FullSync) }
func TestForkedSyncProgress63Fast(t *testing.T)  { testForkedSyncProgress(t, 63, FastSync) }
func TestForkedSyncProgress64Full(t *testing.T)  { testForkedSyncProgress(t, 64, FullSync) }
func TestForkedSyncProgress64Fast(t *testing.T)  { testForkedSyncProgress(t, 64, FastSync) }
func TestForkedSyncProgress64Light(t *testing.T) { testForkedSyncProgress(t, 64, LightSync) }

func testForkedSyncProgress(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建分叉链以模拟原点还原
	common, fork := MaxHashFetch, 2*MaxHashFetch
	hashesA, hashesB, headersA, headersB, blocksA, blocksB, receiptsA, receiptsB := tester.makeChainFork(common+fork, fork, tester.genesis, nil, true)

//设置同步初始化挂钩以捕获进度更改
	starting := make(chan struct{})
	progress := make(chan struct{})

	tester.downloader.syncInitHook = func(origin, latest uint64) {
		starting <- struct{}{}
		<-progress
	}
//检索同步进度并确保它们为零（原始同步）
	if progress := tester.downloader.Progress(); progress.StartingBlock != 0 || progress.CurrentBlock != 0 || progress.HighestBlock != 0 {
		t.Fatalf("Pristine progress mismatch: have %v/%v/%v, want %v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, 0, 0, 0)
	}
//与其中一个拨叉同步并检查进度
	tester.newPeer("fork A", protocol, hashesA, headersA, blocksA, receiptsA)
	pending := new(sync.WaitGroup)
	pending.Add(1)

	go func() {
		defer pending.Done()
		if err := tester.sync("fork A", nil, mode); err != nil {
			panic(fmt.Sprintf("failed to synchronise blocks: %v", err))
		}
	}()
	<-starting
	if progress := tester.downloader.Progress(); progress.StartingBlock != 0 || progress.CurrentBlock != 0 || progress.HighestBlock != uint64(len(hashesA)-1) {
		t.Fatalf("Initial progress mismatch: have %v/%v/%v, want %v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, 0, 0, len(hashesA)-1)
	}
	progress <- struct{}{}
	pending.Wait()

//在分叉上方模拟成功的同步
	tester.downloader.syncStatsChainOrigin = tester.downloader.syncStatsChainHeight

//与第二个拨叉同步并检查进度重置
	tester.newPeer("fork B", protocol, hashesB, headersB, blocksB, receiptsB)
	pending.Add(1)

	go func() {
		defer pending.Done()
		if err := tester.sync("fork B", nil, mode); err != nil {
			panic(fmt.Sprintf("failed to synchronise blocks: %v", err))
		}
	}()
	<-starting
	if progress := tester.downloader.Progress(); progress.StartingBlock != uint64(common) || progress.CurrentBlock != uint64(len(hashesA)-1) || progress.HighestBlock != uint64(len(hashesB)-1) {
		t.Fatalf("Forking progress mismatch: have %v/%v/%v, want %v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, common, len(hashesA)-1, len(hashesB)-1)
	}
	progress <- struct{}{}
	pending.Wait()

//同步成功后检查最终进度
	if progress := tester.downloader.Progress(); progress.StartingBlock != uint64(common) || progress.CurrentBlock != uint64(len(hashesB)-1) || progress.HighestBlock != uint64(len(hashesB)-1) {
		t.Fatalf("Final progress mismatch: have %v/%v/%v, want %v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, common, len(hashesB)-1, len(hashesB)-1)
	}
}

//测试如果同步由于某些故障而中止，则进度
//原点不会在下一个同步周期中更新，因为它应该被视为
//继续上一次同步，而不是新实例。
func TestFailedSyncProgress62(t *testing.T)      { testFailedSyncProgress(t, 62, FullSync) }
func TestFailedSyncProgress63Full(t *testing.T)  { testFailedSyncProgress(t, 63, FullSync) }
func TestFailedSyncProgress63Fast(t *testing.T)  { testFailedSyncProgress(t, 63, FastSync) }
func TestFailedSyncProgress64Full(t *testing.T)  { testFailedSyncProgress(t, 64, FullSync) }
func TestFailedSyncProgress64Fast(t *testing.T)  { testFailedSyncProgress(t, 64, FastSync) }
func TestFailedSyncProgress64Light(t *testing.T) { testFailedSyncProgress(t, 64, LightSync) }

func testFailedSyncProgress(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建一个足够小的区块链来下载
	targetBlocks := blockCacheItems - 15
	hashes, headers, blocks, receipts := tester.makeChain(targetBlocks, 0, tester.genesis, nil, false)

//设置同步初始化挂钩以捕获进度更改
	starting := make(chan struct{})
	progress := make(chan struct{})

	tester.downloader.syncInitHook = func(origin, latest uint64) {
		starting <- struct{}{}
		<-progress
	}
//检索同步进度并确保它们为零（原始同步）
	if progress := tester.downloader.Progress(); progress.StartingBlock != 0 || progress.CurrentBlock != 0 || progress.HighestBlock != 0 {
		t.Fatalf("Pristine progress mismatch: have %v/%v/%v, want %v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, 0, 0, 0)
	}
//尝试与有故障的对等机完全同步
	tester.newPeer("faulty", protocol, hashes, headers, blocks, receipts)
	missing := targetBlocks / 2
	delete(tester.peerHeaders["faulty"], hashes[missing])
	delete(tester.peerBlocks["faulty"], hashes[missing])
	delete(tester.peerReceipts["faulty"], hashes[missing])

	pending := new(sync.WaitGroup)
	pending.Add(1)

	go func() {
		defer pending.Done()
		if err := tester.sync("faulty", nil, mode); err == nil {
			panic("succeeded faulty synchronisation")
		}
	}()
	<-starting
	if progress := tester.downloader.Progress(); progress.StartingBlock != 0 || progress.CurrentBlock != 0 || progress.HighestBlock != uint64(targetBlocks) {
		t.Fatalf("Initial progress mismatch: have %v/%v/%v, want %v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, 0, 0, targetBlocks)
	}
	progress <- struct{}{}
	pending.Wait()

//与一位优秀的同行同步，并检查故障后进度来源是否提醒相同
	tester.newPeer("valid", protocol, hashes, headers, blocks, receipts)
	pending.Add(1)

	go func() {
		defer pending.Done()
		if err := tester.sync("valid", nil, mode); err != nil {
			panic(fmt.Sprintf("failed to synchronise blocks: %v", err))
		}
	}()
	<-starting
	if progress := tester.downloader.Progress(); progress.StartingBlock != 0 || progress.CurrentBlock > uint64(targetBlocks/2) || progress.HighestBlock != uint64(targetBlocks) {
		t.Fatalf("Completing progress mismatch: have %v/%v/%v, want %v/0-%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, 0, targetBlocks/2, targetBlocks)
	}
	progress <- struct{}{}
	pending.Wait()

//同步成功后检查最终进度
	if progress := tester.downloader.Progress(); progress.StartingBlock > uint64(targetBlocks/2) || progress.CurrentBlock != uint64(targetBlocks) || progress.HighestBlock != uint64(targetBlocks) {
		t.Fatalf("Final progress mismatch: have %v/%v/%v, want 0-%v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, targetBlocks/2, targetBlocks, targetBlocks)
	}
}

//测试在检测到攻击后，如果攻击者伪造链高度，
//在下次同步调用时，进度高度已成功降低。
func TestFakedSyncProgress62(t *testing.T)      { testFakedSyncProgress(t, 62, FullSync) }
func TestFakedSyncProgress63Full(t *testing.T)  { testFakedSyncProgress(t, 63, FullSync) }
func TestFakedSyncProgress63Fast(t *testing.T)  { testFakedSyncProgress(t, 63, FastSync) }
func TestFakedSyncProgress64Full(t *testing.T)  { testFakedSyncProgress(t, 64, FullSync) }
func TestFakedSyncProgress64Fast(t *testing.T)  { testFakedSyncProgress(t, 64, FastSync) }
func TestFakedSyncProgress64Light(t *testing.T) { testFakedSyncProgress(t, 64, LightSync) }

func testFakedSyncProgress(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建一个小的区块链
	targetBlocks := blockCacheItems - 15
	hashes, headers, blocks, receipts := tester.makeChain(targetBlocks+3, 0, tester.genesis, nil, false)

//设置同步初始化挂钩以捕获进度更改
	starting := make(chan struct{})
	progress := make(chan struct{})

	tester.downloader.syncInitHook = func(origin, latest uint64) {
		starting <- struct{}{}
		<-progress
	}
//检索同步进度并确保它们为零（原始同步）
	if progress := tester.downloader.Progress(); progress.StartingBlock != 0 || progress.CurrentBlock != 0 || progress.HighestBlock != 0 {
		t.Fatalf("Pristine progress mismatch: have %v/%v/%v, want %v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, 0, 0, 0)
	}
//创建并与攻击者同步，该攻击者承诺比可用的链更高
	tester.newPeer("attack", protocol, hashes, headers, blocks, receipts)
	for i := 1; i < 3; i++ {
		delete(tester.peerHeaders["attack"], hashes[i])
		delete(tester.peerBlocks["attack"], hashes[i])
		delete(tester.peerReceipts["attack"], hashes[i])
	}

	pending := new(sync.WaitGroup)
	pending.Add(1)

	go func() {
		defer pending.Done()
		if err := tester.sync("attack", nil, mode); err == nil {
			panic("succeeded attacker synchronisation")
		}
	}()
	<-starting
	if progress := tester.downloader.Progress(); progress.StartingBlock != 0 || progress.CurrentBlock != 0 || progress.HighestBlock != uint64(targetBlocks+3) {
		t.Fatalf("Initial progress mismatch: have %v/%v/%v, want %v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, 0, 0, targetBlocks+3)
	}
	progress <- struct{}{}
	pending.Wait()

//与良好的同行同步，并检查进度高度是否已降低到真实值。
	tester.newPeer("valid", protocol, hashes[3:], headers, blocks, receipts)
	pending.Add(1)

	go func() {
		defer pending.Done()
		if err := tester.sync("valid", nil, mode); err != nil {
			panic(fmt.Sprintf("failed to synchronise blocks: %v", err))
		}
	}()
	<-starting
	if progress := tester.downloader.Progress(); progress.StartingBlock != 0 || progress.CurrentBlock > uint64(targetBlocks) || progress.HighestBlock != uint64(targetBlocks) {
		t.Fatalf("Completing progress mismatch: have %v/%v/%v, want %v/0-%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, 0, targetBlocks, targetBlocks)
	}
	progress <- struct{}{}
	pending.Wait()

//同步成功后检查最终进度
	if progress := tester.downloader.Progress(); progress.StartingBlock > uint64(targetBlocks) || progress.CurrentBlock != uint64(targetBlocks) || progress.HighestBlock != uint64(targetBlocks) {
		t.Fatalf("Final progress mismatch: have %v/%v/%v, want 0-%v/%v/%v", progress.StartingBlock, progress.CurrentBlock, progress.HighestBlock, targetBlocks, targetBlocks, targetBlocks)
	}
}

//此测试复制了一个问题，即意外交付
//如果他们到达正确的时间，无限期地封锁。
//我们使用数据驱动的子测试来管理它，这样它就可以自己并行。
//而不是其他测试，避免间歇性故障。
func TestDeliverHeadersHang(t *testing.T) {
	testCases := []struct {
		protocol int
		syncMode SyncMode
	}{
		{62, FullSync},
		{63, FullSync},
		{63, FastSync},
		{64, FullSync},
		{64, FastSync},
		{64, LightSync},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("protocol %d mode %v", tc.protocol, tc.syncMode), func(t *testing.T) {
			testDeliverHeadersHang(t, tc.protocol, tc.syncMode)
		})
	}
}

type floodingTestPeer struct {
	peer   Peer
	tester *downloadTester
	pend   sync.WaitGroup
}

func (ftp *floodingTestPeer) Head() (common.Hash, *big.Int) { return ftp.peer.Head() }
func (ftp *floodingTestPeer) RequestHeadersByHash(hash common.Hash, count int, skip int, reverse bool) error {
	return ftp.peer.RequestHeadersByHash(hash, count, skip, reverse)
}
func (ftp *floodingTestPeer) RequestBodies(hashes []common.Hash) error {
	return ftp.peer.RequestBodies(hashes)
}
func (ftp *floodingTestPeer) RequestReceipts(hashes []common.Hash) error {
	return ftp.peer.RequestReceipts(hashes)
}
func (ftp *floodingTestPeer) RequestNodeData(hashes []common.Hash) error {
	return ftp.peer.RequestNodeData(hashes)
}

func (ftp *floodingTestPeer) RequestHeadersByNumber(from uint64, count, skip int, reverse bool) error {
	deliveriesDone := make(chan struct{}, 500)
	for i := 0; i < cap(deliveriesDone); i++ {
		peer := fmt.Sprintf("fake-peer%d", i)
		ftp.pend.Add(1)

		go func() {
			ftp.tester.downloader.DeliverHeaders(peer, []*types.Header{{}, {}, {}, {}})
			deliveriesDone <- struct{}{}
			ftp.pend.Done()
		}()
	}
//传递实际请求的头。
	go ftp.peer.RequestHeadersByNumber(from, count, skip, reverse)
//额外交货不应受阻。
	timeout := time.After(60 * time.Second)
	for i := 0; i < cap(deliveriesDone); i++ {
		select {
		case <-deliveriesDone:
		case <-timeout:
			panic("blocked")
		}
	}
	return nil
}

func testDeliverHeadersHang(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	master := newTester()
	defer master.terminate()

	hashes, headers, blocks, receipts := master.makeChain(5, 0, master.genesis, nil, false)
	for i := 0; i < 200; i++ {
		tester := newTester()
		tester.peerDb = master.peerDb

		tester.newPeer("peer", protocol, hashes, headers, blocks, receipts)
//每当下载程序请求报头时，就用
//很多未请求的头部交付。
		tester.downloader.peers.peers["peer"].peer = &floodingTestPeer{
			peer:   tester.downloader.peers.peers["peer"].peer,
			tester: tester,
		}
		if err := tester.sync("peer", nil, mode); err != nil {
			t.Errorf("test %d: sync failed: %v", i, err)
		}
		tester.terminate()

//冲洗所有Goroutine，以防干扰后续测试
		tester.downloader.peers.peers["peer"].peer.(*floodingTestPeer).pend.Wait()
	}
}
