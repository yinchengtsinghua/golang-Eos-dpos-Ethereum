
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Go Ethereum作者
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
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

//类型枚举器
const (
	IntTy byte = iota
	UintTy
	BoolTy
	StringTy
	SliceTy
	ArrayTy
	AddressTy
	FixedBytesTy
	BytesTy
	HashTy
	FixedPointTy
	FunctionTy
)

//类型是受支持的参数类型的反射
type Type struct {
	Elem *Type

	Kind reflect.Kind
	Type reflect.Type
	Size int
T    byte //我们自己的类型检查

stringKind string //保留用于派生签名的未分析字符串
}

var (
//typeregex解析ABI子类型
	typeRegex = regexp.MustCompile("([a-zA-Z]+)(([0-9]+)(x([0-9]+))?)?")
)

//new type创建在t中给定的abi类型的新反射类型。
func NewType(t string) (typ Type, err error) {
//检查数组括号是否相等（如果存在）
	if strings.Count(t, "[") != strings.Count(t, "]") {
		return Type{}, fmt.Errorf("invalid arg type in abi")
	}

	typ.stringKind = t

//如果有括号，准备进入切片/阵列模式
//递归创建类型
	if strings.Count(t, "[") != 0 {
		i := strings.LastIndex(t, "[")
//递归嵌入类型
		embeddedType, err := NewType(t[:i])
		if err != nil {
			return Type{}, err
		}
//抓取最后一个单元格并从中创建类型
		sliced := t[i:]
//用regexp获取切片大小
		re := regexp.MustCompile("[0-9]+")
		intz := re.FindAllString(sliced, -1)

		if len(intz) == 0 {
//是一片
			typ.T = SliceTy
			typ.Kind = reflect.Slice
			typ.Elem = &embeddedType
			typ.Type = reflect.SliceOf(embeddedType.Type)
		} else if len(intz) == 1 {
//是一个数组
			typ.T = ArrayTy
			typ.Kind = reflect.Array
			typ.Elem = &embeddedType
			typ.Size, err = strconv.Atoi(intz[0])
			if err != nil {
				return Type{}, fmt.Errorf("abi: error parsing variable size: %v", err)
			}
			typ.Type = reflect.ArrayOf(typ.Size, embeddedType.Type)
		} else {
			return Type{}, fmt.Errorf("invalid formatting of array type")
		}
		return typ, err
	}
//分析ABI类型的类型和大小。
	parsedType := typeRegex.FindAllStringSubmatch(t, -1)[0]
//varsize是变量的大小
	var varSize int
	if len(parsedType[3]) > 0 {
		var err error
		varSize, err = strconv.Atoi(parsedType[2])
		if err != nil {
			return Type{}, fmt.Errorf("abi: error parsing variable size: %v", err)
		}
	} else {
		if parsedType[0] == "uint" || parsedType[0] == "int" {
//这应该失败，因为这意味着
//ABI类型（编译器应始终将其格式化为大小…始终）
			return Type{}, fmt.Errorf("unsupported arg type: %s", t)
		}
	}
//vartype是解析的abi类型
	switch varType := parsedType[1]; varType {
	case "int":
		typ.Kind, typ.Type = reflectIntKindAndType(false, varSize)
		typ.Size = varSize
		typ.T = IntTy
	case "uint":
		typ.Kind, typ.Type = reflectIntKindAndType(true, varSize)
		typ.Size = varSize
		typ.T = UintTy
	case "bool":
		typ.Kind = reflect.Bool
		typ.T = BoolTy
		typ.Type = reflect.TypeOf(bool(false))
	case "address":
		typ.Kind = reflect.Array
		typ.Type = addressT
		typ.Size = 20
		typ.T = AddressTy
	case "string":
		typ.Kind = reflect.String
		typ.Type = reflect.TypeOf("")
		typ.T = StringTy
	case "bytes":
		if varSize == 0 {
			typ.T = BytesTy
			typ.Kind = reflect.Slice
			typ.Type = reflect.SliceOf(reflect.TypeOf(byte(0)))
		} else {
			typ.T = FixedBytesTy
			typ.Kind = reflect.Array
			typ.Size = varSize
			typ.Type = reflect.ArrayOf(varSize, reflect.TypeOf(byte(0)))
		}
	case "function":
		typ.Kind = reflect.Array
		typ.T = FunctionTy
		typ.Size = 24
		typ.Type = reflect.ArrayOf(24, reflect.TypeOf(byte(0)))
	default:
		return Type{}, fmt.Errorf("unsupported arg type: %s", t)
	}

	return
}

//字符串实现字符串
func (t Type) String() (out string) {
	return t.stringKind
}

func (t Type) pack(v reflect.Value) ([]byte, error) {
//如果指针是指针，则先取消引用指针
	v = indirect(v)

	if err := typeCheck(t, v); err != nil {
		return nil, err
	}

	if t.T == SliceTy || t.T == ArrayTy {
		var packed []byte

		for i := 0; i < v.Len(); i++ {
			val, err := t.Elem.pack(v.Index(i))
			if err != nil {
				return nil, err
			}
			packed = append(packed, val...)
		}
		if t.T == SliceTy {
			return packBytesSlice(packed, v.Len()), nil
		} else if t.T == ArrayTy {
			return packed, nil
		}
	}
	return packElement(t, v), nil
}

//RequireLengthPrefix返回类型是否需要任何长度
//前缀。
func (t Type) requiresLengthPrefix() bool {
	return t.T == StringTy || t.T == BytesTy || t.T == SliceTy
}
