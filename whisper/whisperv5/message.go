
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

package whisperv5

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/ethereum/go-ethereum/log"
)

//
type MessageParams struct {
	TTL      uint32
	Src      *ecdsa.PrivateKey
	Dst      *ecdsa.PublicKey
	KeySym   []byte
	Topic    TopicType
	WorkTime uint32
	PoW      float64
	Payload  []byte
	Padding  []byte
}

//
//
//
type sentMessage struct {
	Raw []byte
}

//
//
type ReceivedMessage struct {
	Raw []byte

	Payload   []byte
	Padding   []byte
	Signature []byte

PoW   float64          //
Sent  uint32           //
TTL   uint32           //
Src   *ecdsa.PublicKey //
Dst   *ecdsa.PublicKey //
	Topic TopicType

SymKeyHash      common.Hash //
EnvelopeHash    common.Hash //
	EnvelopeVersion uint64
}

func isMessageSigned(flags byte) bool {
	return (flags & signatureFlag) != 0
}

func (msg *ReceivedMessage) isSymmetricEncryption() bool {
	return msg.SymKeyHash != common.Hash{}
}

func (msg *ReceivedMessage) isAsymmetricEncryption() bool {
	return msg.Dst != nil
}

//
func NewSentMessage(params *MessageParams) (*sentMessage, error) {
	msg := sentMessage{}
	msg.Raw = make([]byte, 1, len(params.Payload)+len(params.Padding)+signatureLength+padSizeLimit)
msg.Raw[0] = 0 //
	err := msg.appendPadding(params)
	if err != nil {
		return nil, err
	}
	msg.Raw = append(msg.Raw, params.Payload...)
	return &msg, nil
}

//
func getSizeOfLength(b []byte) (sz int, err error) {
sz = intSize(len(b))      //
sz = intSize(len(b) + sz) //
	if sz > 3 {
		err = errors.New("oversized padding parameter")
	}
	return sz, err
}

//
func intSize(i int) (s int) {
	for s = 1; i >= 256; s++ {
		i /= 256
	}
	return s
}

//
//
func (msg *sentMessage) appendPadding(params *MessageParams) error {
	rawSize := len(params.Payload) + 1
	if params.Src != nil {
		rawSize += signatureLength
	}
	odd := rawSize % padSizeLimit

	if len(params.Padding) != 0 {
		padSize := len(params.Padding)
		padLengthSize, err := getSizeOfLength(params.Padding)
		if err != nil {
			return err
		}
		totalPadSize := padSize + padLengthSize
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint32(buf, uint32(totalPadSize))
		buf = buf[:padLengthSize]
		msg.Raw = append(msg.Raw, buf...)
		msg.Raw = append(msg.Raw, params.Padding...)
msg.Raw[0] |= byte(padLengthSize) //
	} else if odd != 0 {
		totalPadSize := padSizeLimit - odd
		if totalPadSize > 255 {
//
//
//
			panic("please fix the padding algorithm before releasing new version")
		}
		buf := make([]byte, totalPadSize)
		_, err := crand.Read(buf[1:])
		if err != nil {
			return err
		}
		if totalPadSize > 6 && !validateSymmetricKey(buf) {
			return errors.New("failed to generate random padding of size " + strconv.Itoa(totalPadSize))
		}
		buf[0] = byte(totalPadSize)
		msg.Raw = append(msg.Raw, buf...)
msg.Raw[0] |= byte(0x1) //
	}
	return nil
}

//
//
func (msg *sentMessage) sign(key *ecdsa.PrivateKey) error {
	if isMessageSigned(msg.Raw[0]) {
//
		log.Error("failed to sign the message: already signed")
		return nil
	}

	msg.Raw[0] |= signatureFlag
	hash := crypto.Keccak256(msg.Raw)
	signature, err := crypto.Sign(hash, key)
	if err != nil {
msg.Raw[0] &= ^signatureFlag //
		return err
	}
	msg.Raw = append(msg.Raw, signature...)
	return nil
}

//
func (msg *sentMessage) encryptAsymmetric(key *ecdsa.PublicKey) error {
	if !ValidatePublicKey(key) {
		return errors.New("invalid public key provided for asymmetric encryption")
	}
	encrypted, err := ecies.Encrypt(crand.Reader, ecies.ImportECDSAPublic(key), msg.Raw, nil, nil)
	if err == nil {
		msg.Raw = encrypted
	}
	return err
}

//
//
func (msg *sentMessage) encryptSymmetric(key []byte) (nonce []byte, err error) {
	if !validateSymmetricKey(key) {
		return nil, errors.New("invalid key provided for symmetric encryption")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

//
	nonce = make([]byte, aesgcm.NonceSize())
	_, err = crand.Read(nonce)
	if err != nil {
		return nil, err
	} else if !validateSymmetricKey(nonce) {
		return nil, errors.New("crypto/rand failed to generate nonce")
	}

	msg.Raw = aesgcm.Seal(nil, nonce, msg.Raw, nil)
	return nonce, nil
}

//
func (msg *sentMessage) Wrap(options *MessageParams) (envelope *Envelope, err error) {
	if options.TTL == 0 {
		options.TTL = DefaultTTL
	}
	if options.Src != nil {
		if err = msg.sign(options.Src); err != nil {
			return nil, err
		}
	}
	var nonce []byte
	if options.Dst != nil {
		err = msg.encryptAsymmetric(options.Dst)
	} else if options.KeySym != nil {
		nonce, err = msg.encryptSymmetric(options.KeySym)
	} else {
		err = errors.New("unable to encrypt the message: neither symmetric nor assymmetric key provided")
	}
	if err != nil {
		return nil, err
	}

	envelope = NewEnvelope(options.TTL, options.Topic, nonce, msg)
	if err = envelope.Seal(options); err != nil {
		return nil, err
	}
	return envelope, nil
}

//
//
func (msg *ReceivedMessage) decryptSymmetric(key []byte, nonce []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if len(nonce) != aesgcm.NonceSize() {
		log.Error("decrypting the message", "AES nonce size", len(nonce))
		return errors.New("wrong AES nonce size")
	}
	decrypted, err := aesgcm.Open(nil, nonce, msg.Raw, nil)
	if err != nil {
		return err
	}
	msg.Raw = decrypted
	return nil
}

//
func (msg *ReceivedMessage) decryptAsymmetric(key *ecdsa.PrivateKey) error {
	decrypted, err := ecies.ImportECDSA(key).Decrypt(msg.Raw, nil, nil)
	if err == nil {
		msg.Raw = decrypted
	}
	return err
}

//
func (msg *ReceivedMessage) Validate() bool {
	end := len(msg.Raw)
	if end < 1 {
		return false
	}

	if isMessageSigned(msg.Raw[0]) {
		end -= signatureLength
		if end <= 1 {
			return false
		}
		msg.Signature = msg.Raw[end:]
		msg.Src = msg.SigToPubKey()
		if msg.Src == nil {
			return false
		}
	}

	padSize, ok := msg.extractPadding(end)
	if !ok {
		return false
	}

	msg.Payload = msg.Raw[1+padSize : end]
	return true
}

//
//
//
//
func (msg *ReceivedMessage) extractPadding(end int) (int, bool) {
	paddingSize := 0
sz := int(msg.Raw[0] & paddingMask) //
//
	if sz != 0 {
		paddingSize = int(bytesToUintLittleEndian(msg.Raw[1 : 1+sz]))
		if paddingSize < sz || paddingSize+1 > end {
			return 0, false
		}
		msg.Padding = msg.Raw[1+sz : 1+paddingSize]
	}
	return paddingSize, true
}

//
func (msg *ReceivedMessage) SigToPubKey() *ecdsa.PublicKey {
defer func() { recover() }() //

	pub, err := crypto.SigToPub(msg.hash(), msg.Signature)
	if err != nil {
		log.Error("failed to recover public key from signature", "err", err)
		return nil
	}
	return pub
}

//
func (msg *ReceivedMessage) hash() []byte {
	if isMessageSigned(msg.Raw[0]) {
		sz := len(msg.Raw) - signatureLength
		return crypto.Keccak256(msg.Raw[:sz])
	}
	return crypto.Keccak256(msg.Raw)
}
