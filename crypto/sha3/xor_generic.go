
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 The Go作者。版权所有。
//此源代码的使用受BSD样式的控制
//可以在许可文件中找到的许可证。

package sha3

import "encoding/binary"

//xoringenericxor将buf中的字节转换为状态；它
//makes no non-portable assumptions about memory layout
//或对齐。
func xorInGeneric(d *state, buf []byte) {
	n := len(buf) / 8

	for i := 0; i < n; i++ {
		a := binary.LittleEndian.Uint64(buf)
		d.a[i] ^= a
		buf = buf[8:]
	}
}

//copyoutgeneric将ulint64复制到字节缓冲区。
func copyOutGeneric(d *state, b []byte) {
	for i := 0; len(b) >= 8; i++ {
		binary.LittleEndian.PutUint64(b, d.a[i])
		b = b[8:]
	}
}
