
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

package pss

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/pot"
	"github.com/ethereum/go-ethereum/swarm/storage"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv5"
)

const (
	defaultPaddingByteSize     = 16
	DefaultMsgTTL              = time.Second * 120
	defaultDigestCacheTTL      = time.Second * 10
	defaultSymKeyCacheCapacity = 512
digestLength               = 32 //
	defaultWhisperWorkTime     = 3
	defaultWhisperPoW          = 0.0000000001
	defaultMaxMsgSize          = 1024 * 1024
	defaultCleanInterval       = time.Second * 60 * 10
	defaultOutboxCapacity      = 100000
	pssProtocolName            = "pss"
	pssVersion                 = 2
	hasherCount                = 8
)

var (
	addressLength = len(pot.Address{})
)

//
//
//
type pssCacheEntry struct {
	expiresAt time.Time
}

//
type senderPeer interface {
	Info() *p2p.PeerInfo
	ID() discover.NodeID
	Address() []byte
	Send(context.Context, interface{}) error
}

//
//
type pssPeer struct {
	lastSeen  time.Time
	address   *PssAddress
	protected bool
}

//
type PssParams struct {
	MsgTTL              time.Duration
	CacheTTL            time.Duration
	privateKey          *ecdsa.PrivateKey
	SymKeyCacheCapacity int
AllowRaw            bool //
}

//
func NewPssParams() *PssParams {
	return &PssParams{
		MsgTTL:              DefaultMsgTTL,
		CacheTTL:            defaultDigestCacheTTL,
		SymKeyCacheCapacity: defaultSymKeyCacheCapacity,
	}
}

func (params *PssParams) WithPrivateKey(privatekey *ecdsa.PrivateKey) *PssParams {
	params.privateKey = privatekey
	return params
}

//
//
//
type Pss struct {
network.Overlay                   //
privateKey      *ecdsa.PrivateKey //
w               *whisper.Whisper  //
auxAPIs         []rpc.API         //

//
fwdPool         map[string]*protocols.Peer //
	fwdPoolMu       sync.RWMutex
fwdCache        map[pssDigest]pssCacheEntry //
	fwdCacheMu      sync.RWMutex
cacheTTL        time.Duration //
	msgTTL          time.Duration
	paddingByteSize int
	capstring       string
	outbox          chan *PssMsg

//
pubKeyPool                 map[string]map[Topic]*pssPeer //
	pubKeyPoolMu               sync.RWMutex
symKeyPool                 map[string]map[Topic]*pssPeer //
	symKeyPoolMu               sync.RWMutex
symKeyDecryptCache         []*string //
symKeyDecryptCacheCursor   int       //
symKeyDecryptCacheCapacity int       //

//
handlers   map[Topic]map[*Handler]bool //
	handlersMu sync.RWMutex
	allowRaw   bool
	hashPool   sync.Pool

//
	quitC chan struct{}
}

func (p *Pss) String() string {
	return fmt.Sprintf("pss: addr %x, pubkey %v", p.BaseAddr(), common.ToHex(crypto.FromECDSAPub(&p.privateKey.PublicKey)))
}

//
//
//
//
func NewPss(k network.Overlay, params *PssParams) (*Pss, error) {
	if params.privateKey == nil {
		return nil, errors.New("missing private key for pss")
	}
	cap := p2p.Cap{
		Name:    pssProtocolName,
		Version: pssVersion,
	}
	ps := &Pss{
		Overlay:    k,
		privateKey: params.privateKey,
		w:          whisper.New(&whisper.DefaultConfig),
		quitC:      make(chan struct{}),

		fwdPool:         make(map[string]*protocols.Peer),
		fwdCache:        make(map[pssDigest]pssCacheEntry),
		cacheTTL:        params.CacheTTL,
		msgTTL:          params.MsgTTL,
		paddingByteSize: defaultPaddingByteSize,
		capstring:       cap.String(),
		outbox:          make(chan *PssMsg, defaultOutboxCapacity),

		pubKeyPool:                 make(map[string]map[Topic]*pssPeer),
		symKeyPool:                 make(map[string]map[Topic]*pssPeer),
		symKeyDecryptCache:         make([]*string, params.SymKeyCacheCapacity),
		symKeyDecryptCacheCapacity: params.SymKeyCacheCapacity,

		handlers: make(map[Topic]map[*Handler]bool),
		allowRaw: params.AllowRaw,
		hashPool: sync.Pool{
			New: func() interface{} {
				return storage.MakeHashFunc(storage.DefaultHash)()
			},
		},
	}

	for i := 0; i < hasherCount; i++ {
		hashfunc := storage.MakeHashFunc(storage.DefaultHash)()
		ps.hashPool.Put(hashfunc)
	}

	return ps, nil
}

//
//
//

func (p *Pss) Start(srv *p2p.Server) error {
	go func() {
		ticker := time.NewTicker(defaultCleanInterval)
		cacheTicker := time.NewTicker(p.cacheTTL)
		defer ticker.Stop()
		defer cacheTicker.Stop()
		for {
			select {
			case <-cacheTicker.C:
				p.cleanFwdCache()
			case <-ticker.C:
				p.cleanKeys()
			case <-p.quitC:
				return
			}
		}
	}()
	go func() {
		for {
			select {
			case msg := <-p.outbox:
				err := p.forward(msg)
				if err != nil {
					log.Error(err.Error())
					metrics.GetOrRegisterCounter("pss.forward.err", nil).Inc(1)
				}
			case <-p.quitC:
				return
			}
		}
	}()
	log.Info("Started Pss")
	log.Info("Loaded EC keys", "pubkey", common.ToHex(crypto.FromECDSAPub(p.PublicKey())), "secp256", common.ToHex(crypto.CompressPubkey(p.PublicKey())))
	return nil
}

func (p *Pss) Stop() error {
	log.Info("Pss shutting down")
	close(p.quitC)
	return nil
}

var pssSpec = &protocols.Spec{
	Name:       pssProtocolName,
	Version:    pssVersion,
	MaxMsgSize: defaultMaxMsgSize,
	Messages: []interface{}{
		PssMsg{},
	},
}

func (p *Pss) Protocols() []p2p.Protocol {
	return []p2p.Protocol{
		{
			Name:    pssSpec.Name,
			Version: pssSpec.Version,
			Length:  pssSpec.Length(),
			Run:     p.Run,
		},
	}
}

func (p *Pss) Run(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
	pp := protocols.NewPeer(peer, rw, pssSpec)
	p.fwdPoolMu.Lock()
	p.fwdPool[peer.Info().ID] = pp
	p.fwdPoolMu.Unlock()
	return pp.Run(p.handlePssMsg)
}

func (p *Pss) APIs() []rpc.API {
	apis := []rpc.API{
		{
			Namespace: "pss",
			Version:   "1.0",
			Service:   NewAPI(p),
			Public:    true,
		},
	}
	apis = append(apis, p.auxAPIs...)
	return apis
}

//
//
func (p *Pss) addAPI(api rpc.API) {
	p.auxAPIs = append(p.auxAPIs, api)
}

//
func (p *Pss) BaseAddr() []byte {
	return p.Overlay.BaseAddr()
}

//
func (p *Pss) PublicKey() *ecdsa.PublicKey {
	return &p.privateKey.PublicKey
}

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
func (p *Pss) Register(topic *Topic, handler Handler) func() {
	p.handlersMu.Lock()
	defer p.handlersMu.Unlock()
	handlers := p.handlers[*topic]
	if handlers == nil {
		handlers = make(map[*Handler]bool)
		p.handlers[*topic] = handlers
	}
	handlers[&handler] = true
	return func() { p.deregister(topic, &handler) }
}
func (p *Pss) deregister(topic *Topic, h *Handler) {
	p.handlersMu.Lock()
	defer p.handlersMu.Unlock()
	handlers := p.handlers[*topic]
	if len(handlers) == 1 {
		delete(p.handlers, *topic)
		return
	}
	delete(handlers, h)
}

//
func (p *Pss) getHandlers(topic Topic) map[*Handler]bool {
	p.handlersMu.RLock()
	defer p.handlersMu.RUnlock()
	return p.handlers[topic]
}

//
//
//
//
func (p *Pss) handlePssMsg(ctx context.Context, msg interface{}) error {
	metrics.GetOrRegisterCounter("pss.handlepssmsg", nil).Inc(1)

	pssmsg, ok := msg.(*PssMsg)

	if !ok {
		return fmt.Errorf("invalid message type. Expected *PssMsg, got %T ", msg)
	}
	if int64(pssmsg.Expire) < time.Now().Unix() {
		metrics.GetOrRegisterCounter("pss.expire", nil).Inc(1)
		log.Warn("pss filtered expired message", "from", common.ToHex(p.Overlay.BaseAddr()), "to", common.ToHex(pssmsg.To))
		return nil
	}
	if p.checkFwdCache(pssmsg) {
		log.Trace("pss relay block-cache match (process)", "from", common.ToHex(p.Overlay.BaseAddr()), "to", (common.ToHex(pssmsg.To)))
		return nil
	}
	p.addFwdCache(pssmsg)

	if !p.isSelfPossibleRecipient(pssmsg) {
		log.Trace("pss was for someone else :'( ... forwarding", "pss", common.ToHex(p.BaseAddr()))
		return p.enqueue(pssmsg)
	}

	log.Trace("pss for us, yay! ... let's process!", "pss", common.ToHex(p.BaseAddr()))
	if err := p.process(pssmsg); err != nil {
		qerr := p.enqueue(pssmsg)
		if qerr != nil {
			return fmt.Errorf("process fail: processerr %v, queueerr: %v", err, qerr)
		}
	}
	return nil

}

//
//
//
func (p *Pss) process(pssmsg *PssMsg) error {
	metrics.GetOrRegisterCounter("pss.process", nil).Inc(1)

	var err error
	var recvmsg *whisper.ReceivedMessage
	var payload []byte
	var from *PssAddress
	var asymmetric bool
	var keyid string
	var keyFunc func(envelope *whisper.Envelope) (*whisper.ReceivedMessage, string, *PssAddress, error)

	envelope := pssmsg.Payload
	psstopic := Topic(envelope.Topic)
	if pssmsg.isRaw() {
		if !p.allowRaw {
			return errors.New("raw message support disabled")
		}
		payload = pssmsg.Payload.Data
	} else {
		if pssmsg.isSym() {
			keyFunc = p.processSym
		} else {
			asymmetric = true
			keyFunc = p.processAsym
		}

		recvmsg, keyid, from, err = keyFunc(envelope)
		if err != nil {
			return errors.New("Decryption failed")
		}
		payload = recvmsg.Payload
	}

	if len(pssmsg.To) < addressLength {
		if err := p.enqueue(pssmsg); err != nil {
			return err
		}
	}
	p.executeHandlers(psstopic, payload, from, asymmetric, keyid)

	return nil

}

func (p *Pss) executeHandlers(topic Topic, payload []byte, from *PssAddress, asymmetric bool, keyid string) {
	handlers := p.getHandlers(topic)
nid, _ := discover.HexID("0x00") //
	peer := p2p.NewPeer(nid, fmt.Sprintf("%x", from), []p2p.Cap{})
	for f := range handlers {
		err := (*f)(payload, peer, asymmetric, keyid)
		if err != nil {
			log.Warn("Pss handler %p failed: %v", f, err)
		}
	}
}

//
func (p *Pss) isSelfRecipient(msg *PssMsg) bool {
	return bytes.Equal(msg.To, p.Overlay.BaseAddr())
}

//
func (p *Pss) isSelfPossibleRecipient(msg *PssMsg) bool {
	local := p.Overlay.BaseAddr()
	return bytes.Equal(msg.To[:], local[:len(msg.To)])
}

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
func (p *Pss) SetPeerPublicKey(pubkey *ecdsa.PublicKey, topic Topic, address *PssAddress) error {
	pubkeybytes := crypto.FromECDSAPub(pubkey)
	if len(pubkeybytes) == 0 {
		return fmt.Errorf("invalid public key: %v", pubkey)
	}
	pubkeyid := common.ToHex(pubkeybytes)
	psp := &pssPeer{
		address: address,
	}
	p.pubKeyPoolMu.Lock()
	if _, ok := p.pubKeyPool[pubkeyid]; !ok {
		p.pubKeyPool[pubkeyid] = make(map[Topic]*pssPeer)
	}
	p.pubKeyPool[pubkeyid][topic] = psp
	p.pubKeyPoolMu.Unlock()
	log.Trace("added pubkey", "pubkeyid", pubkeyid, "topic", topic, "address", common.ToHex(*address))
	return nil
}

//
func (p *Pss) GenerateSymmetricKey(topic Topic, address *PssAddress, addToCache bool) (string, error) {
	keyid, err := p.w.GenerateSymKey()
	if err != nil {
		return "", err
	}
	p.addSymmetricKeyToPool(keyid, topic, address, addToCache, false)
	return keyid, nil
}

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
func (p *Pss) SetSymmetricKey(key []byte, topic Topic, address *PssAddress, addtocache bool) (string, error) {
	return p.setSymmetricKey(key, topic, address, addtocache, true)
}

func (p *Pss) setSymmetricKey(key []byte, topic Topic, address *PssAddress, addtocache bool, protected bool) (string, error) {
	keyid, err := p.w.AddSymKeyDirect(key)
	if err != nil {
		return "", err
	}
	p.addSymmetricKeyToPool(keyid, topic, address, addtocache, protected)
	return keyid, nil
}

//
//
//
func (p *Pss) addSymmetricKeyToPool(keyid string, topic Topic, address *PssAddress, addtocache bool, protected bool) {
	psp := &pssPeer{
		address:   address,
		protected: protected,
	}
	p.symKeyPoolMu.Lock()
	if _, ok := p.symKeyPool[keyid]; !ok {
		p.symKeyPool[keyid] = make(map[Topic]*pssPeer)
	}
	p.symKeyPool[keyid][topic] = psp
	p.symKeyPoolMu.Unlock()
	if addtocache {
		p.symKeyDecryptCacheCursor++
		p.symKeyDecryptCache[p.symKeyDecryptCacheCursor%cap(p.symKeyDecryptCache)] = &keyid
	}
	key, _ := p.GetSymmetricKey(keyid)
	log.Trace("added symkey", "symkeyid", keyid, "symkey", common.ToHex(key), "topic", topic, "address", fmt.Sprintf("%p", address), "cache", addtocache)
}

//
//
//
//
func (p *Pss) GetSymmetricKey(symkeyid string) ([]byte, error) {
	symkey, err := p.w.GetSymKey(symkeyid)
	if err != nil {
		return nil, err
	}
	return symkey, nil
}

//
func (p *Pss) GetPublickeyPeers(keyid string) (topic []Topic, address []PssAddress, err error) {
	p.pubKeyPoolMu.RLock()
	defer p.pubKeyPoolMu.RUnlock()
	for t, peer := range p.pubKeyPool[keyid] {
		topic = append(topic, t)
		address = append(address, *peer.address)
	}

	return topic, address, nil
}

func (p *Pss) getPeerAddress(keyid string, topic Topic) (PssAddress, error) {
	p.pubKeyPoolMu.RLock()
	defer p.pubKeyPoolMu.RUnlock()
	if peers, ok := p.pubKeyPool[keyid]; ok {
		if t, ok := peers[topic]; ok {
			return *t.address, nil
		}
	}
	return nil, fmt.Errorf("peer with pubkey %s, topic %x not found", keyid, topic)
}

//
//
//
//
//
//
func (p *Pss) processSym(envelope *whisper.Envelope) (*whisper.ReceivedMessage, string, *PssAddress, error) {
	metrics.GetOrRegisterCounter("pss.process.sym", nil).Inc(1)

	for i := p.symKeyDecryptCacheCursor; i > p.symKeyDecryptCacheCursor-cap(p.symKeyDecryptCache) && i > 0; i-- {
		symkeyid := p.symKeyDecryptCache[i%cap(p.symKeyDecryptCache)]
		symkey, err := p.w.GetSymKey(*symkeyid)
		if err != nil {
			continue
		}
		recvmsg, err := envelope.OpenSymmetric(symkey)
		if err != nil {
			continue
		}
		if !recvmsg.Validate() {
			return nil, "", nil, fmt.Errorf("symmetrically encrypted message has invalid signature or is corrupt")
		}
		p.symKeyPoolMu.Lock()
		from := p.symKeyPool[*symkeyid][Topic(envelope.Topic)].address
		p.symKeyPoolMu.Unlock()
		p.symKeyDecryptCacheCursor++
		p.symKeyDecryptCache[p.symKeyDecryptCacheCursor%cap(p.symKeyDecryptCache)] = symkeyid
		return recvmsg, *symkeyid, from, nil
	}
	return nil, "", nil, fmt.Errorf("could not decrypt message")
}

//
//
//
//
//
//
func (p *Pss) processAsym(envelope *whisper.Envelope) (*whisper.ReceivedMessage, string, *PssAddress, error) {
	metrics.GetOrRegisterCounter("pss.process.asym", nil).Inc(1)

	recvmsg, err := envelope.OpenAsymmetric(p.privateKey)
	if err != nil {
		return nil, "", nil, fmt.Errorf("could not decrypt message: %s", err)
	}
//
	if !recvmsg.Validate() {
		return nil, "", nil, fmt.Errorf("invalid message")
	}
	pubkeyid := common.ToHex(crypto.FromECDSAPub(recvmsg.Src))
	var from *PssAddress
	p.pubKeyPoolMu.Lock()
	if p.pubKeyPool[pubkeyid][Topic(envelope.Topic)] != nil {
		from = p.pubKeyPool[pubkeyid][Topic(envelope.Topic)].address
	}
	p.pubKeyPoolMu.Unlock()
	return recvmsg, pubkeyid, from, nil
}

//
//
//
//
func (p *Pss) cleanKeys() (count int) {
	for keyid, peertopics := range p.symKeyPool {
		var expiredtopics []Topic
		for topic, psp := range peertopics {
			if psp.protected {
				continue
			}

			var match bool
			for i := p.symKeyDecryptCacheCursor; i > p.symKeyDecryptCacheCursor-cap(p.symKeyDecryptCache) && i > 0; i-- {
				cacheid := p.symKeyDecryptCache[i%cap(p.symKeyDecryptCache)]
				if *cacheid == keyid {
					match = true
				}
			}
			if !match {
				expiredtopics = append(expiredtopics, topic)
			}
		}
		for _, topic := range expiredtopics {
			p.symKeyPoolMu.Lock()
			delete(p.symKeyPool[keyid], topic)
			log.Trace("symkey cleanup deletion", "symkeyid", keyid, "topic", topic, "val", p.symKeyPool[keyid])
			p.symKeyPoolMu.Unlock()
			count++
		}
	}
	return
}

//
//
//

func (p *Pss) enqueue(msg *PssMsg) error {
	select {
	case p.outbox <- msg:
		return nil
	default:
	}

	metrics.GetOrRegisterCounter("pss.enqueue.outbox.full", nil).Inc(1)
	return errors.New("outbox full")
}

//
//
//
func (p *Pss) SendRaw(address PssAddress, topic Topic, msg []byte) error {
	if !p.allowRaw {
		return errors.New("Raw messages not enabled")
	}
	pssMsgParams := &msgParams{
		raw: true,
	}
	payload := &whisper.Envelope{
		Data:  msg,
		Topic: whisper.TopicType(topic),
	}
	pssMsg := newPssMsg(pssMsgParams)
	pssMsg.To = address
	pssMsg.Expire = uint32(time.Now().Add(p.msgTTL).Unix())
	pssMsg.Payload = payload
	p.addFwdCache(pssMsg)
	return p.enqueue(pssMsg)
}

//
//
//
func (p *Pss) SendSym(symkeyid string, topic Topic, msg []byte) error {
	symkey, err := p.GetSymmetricKey(symkeyid)
	if err != nil {
		return fmt.Errorf("missing valid send symkey %s: %v", symkeyid, err)
	}
	p.symKeyPoolMu.Lock()
	psp, ok := p.symKeyPool[symkeyid][topic]
	p.symKeyPoolMu.Unlock()
	if !ok {
		return fmt.Errorf("invalid topic '%s' for symkey '%s'", topic.String(), symkeyid)
	} else if psp.address == nil {
		return fmt.Errorf("no address hint for topic '%s' symkey '%s'", topic.String(), symkeyid)
	}
	err = p.send(*psp.address, topic, msg, false, symkey)
	return err
}

//
//
//
func (p *Pss) SendAsym(pubkeyid string, topic Topic, msg []byte) error {
	if _, err := crypto.UnmarshalPubkey(common.FromHex(pubkeyid)); err != nil {
		return fmt.Errorf("Cannot unmarshal pubkey: %x", pubkeyid)
	}
	p.pubKeyPoolMu.Lock()
	psp, ok := p.pubKeyPool[pubkeyid][topic]
	p.pubKeyPoolMu.Unlock()
	if !ok {
		return fmt.Errorf("invalid topic '%s' for pubkey '%s'", topic.String(), pubkeyid)
	} else if psp.address == nil {
		return fmt.Errorf("no address hint for topic '%s' pubkey '%s'", topic.String(), pubkeyid)
	}
	go func() {
		p.send(*psp.address, topic, msg, true, common.FromHex(pubkeyid))
	}()
	return nil
}

//
//
//
//
func (p *Pss) send(to []byte, topic Topic, msg []byte, asymmetric bool, key []byte) error {
	metrics.GetOrRegisterCounter("pss.send", nil).Inc(1)

	if key == nil || bytes.Equal(key, []byte{}) {
		return fmt.Errorf("Zero length key passed to pss send")
	}
	padding := make([]byte, p.paddingByteSize)
	c, err := rand.Read(padding)
	if err != nil {
		return err
	} else if c < p.paddingByteSize {
		return fmt.Errorf("invalid padding length: %d", c)
	}
	wparams := &whisper.MessageParams{
		TTL:      defaultWhisperTTL,
		Src:      p.privateKey,
		Topic:    whisper.TopicType(topic),
		WorkTime: defaultWhisperWorkTime,
		PoW:      defaultWhisperPoW,
		Payload:  msg,
		Padding:  padding,
	}
	if asymmetric {
		pk, err := crypto.UnmarshalPubkey(key)
		if err != nil {
			return fmt.Errorf("Cannot unmarshal pubkey: %x", key)
		}
		wparams.Dst = pk
	} else {
		wparams.KeySym = key
	}
//
	woutmsg, err := whisper.NewSentMessage(wparams)
	if err != nil {
		return fmt.Errorf("failed to generate whisper message encapsulation: %v", err)
	}
//
//
//
	envelope, err := woutmsg.Wrap(wparams)
	if err != nil {
		return fmt.Errorf("failed to perform whisper encryption: %v", err)
	}
	log.Trace("pssmsg whisper done", "env", envelope, "wparams payload", common.ToHex(wparams.Payload), "to", common.ToHex(to), "asym", asymmetric, "key", common.ToHex(key))

//
	pssMsgParams := &msgParams{
		sym: !asymmetric,
	}
	pssMsg := newPssMsg(pssMsgParams)
	pssMsg.To = to
	pssMsg.Expire = uint32(time.Now().Add(p.msgTTL).Unix())
	pssMsg.Payload = envelope
	return p.enqueue(pssMsg)
}

//
//
//
func (p *Pss) forward(msg *PssMsg) error {
	metrics.GetOrRegisterCounter("pss.forward", nil).Inc(1)

	to := make([]byte, addressLength)
	copy(to[:len(msg.To)], msg.To)

//
//
	sent := 0
	p.Overlay.EachConn(to, 256, func(op network.OverlayConn, po int, isproxbin bool) bool {
//
//
		sp, ok := op.(senderPeer)
		if !ok {
			log.Crit("Pss cannot use kademlia peer type")
			return false
		}
		info := sp.Info()

//
		var ispss bool
		for _, cap := range info.Caps {
			if cap == p.capstring {
				ispss = true
				break
			}
		}
		if !ispss {
			log.Trace("peer doesn't have matching pss capabilities, skipping", "peer", info.Name, "caps", info.Caps)
			return true
		}

//
		sendMsg := fmt.Sprintf("MSG TO %x FROM %x VIA %x", to, p.BaseAddr(), op.Address())
		p.fwdPoolMu.RLock()
		pp := p.fwdPool[sp.Info().ID]
		p.fwdPoolMu.RUnlock()

//
		err := pp.Send(context.TODO(), msg)
		if err != nil {
			metrics.GetOrRegisterCounter("pss.pp.send.error", nil).Inc(1)
			log.Error(err.Error())
			return true
		}
		sent++
		log.Trace(fmt.Sprintf("%v: successfully forwarded", sendMsg))

//
//
//
//
		if len(msg.To) < addressLength && bytes.Equal(msg.To, op.Address()[:len(msg.To)]) {
			log.Trace(fmt.Sprintf("Pss keep forwarding: Partial address + full partial match"))
			return true
		} else if isproxbin {
			log.Trace(fmt.Sprintf("%x is in proxbin, keep forwarding", common.ToHex(op.Address())))
			return true
		}
//
//
//
//
		return false
	})

	if sent == 0 {
		log.Debug("unable to forward to any peers")
		if err := p.enqueue(msg); err != nil {
			metrics.GetOrRegisterCounter("pss.forward.enqueue.error", nil).Inc(1)
			log.Error(err.Error())
			return err
		}
	}

//
	p.addFwdCache(msg)
	return nil
}

//
//
//

//
func (p *Pss) cleanFwdCache() {
	metrics.GetOrRegisterCounter("pss.cleanfwdcache", nil).Inc(1)
	p.fwdCacheMu.Lock()
	defer p.fwdCacheMu.Unlock()
	for k, v := range p.fwdCache {
		if v.expiresAt.Before(time.Now()) {
			delete(p.fwdCache, k)
		}
	}
}

//
func (p *Pss) addFwdCache(msg *PssMsg) error {
	metrics.GetOrRegisterCounter("pss.addfwdcache", nil).Inc(1)

	var entry pssCacheEntry
	var ok bool

	p.fwdCacheMu.Lock()
	defer p.fwdCacheMu.Unlock()

	digest := p.digest(msg)
	if entry, ok = p.fwdCache[digest]; !ok {
		entry = pssCacheEntry{}
	}
	entry.expiresAt = time.Now().Add(p.cacheTTL)
	p.fwdCache[digest] = entry
	return nil
}

//
func (p *Pss) checkFwdCache(msg *PssMsg) bool {
	p.fwdCacheMu.Lock()
	defer p.fwdCacheMu.Unlock()

	digest := p.digest(msg)
	entry, ok := p.fwdCache[digest]
	if ok {
		if entry.expiresAt.After(time.Now()) {
			log.Trace("unexpired cache", "digest", fmt.Sprintf("%x", digest))
			metrics.GetOrRegisterCounter("pss.checkfwdcache.unexpired", nil).Inc(1)
			return true
		}
		metrics.GetOrRegisterCounter("pss.checkfwdcache.expired", nil).Inc(1)
	}
	return false
}

//
func (p *Pss) digest(msg *PssMsg) pssDigest {
	hasher := p.hashPool.Get().(storage.SwarmHash)
	defer p.hashPool.Put(hasher)
	hasher.Reset()
	hasher.Write(msg.serialize())
	digest := pssDigest{}
	key := hasher.Sum(nil)
	copy(digest[:], key[:digestLength])
	return digest
}
