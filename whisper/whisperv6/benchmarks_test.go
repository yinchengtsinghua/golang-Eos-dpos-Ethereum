
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

package whisperv6

import (
	"crypto/sha256"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/pbkdf2"
)

func BenchmarkDeriveKeyMaterial(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pbkdf2.Key([]byte("test"), nil, 65356, aesKeyLength, sha256.New)
	}
}

func BenchmarkEncryptionSym(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}

	for i := 0; i < b.N; i++ {
		msg, _ := NewSentMessage(params)
		_, err := msg.Wrap(params)
		if err != nil {
			b.Errorf("failed Wrap with seed %d: %s.", seed, err)
			b.Errorf("i = %d, len(msg.Raw) = %d, params.Payload = %d.", i, len(msg.Raw), len(params.Payload))
			return
		}
	}
}

func BenchmarkEncryptionAsym(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		b.Fatalf("failed GenerateKey with seed %d: %s.", seed, err)
	}
	params.KeySym = nil
	params.Dst = &key.PublicKey

	for i := 0; i < b.N; i++ {
		msg, _ := NewSentMessage(params)
		_, err := msg.Wrap(params)
		if err != nil {
			b.Fatalf("failed Wrap with seed %d: %s.", seed, err)
		}
	}
}

func BenchmarkDecryptionSymValid(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}
	msg, _ := NewSentMessage(params)
	env, err := msg.Wrap(params)
	if err != nil {
		b.Fatalf("failed Wrap with seed %d: %s.", seed, err)
	}
	f := Filter{KeySym: params.KeySym}

	for i := 0; i < b.N; i++ {
		msg := env.Open(&f)
		if msg == nil {
			b.Fatalf("failed to open with seed %d.", seed)
		}
	}
}

func BenchmarkDecryptionSymInvalid(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}
	msg, _ := NewSentMessage(params)
	env, err := msg.Wrap(params)
	if err != nil {
		b.Fatalf("failed Wrap with seed %d: %s.", seed, err)
	}
	f := Filter{KeySym: []byte("arbitrary stuff here")}

	for i := 0; i < b.N; i++ {
		msg := env.Open(&f)
		if msg != nil {
			b.Fatalf("opened envelope with invalid key, seed: %d.", seed)
		}
	}
}

func BenchmarkDecryptionAsymValid(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		b.Fatalf("failed GenerateKey with seed %d: %s.", seed, err)
	}
	f := Filter{KeyAsym: key}
	params.KeySym = nil
	params.Dst = &key.PublicKey
	msg, _ := NewSentMessage(params)
	env, err := msg.Wrap(params)
	if err != nil {
		b.Fatalf("failed Wrap with seed %d: %s.", seed, err)
	}

	for i := 0; i < b.N; i++ {
		msg := env.Open(&f)
		if msg == nil {
			b.Fatalf("fail to open, seed: %d.", seed)
		}
	}
}

func BenchmarkDecryptionAsymInvalid(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		b.Fatalf("failed GenerateKey with seed %d: %s.", seed, err)
	}
	params.KeySym = nil
	params.Dst = &key.PublicKey
	msg, _ := NewSentMessage(params)
	env, err := msg.Wrap(params)
	if err != nil {
		b.Fatalf("failed Wrap with seed %d: %s.", seed, err)
	}

	key, err = crypto.GenerateKey()
	if err != nil {
		b.Fatalf("failed GenerateKey with seed %d: %s.", seed, err)
	}
	f := Filter{KeyAsym: key}

	for i := 0; i < b.N; i++ {
		msg := env.Open(&f)
		if msg != nil {
			b.Fatalf("opened envelope with invalid key, seed: %d.", seed)
		}
	}
}

func increment(x []byte) {
	for i := 0; i < len(x); i++ {
		x[i]++
		if x[i] != 0 {
			break
		}
	}
}

func BenchmarkPoW(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}
	params.Payload = make([]byte, 32)
	params.PoW = 10.0
	params.TTL = 1

	for i := 0; i < b.N; i++ {
		increment(params.Payload)
		msg, _ := NewSentMessage(params)
		_, err := msg.Wrap(params)
		if err != nil {
			b.Fatalf("failed Wrap with seed %d: %s.", seed, err)
		}
	}
}
