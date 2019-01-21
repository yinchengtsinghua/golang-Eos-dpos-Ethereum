
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

//测试包括Keccak团队在
//网址：https://github.com/gvanas/keccakcodepackage
//
//它们只包括按位测试向量的零位情况。
//由NIST在FIPS-202草案中发布。

import (
	"bytes"
	"compress/flate"
	"encoding/hex"
	"encoding/json"
	"hash"
	"os"
	"strings"
	"testing"
)

const (
	testString  = "brekeccakkeccak koax koax"
	katFilename = "testdata/keccakKats.json.deflate"
)

//用于测试KAT的震动的内部使用实例。
func newHashShake128() hash.Hash {
	return &state{rate: 168, dsbyte: 0x1f, outputLen: 512}
}
func newHashShake256() hash.Hash {
	return &state{rate: 136, dsbyte: 0x1f, outputLen: 512}
}

//testDigests contains functions returning hash.Hash instances
//输出长度等于SHA-3和
//震动实例。
var testDigests = map[string]func() hash.Hash{
	"SHA3-224": New224,
	"SHA3-256": New256,
	"SHA3-384": New384,
	"SHA3-512": New512,
	"SHAKE128": newHashShake128,
	"SHAKE256": newHashShake256,
}

//testshakes包含返回的shakehash实例的函数
//测试特定于ShakeHash的接口。
var testShakes = map[string]func() ShakeHash{
	"SHAKE128": NewShake128,
	"SHAKE256": NewShake256,
}

//用于封送JSON测试用例的结构。
type KeccakKats struct {
	Kats map[string][]struct {
		Digest  string `json:"digest"`
		Length  int64  `json:"length"`
		Message string `json:"message"`
	}
}

func testUnalignedAndGeneric(t *testing.T, testf func(impl string)) {
	xorInOrig, copyOutOrig := xorIn, copyOut
	xorIn, copyOut = xorInGeneric, copyOutGeneric
	testf("generic")
	if xorImplementationUnaligned != "generic" {
		xorIn, copyOut = xorInUnaligned, copyOutUnaligned
		testf("unaligned")
	}
	xorIn, copyOut = xorInOrig, copyOutOrig
}

//testkeccakkats针对所有
//来自HTTPS://GITHUBCOM/GVANAS/KECAKCOCODECAGE的StimeMgKATS
//（由于其长度，测试向量存储在keccakcats.json.deflate中。）
func TestKeccakKats(t *testing.T) {
	testUnalignedAndGeneric(t, func(impl string) {
//阅读KATS。
		deflated, err := os.Open(katFilename)
		if err != nil {
			t.Errorf("error opening %s: %s", katFilename, err)
		}
		file := flate.NewReader(deflated)
		dec := json.NewDecoder(file)
		var katSet KeccakKats
		err = dec.Decode(&katSet)
		if err != nil {
			t.Errorf("error decoding KATs: %s", err)
		}

//做KATS。
		for functionName, kats := range katSet.Kats {
			d := testDigests[functionName]()
			for _, kat := range kats {
				d.Reset()
				in, err := hex.DecodeString(kat.Message)
				if err != nil {
					t.Errorf("error decoding KAT: %s", err)
				}
				d.Write(in[:kat.Length/8])
				got := strings.ToUpper(hex.EncodeToString(d.Sum(nil)))
				if got != kat.Digest {
					t.Errorf("function=%s, implementation=%s, length=%d\nmessage:\n  %s\ngot:\n  %s\nwanted:\n %s",
						functionName, impl, kat.Length, kat.Message, got, kat.Digest)
					t.Logf("wanted %+v", kat)
					t.FailNow()
				}
				continue
			}
		}
	})
}

//TestUnAlignedWrite测试以任意模式写入数据
//小输入缓冲器。
func TestUnalignedWrite(t *testing.T) {
	testUnalignedAndGeneric(t, func(impl string) {
		buf := sequentialBytes(0x10000)
		for alg, df := range testDigests {
			d := df()
			d.Reset()
			d.Write(buf)
			want := d.Sum(nil)
			d.Reset()
			for i := 0; i < len(buf); {
//循环通过偏移，形成137字节序列。
//因为137是素数，所以这个序列应该练习所有的角情况。
				offsets := [17]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 1}
				for _, j := range offsets {
					if v := len(buf) - i; v < j {
						j = v
					}
					d.Write(buf[i : i+j])
					i += j
				}
			}
			got := d.Sum(nil)
			if !bytes.Equal(got, want) {
				t.Errorf("Unaligned writes, implementation=%s, alg=%s\ngot %q, want %q", impl, alg, got, want)
			}
		}
	})
}

//TestAppend checks that appending works when reallocation is necessary.
func TestAppend(t *testing.T) {
	testUnalignedAndGeneric(t, func(impl string) {
		d := New224()

		for capacity := 2; capacity <= 66; capacity += 64 {
//第一次循环时，sum必须重新分配。
//第二次，不会。
			buf := make([]byte, 2, capacity)
			d.Reset()
			d.Write([]byte{0xcc})
			buf = d.Sum(buf)
			expected := "0000DF70ADC49B2E76EEE3A6931B93FA41841C3AF2CDF5B32A18B5478C39"
			if got := strings.ToUpper(hex.EncodeToString(buf)); got != expected {
				t.Errorf("got %s, want %s", got, expected)
			}
		}
	})
}

//testappendnorealloc在不需要重新分配的情况下测试附加是否有效。
func TestAppendNoRealloc(t *testing.T) {
	testUnalignedAndGeneric(t, func(impl string) {
		buf := make([]byte, 1, 200)
		d := New224()
		d.Write([]byte{0xcc})
		buf = d.Sum(buf)
		expected := "00DF70ADC49B2E76EEE3A6931B93FA41841C3AF2CDF5B32A18B5478C39"
		if got := strings.ToUpper(hex.EncodeToString(buf)); got != expected {
			t.Errorf("%s: got %s, want %s", impl, got, expected)
		}
	})
}

//测试压缩检查压缩一次产生的全部输出
//与重复压缩实例的输出相同。
func TestSqueezing(t *testing.T) {
	testUnalignedAndGeneric(t, func(impl string) {
		for functionName, newShakeHash := range testShakes {
			d0 := newShakeHash()
			d0.Write([]byte(testString))
			ref := make([]byte, 32)
			d0.Read(ref)

			d1 := newShakeHash()
			d1.Write([]byte(testString))
			var multiple []byte
			for range ref {
				one := make([]byte, 1)
				d1.Read(one)
				multiple = append(multiple, one...)
			}
			if !bytes.Equal(ref, multiple) {
				t.Errorf("%s (%s): squeezing %d bytes one at a time failed", functionName, impl, len(ref))
			}
		}
	})
}

//sequentialBytes produces a buffer of size consecutive bytes 0x00, 0x01, ..., used for testing.
func sequentialBytes(size int) []byte {
	result := make([]byte, size)
	for i := range result {
		result[i] = byte(i)
	}
	return result
}

//基准置换函数测量置换函数的速度
//没有输入数据。
func BenchmarkPermutationFunction(b *testing.B) {
	b.SetBytes(int64(200))
	var lanes [25]uint64
	for i := 0; i < b.N; i++ {
		keccakF1600(&lanes)
	}
}

//BenchmarkHash测试每个buflen的哈希num缓冲区的速度。
func benchmarkHash(b *testing.B, h hash.Hash, size, num int) {
	b.StopTimer()
	h.Reset()
	data := sequentialBytes(size)
	b.SetBytes(int64(size * num))
	b.StartTimer()

	var state []byte
	for i := 0; i < b.N; i++ {
		for j := 0; j < num; j++ {
			h.Write(data)
		}
		state = h.Sum(state[:0])
	}
	b.StopTimer()
	h.Reset()
}

//BenchmarkShake专门针对那些不支持
//读取输出时需要副本。
func benchmarkShake(b *testing.B, h ShakeHash, size, num int) {
	b.StopTimer()
	h.Reset()
	data := sequentialBytes(size)
	d := make([]byte, 32)

	b.SetBytes(int64(size * num))
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		h.Reset()
		for j := 0; j < num; j++ {
			h.Write(data)
		}
		h.Read(d)
	}
}

func BenchmarkSha3_512_MTU(b *testing.B) { benchmarkHash(b, New512(), 1350, 1) }
func BenchmarkSha3_384_MTU(b *testing.B) { benchmarkHash(b, New384(), 1350, 1) }
func BenchmarkSha3_256_MTU(b *testing.B) { benchmarkHash(b, New256(), 1350, 1) }
func BenchmarkSha3_224_MTU(b *testing.B) { benchmarkHash(b, New224(), 1350, 1) }

func BenchmarkShake128_MTU(b *testing.B)  { benchmarkShake(b, NewShake128(), 1350, 1) }
func BenchmarkShake256_MTU(b *testing.B)  { benchmarkShake(b, NewShake256(), 1350, 1) }
func BenchmarkShake256_16x(b *testing.B)  { benchmarkShake(b, NewShake256(), 16, 1024) }
func BenchmarkShake256_1MiB(b *testing.B) { benchmarkShake(b, NewShake256(), 1024, 1024) }

func BenchmarkSha3_512_1MiB(b *testing.B) { benchmarkHash(b, New512(), 1024, 1024) }

func Example_sum() {
	buf := []byte("some data to hash")
//哈希需要64字节长才能具有256位的抗冲突性。
	h := make([]byte, 64)
//计算一个64字节的buf散列并将其放入h中。
	ShakeSum256(h, buf)
}

func Example_mac() {
	k := []byte("this is a secret key; you should generate a strong random key that's at least 32 bytes long")
	buf := []byte("and this is some data to authenticate")
//一个输出为32字节的MAC具有256位的安全性——如果您至少使用一个32字节长的密钥。
	h := make([]byte, 32)
	d := NewShake256()
//将密钥写入哈希。
	d.Write(k)
//现在写数据。
	d.Write(buf)
//从散列中读取32个字节的输出到h。
	d.Read(h)
}
