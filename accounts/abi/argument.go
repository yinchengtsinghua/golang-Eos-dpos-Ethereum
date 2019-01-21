
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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

//参数包含参数的名称和相应的类型。
//类型在打包和测试参数时使用。
type Argument struct {
	Name    string
	Type    Type
Indexed bool //索引仅由事件使用
}

type Arguments []Argument

//unmashaljson实现json.unmasheler接口
func (argument *Argument) UnmarshalJSON(data []byte) error {
	var extarg struct {
		Name    string
		Type    string
		Indexed bool
	}
	err := json.Unmarshal(data, &extarg)
	if err != nil {
		return fmt.Errorf("argument json err: %v", err)
	}

	argument.Type, err = NewType(extarg.Type)
	if err != nil {
		return err
	}
	argument.Name = extarg.Name
	argument.Indexed = extarg.Indexed

	return nil
}

//lengthnonindexed返回不计算“indexed”参数时的参数数。只有事件
//不能有“indexed”参数，方法输入/输出的参数应始终为false
func (arguments Arguments) LengthNonIndexed() int {
	out := 0
	for _, arg := range arguments {
		if !arg.Indexed {
			out++
		}
	}
	return out
}

//无索引返回已筛选出索引参数的参数
func (arguments Arguments) NonIndexed() Arguments {
	var ret []Argument
	for _, arg := range arguments {
		if !arg.Indexed {
			ret = append(ret, arg)
		}
	}
	return ret
}

//Istuple为非原子结构返回true，如（uint、uint）或uint[]
func (arguments Arguments) isTuple() bool {
	return len(arguments) > 1
}

//unpack执行hexdata->go-format操作
func (arguments Arguments) Unpack(v interface{}, data []byte) error {

//确保传递的值是参数指针
	if reflect.Ptr != reflect.ValueOf(v).Kind() {
		return fmt.Errorf("abi: Unpack(non-pointer %T)", v)
	}
	marshalledValues, err := arguments.UnpackValues(data)
	if err != nil {
		return err
	}
	if arguments.isTuple() {
		return arguments.unpackTuple(v, marshalledValues)
	}
	return arguments.unpackAtomic(v, marshalledValues)
}

func (arguments Arguments) unpackTuple(v interface{}, marshalledValues []interface{}) error {

	var (
		value = reflect.ValueOf(v).Elem()
		typ   = value.Type()
		kind  = value.Kind()
	)

	if err := requireUnpackKind(value, typ, kind, arguments); err != nil {
		return err
	}

//如果接口是结构，则获取abi->struct_字段映射

	var abi2struct map[string]string
	if kind == reflect.Struct {
		var err error
		abi2struct, err = mapAbiToStructFields(arguments, value)
		if err != nil {
			return err
		}
	}
	for i, arg := range arguments.NonIndexed() {

		reflectValue := reflect.ValueOf(marshalledValues[i])

		switch kind {
		case reflect.Struct:
			if structField, ok := abi2struct[arg.Name]; ok {
				if err := set(value.FieldByName(structField), reflectValue, arg); err != nil {
					return err
				}
			}
		case reflect.Slice, reflect.Array:
			if value.Len() < i {
				return fmt.Errorf("abi: insufficient number of arguments for unpack, want %d, got %d", len(arguments), value.Len())
			}
			v := value.Index(i)
			if err := requireAssignable(v, reflectValue); err != nil {
				return err
			}

			if err := set(v.Elem(), reflectValue, arg); err != nil {
				return err
			}
		default:
			return fmt.Errorf("abi:[2] cannot unmarshal tuple in to %v", typ)
		}
	}
	return nil
}

//解包原子解包（hexdata->go）单个值
func (arguments Arguments) unpackAtomic(v interface{}, marshalledValues []interface{}) error {
	if len(marshalledValues) != 1 {
		return fmt.Errorf("abi: wrong length, expected single value, got %d", len(marshalledValues))
	}

	elem := reflect.ValueOf(v).Elem()
	kind := elem.Kind()
	reflectValue := reflect.ValueOf(marshalledValues[0])

	var abi2struct map[string]string
	if kind == reflect.Struct {
		var err error
		if abi2struct, err = mapAbiToStructFields(arguments, elem); err != nil {
			return err
		}
		arg := arguments.NonIndexed()[0]
		if structField, ok := abi2struct[arg.Name]; ok {
			return set(elem.FieldByName(structField), reflectValue, arg)
		}
		return nil
	}

	return set(elem, reflectValue, arguments.NonIndexed()[0])

}

//计算数组的完整大小；
//也就是说，对嵌套数组进行计数，这些数组将计入解包的大小。
func getArraySize(arr *Type) int {
	size := arr.Size
//数组可以嵌套，每个元素的大小相同
	arr = arr.Elem
	for arr.T == ArrayTy {
//当elem是数组时，继续乘以elem.size。
		size *= arr.Size
		arr = arr.Elem
	}
//现在我们有了完整的数组大小，包括它的子数组。
	return size
}

//解包值可用于根据ABI规范解包ABI编码的十六进制数据。
//不提供要解包的结构。相反，此方法返回一个包含
//价值观。原子参数将是一个包含一个元素的列表。
func (arguments Arguments) UnpackValues(data []byte) ([]interface{}, error) {
	retval := make([]interface{}, 0, arguments.LengthNonIndexed())
	virtualArgs := 0
	for index, arg := range arguments.NonIndexed() {
		marshalledValue, err := toGoType((index+virtualArgs)*32, arg.Type, data)
		if arg.Type.T == ArrayTy {
//如果我们有一个静态数组，比如[3]uint256，那么这些数组被编码为
//就像uint256、uint256、uint256一样。
//这意味着当
//我们从现在开始计算索引。
//
//嵌套多层深度的数组值也以内联方式编码：
//[2][3]uint256:uint256、uint256、uint256、uint256、uint256、uint256、uint256
//
//计算完整的数组大小以获得下一个参数的正确偏移量。
//将其减小1，因为仍应用正常索引增量。
			virtualArgs += getArraySize(&arg.Type) - 1
		}
		if err != nil {
			return nil, err
		}
		retval = append(retval, marshalledValue)
	}
	return retval, nil
}

//packvalues执行操作go format->hexdata
//它在语义上与unpackvalues相反
func (arguments Arguments) PackValues(args []interface{}) ([]byte, error) {
	return arguments.Pack(args...)
}

//pack执行操作go format->hexdata
func (arguments Arguments) Pack(args ...interface{}) ([]byte, error) {
//确保参数匹配并打包
	abiArgs := arguments
	if len(args) != len(abiArgs) {
		return nil, fmt.Errorf("argument count mismatch: %d for %d", len(args), len(abiArgs))
	}
//变量输入是在压缩结束时附加的输出
//输出。这用于字符串和字节类型输入。
	var variableInput []byte

//输入偏移量是压缩输出的字节偏移量
	inputOffset := 0
	for _, abiArg := range abiArgs {
		if abiArg.Type.T == ArrayTy {
			inputOffset += 32 * abiArg.Type.Size
		} else {
			inputOffset += 32
		}
	}
	var ret []byte
	for i, a := range args {
		input := abiArgs[i]
//打包输入
		packed, err := input.Type.pack(reflect.ValueOf(a))
		if err != nil {
			return nil, err
		}
//检查切片类型（字符串、字节、切片）
		if input.Type.requiresLengthPrefix() {
//计算偏移量
			offset := inputOffset + len(variableInput)
//设置偏移量
			ret = append(ret, packNum(reflect.ValueOf(offset))...)
//将打包输出附加到变量输入。变量输入
//将附加在输入的末尾。
			variableInput = append(variableInput, packed...)
		} else {
//将压缩值追加到输入
			ret = append(ret, packed...)
		}
	}
//在压缩输入的末尾附加变量输入
	ret = append(ret, variableInput...)

	return ret, nil
}

//大写将字符串的第一个字符设为大写，同时删除
//在变量名中添加下划线前缀。
func capitalise(input string) string {
	for len(input) > 0 && input[0] == '_' {
		input = input[1:]
	}
	if len(input) == 0 {
		return ""
	}
	return strings.ToUpper(input[:1]) + input[1:]
}
