
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

package multihash

import (
	"bytes"
	"math/rand"
	"testing"
)

//
func TestCheckMultihash(t *testing.T) {
	hashbytes := make([]byte, 32)
	c, err := rand.Read(hashbytes)
	if err != nil {
		t.Fatal(err)
	} else if c < 32 {
		t.Fatal("short read")
	}

	expected := ToMultihash(hashbytes)

	l, hl, _ := GetMultihashLength(expected)
	if l != 32 {
		t.Fatalf("expected length %d, got %d", 32, l)
	} else if hl != 2 {
		t.Fatalf("expected header length %d, got %d", 2, hl)
	}
	if _, _, err := GetMultihashLength(expected[1:]); err == nil {
		t.Fatal("expected failure on corrupt header")
	}
	if _, _, err := GetMultihashLength(expected[:len(expected)-2]); err == nil {
		t.Fatal("expected failure on short content")
	}
	dh, _ := FromMultihash(expected)
	if !bytes.Equal(dh, hashbytes) {
		t.Fatalf("expected content hash %x, got %x", hashbytes, dh)
	}
}
