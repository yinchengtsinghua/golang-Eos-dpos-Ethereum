
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package dpos

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
)

type EpochContext struct {
	TimeStamp   int64
	DposContext *types.DposContext
	statedb     *state.StateDB
}

/*特赦
return:返回票人对应选人代表
  “0XFDB9694B92A33663F89C1FE8FCB3BD0BF07A9E09”：18000_
**/

func (ec *EpochContext) countVotes() (votes map[common.Address]*big.Int, err error) {
	votes = map[common.Address]*big.Int{}

//获得投票者列表、候选人列表以及用户基本信息列表
	delegateTrie := ec.DposContext.DelegateTrie()
	candidateTrie := ec.DposContext.CandidateTrie()
	statedb := ec.statedb

//代理人获得候选人名单
	iterCandidate := trie.NewIterator(candidateTrie.NodeIterator(nil))
	existCandidate := iterCandidate.Next()
	if !existCandidate {
		return votes, errors.New("no candidates")
	}
//末代皇帝5.srt
	for existCandidate {
candidate := iterCandidate.Value   //获取每个选项--bytes
candidateAddr := common.BytesToAddress(candidate) //将bytes转换为地址
delegateIterator := trie.NewIterator(delegateTrie.PrefixIterator(candidate))   //通过候选人找到每一个候选人对应票信息列表
existDelegator := delegateIterator.Next()                                     //调用代理next（）判断代理
if !existDelegator {                                                          //如果在候选人名单中为空
votes[candidateAddr] = new(big.Int)                                       //在投票者隐蔽中追踪候选人信息
			existCandidate = iterCandidate.Next()
			continue
		}
for existDelegator {                                                         //历任候选人对应票人信息列表
delegator := delegateIterator.Value                                      //
score, ok := votes[candidateAddr]                                        //获得候选人投票权
			if !ok {
score = new(big.Int)                                                 //当没有查到投票者信息时，将定义一个局部历史分数
			}
delegatorAddr := common.BytesToAddress(delegator)                        //将投票者字节类型转换为address
//获得投票者的余款作为票号累计到投票者的票号中
			weight := statedb.GetBalance(delegatorAddr)
			score.Add(score, weight)
			votes[candidateAddr] = score
			existDelegator = delegateIterator.Next()
		}
		existCandidate = iterCandidate.Next()
	}
	return votes, nil
}

//除害验证人计算法
func (ec *EpochContext) kickoutValidator(epoch int64,genesis *types.Header) error {
	validators, err := ec.DposContext.GetValidators()


//var maxvalidator大小Int64
//var safesize int64
	fmt.Println("++++++++++++++++++++++++++9999++++++++++++++++++++++\n")
	fmt.Println("kickoutValidator test")
	maxValidatorSize := genesis.MaxValidatorSize
	safeSize := int(maxValidatorSize*2/3+1)

	if err != nil {
		return fmt.Errorf("failed to get validator: %s", err)
	}
	if len(validators) == 0 {
		return errors.New("no validator could be kickout")
	}

	epochDuration := epochInterval
	fmt.Println("0000000000000000000",epochDuration,"00000000000000\n")
	blockInterval := genesis.BlockInterval
//第一个历元的持续时间可以是历元间隔，
//虽然第一个街区时间并不总是与时代间隔一致，
//所以用第一块时间而不是年代间隔来计算第一个时期的二分之一。
//防止验证器被错误地踢出。
	if ec.TimeStamp-timeOfFirstBlock < epochInterval {
		epochDuration = ec.TimeStamp - timeOfFirstBlock
	}

	needKickoutValidators := sortableAddresses{}
	for _, validator := range validators {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(epoch))
		key = append(key, validator.Bytes()...)
		cnt := int64(0)
		if cntBytes := ec.DposContext.MintCntTrie().Get(key); cntBytes != nil {
			cnt = int64(binary.BigEndian.Uint64(cntBytes))
		}

		if cnt < epochDuration/int64(blockInterval)/ int64(maxValidatorSize) /2 {
//非活动验证器需要启动
			needKickoutValidators = append(needKickoutValidators, &sortableAddress{validator, big.NewInt(cnt)})
		}
	}
//没有验证器需要启动
	needKickoutValidatorCnt := len(needKickoutValidators)
	if needKickoutValidatorCnt <= 0 {
		return nil
	}
	sort.Sort(sort.Reverse(needKickoutValidators))

	candidateCount := 0
	iter := trie.NewIterator(ec.DposContext.CandidateTrie().NodeIterator(nil))
	for iter.Next() {
		candidateCount++
		if candidateCount >= needKickoutValidatorCnt+int(safeSize) {
			break
		}
	}

	for i, validator := range needKickoutValidators {
//确保候选计数大于或等于safesize
		if candidateCount <= int(safeSize) {
			log.Info("No more candidate can be kickout", "prevEpochID", epoch, "candidateCount", candidateCount, "needKickoutCount", len(needKickoutValidators)-i)
			return nil
		}

		if err := ec.DposContext.KickoutCandidate(validator.address); err != nil {
			return err
		}
//
		candidateCount--
		log.Info("Kickout candidate", "prevEpochID", epoch, "candidate", validator.address.String(), "mintCnt", validator.weight.String())
	}
	return nil
}

//实时检查出块者是否是本节点
func (ec *EpochContext) lookupValidator(now int64, blockInterval uint64) (validator common.Address, err error) {
	validator = common.Address{}
	offset := now % epochInterval
if offset%int64(blockInterval) != 0 {    //判断当前时间是否在出块周期内
		return common.Address{}, ErrInvalidMintBlockTime
	}
	offset /= int64(blockInterval)

	validators, err := ec.DposContext.GetValidators()
	if err != nil {
		return common.Address{}, err
	}
	validatorSize := len(validators)
	if validatorSize == 0 {
		return common.Address{}, errors.New("failed to lookup validator")
	}
	offset %= int64(validatorSize)
	return validators[offset], nil
}

type sortableAddress struct {
	address common.Address
	weight  *big.Int
}
type sortableAddresses []*sortableAddress

func (p sortableAddresses) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p sortableAddresses) Len() int      { return len(p) }
func (p sortableAddresses) Less(i, j int) bool {
	if p[i].weight.Cmp(p[j].weight) < 0 {
		return false
	} else if p[i].weight.Cmp(p[j].weight) > 0 {
		return true
	} else {
		return p[i].address.String() < p[j].address.String()
	}
}
