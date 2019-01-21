
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//代码由github.com/fjl/gencodec生成。不要编辑。

package types

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var _ = (*headerMarshaling)(nil)


func (h Header) MarshalJSON() ([]byte, error) {
	type Header struct {
		ParentHash  common.Hash    `json:"parentHash"       gencodec:"required"`
		UncleHash   common.Hash    `json:"sha3Uncles"       gencodec:"required"`
		Validator   common.Address    `json:"validator"        gencodec:"required"`
		Coinbase    common.Address    `json:"coinbase"         gencodec:"required"`
		Root        common.Hash    `json:"stateRoot"        gencodec:"required"`
		TxHash      common.Hash    `json:"transactionsRoot" gencodec:"required"`
		ReceiptHash common.Hash    `json:"receiptsRoot"     gencodec:"required"`
		DposContext *DposContextProto `json:"dposContext"      gencodec:"required"`
		Bloom       Bloom          `json:"logsBloom"        gencodec:"required"`
		Difficulty  *hexutil.Big   `json:"difficulty"       gencodec:"required"`
		Number      *hexutil.Big   `json:"number"           gencodec:"required"`
		GasLimit    hexutil.Uint64 `json:"gasLimit"         gencodec:"required"`
		GasUsed     hexutil.Uint64 `json:"gasUsed"          gencodec:"required"`
		Time        *hexutil.Big   `json:"timestamp"        gencodec:"required"`
		Extra       hexutil.Bytes  `json:"extraData"        gencodec:"required"`
		MixDigest   common.Hash    `json:"mixHash"          gencodec:"required"`
		Nonce       BlockNonce     `json:"nonce"            gencodec:"required"`
		Hash        common.Hash    `json:"hash"`
	}
	var enc Header
	enc.ParentHash = h.ParentHash
	enc.UncleHash = h.UncleHash
	enc.Validator = h.Validator
	enc.Coinbase = h.Coinbase
	enc.Root = h.Root
	enc.TxHash = h.TxHash
	enc.ReceiptHash = h.ReceiptHash
	enc.DposContext = h.DposContext
	enc.Bloom = h.Bloom
	enc.Difficulty = (*hexutil.Big)(h.Difficulty)
	enc.Number = (*hexutil.Big)(h.Number)
	enc.GasLimit = hexutil.Uint64(h.GasLimit)
	enc.GasUsed = hexutil.Uint64(h.GasUsed)
	enc.Time = (*hexutil.Big)(h.Time)
	enc.Extra = h.Extra
	enc.MixDigest = h.MixDigest
	enc.Nonce = h.Nonce
	enc.Hash = h.Hash()
	return json.Marshal(&enc)
}

//取消标记JSON取消标记JSON。
func (h *Header) UnmarshalJSON(input []byte) error {
	type Header struct {
		ParentHash  *common.Hash    `json:"parentHash"       gencodec:"required"`
		UncleHash   *common.Hash    `json:"sha3Uncles"       gencodec:"required"`
		Validator   *common.Address   `json:"validator"        gencodec:"required"`
		Coinbase    *common.Address   `json:"coinbase"         gencodec:"required"`
		Root        *common.Hash    `json:"stateRoot"        gencodec:"required"`
		TxHash      *common.Hash    `json:"transactionsRoot" gencodec:"required"`
		ReceiptHash *common.Hash    `json:"receiptsRoot"     gencodec:"required"`
		DposContext *DposContextProto `json:"dposContext"      gencodec:"required"`
		Bloom       *Bloom          `json:"logsBloom"        gencodec:"required"`
		Difficulty  *hexutil.Big    `json:"difficulty"       gencodec:"required"`
		Number      *hexutil.Big    `json:"number"           gencodec:"required"`
		GasLimit    *hexutil.Uint64 `json:"gasLimit"         gencodec:"required"`
		GasUsed     *hexutil.Uint64 `json:"gasUsed"          gencodec:"required"`
		Time        *hexutil.Big    `json:"timestamp"        gencodec:"required"`
		Extra       *hexutil.Bytes  `json:"extraData"        gencodec:"required"`
		MixDigest   *common.Hash    `json:"mixHash"          gencodec:"required"`
		Nonce       *BlockNonce     `json:"nonce"            gencodec:"required"`
	}
	var dec Header
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.ParentHash == nil {
		return errors.New("missing required field 'parentHash' for Header")
	}
	h.ParentHash = *dec.ParentHash
	if dec.UncleHash == nil {
		return errors.New("missing required field 'sha3Uncles' for Header")
	}
	h.UncleHash = *dec.UncleHash
	if dec.Validator == nil {
		return errors.New("missing required field 'validator' for Header")
	}
	h.Validator = *dec.Validator
	if dec.Coinbase == nil {
		return errors.New("missing required field 'coinbase' for Header")
	}
	h.Coinbase = *dec.Coinbase
	if dec.Root == nil {
		return errors.New("missing required field 'stateRoot' for Header")
	}
	h.Root = *dec.Root
	if dec.TxHash == nil {
		return errors.New("missing required field 'transactionsRoot' for Header")
	}
	h.TxHash = *dec.TxHash
	if dec.ReceiptHash == nil {
		return errors.New("missing required field 'receiptsRoot' for Header")
	}
	h.ReceiptHash = *dec.ReceiptHash
	if dec.DposContext == nil {
		return errors.New("missing required field 'dposContext' for Header")
	}
	h.DposContext = dec.DposContext
	if dec.Bloom == nil {
		return errors.New("missing required field 'logsBloom' for Header")
	}
	h.Bloom = *dec.Bloom
	if dec.Difficulty == nil {
		return errors.New("missing required field 'difficulty' for Header")
	}
	h.Difficulty = (*big.Int)(dec.Difficulty)
	if dec.Number == nil {
		return errors.New("missing required field 'number' for Header")
	}
	h.Number = (*big.Int)(dec.Number)
	if dec.GasLimit == nil {
		return errors.New("missing required field 'gasLimit' for Header")
	}
	h.GasLimit = uint64(*dec.GasLimit)
	if dec.GasUsed == nil {
		return errors.New("missing required field 'gasUsed' for Header")
	}
	h.GasUsed = uint64(*dec.GasUsed)
	if dec.Time == nil {
		return errors.New("missing required field 'timestamp' for Header")
	}
	h.Time = (*big.Int)(dec.Time)
	if dec.Extra == nil {
		return errors.New("missing required field 'extraData' for Header")
	}
	h.Extra = *dec.Extra
	if dec.MixDigest == nil {
		return errors.New("missing required field 'mixHash' for Header")
	}
	h.MixDigest = *dec.MixDigest
	if dec.Nonce == nil {
		return errors.New("missing required field 'nonce' for Header")
	}
	h.Nonce = *dec.Nonce
	return nil
}
