
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2014 Go作者。版权所有。
//此源代码的使用受BSD样式的控制
//可以在许可文件中找到的许可证。

package sha3

//此文件提供用于创建SHA-3实例的函数
//和Shake哈希函数，以及用于哈希的实用函数
//字节。

import (
	"hash"
)

//new keccak 256创建新的keccak-256哈希。
func NewKeccak256() hash.Hash { return &state{rate: 136, outputLen: 32, dsbyte: 0x01} }

//new keccak 512创建新的keccak-512哈希。
func NewKeccak512() hash.Hash { return &state{rate: 72, outputLen: 64, dsbyte: 0x01} }

//new224创建新的sha3-224哈希。
//它的通用安全强度是224位，可以抵御图像前攻击，
//以及112位数据，以防碰撞攻击。
func New224() hash.Hash { return &state{rate: 144, outputLen: 28, dsbyte: 0x06} }

//new256创建新的sha3-256哈希。
//它的通用安全强度是256位，可以抵御图像前攻击，
//以及128位来抵御碰撞攻击。
func New256() hash.Hash { return &state{rate: 136, outputLen: 32, dsbyte: 0x06} }

//new384创建新的sha3-384哈希。
//它的通用安全强度是384位，可以抵御图像前攻击，
//以及192位数据，以防碰撞攻击。
func New384() hash.Hash { return &state{rate: 104, outputLen: 48, dsbyte: 0x06} }

//new512创建新的sha3-512哈希。
//它的通用安全强度是512位，可以抵御图像前攻击，
//和256位，以防碰撞攻击。
func New512() hash.Hash { return &state{rate: 72, outputLen: 64, dsbyte: 0x06} }

//SUM224返回数据的SHA3-224摘要。
func Sum224(data []byte) (digest [28]byte) {
	h := New224()
	h.Write(data)
	h.Sum(digest[:0])
	return
}

//SUM256返回数据的SHA3-256摘要。
func Sum256(data []byte) (digest [32]byte) {
	h := New256()
	h.Write(data)
	h.Sum(digest[:0])
	return
}

//SUM384返回数据的SHA3-384摘要。
func Sum384(data []byte) (digest [48]byte) {
	h := New384()
	h.Write(data)
	h.Sum(digest[:0])
	return
}

//SUM512返回数据的sha3-512摘要。
func Sum512(data []byte) (digest [64]byte) {
	h := New512()
	h.Write(data)
	h.Sum(digest[:0])
	return
}
