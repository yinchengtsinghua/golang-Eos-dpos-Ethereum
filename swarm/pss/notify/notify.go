
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package notify

import (
	"crypto/ecdsa"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/pss"
)

const (
//
	MsgCodeStart = iota

//
	MsgCodeNotifyWithKey

//
	MsgCodeNotify

//
	MsgCodeStop
	MsgCodeMax
)

const (
	DefaultAddressLength = 1
symKeyLength         = 32 //
)

var (
//
	controlTopic = pss.Topic{0x00, 0x00, 0x00, 0x01}
)

//
//
//
//
type Msg struct {
	Code       byte
	Name       []byte
	Payload    []byte
	namestring string
}

//
func NewMsg(code byte, name string, payload []byte) *Msg {
	return &Msg{
		Code:       code,
		Name:       []byte(name),
		Payload:    payload,
		namestring: name,
	}
}

//
func NewMsgFromPayload(payload []byte) (*Msg, error) {
	msg := &Msg{}
	err := rlp.DecodeBytes(payload, msg)
	if err != nil {
		return nil, err
	}
	msg.namestring = string(msg.Name)
	return msg, nil
}

//
type sendBin struct {
	address  pss.PssAddress
	symKeyId string
	count    int
}

//
//
type notifier struct {
	bins      map[string]*sendBin
topic     pss.Topic //
threshold int       //
	updateC   <-chan []byte
	quitC     chan struct{}
}

func (n *notifier) removeSubscription() {
	n.quitC <- struct{}{}
}

//
type subscription struct {
	pubkeyId string
	address  pss.PssAddress
	handler  func(string, []byte) error
}

//
type Controller struct {
	pss           *pss.Pss
	notifiers     map[string]*notifier
	subscriptions map[string]*subscription
	mu            sync.Mutex
}

//
func NewController(ps *pss.Pss) *Controller {
	ctrl := &Controller{
		pss:           ps,
		notifiers:     make(map[string]*notifier),
		subscriptions: make(map[string]*subscription),
	}
	ctrl.pss.Register(&controlTopic, ctrl.Handler)
	return ctrl
}

//
//
func (c *Controller) IsActive(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isActive(name)
}

func (c *Controller) isActive(name string) bool {
	_, ok := c.notifiers[name]
	return ok
}

//
//
//
//
func (c *Controller) Subscribe(name string, pubkey *ecdsa.PublicKey, address pss.PssAddress, handler func(string, []byte) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	msg := NewMsg(MsgCodeStart, name, c.pss.BaseAddr())
	c.pss.SetPeerPublicKey(pubkey, controlTopic, &address)
	pubkeyId := hexutil.Encode(crypto.FromECDSAPub(pubkey))
	smsg, err := rlp.EncodeToBytes(msg)
	if err != nil {
		return err
	}
	err = c.pss.SendAsym(pubkeyId, controlTopic, smsg)
	if err != nil {
		return err
	}
	c.subscriptions[name] = &subscription{
		pubkeyId: pubkeyId,
		address:  address,
		handler:  handler,
	}
	return nil
}

//
//
func (c *Controller) Unsubscribe(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	sub, ok := c.subscriptions[name]
	if !ok {
		return fmt.Errorf("Unknown subscription '%s'", name)
	}
	msg := NewMsg(MsgCodeStop, name, sub.address)
	smsg, err := rlp.EncodeToBytes(msg)
	if err != nil {
		return err
	}
	err = c.pss.SendAsym(sub.pubkeyId, controlTopic, smsg)
	if err != nil {
		return err
	}
	delete(c.subscriptions, name)
	return nil
}

//
//
//
//
//
func (c *Controller) NewNotifier(name string, threshold int, updateC <-chan []byte) (func(), error) {
	c.mu.Lock()
	if c.isActive(name) {
		c.mu.Unlock()
		return nil, fmt.Errorf("Notification service %s already exists in controller", name)
	}
	quitC := make(chan struct{})
	c.notifiers[name] = &notifier{
		bins:      make(map[string]*sendBin),
		topic:     pss.BytesToTopic([]byte(name)),
		threshold: threshold,
		updateC:   updateC,
		quitC:     quitC,
//
	}
	c.mu.Unlock()
	go func() {
		for {
			select {
			case <-quitC:
				return
			case data := <-updateC:
				c.notify(name, data)
			}
		}
	}()

	return c.notifiers[name].removeSubscription, nil
}

//
//
func (c *Controller) RemoveNotifier(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	currentNotifier, ok := c.notifiers[name]
	if !ok {
		return fmt.Errorf("Unknown notification service %s", name)
	}
	currentNotifier.removeSubscription()
	delete(c.notifiers, name)
	return nil
}

//
//
//
//
func (c *Controller) notify(name string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.isActive(name) {
		return fmt.Errorf("Notification service %s doesn't exist", name)
	}
	msg := NewMsg(MsgCodeNotify, name, data)
	smsg, err := rlp.EncodeToBytes(msg)
	if err != nil {
		return err
	}
	for _, m := range c.notifiers[name].bins {
		log.Debug("sending pss notify", "name", name, "addr", fmt.Sprintf("%x", m.address), "topic", fmt.Sprintf("%x", c.notifiers[name].topic), "data", data)
		go func(m *sendBin) {
			err = c.pss.SendSym(m.symKeyId, c.notifiers[name].topic, smsg)
			if err != nil {
				log.Warn("Failed to send notify to addr %x: %v", m.address, err)
			}
		}(m)
	}
	return nil
}

//
//
//
func (c *Controller) addToBin(ntfr *notifier, address []byte) (symKeyId string, pssAddress pss.PssAddress, err error) {

//
	if len(address) > ntfr.threshold {
		address = address[:ntfr.threshold]
	}

	pssAddress = pss.PssAddress(address)
	hexAddress := fmt.Sprintf("%x", address)
	currentBin, ok := ntfr.bins[hexAddress]
	if ok {
		currentBin.count++
		symKeyId = currentBin.symKeyId
	} else {
		symKeyId, err = c.pss.GenerateSymmetricKey(ntfr.topic, &pssAddress, false)
		if err != nil {
			return "", nil, err
		}
		ntfr.bins[hexAddress] = &sendBin{
			address:  address,
			symKeyId: symKeyId,
			count:    1,
		}
	}
	return symKeyId, pssAddress, nil
}

func (c *Controller) handleStartMsg(msg *Msg, keyid string) (err error) {

	keyidbytes, err := hexutil.Decode(keyid)
	if err != nil {
		return err
	}
	pubkey, err := crypto.UnmarshalPubkey(keyidbytes)
	if err != nil {
		return err
	}

//
	currentNotifier, ok := c.notifiers[msg.namestring]
	if !ok {
		return fmt.Errorf("Subscribe attempted on unknown resource '%s'", msg.namestring)
	}

//
	symKeyId, pssAddress, err := c.addToBin(currentNotifier, msg.Payload)
	if err != nil {
		return err
	}

//
	symkey, err := c.pss.GetSymmetricKey(symKeyId)
	if err != nil {
		return err
	}
	err = c.pss.SetPeerPublicKey(pubkey, controlTopic, &pssAddress)
	if err != nil {
		return err
	}

//
	notify := []byte{}
	replyMsg := NewMsg(MsgCodeNotifyWithKey, msg.namestring, make([]byte, len(notify)+symKeyLength))
	copy(replyMsg.Payload, notify)
	copy(replyMsg.Payload[len(notify):], symkey)
	sReplyMsg, err := rlp.EncodeToBytes(replyMsg)
	if err != nil {
		return err
	}
	return c.pss.SendAsym(keyid, controlTopic, sReplyMsg)
}

func (c *Controller) handleNotifyWithKeyMsg(msg *Msg) error {
	symkey := msg.Payload[len(msg.Payload)-symKeyLength:]
	topic := pss.BytesToTopic(msg.Name)

//
	updaterAddr := pss.PssAddress([]byte{})
	c.pss.SetSymmetricKey(symkey, topic, &updaterAddr, true)
	c.pss.Register(&topic, c.Handler)
	return c.subscriptions[msg.namestring].handler(msg.namestring, msg.Payload[:len(msg.Payload)-symKeyLength])
}

func (c *Controller) handleStopMsg(msg *Msg) error {
//
	currentNotifier, ok := c.notifiers[msg.namestring]
	if !ok {
		return fmt.Errorf("Unsubscribe attempted on unknown resource '%s'", msg.namestring)
	}

//
	address := msg.Payload
	if len(msg.Payload) > currentNotifier.threshold {
		address = address[:currentNotifier.threshold]
	}

//
	hexAddress := fmt.Sprintf("%x", address)
	currentBin, ok := currentNotifier.bins[hexAddress]
	if !ok {
		return fmt.Errorf("found no active bin for address %s", hexAddress)
	}
	currentBin.count--
if currentBin.count == 0 { //
		delete(currentNotifier.bins, hexAddress)
	}
	return nil
}

//
//
func (c *Controller) Handler(smsg []byte, p *p2p.Peer, asymmetric bool, keyid string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	log.Debug("notify controller handler", "keyid", keyid)

//
	msg, err := NewMsgFromPayload(smsg)
	if err != nil {
		return err
	}

	switch msg.Code {
	case MsgCodeStart:
		return c.handleStartMsg(msg, keyid)
	case MsgCodeNotifyWithKey:
		return c.handleNotifyWithKeyMsg(msg)
	case MsgCodeNotify:
		return c.subscriptions[msg.namestring].handler(msg.namestring, msg.Payload)
	case MsgCodeStop:
		return c.handleStopMsg(msg)
	}

	return fmt.Errorf("Invalid message code: %d", msg.Code)
}
