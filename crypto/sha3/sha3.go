
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

//sponge direction表示字节在海绵中流动的方向。
type spongeDirection int

const (
//海绵吸收表示海绵吸收输入。
	spongeAbsorbing spongeDirection = iota
//海绵挤压表示海绵被挤压。
	spongeSqueezing
)

const (
//maxRate is the maximum size of the internal buffer. 沙克-256
//目前需要最大的缓冲区。
	maxRate = 168
)

type state struct {
//普通海绵组件。
a    [25]uint64 //哈希的主状态
buf  []byte     //指向存储区
rate int        //要使用的状态字节数

//dsbyte包含“域分隔”位和
//填充物。Sections 6.1 and 6.2 of [1] separate the outputs of the
//sha-3和通过向消息附加位串来震动函数。
//使用一个小的endian位排序约定，这些是sha-3的“01”
//震动为“1111”，或分别为00000010B和000001111B。然后
//第5.1节中的填充规则用于将消息填充到多个
//速率，包括添加“1”位、零位或更多“0”位，以及
//最后一个“1”位。我们将填充的第一个“1”位合并为dsbyte，
//给出00000110B（0x06）和00011111B（0x1F）。
//[1] http://csrc.nist.gov/publications/drafts/fips-202/fips_202_draft.pdf
//“FIPS 202草案：SHA-3标准：基于排列的哈希和
//可扩展输出功能（2014年5月）
	dsbyte  byte
	storage [maxRate]byte

//具体到sha-3和震动。
outputLen int             //默认输出大小（字节）
state     spongeDirection //海绵是吸收还是挤压
}

//blocksize返回此哈希函数的海绵速率。
func (d *state) BlockSize() int { return d.rate }

//大小返回哈希函数的输出大小为字节。
func (d *state) Size() int { return d.outputLen }

//重置通过归零海绵状态清除内部状态，并且
//字节缓冲区，并将sponce.state设置为absorption。
func (d *state) Reset() {
//使排列状态归零。
	for i := range d.a {
		d.a[i] = 0
	}
	d.state = spongeAbsorbing
	d.buf = d.storage[:0]
}

func (d *state) clone() *state {
	ret := *d
	if ret.state == spongeAbsorbing {
		ret.buf = ret.storage[:len(ret.buf)]
	} else {
		ret.buf = ret.storage[d.rate-cap(d.buf) : d.rate]
	}

	return &ret
}

//Permute采用KECCAKF-1600排列。它处理
//任何输入输出缓冲。
func (d *state) permute() {
	switch d.state {
	case spongeAbsorbing:
//如果我们正在吸收，我们需要将输入xor转换成状态
//在应用排列之前。
		xorIn(d, d.buf)
		d.buf = d.storage[:0]
		keccakF1600(&d.a)
	case spongeSqueezing:
//如果我们在挤压，我们需要在挤压前涂上置换素。
//复制更多输出。
		keccakF1600(&d.a)
		d.buf = d.storage[:d.rate]
		copyOut(d, d.buf)
	}
}

//PAD在dsbyte中附加域分隔位，应用
//多比特率10..1填充规则，并排列状态。
func (d *state) padAndPermute(dsbyte byte) {
	if d.buf == nil {
		d.buf = d.storage[:0]
	}
//具有此实例的域分隔位的PAD。我们知道有
//d.buf中至少有一个字节的空间，因为如果空间已满，
//会叫珀穆特清空它。dsbyte还包含
//第一位是填充。请参见state结构中的注释。
	d.buf = append(d.buf, dsbyte)
	zerosStart := len(d.buf)
	d.buf = d.storage[:d.rate]
	for i := zerosStart; i < d.rate; i++ {
		d.buf[i] = 0
	}
//这会为填充添加最后一位。因为这样
//位从LSB向上编号，最后一位是
//最后一个字节。
	d.buf[d.rate-1] ^= 0x80
//应用排列
	d.permute()
	d.state = spongeSqueezing
	d.buf = d.storage[:d.rate]
	copyOut(d, d.buf)
}

//写将吸收更多的数据进入散列状态。它会产生一个错误
//如果在写入之后有更多的数据写入shakehash
func (d *state) Write(p []byte) (written int, err error) {
	if d.state != spongeAbsorbing {
		panic("sha3: write to sponge after read")
	}
	if d.buf == nil {
		d.buf = d.storage[:0]
	}
	written = len(p)

	for len(p) > 0 {
		if len(d.buf) == 0 && len(p) >= d.rate {
//快速路径；吸收输入的完整“速率”字节并应用排列。
			xorIn(d, p[:d.rate])
			p = p[d.rate:]
			keccakF1600(&d.a)
		} else {
//慢路径；缓冲输入，直到我们可以填充海绵，然后将其XOR。
			todo := d.rate - len(d.buf)
			if todo > len(p) {
				todo = len(p)
			}
			d.buf = append(d.buf, p[:todo]...)
			p = p[todo:]

//If the sponge is full, apply the permutation.
			if len(d.buf) == d.rate {
				d.permute()
			}
		}
	}

	return
}

//read挤压海绵中任意数量的字节。
func (d *state) Read(out []byte) (n int, err error) {
//If we're still absorbing, pad and apply the permutation.
	if d.state == spongeAbsorbing {
		d.padAndPermute(d.dsbyte)
	}

	n = len(out)

//现在，挤压一下。
	for len(out) > 0 {
		n := copy(out, d.buf)
		d.buf = d.buf[n:]
		out = out[n:]

//如果我们把海绵挤干了，就按顺序排列。
		if len(d.buf) == 0 {
			d.permute()
		}
	}

	return
}

//Sum applies padding to the hash state and then squeezes out the desired
//输出字节数。
func (d *state) Sum(in []byte) []byte {
//复制原始哈希，以便调用方可以继续写入
//求和。
	dup := d.clone()
	hash := make([]byte, dup.outputLen)
	dup.Read(hash)
	return append(in, hash...)
}
