
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

//
package pot

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

var (
	zerosBin = Address{}.Bin()
)

//
type Address common.Hash

//
func NewAddressFromBytes(b []byte) Address {
	h := common.Hash{}
	copy(h[:], b)
	return Address(h)
}

func (a Address) IsZero() bool {
	return a.Bin() == zerosBin
}

func (a Address) String() string {
	return fmt.Sprintf("%x", a[:])
}

//
func (a *Address) MarshalJSON() (out []byte, err error) {
	return []byte(`"` + a.String() + `"`), nil
}

//
func (a *Address) UnmarshalJSON(value []byte) error {
	*a = Address(common.HexToHash(string(value[1 : len(value)-1])))
	return nil
}

//
func (a Address) Bin() string {
	return ToBin(a[:])
}

//
func ToBin(a []byte) string {
	var bs []string
	for _, b := range a {
		bs = append(bs, fmt.Sprintf("%08b", b))
	}
	return strings.Join(bs, "")
}

//
func (a Address) Bytes() []byte {
	return a[:]
}

/*













*/

func proximity(one, other Address) (ret int, eq bool) {
	return posProximity(one, other, 0)
}

//
//
func posProximity(one, other Address, pos int) (ret int, eq bool) {
	for i := pos / 8; i < len(one); i++ {
		if one[i] == other[i] {
			continue
		}
		oxo := one[i] ^ other[i]
		start := 0
		if i == pos/8 {
			start = pos % 8
		}
		for j := start; j < 8; j++ {
			if (oxo>>uint8(7-j))&0x01 != 0 {
				return i*8 + j, false
			}
		}
	}
	return len(one) * 8, true
}

//
//
//
func ProxCmp(a, x, y interface{}) int {
	return proxCmp(ToBytes(a), ToBytes(x), ToBytes(y))
}

func proxCmp(a, x, y []byte) int {
	for i := range a {
		dx := x[i] ^ a[i]
		dy := y[i] ^ a[i]
		if dx > dy {
			return 1
		} else if dx < dy {
			return -1
		}
	}
	return 0
}

//
//
//
func RandomAddressAt(self Address, prox int) (addr Address) {
	addr = self
	pos := -1
	if prox >= 0 {
		pos = prox / 8
		trans := prox % 8
		transbytea := byte(0)
		for j := 0; j <= trans; j++ {
			transbytea |= 1 << uint8(7-j)
		}
		flipbyte := byte(1 << uint8(7-trans))
		transbyteb := transbytea ^ byte(255)
		randbyte := byte(rand.Intn(255))
		addr[pos] = ((addr[pos] & transbytea) ^ flipbyte) | randbyte&transbyteb
	}
	for i := pos + 1; i < len(addr); i++ {
		addr[i] = byte(rand.Intn(255))
	}

	return
}

//
func RandomAddress() Address {
	return RandomAddressAt(Address{}, -1)
}

//
func NewAddressFromString(s string) []byte {
	ha := [32]byte{}

	t := s + zerosBin[:len(zerosBin)-len(s)]
	for i := 0; i < 4; i++ {
		n, err := strconv.ParseUint(t[i*64:(i+1)*64], 2, 64)
		if err != nil {
			panic("wrong format: " + err.Error())
		}
		binary.BigEndian.PutUint64(ha[i*8:(i+1)*8], n)
	}
	return ha[:]
}

//
type BytesAddress interface {
	Address() []byte
}

//
func ToBytes(v Val) []byte {
	if v == nil {
		return nil
	}
	b, ok := v.([]byte)
	if !ok {
		ba, ok := v.(BytesAddress)
		if !ok {
			panic(fmt.Sprintf("unsupported value type %T", v))
		}
		b = ba.Address()
	}
	return b
}

//
//
func DefaultPof(max int) func(one, other Val, pos int) (int, bool) {
	return func(one, other Val, pos int) (int, bool) {
		po, eq := proximityOrder(ToBytes(one), ToBytes(other), pos)
		if po >= max {
			eq = true
			po = max
		}
		return po, eq
	}
}

func proximityOrder(one, other []byte, pos int) (int, bool) {
	for i := pos / 8; i < len(one); i++ {
		if one[i] == other[i] {
			continue
		}
		oxo := one[i] ^ other[i]
		start := 0
		if i == pos/8 {
			start = pos % 8
		}
		for j := start; j < 8; j++ {
			if (oxo>>uint8(7-j))&0x01 != 0 {
				return i*8 + j, false
			}
		}
	}
	return len(one) * 8, true
}

//
func Label(v Val) string {
	if v == nil {
		return "<nil>"
	}
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	if b, ok := v.([]byte); ok {
		return ToBin(b)
	}
	panic(fmt.Sprintf("unsupported value type %T", v))
}
