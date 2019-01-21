
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package dpos

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	lru "github.com/hashicorp/golang-lru"
)


const (
extraVanity        = 32   //固定为签名者虚荣保留的额外数据前缀字节数
extraSeal          = 65   //固定为签名者密封保留的额外数据后缀字节数
inmemorySignatures = 4096 //要保存在内存中的最近块签名数

//blockinterval=int64（10）//出块间隔
//epochinterval=int64（86400）//选举周期间隔24*60*60 s
//最大验证大小=21
//
//conensusize=15//maxvalidatorsize*2/3+1
blockInterval    = int64(10)  	//附带条件
epochInterval    = int64(60)  //选举周间隔24*60*60 s
	maxValidatorSize = 3
safeSize         =  2	//maxvalidator大小*2/3+1
consensusSize    =  2	//maxvalidator大小*2/3+1
)



var (
	big0  = big.NewInt(0)
	big8  = big.NewInt(8)
	big32 = big.NewInt(32)

frontierBlockReward  *big.Int = big.NewInt(5e+18) //
byzantiumBlockReward *big.Int = big.NewInt(3e+18) //从拜占庭向上成功开采一个区块，在魏城获得区块奖励

	timeOfFirstBlock = int64(0)

	confirmedBlockHead = []byte("confirmed-block-head")
)

var (
//当请求块的签名者列表时，返回errunknownblock。
//这不是本地区块链的一部分。
	errUnknownBlock = errors.New("unknown block")
//如果块的额外数据节短于
//32字节，这是存储签名者虚荣所必需的。
	errMissingVanity = errors.New("extra-data 32 byte vanity prefix missing")
//如果块的额外数据节似乎不存在，则返回errmissingsignature
//包含65字节的secp256k1签名。
	errMissingSignature = errors.New("extra-data 65 byte suffix signature missing")
//如果块的mix digest为非零，则返回errInvalidMixDigest。
	errInvalidMixDigest = errors.New("non-zero mix digest")
//如果块包含非空的叔叔列表，则返回errInvalidUncleHash。
	errInvalidUncleHash  = errors.New("non empty uncle hash")
	errInvalidDifficulty = errors.New("invalid difficulty")

//如果块的时间戳低于，则返回errInvalidTimestamp
//上一个块的时间戳+最小块周期。
	ErrInvalidTimestamp           = errors.New("invalid timestamp")
	ErrWaitForPrevBlock           = errors.New("wait for last block arrived")
	ErrMintFutureBlock            = errors.New("mint the future block")
	ErrMismatchSignerAndValidator = errors.New("mismatch block signer and validator")
	ErrInvalidBlockValidator      = errors.New("invalid block validator")
	ErrInvalidMintBlockTime       = errors.New("invalid time to mint the block")
	ErrNilBlockHeader             = errors.New("nil block header returned")
)
var (
uncleHash = types.CalcUncleHash(nil) //作为叔叔，Keccak256（rlp（[]）在POW之外总是毫无意义的。
)

type Dpos struct {
config *params.DposConfig //共识引擎配置参数
db      ethdb.Database     //存储和检索快照检查点的数据库

	signer               common.Address
	signFn               SignerFn
signatures           *lru.ARCCache //加快开采速度的近期区块特征
	confirmedBlockHeader *types.Header

	mu   sync.RWMutex
	stop chan bool
}

type SignerFn func(accounts.Account, []byte) ([]byte, error)

//注：Sighash是从集团复制的
//sighash返回用作权限证明输入的哈希
//签署。它是除65字节签名之外的整个头的哈希
//包含在额外数据的末尾。
//
//注意，该方法要求额外数据至少为65字节，否则
//恐慌。这样做是为了避免意外使用这两个表单（存在签名
//或者不是），这可能会被滥用，从而为同一个头产生不同的散列。
func sigHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewKeccak256()

	rlp.Encode(hasher, []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Validator,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
header.Extra[:len(header.Extra)-65], //是的，如果多余的太短，这会很恐慌的
		header.MixDigest,
		header.Nonce,
		header.DposContext.Root(),
header.MaxValidatorSize,			//
	})
	hasher.Sum(hash[:0])

	return hash
}

func New(config *params.DposConfig, db ethdb.Database) *Dpos {
	signatures, _ := lru.NewARC(inmemorySignatures)

	return &Dpos{
		config:     config,
		db:         db,
		signatures: signatures,
	}
}

func (d *Dpos) Author(header *types.Header) (common.Address, error) {
	return header.Validator, nil
}

//验证批量是否符合共识算法规则
func (d *Dpos) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool, blockInterval uint64) error {
	return d.verifyHeader(chain, header, nil, blockInterval)
}

func (d *Dpos) verifyHeader(chain consensus.ChainReader, header *types.Header, parents []*types.Header,blockInterval uint64 ) error {
	if header.Number == nil {
		return errUnknownBlock
	}
	number := header.Number.Uint64()
//不需要验证功能块
	if header.Time.Cmp(big.NewInt(time.Now().Unix())) > 0 {
		return consensus.ErrFutureBlock
	}
//检查额外数据是否包含虚荣和签名
	if len(header.Extra) < extraVanity {
		return errMissingVanity
	}
	if len(header.Extra) < extraVanity+extraSeal {
		return errMissingSignature
	}
//确保混合摘要为零，因为我们当前没有分叉保护
	if header.MixDigest != (common.Hash{}) {
		return errInvalidMixDigest
	}
//困难总是1
//度设定为1
//
	if header.Difficulty.Uint64() != 1 {		
		return errInvalidDifficulty
	}

//确保块中不包含任何在DPO中无意义的叔叔。
	if header.UncleHash != uncleHash {
		return errInvalidUncleHash
	}
//如果所有检查都通过，则验证硬分叉的任何特殊字段
	if err := misc.VerifyForkHashes(chain.Config(), header, false); err != nil {
		return err
	}

	var parent *types.Header
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetHeader(header.ParentHash, number-1)
	}
	if parent == nil || parent.Number.Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}
	if parent.Time.Uint64()+blockInterval> header.Time.Uint64() {
		return ErrInvalidTimestamp
	}
	return nil
}

//批量验证区块头是否符合公共计算法规
func (d *Dpos) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool,) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))
	blockInterval := chain.GetHeaderByNumber(0).BlockInterval

	go func() {
		for i, header := range headers {
//header.extra=make（[]字节，ExtraVanity+ExtraSeal）
			err := d.verifyHeader(chain, header, headers[:i],blockInterval)
			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

//verifyuncles实现converse.engine，始终返回任何
//因为这个共识机制不允许叔叔。
func (d *Dpos) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed")
	}
	return nil
}

//验证seal是否执行consension.engine，检查签名是否包含
//头部满足共识协议要求。
func (d *Dpos) VerifySeal(chain consensus.ChainReader, currentheader, genesisheader *types.Header) error {
	return d.verifySeal(chain, currentheader, genesisheader,nil)
}

func (d *Dpos) verifySeal(chain consensus.ChainReader, currentheader, genesisheader *types.Header, parents []*types.Header) error {
//验证不支持Genesis块
	number := currentheader.Number.Uint64()
	if number == 0 {
		return errUnknownBlock
	}
	var parent *types.Header
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetHeader(currentheader.ParentHash, number-1)
	}

	trieDB := trie.NewDatabase(d.db)
dposContext, err := types.NewDposContextFromProto(trieDB, parent.DposContext) //零位

	if err != nil {
		return err
	}
	epochContext := &EpochContext{DposContext: dposContext}
	blockInterVal := genesisheader.BlockInterval
	validator, err := epochContext.lookupValidator(currentheader.Time.Int64(),blockInterVal)
	if err != nil {
		return err
	}
//
	if err := d.verifyBlockSigner(validator, currentheader); err != nil {
		return err
	}
	return d.updateConfirmedBlockHeader(chain)
}

func (d *Dpos) verifyBlockSigner(validator common.Address, header *types.Header) error {
	signer, err := ecrecover(header, d.signatures)
	if err != nil {
		return err
	}
	if bytes.Compare(signer.Bytes(), validator.Bytes()) != 0 {
		return ErrInvalidBlockValidator
	}
	if bytes.Compare(signer.Bytes(), header.Validator.Bytes()) != 0 {
		return ErrMismatchSignerAndValidator
	}
	return nil
}

func (d *Dpos) updateConfirmedBlockHeader(chain consensus.ChainReader) error {
	if d.confirmedBlockHeader == nil {
		header, err := d.loadConfirmedBlockHeader(chain)
		if err != nil {
			header = chain.GetHeaderByNumber(0)
			if header == nil {
				return err
			}
		}
		d.confirmedBlockHeader = header
	}

	curHeader := chain.CurrentHeader()

	fmt.Println("+++++++++++++++++++555555++++++++++++++++++++++\n")
	genesisHeader := chain.GetHeaderByNumber(0)
	fmt.Println("+++++++++++++++++++from genesisBlock to get Maxvalidatorsize++++++++++++++++++++++\n")
	epoch := int64(-1)
	validatorMap := make(map[common.Address]bool)
	for d.confirmedBlockHeader.Hash() != curHeader.Hash() &&
		d.confirmedBlockHeader.Number.Uint64() < curHeader.Number.Uint64() {
		curEpoch := curHeader.Time.Int64() / epochInterval
		if curEpoch != epoch {
			epoch = curEpoch
			validatorMap = make(map[common.Address]bool)
		}
//快速返回
//如果块数差小于一致同意的见证数
//无需检查是否确认阻塞
		consensusSize :=int(genesisHeader.MaxValidatorSize*2/3+1)
		if curHeader.Number.Int64()-d.confirmedBlockHeader.Number.Int64() < int64(consensusSize-len(validatorMap)) {
			log.Debug("Dpos fast return", "current", curHeader.Number.String(), "confirmed", d.confirmedBlockHeader.Number.String(), "witnessCount", len(validatorMap))
			return nil
		}
		validatorMap[curHeader.Validator] = true
		if len(validatorMap) >= consensusSize {
			d.confirmedBlockHeader = curHeader
			if err := d.storeConfirmedBlockHeader(d.db); err != nil {
				return err
			}
			log.Debug("dpos set confirmed block header success", "currentHeader", curHeader.Number.String())
			return nil
		}
		curHeader = chain.GetHeaderByHash(curHeader.ParentHash)
		if curHeader == nil {
			return ErrNilBlockHeader
		}
	}
	return nil
}

func (s *Dpos) loadConfirmedBlockHeader(chain consensus.ChainReader) (*types.Header, error) {
	key, err := s.db.Get(confirmedBlockHead)
	if err != nil {
		return nil, err
	}
	header := chain.GetHeaderByHash(common.BytesToHash(key))
	if header == nil {
		return nil, ErrNilBlockHeader
	}
	return header, nil
}

//存储将快照插入数据库。
func (s *Dpos) storeConfirmedBlockHeader(db ethdb.Database) error {
	return db.Put(confirmedBlockHead, s.confirmedBlockHeader.Hash().Bytes())
}

func (d *Dpos) Prepare(chain consensus.ChainReader, header *types.Header) error {
	header.Nonce = types.BlockNonce{}
	number := header.Number.Uint64()
	if len(header.Extra) < extraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, extraVanity-len(header.Extra))...)
	}
	header.Extra = header.Extra[:extraVanity]
	header.Extra = append(header.Extra, make([]byte, extraSeal)...)
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Difficulty = d.CalcDifficulty(chain, header.Time.Uint64(), parent)
	header.Validator = d.signer
	return nil
}

func AccumulateRewards(config *params.ChainConfig, state *state.StateDB, header *types.Header, uncles []*types.Header) {
//根据链进程选择正确的区块奖励
	blockReward := frontierBlockReward
	if config.IsByzantium(header.Number) {
		blockReward = byzantiumBlockReward
	}
//为矿工和任何包括叔叔的人累积奖励
	reward := new(big.Int).Set(blockReward)
	state.AddBalance(header.Coinbase, reward)
}

//将出块周期内的交易打包进新的区域块中
func (d *Dpos) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
	uncles []*types.Header, receipts []*types.Receipt, dposContext *types.DposContext) (*types.Block, error) {
//累积积木奖励并提交最终状态根
	AccumulateRewards(chain.Config(), state, header, uncles)
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))

	parent := chain.GetHeaderByHash(header.ParentHash)
	epochContext := &EpochContext{
		statedb:     state,
		DposContext: dposContext,
		TimeStamp:   header.Time.Int64(),
	}
	if timeOfFirstBlock == 0 {
		if firstBlockHeader := chain.GetHeaderByNumber(1); firstBlockHeader != nil {
			timeOfFirstBlock = firstBlockHeader.Time.Int64()
		}
	}
	fmt.Println("++++++++++++++77777++++++++++++++++++\n")
	fmt.Println("**************get genesis header********\n")
	genesis := chain.GetHeaderByNumber(0)

	err := epochContext.tryElect(genesis, parent)
	if err != nil {
		return nil, fmt.Errorf("got error when elect next epoch, err: %s", err)
	}

//更新薄荷计数trie
	updateMintCnt(parent.Time.Int64(), header.Time.Int64(), header.Validator, dposContext)
	header.DposContext = dposContext.ToProto()
	return types.NewBlock(header, txs, uncles, receipts), nil
}

func (d *Dpos) checkDeadline(lastBlock *types.Block, now int64, blockInterval uint64) error {
	prevSlot := PrevSlot(now, blockInterval)
	nextSlot := NextSlot(now, blockInterval)
	if lastBlock.Time().Int64() >= nextSlot {
		return ErrMintFutureBlock
	}
//最后一个街区到了，或者时间到了
	if lastBlock.Time().Int64() == prevSlot || nextSlot-now <= 1 {
		return nil
	}
	return ErrWaitForPrevBlock
}

//检查当前的验证人员是否在当前的节点上
func (d *Dpos) CheckValidator(lastBlock *types.Block, now int64,blockInterval uint64) error {
	if err := d.checkDeadline(lastBlock, now, blockInterval); err != nil {
		return err
	}
//
	dposContext, err := types.NewDposContextFromProto(trie.NewDatabase(d.db), lastBlock.Header().DposContext)
	if err != nil {
		return err
	}
	epochContext := &EpochContext{DposContext: dposContext}
	validator, err := epochContext.lookupValidator(now,blockInterval)
	if err != nil {
		return err
	}
	if (validator == common.Address{}) || bytes.Compare(validator.Bytes(), d.signer.Bytes()) != 0 {
		return ErrInvalidBlockValidator
	}
	return nil
}

//Seal使用本地矿工的
//密封顶部。
//验证模块内容是否符合DPOSS计算法规（验证新模块是否应由该验证人员提出模块）
func (d *Dpos) Seal(chain consensus.ChainReader, block *types.Block, stop <-chan struct{}) (*types.Block, error) {
	header := block.Header()
	number := header.Number.Uint64()
//不支持密封Genesis块
	if number == 0 {
		return nil, errUnknownBlock
	}
	now := time.Now().Unix()
	delay := NextSlot(now,chain.GetHeaderByNumber(0).BlockInterval) - now
	if delay > 0 {
		select {
		case <-stop:
			return nil, nil
		case <-time.After(time.Duration(delay) * time.Second):
		}
	}
	block.Header().Time.SetInt64(time.Now().Unix())

//时间到了，在街区签名
//对新块进行签名
	sighash, err := d.signFn(accounts.Account{Address: d.signer}, sigHash(header).Bytes())
	if err != nil {
		return nil, err
	}
	copy(header.Extra[len(header.Extra)-extraSeal:], sighash)
	return block.WithSeal(header), nil
}

func (d *Dpos) CalcDifficulty(chain consensus.ChainReader, time uint64, parent *types.Header) *big.Int {
	return big.NewInt(1)
}

func (d *Dpos) APIs(chain consensus.ChainReader) []rpc.API {
	return []rpc.API{{
		Namespace: "dpos",
		Version:   "1.0",
		Service:   &API{chain: chain, dpos: d},
		Public:    true,
	}}
}

func (d *Dpos) Authorize(signer common.Address, signFn SignerFn) {
	d.mu.Lock()
	d.signer = signer
	d.signFn = signFn
	d.mu.Unlock()
}

func (d *Dpos) Close() error {
	return nil
}

//ecrecover从签名的头中提取以太坊帐户地址。
func ecrecover(header *types.Header, sigcache *lru.ARCCache) (common.Address, error) {
//如果签名已经缓存，则返回
	hash := header.Hash()
	if address, known := sigcache.Get(hash); known {
		return address.(common.Address), nil
	}
//从头中检索签名额外数据
	if len(header.Extra) < extraSeal {
		return common.Address{}, errMissingSignature
	}
	signature := header.Extra[len(header.Extra)-extraSeal:]
//恢复公钥和以太坊地址
	pubkey, err := crypto.Ecrecover(sigHash(header).Bytes(), signature)
	if err != nil {
		return common.Address{}, err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])
	sigcache.Add(hash, signer)
	return signer, nil
}

func PrevSlot(now int64, blockInterval uint64) int64 {
	return int64((now-1)/int64(blockInterval)) * int64(blockInterval)
}

func NextSlot(now int64, blockInterval uint64) int64 {
	return int64((now+int64(blockInterval)-1)/int64(blockInterval)) * int64(blockInterval)
}

//更新Newblock矿工的mintcntrie计数
//更新周期内验证人员出块数目的
func updateMintCnt(parentBlockTime, currentBlockTime int64, validator common.Address, dposContext *types.DposContext) {
	currentMintCntTrie := dposContext.MintCntTrie()
	currentEpoch := parentBlockTime / epochInterval
	currentEpochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(currentEpochBytes, uint64(currentEpoch))

	cnt := int64(1)
	newEpoch := currentBlockTime / epochInterval
//仍在当前报告期间
	if currentEpoch == newEpoch {
		iter := trie.NewIterator(currentMintCntTrie.NodeIterator(currentEpochBytes))

//当电流不是起源时，从mintcntrie读取最后一个计数。
		if iter.Next() {
			cntBytes := currentMintCntTrie.Get(append(currentEpochBytes, validator.Bytes()...))

//不是第一次造币
			if cntBytes != nil {
				cnt = int64(binary.BigEndian.Uint64(cntBytes)) + 1
			}
		}
	}

	newCntBytes := make([]byte, 8)
	newEpochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(newEpochBytes, uint64(newEpoch))
	binary.BigEndian.PutUint64(newCntBytes, uint64(cnt))
	dposContext.MintCntTrie().TryUpdate(append(newEpochBytes, validator.Bytes()...), newCntBytes)
}
