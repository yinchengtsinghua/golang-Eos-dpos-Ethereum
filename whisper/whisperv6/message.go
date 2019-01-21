
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
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	mrand "math/rand"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/ethereum/go-ethereum/log"
)

//
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
	Salt      []byte

PoW   float64          //
Sent  uint32           //
TTL   uint32           //
Src   *ecdsa.PublicKey //
Dst   *ecdsa.PublicKey //
	Topic TopicType

SymKeyHash   common.Hash //
EnvelopeHash common.Hash //
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
	const payloadSizeFieldMaxSize = 4
	msg := sentMessage{}
	msg.Raw = make([]byte, 1,
		flagsLength+payloadSizeFieldMaxSize+len(params.Payload)+len(params.Padding)+signatureLength+padSizeLimit)
msg.Raw[0] = 0 //
	msg.addPayloadSizeField(params.Payload)
	msg.Raw = append(msg.Raw, params.Payload...)
	err := msg.appendPadding(params)
	return &msg, err
}

//
func (msg *sentMessage) addPayloadSizeField(payload []byte) {
	fieldSize := getSizeOfPayloadSizeField(payload)
	field := make([]byte, 4)
	binary.LittleEndian.PutUint32(field, uint32(len(payload)))
	field = field[:fieldSize]
	msg.Raw = append(msg.Raw, field...)
	msg.Raw[0] |= byte(fieldSize)
}

//
func getSizeOfPayloadSizeField(payload []byte) int {
	s := 1
	for i := len(payload); i >= 256; i /= 256 {
		s++
	}
	return s
}

//
//
func (msg *sentMessage) appendPadding(params *MessageParams) error {
	if len(params.Padding) != 0 {
//
		msg.Raw = append(msg.Raw, params.Padding...)
		return nil
	}

	rawSize := flagsLength + getSizeOfPayloadSizeField(params.Payload) + len(params.Payload)
	if params.Src != nil {
		rawSize += signatureLength
	}
	odd := rawSize % padSizeLimit
	paddingSize := padSizeLimit - odd
	pad := make([]byte, paddingSize)
	_, err := crand.Read(pad)
	if err != nil {
		return err
	}
	if !validateDataIntegrity(pad, paddingSize) {
		return errors.New("failed to generate random padding of size " + strconv.Itoa(paddingSize))
	}
	msg.Raw = append(msg.Raw, pad...)
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

msg.Raw[0] |= signatureFlag //
	hash := crypto.Keccak256(msg.Raw)
	signature, err := crypto.Sign(hash, key)
	if err != nil {
msg.Raw[0] &= (0xFF ^ signatureFlag) //
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
func (msg *sentMessage) encryptSymmetric(key []byte) (err error) {
	if !validateDataIntegrity(key, aesKeyLength) {
		return errors.New("invalid key provided for symmetric encryption, size: " + strconv.Itoa(len(key)))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
salt, err := generateSecureRandomData(aesNonceLength) //
	if err != nil {
		return err
	}
	encrypted := aesgcm.Seal(nil, salt, msg.Raw, nil)
	msg.Raw = append(encrypted, salt...)
	return nil
}

//
//
//
//
//
func generateSecureRandomData(length int) ([]byte, error) {
	x := make([]byte, length)
	y := make([]byte, length)
	res := make([]byte, length)

	_, err := crand.Read(x)
	if err != nil {
		return nil, err
	} else if !validateDataIntegrity(x, length) {
		return nil, errors.New("crypto/rand failed to generate secure random data")
	}
	_, err = mrand.Read(y)
	if err != nil {
		return nil, err
	} else if !validateDataIntegrity(y, length) {
		return nil, errors.New("math/rand failed to generate secure random data")
	}
	for i := 0; i < length; i++ {
		res[i] = x[i] ^ y[i]
	}
	if !validateDataIntegrity(res, length) {
		return nil, errors.New("failed to generate secure random data")
	}
	return res, nil
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
	if options.Dst != nil {
		err = msg.encryptAsymmetric(options.Dst)
	} else if options.KeySym != nil {
		err = msg.encryptSymmetric(options.KeySym)
	} else {
		err = errors.New("unable to encrypt the message: neither symmetric nor assymmetric key provided")
	}
	if err != nil {
		return nil, err
	}

	envelope = NewEnvelope(options.TTL, options.Topic, msg)
	if err = envelope.Seal(options); err != nil {
		return nil, err
	}
	return envelope, nil
}

//
//
func (msg *ReceivedMessage) decryptSymmetric(key []byte) error {
//
	if len(msg.Raw) < aesNonceLength {
		return errors.New("missing salt or invalid payload in symmetric message")
	}
	salt := msg.Raw[len(msg.Raw)-aesNonceLength:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	decrypted, err := aesgcm.Open(nil, salt, msg.Raw[:len(msg.Raw)-aesNonceLength], nil)
	if err != nil {
		return err
	}
	msg.Raw = decrypted
	msg.Salt = salt
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
func (msg *ReceivedMessage) ValidateAndParse() bool {
	end := len(msg.Raw)
	if end < 1 {
		return false
	}

	if isMessageSigned(msg.Raw[0]) {
		end -= signatureLength
		if end <= 1 {
			return false
		}
		msg.Signature = msg.Raw[end : end+signatureLength]
		msg.Src = msg.SigToPubKey()
		if msg.Src == nil {
			return false
		}
	}

	beg := 1
	payloadSize := 0
sizeOfPayloadSizeField := int(msg.Raw[0] & SizeMask) //
	if sizeOfPayloadSizeField != 0 {
		payloadSize = int(bytesToUintLittleEndian(msg.Raw[beg : beg+sizeOfPayloadSizeField]))
		if payloadSize+1 > end {
			return false
		}
		beg += sizeOfPayloadSizeField
		msg.Payload = msg.Raw[beg : beg+payloadSize]
	}

	beg += payloadSize
	msg.Padding = msg.Raw[beg:end]
	return true
}

//
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
