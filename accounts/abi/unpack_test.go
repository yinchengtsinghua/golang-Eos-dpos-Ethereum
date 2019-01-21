
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

package abi

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type unpackTest struct {
def  string      //ABI定义JSON
enc  string      //返回数据
want interface{} //预期产量
err  string      //如果需要，则为空或错误
}

func (test unpackTest) checkError(err error) error {
	if err != nil {
		if len(test.err) == 0 {
			return fmt.Errorf("expected no err but got: %v", err)
		} else if err.Error() != test.err {
			return fmt.Errorf("expected err: '%v' got err: %q", test.err, err)
		}
	} else if len(test.err) > 0 {
		return fmt.Errorf("expected err: %v but got none", test.err)
	}
	return nil
}

var unpackTests = []unpackTest{
	{
		def:  `[{ "type": "bool" }]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000001",
		want: true,
	},
	{
		def:  `[{ "type": "bool" }]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000000",
		want: false,
	},
	{
		def:  `[{ "type": "bool" }]`,
		enc:  "0000000000000000000000000000000000000000000000000001000000000001",
		want: false,
		err:  "abi: improperly encoded boolean value",
	},
	{
		def:  `[{ "type": "bool" }]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000003",
		want: false,
		err:  "abi: improperly encoded boolean value",
	},
	{
		def:  `[{"type": "uint32"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000001",
		want: uint32(1),
	},
	{
		def:  `[{"type": "uint32"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000001",
		want: uint16(0),
		err:  "abi: cannot unmarshal uint32 in to uint16",
	},
	{
		def:  `[{"type": "uint17"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000001",
		want: uint16(0),
		err:  "abi: cannot unmarshal *big.Int in to uint16",
	},
	{
		def:  `[{"type": "uint17"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000001",
		want: big.NewInt(1),
	},
	{
		def:  `[{"type": "int32"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000001",
		want: int32(1),
	},
	{
		def:  `[{"type": "int32"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000001",
		want: int16(0),
		err:  "abi: cannot unmarshal int32 in to int16",
	},
	{
		def:  `[{"type": "int17"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000001",
		want: int16(0),
		err:  "abi: cannot unmarshal *big.Int in to int16",
	},
	{
		def:  `[{"type": "int17"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000001",
		want: big.NewInt(1),
	},
	{
		def:  `[{"type": "address"}]`,
		enc:  "0000000000000000000000000100000000000000000000000000000000000000",
		want: common.Address{1},
	},
	{
		def:  `[{"type": "bytes32"}]`,
		enc:  "0100000000000000000000000000000000000000000000000000000000000000",
		want: [32]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	},
	{
		def:  `[{"type": "bytes"}]`,
		enc:  "000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000200100000000000000000000000000000000000000000000000000000000000000",
		want: common.Hex2Bytes("0100000000000000000000000000000000000000000000000000000000000000"),
	},
	{
		def:  `[{"type": "bytes"}]`,
		enc:  "000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000200100000000000000000000000000000000000000000000000000000000000000",
		want: [32]byte{},
		err:  "abi: cannot unmarshal []uint8 in to [32]uint8",
	},
	{
		def:  `[{"type": "bytes32"}]`,
		enc:  "000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000200100000000000000000000000000000000000000000000000000000000000000",
		want: []byte(nil),
		err:  "abi: cannot unmarshal [32]uint8 in to []uint8",
	},
	{
		def:  `[{"type": "bytes32"}]`,
		enc:  "0100000000000000000000000000000000000000000000000000000000000000",
		want: [32]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	},
	{
		def:  `[{"type": "function"}]`,
		enc:  "0100000000000000000000000000000000000000000000000000000000000000",
		want: [24]byte{1},
	},
//片
	{
		def:  `[{"type": "uint8[]"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: []uint8{1, 2},
	},
	{
		def:  `[{"type": "uint8[2]"}]`,
		enc:  "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: [2]uint8{1, 2},
	},
//多维，如果这些通过，则不需要长度前缀的所有类型都应通过
	{
		def:  `[{"type": "uint8[][]"}]`,
		enc:  "00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000E0000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: [][]uint8{{1, 2}, {1, 2}},
	},
	{
		def:  `[{"type": "uint8[2][2]"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: [2][2]uint8{{1, 2}, {1, 2}},
	},
	{
		def:  `[{"type": "uint8[][2]"}]`,
		enc:  "000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001",
		want: [2][]uint8{{1}, {1}},
	},
	{
		def:  `[{"type": "uint8[2][]"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: [][2]uint8{{1, 2}},
	},
	{
		def:  `[{"type": "uint16[]"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: []uint16{1, 2},
	},
	{
		def:  `[{"type": "uint16[2]"}]`,
		enc:  "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: [2]uint16{1, 2},
	},
	{
		def:  `[{"type": "uint32[]"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: []uint32{1, 2},
	},
	{
		def:  `[{"type": "uint32[2]"}]`,
		enc:  "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: [2]uint32{1, 2},
	},
	{
		def:  `[{"type": "uint32[2][3][4]"}]`,
		enc:  "000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000050000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000700000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000009000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000b000000000000000000000000000000000000000000000000000000000000000c000000000000000000000000000000000000000000000000000000000000000d000000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000000f000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000110000000000000000000000000000000000000000000000000000000000000012000000000000000000000000000000000000000000000000000000000000001300000000000000000000000000000000000000000000000000000000000000140000000000000000000000000000000000000000000000000000000000000015000000000000000000000000000000000000000000000000000000000000001600000000000000000000000000000000000000000000000000000000000000170000000000000000000000000000000000000000000000000000000000000018",
		want: [4][3][2]uint32{{{1, 2}, {3, 4}, {5, 6}}, {{7, 8}, {9, 10}, {11, 12}}, {{13, 14}, {15, 16}, {17, 18}}, {{19, 20}, {21, 22}, {23, 24}}},
	},
	{
		def:  `[{"type": "uint64[]"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: []uint64{1, 2},
	},
	{
		def:  `[{"type": "uint64[2]"}]`,
		enc:  "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: [2]uint64{1, 2},
	},
	{
		def:  `[{"type": "uint256[]"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: []*big.Int{big.NewInt(1), big.NewInt(2)},
	},
	{
		def:  `[{"type": "uint256[3]"}]`,
		enc:  "000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
		want: [3]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
	},
	{
		def:  `[{"type": "int8[]"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: []int8{1, 2},
	},
	{
		def:  `[{"type": "int8[2]"}]`,
		enc:  "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: [2]int8{1, 2},
	},
	{
		def:  `[{"type": "int16[]"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: []int16{1, 2},
	},
	{
		def:  `[{"type": "int16[2]"}]`,
		enc:  "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: [2]int16{1, 2},
	},
	{
		def:  `[{"type": "int32[]"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: []int32{1, 2},
	},
	{
		def:  `[{"type": "int32[2]"}]`,
		enc:  "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: [2]int32{1, 2},
	},
	{
		def:  `[{"type": "int64[]"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: []int64{1, 2},
	},
	{
		def:  `[{"type": "int64[2]"}]`,
		enc:  "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: [2]int64{1, 2},
	},
	{
		def:  `[{"type": "int256[]"}]`,
		enc:  "0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: []*big.Int{big.NewInt(1), big.NewInt(2)},
	},
	{
		def:  `[{"type": "int256[3]"}]`,
		enc:  "000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
		want: [3]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
	},
//结构输出
	{
		def: `[{"name":"int1","type":"int256"},{"name":"int2","type":"int256"}]`,
		enc: "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			Int1 *big.Int
			Int2 *big.Int
		}{big.NewInt(1), big.NewInt(2)},
	},
	{
		def: `[{"name":"int","type":"int256"},{"name":"Int","type":"int256"}]`,
		enc: "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			Int1 *big.Int
			Int2 *big.Int
		}{},
		err: "abi: multiple outputs mapping to the same struct field 'Int'",
	},
	{
		def: `[{"name":"int","type":"int256"},{"name":"_int","type":"int256"}]`,
		enc: "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			Int1 *big.Int
			Int2 *big.Int
		}{},
		err: "abi: multiple outputs mapping to the same struct field 'Int'",
	},
	{
		def: `[{"name":"Int","type":"int256"},{"name":"_int","type":"int256"}]`,
		enc: "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			Int1 *big.Int
			Int2 *big.Int
		}{},
		err: "abi: multiple outputs mapping to the same struct field 'Int'",
	},
	{
		def: `[{"name":"Int","type":"int256"},{"name":"_","type":"int256"}]`,
		enc: "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			Int1 *big.Int
			Int2 *big.Int
		}{},
		err: "abi: purely underscored output cannot unpack to struct",
	},
}

func TestUnpack(t *testing.T) {
	for i, test := range unpackTests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			def := fmt.Sprintf(`[{ "name" : "method", "outputs": %s}]`, test.def)
			abi, err := JSON(strings.NewReader(def))
			if err != nil {
				t.Fatalf("invalid ABI definition %s: %v", def, err)
			}
			encb, err := hex.DecodeString(test.enc)
			if err != nil {
				t.Fatalf("invalid hex: %s" + test.enc)
			}
			outptr := reflect.New(reflect.TypeOf(test.want))
			err = abi.Unpack(outptr.Interface(), "method", encb)
			if err := test.checkError(err); err != nil {
				t.Errorf("test %d (%v) failed: %v", i, test.def, err)
				return
			}
			out := outptr.Elem().Interface()
			if !reflect.DeepEqual(test.want, out) {
				t.Errorf("test %d (%v) failed: expected %v, got %v", i, test.def, test.want, out)
			}
		})
	}
}

type methodMultiOutput struct {
	Int    *big.Int
	String string
}

func methodMultiReturn(require *require.Assertions) (ABI, []byte, methodMultiOutput) {
	const definition = `[
	{ "name" : "multi", "constant" : false, "outputs": [ { "name": "Int", "type": "uint256" }, { "name": "String", "type": "string" } ] }]`
	var expected = methodMultiOutput{big.NewInt(1), "hello"}

	abi, err := JSON(strings.NewReader(definition))
	require.NoError(err)
//使用buff使代码可读
	buff := new(bytes.Buffer)
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000001"))
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000040"))
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000005"))
	buff.Write(common.RightPadBytes([]byte(expected.String), 32))
	return abi, buff.Bytes(), expected
}

func TestMethodMultiReturn(t *testing.T) {
	type reversed struct {
		String string
		Int    *big.Int
	}

	abi, data, expected := methodMultiReturn(require.New(t))
	bigint := new(big.Int)
	var testCases = []struct {
		dest     interface{}
		expected interface{}
		error    string
		name     string
	}{{
		&methodMultiOutput{},
		&expected,
		"",
		"Can unpack into structure",
	}, {
		&reversed{},
		&reversed{expected.String, expected.Int},
		"",
		"Can unpack into reversed structure",
	}, {
		&[]interface{}{&bigint, new(string)},
		&[]interface{}{&expected.Int, &expected.String},
		"",
		"Can unpack into a slice",
	}, {
		&[2]interface{}{&bigint, new(string)},
		&[2]interface{}{&expected.Int, &expected.String},
		"",
		"Can unpack into an array",
	}, {
		&[]interface{}{new(int), new(int)},
		&[]interface{}{&expected.Int, &expected.String},
		"abi: cannot unmarshal *big.Int in to int",
		"Can not unpack into a slice with wrong types",
	}, {
		&[]interface{}{new(int)},
		&[]interface{}{},
		"abi: insufficient number of elements in the list/array for unpack, want 2, got 1",
		"Can not unpack into a slice with wrong types",
	}}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)
			err := abi.Unpack(tc.dest, "multi", data)
			if tc.error == "" {
				require.Nil(err, "Should be able to unpack method outputs.")
				require.Equal(tc.expected, tc.dest)
			} else {
				require.EqualError(err, tc.error)
			}
		})
	}
}

func TestMultiReturnWithArray(t *testing.T) {
	const definition = `[{"name" : "multi", "outputs": [{"type": "uint64[3]"}, {"type": "uint64"}]}]`
	abi, err := JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}
	buff := new(bytes.Buffer)
	buff.Write(common.Hex2Bytes("000000000000000000000000000000000000000000000000000000000000000900000000000000000000000000000000000000000000000000000000000000090000000000000000000000000000000000000000000000000000000000000009"))
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000008"))

	ret1, ret1Exp := new([3]uint64), [3]uint64{9, 9, 9}
	ret2, ret2Exp := new(uint64), uint64(8)
	if err := abi.Unpack(&[]interface{}{ret1, ret2}, "multi", buff.Bytes()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*ret1, ret1Exp) {
		t.Error("array result", *ret1, "!= Expected", ret1Exp)
	}
	if *ret2 != ret2Exp {
		t.Error("int result", *ret2, "!= Expected", ret2Exp)
	}
}

func TestMultiReturnWithDeeplyNestedArray(t *testing.T) {
//类似于TestMultiReturnWithArray，但考虑到特殊情况：
//嵌套静态数组的值也将计入大小，以及后面的任何元素
//在这种嵌套数组参数之后，应该用正确的偏移量读取，
//这样它就不会从前面的数组参数中读取内容。
	const definition = `[{"name" : "multi", "outputs": [{"type": "uint64[3][2][4]"}, {"type": "uint64"}]}]`
	abi, err := JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}
	buff := new(bytes.Buffer)
//构造测试数组，每个3个char元素与61个“0”chars连接，
//从（（3+61）*0.5）=32字节的数组元素。
	buff.Write(common.Hex2Bytes(strings.Join([]string{
"", //空，以便将61个字符分隔符也应用于第一个元素。
		"111", "112", "113", "121", "122", "123",
		"211", "212", "213", "221", "222", "223",
		"311", "312", "313", "321", "322", "323",
		"411", "412", "413", "421", "422", "423",
	}, "0000000000000000000000000000000000000000000000000000000000000")))
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000009876"))

	ret1, ret1Exp := new([4][2][3]uint64), [4][2][3]uint64{
		{{0x111, 0x112, 0x113}, {0x121, 0x122, 0x123}},
		{{0x211, 0x212, 0x213}, {0x221, 0x222, 0x223}},
		{{0x311, 0x312, 0x313}, {0x321, 0x322, 0x323}},
		{{0x411, 0x412, 0x413}, {0x421, 0x422, 0x423}},
	}
	ret2, ret2Exp := new(uint64), uint64(0x9876)
	if err := abi.Unpack(&[]interface{}{ret1, ret2}, "multi", buff.Bytes()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*ret1, ret1Exp) {
		t.Error("array result", *ret1, "!= Expected", ret1Exp)
	}
	if *ret2 != ret2Exp {
		t.Error("int result", *ret2, "!= Expected", ret2Exp)
	}
}

func TestUnmarshal(t *testing.T) {
	const definition = `[
	{ "name" : "int", "constant" : false, "outputs": [ { "type": "uint256" } ] },
	{ "name" : "bool", "constant" : false, "outputs": [ { "type": "bool" } ] },
	{ "name" : "bytes", "constant" : false, "outputs": [ { "type": "bytes" } ] },
	{ "name" : "fixed", "constant" : false, "outputs": [ { "type": "bytes32" } ] },
	{ "name" : "multi", "constant" : false, "outputs": [ { "type": "bytes" }, { "type": "bytes" } ] },
	{ "name" : "intArraySingle", "constant" : false, "outputs": [ { "type": "uint256[3]" } ] },
	{ "name" : "addressSliceSingle", "constant" : false, "outputs": [ { "type": "address[]" } ] },
	{ "name" : "addressSliceDouble", "constant" : false, "outputs": [ { "name": "a", "type": "address[]" }, { "name": "b", "type": "address[]" } ] },
	{ "name" : "mixedBytes", "constant" : true, "outputs": [ { "name": "a", "type": "bytes" }, { "name": "b", "type": "bytes32" } ] }]`

	abi, err := JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}
	buff := new(bytes.Buffer)

//Marshall混合字节（MixedBytes）
	p0, p0Exp := []byte{}, common.Hex2Bytes("01020000000000000000")
	p1, p1Exp := [32]byte{}, common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000ddeeff")
	mixedBytes := []interface{}{&p0, &p1}

	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000040"))
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000ddeeff"))
	buff.Write(common.Hex2Bytes("000000000000000000000000000000000000000000000000000000000000000a"))
	buff.Write(common.Hex2Bytes("0102000000000000000000000000000000000000000000000000000000000000"))

	err = abi.Unpack(&mixedBytes, "mixedBytes", buff.Bytes())
	if err != nil {
		t.Error(err)
	} else {
		if !bytes.Equal(p0, p0Exp) {
			t.Errorf("unexpected value unpacked: want %x, got %x", p0Exp, p0)
		}

		if !bytes.Equal(p1[:], p1Exp) {
			t.Errorf("unexpected value unpacked: want %x, got %x", p1Exp, p1)
		}
	}

//元帅INT
	var Int *big.Int
	err = abi.Unpack(&Int, "int", common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000001"))
	if err != nil {
		t.Error(err)
	}

	if Int == nil || Int.Cmp(big.NewInt(1)) != 0 {
		t.Error("expected Int to be 1 got", Int)
	}

//元帅
	var Bool bool
	err = abi.Unpack(&Bool, "bool", common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000001"))
	if err != nil {
		t.Error(err)
	}

	if !Bool {
		t.Error("expected Bool to be true")
	}

//封送动态字节最大长度32
	buff.Reset()
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000020"))
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000020"))
	bytesOut := common.RightPadBytes([]byte("hello"), 32)
	buff.Write(bytesOut)

	var Bytes []byte
	err = abi.Unpack(&Bytes, "bytes", buff.Bytes())
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(Bytes, bytesOut) {
		t.Errorf("expected %x got %x", bytesOut, Bytes)
	}

//马歇尔动态字节最大长度64
	buff.Reset()
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000020"))
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000040"))
	bytesOut = common.RightPadBytes([]byte("hello"), 64)
	buff.Write(bytesOut)

	err = abi.Unpack(&Bytes, "bytes", buff.Bytes())
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(Bytes, bytesOut) {
		t.Errorf("expected %x got %x", bytesOut, Bytes)
	}

//马歇尔动态字节最大长度64
	buff.Reset()
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000020"))
	buff.Write(common.Hex2Bytes("000000000000000000000000000000000000000000000000000000000000003f"))
	bytesOut = common.RightPadBytes([]byte("hello"), 64)
	buff.Write(bytesOut)

	err = abi.Unpack(&Bytes, "bytes", buff.Bytes())
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(Bytes, bytesOut[:len(bytesOut)-1]) {
		t.Errorf("expected %x got %x", bytesOut[:len(bytesOut)-1], Bytes)
	}

//封送动态字节输出为空
	err = abi.Unpack(&Bytes, "bytes", nil)
	if err == nil {
		t.Error("expected error")
	}

//封送动态字节长度5
	buff.Reset()
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000020"))
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000005"))
	buff.Write(common.RightPadBytes([]byte("hello"), 32))

	err = abi.Unpack(&Bytes, "bytes", buff.Bytes())
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(Bytes, []byte("hello")) {
		t.Errorf("expected %x got %x", bytesOut, Bytes)
	}

//封送动态字节长度5
	buff.Reset()
	buff.Write(common.RightPadBytes([]byte("hello"), 32))

	var hash common.Hash
	err = abi.Unpack(&hash, "fixed", buff.Bytes())
	if err != nil {
		t.Error(err)
	}

	helloHash := common.BytesToHash(common.RightPadBytes([]byte("hello"), 32))
	if hash != helloHash {
		t.Errorf("Expected %x to equal %x", hash, helloHash)
	}

//元帅错误
	buff.Reset()
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000020"))
	err = abi.Unpack(&Bytes, "bytes", buff.Bytes())
	if err == nil {
		t.Error("expected error")
	}

	err = abi.Unpack(&Bytes, "multi", make([]byte, 64))
	if err == nil {
		t.Error("expected error")
	}

	buff.Reset()
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000001"))
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000002"))
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000003"))
//marshal int数组
	var intArray [3]*big.Int
	err = abi.Unpack(&intArray, "intArraySingle", buff.Bytes())
	if err != nil {
		t.Error(err)
	}
	var testAgainstIntArray [3]*big.Int
	testAgainstIntArray[0] = big.NewInt(1)
	testAgainstIntArray[1] = big.NewInt(2)
	testAgainstIntArray[2] = big.NewInt(3)

	for i, Int := range intArray {
		if Int.Cmp(testAgainstIntArray[i]) != 0 {
			t.Errorf("expected %v, got %v", testAgainstIntArray[i], Int)
		}
	}
//封送地址切片
	buff.Reset()
buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000020")) //抵消
buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000001")) //大小
	buff.Write(common.Hex2Bytes("0000000000000000000000000100000000000000000000000000000000000000"))

	var outAddr []common.Address
	err = abi.Unpack(&outAddr, "addressSliceSingle", buff.Bytes())
	if err != nil {
		t.Fatal("didn't expect error:", err)
	}

	if len(outAddr) != 1 {
		t.Fatal("expected 1 item, got", len(outAddr))
	}

	if outAddr[0] != (common.Address{1}) {
		t.Errorf("expected %x, got %x", common.Address{1}, outAddr[0])
	}

//封送多个地址片
	buff.Reset()
buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000040")) //抵消
buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000080")) //抵消
buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000001")) //大小
	buff.Write(common.Hex2Bytes("0000000000000000000000000100000000000000000000000000000000000000"))
buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000002")) //大小
	buff.Write(common.Hex2Bytes("0000000000000000000000000200000000000000000000000000000000000000"))
	buff.Write(common.Hex2Bytes("0000000000000000000000000300000000000000000000000000000000000000"))

	var outAddrStruct struct {
		A []common.Address
		B []common.Address
	}
	err = abi.Unpack(&outAddrStruct, "addressSliceDouble", buff.Bytes())
	if err != nil {
		t.Fatal("didn't expect error:", err)
	}

	if len(outAddrStruct.A) != 1 {
		t.Fatal("expected 1 item, got", len(outAddrStruct.A))
	}

	if outAddrStruct.A[0] != (common.Address{1}) {
		t.Errorf("expected %x, got %x", common.Address{1}, outAddrStruct.A[0])
	}

	if len(outAddrStruct.B) != 2 {
		t.Fatal("expected 1 item, got", len(outAddrStruct.B))
	}

	if outAddrStruct.B[0] != (common.Address{2}) {
		t.Errorf("expected %x, got %x", common.Address{2}, outAddrStruct.B[0])
	}
	if outAddrStruct.B[1] != (common.Address{3}) {
		t.Errorf("expected %x, got %x", common.Address{3}, outAddrStruct.B[1])
	}

//封送无效的地址切片
	buff.Reset()
	buff.Write(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000100"))

	err = abi.Unpack(&outAddr, "addressSliceSingle", buff.Bytes())
	if err == nil {
		t.Fatal("expected error:", err)
	}
}

func TestOOMMaliciousInput(t *testing.T) {
	oomTests := []unpackTest{
		{
			def: `[{"type": "uint8[]"}]`,
enc: "0000000000000000000000000000000000000000000000000000000000000020" + //抵消
"0000000000000000000000000000000000000000000000000000000000000003" + //努姆埃勒姆
"0000000000000000000000000000000000000000000000000000000000000001" + //ELEM 1
"0000000000000000000000000000000000000000000000000000000000000002", //ELEM 2
		},
{ //长度大于64位
			def: `[{"type": "uint8[]"}]`,
enc: "0000000000000000000000000000000000000000000000000000000000000020" + //抵消
"00ffffffffffffffffffffffffffffffffffffffffffffff0000000000000002" + //努姆埃勒姆
"0000000000000000000000000000000000000000000000000000000000000001" + //ELEM 1
"0000000000000000000000000000000000000000000000000000000000000002", //ELEM 2
		},
{ //偏移量非常大（超过64位）
			def: `[{"type": "uint8[]"}]`,
enc: "00ffffffffffffffffffffffffffffffffffffffffffffff0000000000000020" + //抵消
"0000000000000000000000000000000000000000000000000000000000000002" + //努姆埃勒姆
"0000000000000000000000000000000000000000000000000000000000000001" + //ELEM 1
"0000000000000000000000000000000000000000000000000000000000000002", //ELEM 2
		},
{ //偏移量非常大（低于64位）
			def: `[{"type": "uint8[]"}]`,
enc: "0000000000000000000000000000000000000000000000007ffffffffff00020" + //抵消
"0000000000000000000000000000000000000000000000000000000000000002" + //努姆埃勒姆
"0000000000000000000000000000000000000000000000000000000000000001" + //ELEM 1
"0000000000000000000000000000000000000000000000000000000000000002", //ELEM 2
		},
{ //负偏移量（64位）
			def: `[{"type": "uint8[]"}]`,
enc: "000000000000000000000000000000000000000000000000f000000000000020" + //抵消
"0000000000000000000000000000000000000000000000000000000000000002" + //努姆埃勒姆
"0000000000000000000000000000000000000000000000000000000000000001" + //ELEM 1
"0000000000000000000000000000000000000000000000000000000000000002", //ELEM 2
		},

{ //负长度
			def: `[{"type": "uint8[]"}]`,
enc: "0000000000000000000000000000000000000000000000000000000000000020" + //抵消
"000000000000000000000000000000000000000000000000f000000000000002" + //努姆埃勒姆
"0000000000000000000000000000000000000000000000000000000000000001" + //ELEM 1
"0000000000000000000000000000000000000000000000000000000000000002", //ELEM 2
		},
{ //非常大的长度
			def: `[{"type": "uint8[]"}]`,
enc: "0000000000000000000000000000000000000000000000000000000000000020" + //抵消
"0000000000000000000000000000000000000000000000007fffffffff000002" + //努姆埃勒姆
"0000000000000000000000000000000000000000000000000000000000000001" + //ELEM 1
"0000000000000000000000000000000000000000000000000000000000000002", //ELEM 2
		},
	}
	for i, test := range oomTests {
		def := fmt.Sprintf(`[{ "name" : "method", "outputs": %s}]`, test.def)
		abi, err := JSON(strings.NewReader(def))
		if err != nil {
			t.Fatalf("invalid ABI definition %s: %v", def, err)
		}
		encb, err := hex.DecodeString(test.enc)
		if err != nil {
			t.Fatalf("invalid hex: %s" + test.enc)
		}
		_, err = abi.Methods["method"].Outputs.UnpackValues(encb)
		if err == nil {
			t.Fatalf("Expected error on malicious input, test %d", i)
		}
	}
}
