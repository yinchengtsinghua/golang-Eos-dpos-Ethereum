
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

package api

import (
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestConfig(t *testing.T) {

	var hexprvkey = "65138b2aa745041b372153550584587da326ab440576b2a1191dd95cee30039c"

	prvkey, err := crypto.HexToECDSA(hexprvkey)
	if err != nil {
		t.Fatalf("failed to load private key: %v", err)
	}

	one := NewConfig()
	two := NewConfig()

	one.LocalStoreParams = two.LocalStoreParams
	if equal := reflect.DeepEqual(one, two); !equal {
		t.Fatal("Two default configs are not equal")
	}

	one.Init(prvkey)

//
	if one.BzzKey == "" {
		t.Fatal("Expected BzzKey to be set")
	}
	if one.PublicKey == "" {
		t.Fatal("Expected PublicKey to be set")
	}
	if one.Swap.PayProfile.Beneficiary == (common.Address{}) && one.SwapEnabled {
		t.Fatal("Failed to correctly initialize SwapParams")
	}
	if one.ChunkDbPath == one.Path {
		t.Fatal("Failed to correctly initialize StoreParams")
	}
}
