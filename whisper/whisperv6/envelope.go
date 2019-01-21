
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
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	gmath "math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/ethereum/go-ethereum/rlp"
)

//
//
type Envelope struct {
	Expiry uint32
	TTL    uint32
	Topic  TopicType
	Data   []byte
	Nonce  uint64

pow float64 //

//
hash  common.Hash //
	bloom []byte
}

//
func (e *Envelope) size() int {
	return EnvelopeHeaderLength + len(e.Data)
}

//
func (e *Envelope) rlpWithoutNonce() []byte {
	res, _ := rlp.EncodeToBytes([]interface{}{e.Expiry, e.TTL, e.Topic, e.Data})
	return res
}

//
//
func NewEnvelope(ttl uint32, topic TopicType, msg *sentMessage) *Envelope {
	env := Envelope{
		Expiry: uint32(time.Now().Add(time.Second * time.Duration(ttl)).Unix()),
		TTL:    ttl,
		Topic:  topic,
		Data:   msg.Raw,
		Nonce:  0,
	}

	return &env
}

//
//
func (e *Envelope) Seal(options *MessageParams) error {
	if options.PoW == 0 {
//
		return nil
	}

	var target, bestBit int
	if options.PoW < 0 {
//
//
//
		e.Expiry += options.WorkTime
	} else {
		target = e.powToFirstBit(options.PoW)
	}

	buf := make([]byte, 64)
	h := crypto.Keccak256(e.rlpWithoutNonce())
	copy(buf[:32], h)

	finish := time.Now().Add(time.Duration(options.WorkTime) * time.Second).UnixNano()
	for nonce := uint64(0); time.Now().UnixNano() < finish; {
		for i := 0; i < 1024; i++ {
			binary.BigEndian.PutUint64(buf[56:], nonce)
			d := new(big.Int).SetBytes(crypto.Keccak256(buf))
			firstBit := math.FirstBitSet(d)
			if firstBit > bestBit {
				e.Nonce, bestBit = nonce, firstBit
				if target > 0 && bestBit >= target {
					return nil
				}
			}
			nonce++
		}
	}

	if target > 0 && bestBit < target {
		return fmt.Errorf("failed to reach the PoW target, specified pow time (%d seconds) was insufficient", options.WorkTime)
	}

	return nil
}

//
//
func (e *Envelope) PoW() float64 {
	if e.pow == 0 {
		e.calculatePoW(0)
	}
	return e.pow
}

func (e *Envelope) calculatePoW(diff uint32) {
	buf := make([]byte, 64)
	h := crypto.Keccak256(e.rlpWithoutNonce())
	copy(buf[:32], h)
	binary.BigEndian.PutUint64(buf[56:], e.Nonce)
	d := new(big.Int).SetBytes(crypto.Keccak256(buf))
	firstBit := math.FirstBitSet(d)
	x := gmath.Pow(2, float64(firstBit))
	x /= float64(e.size())
	x /= float64(e.TTL + diff)
	e.pow = x
}

func (e *Envelope) powToFirstBit(pow float64) int {
	x := pow
	x *= float64(e.size())
	x *= float64(e.TTL)
	bits := gmath.Log2(x)
	bits = gmath.Ceil(bits)
	res := int(bits)
	if res < 1 {
		res = 1
	}
	return res
}

//
func (e *Envelope) Hash() common.Hash {
	if (e.hash == common.Hash{}) {
		encoded, _ := rlp.EncodeToBytes(e)
		e.hash = crypto.Keccak256Hash(encoded)
	}
	return e.hash
}

//
func (e *Envelope) DecodeRLP(s *rlp.Stream) error {
	raw, err := s.Raw()
	if err != nil {
		return err
	}
//
//
//
//
//
	type rlpenv Envelope
	if err := rlp.DecodeBytes(raw, (*rlpenv)(e)); err != nil {
		return err
	}
	e.hash = crypto.Keccak256Hash(raw)
	return nil
}

//
func (e *Envelope) OpenAsymmetric(key *ecdsa.PrivateKey) (*ReceivedMessage, error) {
	message := &ReceivedMessage{Raw: e.Data}
	err := message.decryptAsymmetric(key)
	switch err {
	case nil:
		return message, nil
case ecies.ErrInvalidPublicKey: //
		return nil, err
	default:
		return nil, fmt.Errorf("unable to open envelope, decrypt failed: %v", err)
	}
}

//
func (e *Envelope) OpenSymmetric(key []byte) (msg *ReceivedMessage, err error) {
	msg = &ReceivedMessage{Raw: e.Data}
	err = msg.decryptSymmetric(key)
	if err != nil {
		msg = nil
	}
	return msg, err
}

//
func (e *Envelope) Open(watcher *Filter) (msg *ReceivedMessage) {
	if watcher == nil {
		return nil
	}

//
	if watcher.expectsAsymmetricEncryption() && watcher.expectsSymmetricEncryption() {
		return nil
	}

	if watcher.expectsAsymmetricEncryption() {
		msg, _ = e.OpenAsymmetric(watcher.KeyAsym)
		if msg != nil {
			msg.Dst = &watcher.KeyAsym.PublicKey
		}
	} else if watcher.expectsSymmetricEncryption() {
		msg, _ = e.OpenSymmetric(watcher.KeySym)
		if msg != nil {
			msg.SymKeyHash = crypto.Keccak256Hash(watcher.KeySym)
		}
	}

	if msg != nil {
		ok := msg.ValidateAndParse()
		if !ok {
			return nil
		}
		msg.Topic = e.Topic
		msg.PoW = e.PoW()
		msg.TTL = e.TTL
		msg.Sent = e.Expiry - e.TTL
		msg.EnvelopeHash = e.Hash()
	}
	return msg
}

//
func (e *Envelope) Bloom() []byte {
	if e.bloom == nil {
		e.bloom = TopicToBloom(e.Topic)
	}
	return e.bloom
}

//
func TopicToBloom(topic TopicType) []byte {
	b := make([]byte, BloomFilterSize)
	var index [3]int
	for j := 0; j < 3; j++ {
		index[j] = int(topic[j])
		if (topic[3] & (1 << uint(j))) != 0 {
			index[j] += 256
		}
	}

	for j := 0; j < 3; j++ {
		byteIndex := index[j] / 8
		bitIndex := index[j] % 8
		b[byteIndex] = (1 << uint(bitIndex))
	}
	return b
}

//
//
func (w *Whisper) GetEnvelope(hash common.Hash) *Envelope {
	w.poolMu.RLock()
	defer w.poolMu.RUnlock()
	return w.envelopes[hash]
}
