
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

//包ENR实现EIP-778中定义的以太坊节点记录。节点记录保留
//有关对等网络上节点的任意信息。
//
//记录包含命名键。要在记录中存储和检索键/值，请使用条目
//接口。
//
//在将记录传输到另一个节点之前，必须对它们进行签名。解码记录验证
//它的签名。创建记录时，设置所需条目，然后调用sign添加
//签名。修改记录会使签名失效。
//
//ENR包支持“secp256k1 keccak”身份方案。
package enr

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/ethereum/go-ethereum/rlp"
)

const SizeLimit = 300 //节点记录的最大编码大小（字节）

var (
	errNoID           = errors.New("unknown or unspecified identity scheme")
	errInvalidSig     = errors.New("invalid signature")
	errNotSorted      = errors.New("record key/value pairs are not sorted by key")
	errDuplicateKey   = errors.New("record contains duplicate key")
	errIncompletePair = errors.New("record contains incomplete k/v pair")
	errTooBig         = fmt.Errorf("record bigger than %d bytes", SizeLimit)
	errEncodeUnsigned = errors.New("can't encode unsigned record")
	errNotFound       = errors.New("no such key in record")
)

//记录表示节点记录。零值是一个空记录。
type Record struct {
seq       uint64 //序列号
signature []byte //签名
raw       []byte //RLP编码记录
pairs     []pair //所有键/值对的排序列表
}

//对是记录中的键/值对。
type pair struct {
	k string
	v rlp.RawValue
}

//签名报告记录是否具有有效签名。
func (r *Record) Signed() bool {
	return r.signature != nil
}

//seq返回序列号。
func (r *Record) Seq() uint64 {
	return r.seq
}

//setseq更新记录序列号。这将使记录上的任何签名失效。
//通常不需要调用setseq，因为在签名记录中设置了任何键
//增加序列号。
func (r *Record) SetSeq(s uint64) {
	r.signature = nil
	r.raw = nil
	r.seq = s
}

//LOAD检索键/值对的值。给定的项必须是指针，并且将
//设置为记录中条目的值。
//
//加载返回的错误被包装在keyError中。您可以区分解码错误
//使用isNotFound函数来消除丢失的键。
func (r *Record) Load(e Entry) error {
	i := sort.Search(len(r.pairs), func(i int) bool { return r.pairs[i].k >= e.ENRKey() })
	if i < len(r.pairs) && r.pairs[i].k == e.ENRKey() {
		if err := rlp.DecodeBytes(r.pairs[i].v, e); err != nil {
			return &KeyError{Key: e.ENRKey(), Err: err}
		}
		return nil
	}
	return &KeyError{Key: e.ENRKey(), Err: errNotFound}
}

//set添加或更新记录中的给定项。如果值不能
//编码的。如果记录已签名，则set将递增序列号并使其失效。
//序列号。
func (r *Record) Set(e Entry) {
	blob, err := rlp.EncodeToBytes(e)
	if err != nil {
		panic(fmt.Errorf("enr: can't encode %s: %v", e.ENRKey(), err))
	}
	r.invalidate()

	pairs := make([]pair, len(r.pairs))
	copy(pairs, r.pairs)
	i := sort.Search(len(pairs), func(i int) bool { return pairs[i].k >= e.ENRKey() })
	switch {
	case i < len(pairs) && pairs[i].k == e.ENRKey():
//元素存在于r.pairs[i]
		pairs[i].v = blob
	case i < len(r.pairs):
//在第i个元素之前插入对
		el := pair{e.ENRKey(), blob}
		pairs = append(pairs, pair{})
		copy(pairs[i+1:], pairs[i:])
		pairs[i] = el
	default:
//元素应放置在r.pairs的末尾
		pairs = append(pairs, pair{e.ENRKey(), blob})
	}
	r.pairs = pairs
}

func (r *Record) invalidate() {
	if r.signature == nil {
		r.seq++
	}
	r.signature = nil
	r.raw = nil
}

//encoderlp实现rlp.encoder。编码失败，如果
//记录未签名。
func (r Record) EncodeRLP(w io.Writer) error {
	if !r.Signed() {
		return errEncodeUnsigned
	}
	_, err := w.Write(r.raw)
	return err
}

//decoderlp实现rlp.decoder。解码验证签名。
func (r *Record) DecodeRLP(s *rlp.Stream) error {
	raw, err := s.Raw()
	if err != nil {
		return err
	}
	if len(raw) > SizeLimit {
		return errTooBig
	}

//解码RLP容器。
	dec := Record{raw: raw}
	s = rlp.NewStream(bytes.NewReader(raw), 0)
	if _, err := s.List(); err != nil {
		return err
	}
	if err = s.Decode(&dec.signature); err != nil {
		return err
	}
	if err = s.Decode(&dec.seq); err != nil {
		return err
	}
//记录的其余部分包含已排序的k/v对。
	var prevkey string
	for i := 0; ; i++ {
		var kv pair
		if err := s.Decode(&kv.k); err != nil {
			if err == rlp.EOL {
				break
			}
			return err
		}
		if err := s.Decode(&kv.v); err != nil {
			if err == rlp.EOL {
				return errIncompletePair
			}
			return err
		}
		if i > 0 {
			if kv.k == prevkey {
				return errDuplicateKey
			}
			if kv.k < prevkey {
				return errNotSorted
			}
		}
		dec.pairs = append(dec.pairs, kv)
		prevkey = kv.k
	}
	if err := s.ListEnd(); err != nil {
		return err
	}

	_, scheme := dec.idScheme()
	if scheme == nil {
		return errNoID
	}
	if err := scheme.Verify(&dec, dec.signature); err != nil {
		return err
	}
	*r = dec
	return nil
}

//nodeadr返回节点地址。如果记录是
//未签名或使用未知的标识方案。
func (r *Record) NodeAddr() []byte {
	_, scheme := r.idScheme()
	if scheme == nil {
		return nil
	}
	return scheme.NodeAddr(r)
}

//setsig设置记录签名。如果编码的记录较大，则返回错误
//大于大小限制或根据传递的方案签名无效。
func (r *Record) SetSig(idscheme string, sig []byte) error {
//检查“id”是否已设置并与给定方案匹配。这种恐慌是因为
//这里的不一致始终是签名函数调用中的实现错误
//这种方法。
	id, s := r.idScheme()
	if s == nil {
		panic(errNoID)
	}
	if id != idscheme {
		panic(fmt.Errorf("identity scheme mismatch in Sign: record has %s, want %s", id, idscheme))
	}

//对照方案进行验证。
	if err := s.Verify(r, sig); err != nil {
		return err
	}
	raw, err := r.encode(sig)
	if err != nil {
		return err
	}
	r.signature, r.raw = sig, raw
	return nil
}

//AppendElements将序列号和条目追加到给定切片。
func (r *Record) AppendElements(list []interface{}) []interface{} {
	list = append(list, r.seq)
	for _, p := range r.pairs {
		list = append(list, p.k, p.v)
	}
	return list
}

func (r *Record) encode(sig []byte) (raw []byte, err error) {
	list := make([]interface{}, 1, 2*len(r.pairs)+1)
	list[0] = sig
	list = r.AppendElements(list)
	if raw, err = rlp.EncodeToBytes(list); err != nil {
		return nil, err
	}
	if len(raw) > SizeLimit {
		return nil, errTooBig
	}
	return raw, nil
}

func (r *Record) idScheme() (string, IdentityScheme) {
	var id ID
	if err := r.Load(&id); err != nil {
		return "", nil
	}
	return string(id), FindIdentityScheme(string(id))
}
