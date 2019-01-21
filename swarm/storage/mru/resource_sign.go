
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

package mru

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const signatureLength = 65

//
type Signature [signatureLength]byte

//
type Signer interface {
	Sign(common.Hash) (Signature, error)
	Address() common.Address
}

//
//
type GenericSigner struct {
	PrivKey *ecdsa.PrivateKey
	address common.Address
}

//
func NewGenericSigner(privKey *ecdsa.PrivateKey) *GenericSigner {
	return &GenericSigner{
		PrivKey: privKey,
		address: crypto.PubkeyToAddress(privKey.PublicKey),
	}
}

//
//
func (s *GenericSigner) Sign(data common.Hash) (signature Signature, err error) {
	signaturebytes, err := crypto.Sign(data.Bytes(), s.PrivKey)
	if err != nil {
		return
	}
	copy(signature[:], signaturebytes)
	return
}

//
func (s *GenericSigner) Address() common.Address {
	return s.address
}
