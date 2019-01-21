
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package dpos

import (
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/stretchr/testify/assert"
)

func TestEpochContextCountVotes(t *testing.T) {
	voteMap := map[common.Address][]common.Address{
		common.HexToAddress("0x44d1ce0b7cb3588bca96151fe1bc05af38f91b6e"): {
			common.HexToAddress("0xb040353ec0f2c113d5639444f7253681aecda1f8"),
		},
		common.HexToAddress("0xa60a3886b552ff9992cfcd208ec1152079e046c2"): {
			common.HexToAddress("0x14432e15f21237013017fa6ee90fc99433dec82c"),
			common.HexToAddress("0x9f30d0e5c9c88cade54cd1adecf6bc2c7e0e5af6"),
		},
		common.HexToAddress("0x4e080e49f62694554871e669aeb4ebe17c4a9670"): {
			common.HexToAddress("0xd83b44a3719720ec54cdb9f54c0202de68f1ebcb"),
			common.HexToAddress("0x56cc452e450551b7b9cffe25084a069e8c1e9441"),
			common.HexToAddress("0xbcfcb3fa8250be4f2bf2b1e70e1da500c668377b"),
		},
		common.HexToAddress("0x9d9667c71bb09d6ca7c3ed12bfe5e7be24e2ffe1"): {},
	}
	balance := int64(5)
	db := ethdb.NewMemDatabase()
	stateDB, _ := state.New(common.Hash{}, state.NewDatabase(db))
	trieDB := trie.NewDatabase(db)
	dposContext, err := types.NewDposContext(trieDB)
	assert.Nil(t, err)

	epochContext := &EpochContext{
		DposContext: dposContext,
		statedb:     stateDB,
	}
	_, err = epochContext.countVotes()
	assert.NotNil(t, err)

	for candidate, electors := range voteMap {
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
		for _, elector := range electors {
			stateDB.SetBalance(elector, big.NewInt(balance))
			assert.Nil(t, dposContext.Delegate(elector, candidate))
		}
	}
	result, err := epochContext.countVotes()
	assert.Nil(t, err)
	assert.Equal(t, len(voteMap), len(result))
	for candidate, electors := range voteMap {
		voteCount, ok := result[candidate]
		assert.True(t, ok)
		assert.Equal(t, balance*int64(len(electors)), voteCount.Int64())
	}
}

func TestLookupValidator(t *testing.T) {
	db := ethdb.NewMemDatabase()
	trieDB := trie.NewDatabase(db)
	dposCtx, _ := types.NewDposContext(trieDB)
	mockEpochContext := &EpochContext{
		DposContext: dposCtx,
	}
	validators := []common.Address{
		common.StringToAddress("addr1"),
		common.StringToAddress("addr2"),
		common.StringToAddress("addr3"),
	}
	mockEpochContext.DposContext.SetValidators(validators)
	for i, expected := range validators {
		got, _ := mockEpochContext.lookupValidator(int64(i) * blockInterval)
		if got != expected {
			t.Errorf("Failed to test lookup validator, %s was expected but got %s", expected.Str(), got.Str())
		}
	}
	blockInterval := 10
	_, err := mockEpochContext.lookupValidator(blockInterval - 1)
	if err != ErrInvalidMintBlockTime {
		t.Errorf("Failed to test lookup validator. err '%v' was expected but got '%v'", ErrInvalidMintBlockTime, err)
	}
}

func TestEpochContextKickoutValidator(t *testing.T) {
	db := ethdb.NewMemDatabase()
	stateDB, _ := state.New(common.Hash{}, state.NewDatabase(db))
	trieDB := trie.NewDatabase(db)
	dposContext, err := types.NewDposContext(trieDB)
	assert.Nil(t, err)
	epochContext := &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	atLeastMintCnt := epochInterval / blockInterval / maxValidatorSize / 2
	testEpoch := int64(1)

//没有验证器可以被踢出，因为所有验证器至少能制造足够的块。
	validators := []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.StringToAddress("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt)
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, dposContext.BecomeCandidate(common.StringToAddress("addr")))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap := getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, maxValidatorSize +1, len(candidateMap))

//至少有一个最安全的候选人将保留
	trieDB = trie.NewDatabase(db)
	dposContext, err = types.NewDposContext(trieDB)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.StringToAddress("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt-int64(i)-1)
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, safeSize, len(candidateMap))
	for i := maxValidatorSize - 1; i >= safeSize; i-- {
		assert.False(t, candidateMap[common.StringToAddress("addr"+strconv.Itoa(i))])
	}

//所有验证器都将被取消，因为所有验证器至少没有创建足够的块。
	trieDB = trie.NewDatabase(db)
	dposContext, err = types.NewDposContext(trieDB)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.StringToAddress("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt-1)
	}
	for i := maxValidatorSize; i < maxValidatorSize *2; i++ {
		candidate := common.StringToAddress("addr" + strconv.Itoa(i))
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, maxValidatorSize, len(candidateMap))

//只有一个验证器铸币计数不够
	trieDB = trie.NewDatabase(db)
	dposContext, err = types.NewDposContext(trieDB)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.StringToAddress("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		if i == 0 {
			setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt-1)
		} else {
			setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt)
		}
	}
	assert.Nil(t, dposContext.BecomeCandidate(common.StringToAddress("addr")))
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, maxValidatorSize, len(candidateMap))
	assert.False(t, candidateMap[common.StringToAddress("addr"+strconv.Itoa(0))])

//epochtime不完整，所有验证器都至少创建足够的块
	trieDB = trie.NewDatabase(db)
	dposContext, err = types.NewDposContext(trieDB)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval / 2,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.StringToAddress("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt/2)
	}
	for i := maxValidatorSize; i < maxValidatorSize *2; i++ {
		candidate := common.StringToAddress("addr" + strconv.Itoa(i))
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, maxValidatorSize *2, len(candidateMap))

//epochtime不完整，所有验证器至少没有创建足够的块
	trieDB = trie.NewDatabase(db)
	dposContext, err = types.NewDposContext(trieDB)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval / 2,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.StringToAddress("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt/2-1)
	}
	for i := maxValidatorSize; i < maxValidatorSize *2; i++ {
		candidate := common.StringToAddress("addr" + strconv.Itoa(i))
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, maxValidatorSize, len(candidateMap))

	trieDB = trie.NewDatabase(db)
	dposContext, err = types.NewDposContext(trieDB)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval / 2,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	assert.NotNil(t, epochContext.kickoutValidator(testEpoch))
	dposContext.SetValidators([]common.Address{})
	assert.NotNil(t, epochContext.kickoutValidator(testEpoch))
}

func setTestMintCnt(dposContext *types.DposContext, epoch int64, validator common.Address, count int64) {
	for i := int64(0); i < count; i++ {
		updateMintCnt(epoch*epochInterval, epoch*epochInterval+blockInterval, validator, dposContext)
	}
}

func getCandidates(candidateTrie *trie.Trie) map[common.Address]bool {
	candidateMap := map[common.Address]bool{}
	iter := trie.NewIterator(candidateTrie.NodeIterator(nil))
	for iter.Next() {
		candidateMap[common.BytesToAddress(iter.Value)] = true
	}
	return candidateMap
}

func TestEpochContextTryElect(t *testing.T) {
	db := ethdb.NewMemDatabase()
	stateDB, _ := state.New(common.Hash{}, state.NewDatabase(db))
	trieDB := trie.NewDatabase(db)
	dposContext, err := types.NewDposContext(trieDB)
	assert.Nil(t, err)
	epochContext := &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	atLeastMintCnt := epochInterval / blockInterval / maxValidatorSize / 2
	testEpoch := int64(1)
	validators := []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.StringToAddress("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		assert.Nil(t, dposContext.Delegate(validator, validator))
		stateDB.SetBalance(validator, big.NewInt(1))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt-1)
	}
	dposContext.BecomeCandidate(common.StringToAddress("more"))
	assert.Nil(t, dposContext.SetValidators(validators))

//genesisepoch==parentepoch不开始
	genesis := &types.Header{
		Time: big.NewInt(0),
	}
	parent := &types.Header{
		Time: big.NewInt(epochInterval - blockInterval),
	}
	oldHash := dposContext.EpochTrie().Hash()
	assert.Nil(t, epochContext.tryElect(genesis, parent))
	result, err := dposContext.GetValidators()
	assert.Nil(t, err)
	assert.Equal(t, maxValidatorSize, len(result))
	for _, validator := range result {
		assert.True(t, strings.Contains(validator.Str(), "addr"))
	}
	assert.NotEqual(t, oldHash, dposContext.EpochTrie().Hash())

//基因时代！=parentepoch和have none mintcnt不退出
	genesis = &types.Header{
		Time: big.NewInt(-epochInterval),
	}
	parent = &types.Header{
		Difficulty: big.NewInt(1),
		Time:       big.NewInt(epochInterval - blockInterval),
	}
	epochContext.TimeStamp = epochInterval
	oldHash = dposContext.EpochTrie().Hash()
	assert.Nil(t, epochContext.tryElect(genesis, parent))
	result, err = dposContext.GetValidators()
	assert.Nil(t, err)
	assert.Equal(t, maxValidatorSize, len(result))
	for _, validator := range result {
		assert.True(t, strings.Contains(validator.Str(), "addr"))
	}
	assert.NotEqual(t, oldHash, dposContext.EpochTrie().Hash())

//基因时代！=ParentEpoch启动
	genesis = &types.Header{
		Time: big.NewInt(0),
	}
	parent = &types.Header{
		Time: big.NewInt(epochInterval*2 - blockInterval),
	}
	epochContext.TimeStamp = epochInterval * 2
	oldHash = dposContext.EpochTrie().Hash()
	assert.Nil(t, epochContext.tryElect(genesis, parent))
	result, err = dposContext.GetValidators()
	assert.Nil(t, err)
	assert.Equal(t, safeSize, len(result))
	moreCnt := 0
	for _, validator := range result {
		if strings.Contains(validator.Str(), "more") {
			moreCnt++
		}
	}
	assert.Equal(t, 1, moreCnt)
	assert.NotEqual(t, oldHash, dposContext.EpochTrie().Hash())

//parentepoch==currentpoch不选择
	genesis = &types.Header{
		Time: big.NewInt(0),
	}
	parent = &types.Header{
		Time: big.NewInt(epochInterval),
	}
	epochContext.TimeStamp = epochInterval + blockInterval
	oldHash = dposContext.EpochTrie().Hash()
	assert.Nil(t, epochContext.tryElect(genesis, parent))
	result, err = dposContext.GetValidators()
	assert.Nil(t, err)
	assert.Equal(t, safeSize, len(result))
	assert.Equal(t, oldHash, dposContext.EpochTrie().Hash())
}
