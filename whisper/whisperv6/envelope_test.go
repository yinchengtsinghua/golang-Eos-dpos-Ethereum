
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

//

package whisperv6

import (
	mrand "math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestEnvelopeOpenAcceptsOnlyOneKeyTypeInFilter(t *testing.T) {
	symKey := make([]byte, aesKeyLength)
	mrand.Read(symKey)

	asymKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed GenerateKey with seed %d: %s.", seed, err)
	}

	params := MessageParams{
		PoW:      0.01,
		WorkTime: 1,
		TTL:      uint32(mrand.Intn(1024)),
		Payload:  make([]byte, 50),
		KeySym:   symKey,
		Dst:      nil,
	}

	mrand.Read(params.Payload)

	msg, err := NewSentMessage(&params)
	if err != nil {
		t.Fatalf("failed to create new message with seed %d: %s.", seed, err)
	}

	e, err := msg.Wrap(&params)
	if err != nil {
		t.Fatalf("Failed to Wrap the message in an envelope with seed %d: %s", seed, err)
	}

	f := Filter{KeySym: symKey, KeyAsym: asymKey}

	decrypted := e.Open(&f)
	if decrypted != nil {
		t.Fatalf("Managed to decrypt a message with an invalid filter, seed %d", seed)
	}
}
