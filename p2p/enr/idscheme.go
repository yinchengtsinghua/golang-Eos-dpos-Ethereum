
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

package enr

import (
	"crypto/ecdsa"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/rlp"
)

//已知身份方案的注册表。
var schemes sync.Map

//标识方案能够验证记录签名和
//派生节点地址。
type IdentityScheme interface {
	Verify(r *Record, sig []byte) error
	NodeAddr(r *Record) []byte
}

//RegisterIdentityScheme向全局注册表添加标识方案。
func RegisterIdentityScheme(name string, scheme IdentityScheme) {
	if _, loaded := schemes.LoadOrStore(name, scheme); loaded {
		panic("identity scheme " + name + " already registered")
	}
}

//findidentityScheme将名称解析为全局注册表中的标识方案。
func FindIdentityScheme(name string) IdentityScheme {
	s, ok := schemes.Load(name)
	if !ok {
		return nil
	}
	return s.(IdentityScheme)
}

//v4id是“v4”标识方案。
type v4ID struct{}

func init() {
	RegisterIdentityScheme("v4", v4ID{})
}

//signv4使用v4方案对记录进行签名。
func SignV4(r *Record, privkey *ecdsa.PrivateKey) error {
//复制r以避免在签名失败时修改它。
	cpy := *r
	cpy.Set(ID("v4"))
	cpy.Set(Secp256k1(privkey.PublicKey))

	h := sha3.NewKeccak256()
	rlp.Encode(h, cpy.AppendElements(nil))
	sig, err := crypto.Sign(h.Sum(nil), privkey)
	if err != nil {
		return err
	}
sig = sig[:len(sig)-1] //移除V
	if err = cpy.SetSig("v4", sig); err == nil {
		*r = cpy
	}
	return err
}

//s256raw是一个未分析的secp256k1公钥条目。
type s256raw []byte

func (s256raw) ENRKey() string { return "secp256k1" }

func (v4ID) Verify(r *Record, sig []byte) error {
	var entry s256raw
	if err := r.Load(&entry); err != nil {
		return err
	} else if len(entry) != 33 {
		return fmt.Errorf("invalid public key")
	}

	h := sha3.NewKeccak256()
	rlp.Encode(h, r.AppendElements(nil))
	if !crypto.VerifySignature(entry, h.Sum(nil), sig) {
		return errInvalidSig
	}
	return nil
}

func (v4ID) NodeAddr(r *Record) []byte {
	var pubkey Secp256k1
	err := r.Load(&pubkey)
	if err != nil {
		return nil
	}
	buf := make([]byte, 64)
	math.ReadBits(pubkey.X, buf[:32])
	math.ReadBits(pubkey.Y, buf[32:])
	return crypto.Keccak256(buf)
}
