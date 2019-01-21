
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

//此文件定义了ShakeHash接口，并提供
//用于创建震动实例的函数以及实用程序
//用于将字节散列为任意长度输出的函数。

import (
	"io"
)

//shakehash定义哈希函数的接口
//支持任意长度输出。
type ShakeHash interface {
//写将吸收更多的数据进入散列状态。如果输入为
//在从中读取输出后写入。
	io.Writer

//读取从哈希中读取更多输出；读取会影响哈希的
//状态。（因此，shakehash.read与hash.sum非常不同）
//它从不返回错误。
	io.Reader

//Clone returns a copy of the ShakeHash in its current state.
	Clone() ShakeHash

//重置将ShakeHash重置为其初始状态。
	Reset()
}

func (d *state) Clone() ShakeHash {
	return d.clone()
}

//new shake128创建一个新的shake128变量输出长度shakehash。
//Its generic security strength is 128 bits against all attacks if at
//至少使用32个字节的输出。
func NewShake128() ShakeHash { return &state{rate: 168, dsbyte: 0x1f} }

//newshake256创建一个新的shake128变量输出长度shakehash。
//如果
//至少使用64个字节的输出。
func NewShake256() ShakeHash { return &state{rate: 136, dsbyte: 0x1f} }

//shakesum128将任意长度的数据摘要写入哈希。
func ShakeSum128(hash, data []byte) {
	h := NewShake128()
	h.Write(data)
	h.Read(hash)
}

//shakesum256将任意长度的数据摘要写入哈希。
func ShakeSum256(hash, data []byte) {
	h := NewShake256()
	h.Write(data)
	h.Read(hash)
}
