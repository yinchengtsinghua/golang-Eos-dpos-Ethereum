
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
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/rpc"
)

//
var (
	ErrSymAsym              = errors.New("specify either a symmetric or an asymmetric key")
	ErrInvalidSymmetricKey  = errors.New("invalid symmetric key")
	ErrInvalidPublicKey     = errors.New("invalid public key")
	ErrInvalidSigningPubKey = errors.New("invalid signing public key")
	ErrTooLowPoW            = errors.New("message rejected, PoW too low")
	ErrNoTopics             = errors.New("missing topic(s)")
)

//
//
type PublicWhisperAPI struct {
	w *Whisper

	mu       sync.Mutex
lastUsed map[string]time.Time //
}

//
func NewPublicWhisperAPI(w *Whisper) *PublicWhisperAPI {
	api := &PublicWhisperAPI{
		w:        w,
		lastUsed: make(map[string]time.Time),
	}
	return api
}

//
func (api *PublicWhisperAPI) Version(ctx context.Context) string {
	return ProtocolVersionStr
}

//
type Info struct {
Memory         int     `json:"memory"`         //
Messages       int     `json:"messages"`       //
MinPow         float64 `json:"minPow"`         //
MaxMessageSize uint32  `json:"maxMessageSize"` //
}

//
func (api *PublicWhisperAPI) Info(ctx context.Context) Info {
	stats := api.w.Stats()
	return Info{
		Memory:         stats.memoryUsed,
		Messages:       len(api.w.messageQueue) + len(api.w.p2pMsgQueue),
		MinPow:         api.w.MinPow(),
		MaxMessageSize: api.w.MaxMessageSize(),
	}
}

//
//
func (api *PublicWhisperAPI) SetMaxMessageSize(ctx context.Context, size uint32) (bool, error) {
	return true, api.w.SetMaxMessageSize(size)
}

//
func (api *PublicWhisperAPI) SetMinPoW(ctx context.Context, pow float64) (bool, error) {
	return true, api.w.SetMinimumPoW(pow)
}

//
func (api *PublicWhisperAPI) SetBloomFilter(ctx context.Context, bloom hexutil.Bytes) (bool, error) {
	return true, api.w.SetBloomFilter(bloom)
}

//
//
func (api *PublicWhisperAPI) MarkTrustedPeer(ctx context.Context, enode string) (bool, error) {
	n, err := discover.ParseNode(enode)
	if err != nil {
		return false, err
	}
	return true, api.w.AllowP2PMessagesFromPeer(n.ID[:])
}

//
//
func (api *PublicWhisperAPI) NewKeyPair(ctx context.Context) (string, error) {
	return api.w.NewKeyPair()
}

//
func (api *PublicWhisperAPI) AddPrivateKey(ctx context.Context, privateKey hexutil.Bytes) (string, error) {
	key, err := crypto.ToECDSA(privateKey)
	if err != nil {
		return "", err
	}
	return api.w.AddKeyPair(key)
}

//
func (api *PublicWhisperAPI) DeleteKeyPair(ctx context.Context, key string) (bool, error) {
	if ok := api.w.DeleteKeyPair(key); ok {
		return true, nil
	}
	return false, fmt.Errorf("key pair %s not found", key)
}

//
func (api *PublicWhisperAPI) HasKeyPair(ctx context.Context, id string) bool {
	return api.w.HasKeyPair(id)
}

//
//
func (api *PublicWhisperAPI) GetPublicKey(ctx context.Context, id string) (hexutil.Bytes, error) {
	key, err := api.w.GetPrivateKey(id)
	if err != nil {
		return hexutil.Bytes{}, err
	}
	return crypto.FromECDSAPub(&key.PublicKey), nil
}

//
//
func (api *PublicWhisperAPI) GetPrivateKey(ctx context.Context, id string) (hexutil.Bytes, error) {
	key, err := api.w.GetPrivateKey(id)
	if err != nil {
		return hexutil.Bytes{}, err
	}
	return crypto.FromECDSA(key), nil
}

//
//
//
func (api *PublicWhisperAPI) NewSymKey(ctx context.Context) (string, error) {
	return api.w.GenerateSymKey()
}

//
//
//
func (api *PublicWhisperAPI) AddSymKey(ctx context.Context, key hexutil.Bytes) (string, error) {
	return api.w.AddSymKeyDirect([]byte(key))
}

//
func (api *PublicWhisperAPI) GenerateSymKeyFromPassword(ctx context.Context, passwd string) (string, error) {
	return api.w.AddSymKeyFromPassword(passwd)
}

//
func (api *PublicWhisperAPI) HasSymKey(ctx context.Context, id string) bool {
	return api.w.HasSymKey(id)
}

//
func (api *PublicWhisperAPI) GetSymKey(ctx context.Context, id string) (hexutil.Bytes, error) {
	return api.w.GetSymKey(id)
}

//
func (api *PublicWhisperAPI) DeleteSymKey(ctx context.Context, id string) bool {
	return api.w.DeleteSymKey(id)
}

//
//
func (api *PublicWhisperAPI) MakeLightClient(ctx context.Context) bool {
	api.w.lightClient = true
	return api.w.lightClient
}

//
func (api *PublicWhisperAPI) CancelLightClient(ctx context.Context) bool {
	api.w.lightClient = false
	return !api.w.lightClient
}

//

//
type NewMessage struct {
	SymKeyID   string    `json:"symKeyID"`
	PublicKey  []byte    `json:"pubKey"`
	Sig        string    `json:"sig"`
	TTL        uint32    `json:"ttl"`
	Topic      TopicType `json:"topic"`
	Payload    []byte    `json:"payload"`
	Padding    []byte    `json:"padding"`
	PowTime    uint32    `json:"powTime"`
	PowTarget  float64   `json:"powTarget"`
	TargetPeer string    `json:"targetPeer"`
}

type newMessageOverride struct {
	PublicKey hexutil.Bytes
	Payload   hexutil.Bytes
	Padding   hexutil.Bytes
}

//
//
func (api *PublicWhisperAPI) Post(ctx context.Context, req NewMessage) (hexutil.Bytes, error) {
	var (
		symKeyGiven = len(req.SymKeyID) > 0
		pubKeyGiven = len(req.PublicKey) > 0
		err         error
	)

//
	if (symKeyGiven && pubKeyGiven) || (!symKeyGiven && !pubKeyGiven) {
		return nil, ErrSymAsym
	}

	params := &MessageParams{
		TTL:      req.TTL,
		Payload:  req.Payload,
		Padding:  req.Padding,
		WorkTime: req.PowTime,
		PoW:      req.PowTarget,
		Topic:    req.Topic,
	}

//
	if len(req.Sig) > 0 {
		if params.Src, err = api.w.GetPrivateKey(req.Sig); err != nil {
			return nil, err
		}
	}

//
	if symKeyGiven {
if params.Topic == (TopicType{}) { //
			return nil, ErrNoTopics
		}
		if params.KeySym, err = api.w.GetSymKey(req.SymKeyID); err != nil {
			return nil, err
		}
		if !validateDataIntegrity(params.KeySym, aesKeyLength) {
			return nil, ErrInvalidSymmetricKey
		}
	}

//
	if pubKeyGiven {
		if params.Dst, err = crypto.UnmarshalPubkey(req.PublicKey); err != nil {
			return nil, ErrInvalidPublicKey
		}
	}

//
	whisperMsg, err := NewSentMessage(params)
	if err != nil {
		return nil, err
	}

	var result []byte
	env, err := whisperMsg.Wrap(params)
	if err != nil {
		return nil, err
	}

//
	if len(req.TargetPeer) > 0 {
		n, err := discover.ParseNode(req.TargetPeer)
		if err != nil {
			return nil, fmt.Errorf("failed to parse target peer: %s", err)
		}
		err = api.w.SendP2PMessage(n.ID[:], env)
		if err == nil {
			hash := env.Hash()
			result = hash[:]
		}
		return result, err
	}

//
	if req.PowTarget < api.w.MinPow() {
		return nil, ErrTooLowPoW
	}

	err = api.w.Send(env)
	if err == nil {
		hash := env.Hash()
		result = hash[:]
	}
	return result, err
}

//

//
type Criteria struct {
	SymKeyID     string      `json:"symKeyID"`
	PrivateKeyID string      `json:"privateKeyID"`
	Sig          []byte      `json:"sig"`
	MinPow       float64     `json:"minPow"`
	Topics       []TopicType `json:"topics"`
	AllowP2P     bool        `json:"allowP2P"`
}

type criteriaOverride struct {
	Sig hexutil.Bytes
}

//
//
func (api *PublicWhisperAPI) Messages(ctx context.Context, crit Criteria) (*rpc.Subscription, error) {
	var (
		symKeyGiven = len(crit.SymKeyID) > 0
		pubKeyGiven = len(crit.PrivateKeyID) > 0
		err         error
	)

//
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return nil, rpc.ErrNotificationsUnsupported
	}

//
	if (symKeyGiven && pubKeyGiven) || (!symKeyGiven && !pubKeyGiven) {
		return nil, ErrSymAsym
	}

	filter := Filter{
		PoW:      crit.MinPow,
		Messages: make(map[common.Hash]*ReceivedMessage),
		AllowP2P: crit.AllowP2P,
	}

	if len(crit.Sig) > 0 {
		if filter.Src, err = crypto.UnmarshalPubkey(crit.Sig); err != nil {
			return nil, ErrInvalidSigningPubKey
		}
	}

	for i, bt := range crit.Topics {
		if len(bt) == 0 || len(bt) > 4 {
			return nil, fmt.Errorf("subscribe: topic %d has wrong size: %d", i, len(bt))
		}
		filter.Topics = append(filter.Topics, bt[:])
	}

//
	if symKeyGiven {
		if len(filter.Topics) == 0 {
			return nil, ErrNoTopics
		}
		key, err := api.w.GetSymKey(crit.SymKeyID)
		if err != nil {
			return nil, err
		}
		if !validateDataIntegrity(key, aesKeyLength) {
			return nil, ErrInvalidSymmetricKey
		}
		filter.KeySym = key
		filter.SymKeyHash = crypto.Keccak256Hash(filter.KeySym)
	}

//
	if pubKeyGiven {
		filter.KeyAsym, err = api.w.GetPrivateKey(crit.PrivateKeyID)
		if err != nil || filter.KeyAsym == nil {
			return nil, ErrInvalidPublicKey
		}
	}

	id, err := api.w.Subscribe(&filter)
	if err != nil {
		return nil, err
	}

//
	rpcSub := notifier.CreateSubscription()
	go func() {
//
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if filter := api.w.GetFilter(id); filter != nil {
					for _, rpcMessage := range toMessage(filter.Retrieve()) {
						if err := notifier.Notify(rpcSub.ID, rpcMessage); err != nil {
							log.Error("Failed to send notification", "err", err)
						}
					}
				}
			case <-rpcSub.Err():
				api.w.Unsubscribe(id)
				return
			case <-notifier.Closed():
				api.w.Unsubscribe(id)
				return
			}
		}
	}()

	return rpcSub, nil
}

//

//
type Message struct {
	Sig       []byte    `json:"sig,omitempty"`
	TTL       uint32    `json:"ttl"`
	Timestamp uint32    `json:"timestamp"`
	Topic     TopicType `json:"topic"`
	Payload   []byte    `json:"payload"`
	Padding   []byte    `json:"padding"`
	PoW       float64   `json:"pow"`
	Hash      []byte    `json:"hash"`
	Dst       []byte    `json:"recipientPublicKey,omitempty"`
}

type messageOverride struct {
	Sig     hexutil.Bytes
	Payload hexutil.Bytes
	Padding hexutil.Bytes
	Hash    hexutil.Bytes
	Dst     hexutil.Bytes
}

//
func ToWhisperMessage(message *ReceivedMessage) *Message {
	msg := Message{
		Payload:   message.Payload,
		Padding:   message.Padding,
		Timestamp: message.Sent,
		TTL:       message.TTL,
		PoW:       message.PoW,
		Hash:      message.EnvelopeHash.Bytes(),
		Topic:     message.Topic,
	}

	if message.Dst != nil {
		b := crypto.FromECDSAPub(message.Dst)
		if b != nil {
			msg.Dst = b
		}
	}

	if isMessageSigned(message.Raw[0]) {
		b := crypto.FromECDSAPub(message.SigToPubKey())
		if b != nil {
			msg.Sig = b
		}
	}

	return &msg
}

//
func toMessage(messages []*ReceivedMessage) []*Message {
	msgs := make([]*Message, len(messages))
	for i, msg := range messages {
		msgs[i] = ToWhisperMessage(msg)
	}
	return msgs
}

//
//
func (api *PublicWhisperAPI) GetFilterMessages(id string) ([]*Message, error) {
	api.mu.Lock()
	f := api.w.GetFilter(id)
	if f == nil {
		api.mu.Unlock()
		return nil, fmt.Errorf("filter not found")
	}
	api.lastUsed[id] = time.Now()
	api.mu.Unlock()

	receivedMessages := f.Retrieve()
	messages := make([]*Message, 0, len(receivedMessages))
	for _, msg := range receivedMessages {
		messages = append(messages, ToWhisperMessage(msg))
	}

	return messages, nil
}

//
func (api *PublicWhisperAPI) DeleteMessageFilter(id string) (bool, error) {
	api.mu.Lock()
	defer api.mu.Unlock()

	delete(api.lastUsed, id)
	return true, api.w.Unsubscribe(id)
}

//
//
func (api *PublicWhisperAPI) NewMessageFilter(req Criteria) (string, error) {
	var (
		src     *ecdsa.PublicKey
		keySym  []byte
		keyAsym *ecdsa.PrivateKey
		topics  [][]byte

		symKeyGiven  = len(req.SymKeyID) > 0
		asymKeyGiven = len(req.PrivateKeyID) > 0

		err error
	)

//
	if (symKeyGiven && asymKeyGiven) || (!symKeyGiven && !asymKeyGiven) {
		return "", ErrSymAsym
	}

	if len(req.Sig) > 0 {
		if src, err = crypto.UnmarshalPubkey(req.Sig); err != nil {
			return "", ErrInvalidSigningPubKey
		}
	}

	if symKeyGiven {
		if keySym, err = api.w.GetSymKey(req.SymKeyID); err != nil {
			return "", err
		}
		if !validateDataIntegrity(keySym, aesKeyLength) {
			return "", ErrInvalidSymmetricKey
		}
	}

	if asymKeyGiven {
		if keyAsym, err = api.w.GetPrivateKey(req.PrivateKeyID); err != nil {
			return "", err
		}
	}

	if len(req.Topics) > 0 {
		topics = make([][]byte, len(req.Topics))
		for i, topic := range req.Topics {
			topics[i] = make([]byte, TopicLength)
			copy(topics[i], topic[:])
		}
	}

	f := &Filter{
		Src:      src,
		KeySym:   keySym,
		KeyAsym:  keyAsym,
		PoW:      req.MinPow,
		AllowP2P: req.AllowP2P,
		Topics:   topics,
		Messages: make(map[common.Hash]*ReceivedMessage),
	}

	id, err := api.w.Subscribe(f)
	if err != nil {
		return "", err
	}

	api.mu.Lock()
	api.lastUsed[id] = time.Now()
	api.mu.Unlock()

	return id, nil
}
