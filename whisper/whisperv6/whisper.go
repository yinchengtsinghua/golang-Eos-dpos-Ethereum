
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
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/sync/syncmap"
)

//
//
type Statistics struct {
	messagesCleared      int
	memoryCleared        int
	memoryUsed           int
	cycles               int
	totalMessagesCleared int
}

const (
maxMsgSizeIdx           = iota //
overflowIdx                    //
minPowIdx                      //
minPowToleranceIdx             //
bloomFilterIdx                 //
bloomFilterToleranceIdx        //
)

//
//
type Whisper struct {
protocol p2p.Protocol //
filters  *Filters     //

privateKeys map[string]*ecdsa.PrivateKey //
symKeys     map[string][]byte            //
keyMu       sync.RWMutex                 //

poolMu      sync.RWMutex              //
envelopes   map[common.Hash]*Envelope //
expirations map[uint32]mapset.Set     //

peerMu sync.RWMutex       //
peers  map[*Peer]struct{} //

messageQueue chan *Envelope //
p2pMsgQueue  chan *Envelope //
quit         chan struct{}  //

settings syncmap.Map //

syncAllowance int //

lightClient bool //

statsMu sync.Mutex //
stats   Statistics //

mailServer MailServer //
}

//
func New(cfg *Config) *Whisper {
	if cfg == nil {
		cfg = &DefaultConfig
	}

	whisper := &Whisper{
		privateKeys:   make(map[string]*ecdsa.PrivateKey),
		symKeys:       make(map[string][]byte),
		envelopes:     make(map[common.Hash]*Envelope),
		expirations:   make(map[uint32]mapset.Set),
		peers:         make(map[*Peer]struct{}),
		messageQueue:  make(chan *Envelope, messageQueueLimit),
		p2pMsgQueue:   make(chan *Envelope, messageQueueLimit),
		quit:          make(chan struct{}),
		syncAllowance: DefaultSyncAllowance,
	}

	whisper.filters = NewFilters(whisper)

	whisper.settings.Store(minPowIdx, cfg.MinimumAcceptedPOW)
	whisper.settings.Store(maxMsgSizeIdx, cfg.MaxMessageSize)
	whisper.settings.Store(overflowIdx, false)

//
	whisper.protocol = p2p.Protocol{
		Name:    ProtocolName,
		Version: uint(ProtocolVersion),
		Length:  NumberOfMessageCodes,
		Run:     whisper.HandlePeer,
		NodeInfo: func() interface{} {
			return map[string]interface{}{
				"version":        ProtocolVersionStr,
				"maxMessageSize": whisper.MaxMessageSize(),
				"minimumPoW":     whisper.MinPow(),
			}
		},
	}

	return whisper
}

//
func (whisper *Whisper) MinPow() float64 {
	val, exist := whisper.settings.Load(minPowIdx)
	if !exist || val == nil {
		return DefaultMinimumPoW
	}
	v, ok := val.(float64)
	if !ok {
		log.Error("Error loading minPowIdx, using default")
		return DefaultMinimumPoW
	}
	return v
}

//
//
//
func (whisper *Whisper) MinPowTolerance() float64 {
	val, exist := whisper.settings.Load(minPowToleranceIdx)
	if !exist || val == nil {
		return DefaultMinimumPoW
	}
	return val.(float64)
}

//
//
//
//
func (whisper *Whisper) BloomFilter() []byte {
	val, exist := whisper.settings.Load(bloomFilterIdx)
	if !exist || val == nil {
		return nil
	}
	return val.([]byte)
}

//
//
//
//
func (whisper *Whisper) BloomFilterTolerance() []byte {
	val, exist := whisper.settings.Load(bloomFilterToleranceIdx)
	if !exist || val == nil {
		return nil
	}
	return val.([]byte)
}

//
func (whisper *Whisper) MaxMessageSize() uint32 {
	val, _ := whisper.settings.Load(maxMsgSizeIdx)
	return val.(uint32)
}

//
func (whisper *Whisper) Overflow() bool {
	val, _ := whisper.settings.Load(overflowIdx)
	return val.(bool)
}

//
func (whisper *Whisper) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: ProtocolName,
			Version:   ProtocolVersionStr,
			Service:   NewPublicWhisperAPI(whisper),
			Public:    true,
		},
	}
}

//
//
func (whisper *Whisper) RegisterServer(server MailServer) {
	whisper.mailServer = server
}

//
func (whisper *Whisper) Protocols() []p2p.Protocol {
	return []p2p.Protocol{whisper.protocol}
}

//
func (whisper *Whisper) Version() uint {
	return whisper.protocol.Version
}

//
func (whisper *Whisper) SetMaxMessageSize(size uint32) error {
	if size > MaxMessageSize {
		return fmt.Errorf("message size too large [%d>%d]", size, MaxMessageSize)
	}
	whisper.settings.Store(maxMsgSizeIdx, size)
	return nil
}

//
func (whisper *Whisper) SetBloomFilter(bloom []byte) error {
	if len(bloom) != BloomFilterSize {
		return fmt.Errorf("invalid bloom filter size: %d", len(bloom))
	}

	b := make([]byte, BloomFilterSize)
	copy(b, bloom)

	whisper.settings.Store(bloomFilterIdx, b)
	whisper.notifyPeersAboutBloomFilterChange(b)

	go func() {
//
		time.Sleep(time.Duration(whisper.syncAllowance) * time.Second)
		whisper.settings.Store(bloomFilterToleranceIdx, b)
	}()

	return nil
}

//
func (whisper *Whisper) SetMinimumPoW(val float64) error {
	if val < 0.0 {
		return fmt.Errorf("invalid PoW: %f", val)
	}

	whisper.settings.Store(minPowIdx, val)
	whisper.notifyPeersAboutPowRequirementChange(val)

	go func() {
//
		time.Sleep(time.Duration(whisper.syncAllowance) * time.Second)
		whisper.settings.Store(minPowToleranceIdx, val)
	}()

	return nil
}

//
func (whisper *Whisper) SetMinimumPowTest(val float64) {
	whisper.settings.Store(minPowIdx, val)
	whisper.notifyPeersAboutPowRequirementChange(val)
	whisper.settings.Store(minPowToleranceIdx, val)
}

func (whisper *Whisper) notifyPeersAboutPowRequirementChange(pow float64) {
	arr := whisper.getPeers()
	for _, p := range arr {
		err := p.notifyAboutPowRequirementChange(pow)
		if err != nil {
//
			err = p.notifyAboutPowRequirementChange(pow)
		}
		if err != nil {
			log.Warn("failed to notify peer about new pow requirement", "peer", p.ID(), "error", err)
		}
	}
}

func (whisper *Whisper) notifyPeersAboutBloomFilterChange(bloom []byte) {
	arr := whisper.getPeers()
	for _, p := range arr {
		err := p.notifyAboutBloomFilterChange(bloom)
		if err != nil {
//
			err = p.notifyAboutBloomFilterChange(bloom)
		}
		if err != nil {
			log.Warn("failed to notify peer about new bloom filter", "peer", p.ID(), "error", err)
		}
	}
}

func (whisper *Whisper) getPeers() []*Peer {
	arr := make([]*Peer, len(whisper.peers))
	i := 0
	whisper.peerMu.Lock()
	for p := range whisper.peers {
		arr[i] = p
		i++
	}
	whisper.peerMu.Unlock()
	return arr
}

//
func (whisper *Whisper) getPeer(peerID []byte) (*Peer, error) {
	whisper.peerMu.Lock()
	defer whisper.peerMu.Unlock()
	for p := range whisper.peers {
		id := p.peer.ID()
		if bytes.Equal(peerID, id[:]) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("Could not find peer with ID: %x", peerID)
}

//
//
func (whisper *Whisper) AllowP2PMessagesFromPeer(peerID []byte) error {
	p, err := whisper.getPeer(peerID)
	if err != nil {
		return err
	}
	p.trusted = true
	return nil
}

//
//
//
//
//
func (whisper *Whisper) RequestHistoricMessages(peerID []byte, envelope *Envelope) error {
	p, err := whisper.getPeer(peerID)
	if err != nil {
		return err
	}
	p.trusted = true
	return p2p.Send(p.ws, p2pRequestCode, envelope)
}

//
func (whisper *Whisper) SendP2PMessage(peerID []byte, envelope *Envelope) error {
	p, err := whisper.getPeer(peerID)
	if err != nil {
		return err
	}
	return whisper.SendP2PDirect(p, envelope)
}

//
func (whisper *Whisper) SendP2PDirect(peer *Peer, envelope *Envelope) error {
	return p2p.Send(peer.ws, p2pMessageCode, envelope)
}

//
//
func (whisper *Whisper) NewKeyPair() (string, error) {
	key, err := crypto.GenerateKey()
	if err != nil || !validatePrivateKey(key) {
key, err = crypto.GenerateKey() //
	}
	if err != nil {
		return "", err
	}
	if !validatePrivateKey(key) {
		return "", fmt.Errorf("failed to generate valid key")
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

	if whisper.privateKeys[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	whisper.privateKeys[id] = key
	return id, nil
}

//
func (whisper *Whisper) DeleteKeyPair(key string) bool {
	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

	if whisper.privateKeys[key] != nil {
		delete(whisper.privateKeys, key)
		return true
	}
	return false
}

//
func (whisper *Whisper) AddKeyPair(key *ecdsa.PrivateKey) (string, error) {
	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	whisper.keyMu.Lock()
	whisper.privateKeys[id] = key
	whisper.keyMu.Unlock()

	return id, nil
}

//
//
func (whisper *Whisper) HasKeyPair(id string) bool {
	whisper.keyMu.RLock()
	defer whisper.keyMu.RUnlock()
	return whisper.privateKeys[id] != nil
}

//
func (whisper *Whisper) GetPrivateKey(id string) (*ecdsa.PrivateKey, error) {
	whisper.keyMu.RLock()
	defer whisper.keyMu.RUnlock()
	key := whisper.privateKeys[id]
	if key == nil {
		return nil, fmt.Errorf("invalid id")
	}
	return key, nil
}

//
//
func (whisper *Whisper) GenerateSymKey() (string, error) {
	key, err := generateSecureRandomData(aesKeyLength)
	if err != nil {
		return "", err
	} else if !validateDataIntegrity(key, aesKeyLength) {
		return "", fmt.Errorf("error in GenerateSymKey: crypto/rand failed to generate random data")
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

	if whisper.symKeys[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	whisper.symKeys[id] = key
	return id, nil
}

//
func (whisper *Whisper) AddSymKeyDirect(key []byte) (string, error) {
	if len(key) != aesKeyLength {
		return "", fmt.Errorf("wrong key size: %d", len(key))
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

	if whisper.symKeys[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	whisper.symKeys[id] = key
	return id, nil
}

//
func (whisper *Whisper) AddSymKeyFromPassword(password string) (string, error) {
	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}
	if whisper.HasSymKey(id) {
		return "", fmt.Errorf("failed to generate unique ID")
	}

//
//
	derived := pbkdf2.Key([]byte(password), nil, 65356, aesKeyLength, sha256.New)
	if err != nil {
		return "", err
	}

	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

//
	if whisper.symKeys[id] != nil {
		return "", fmt.Errorf("critical error: failed to generate unique ID")
	}
	whisper.symKeys[id] = derived
	return id, nil
}

//
//
func (whisper *Whisper) HasSymKey(id string) bool {
	whisper.keyMu.RLock()
	defer whisper.keyMu.RUnlock()
	return whisper.symKeys[id] != nil
}

//
func (whisper *Whisper) DeleteSymKey(id string) bool {
	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()
	if whisper.symKeys[id] != nil {
		delete(whisper.symKeys, id)
		return true
	}
	return false
}

//
func (whisper *Whisper) GetSymKey(id string) ([]byte, error) {
	whisper.keyMu.RLock()
	defer whisper.keyMu.RUnlock()
	if whisper.symKeys[id] != nil {
		return whisper.symKeys[id], nil
	}
	return nil, fmt.Errorf("non-existent key ID")
}

//
//
func (whisper *Whisper) Subscribe(f *Filter) (string, error) {
	s, err := whisper.filters.Install(f)
	if err == nil {
		whisper.updateBloomFilter(f)
	}
	return s, err
}

//
//
func (whisper *Whisper) updateBloomFilter(f *Filter) {
	aggregate := make([]byte, BloomFilterSize)
	for _, t := range f.Topics {
		top := BytesToTopic(t)
		b := TopicToBloom(top)
		aggregate = addBloom(aggregate, b)
	}

	if !BloomFilterMatch(whisper.BloomFilter(), aggregate) {
//
		aggregate = addBloom(whisper.BloomFilter(), aggregate)
		whisper.SetBloomFilter(aggregate)
	}
}

//
func (whisper *Whisper) GetFilter(id string) *Filter {
	return whisper.filters.Get(id)
}

//
func (whisper *Whisper) Unsubscribe(id string) error {
	ok := whisper.filters.Uninstall(id)
	if !ok {
		return fmt.Errorf("Unsubscribe: Invalid ID")
	}
	return nil
}

//
//
func (whisper *Whisper) Send(envelope *Envelope) error {
	ok, err := whisper.add(envelope, false)
	if err == nil && !ok {
		return fmt.Errorf("failed to add envelope")
	}
	return err
}

//
//
func (whisper *Whisper) Start(*p2p.Server) error {
	log.Info("started whisper v." + ProtocolVersionStr)
	go whisper.update()

	numCPU := runtime.NumCPU()
	for i := 0; i < numCPU; i++ {
		go whisper.processQueue()
	}

	return nil
}

//
//
func (whisper *Whisper) Stop() error {
	close(whisper.quit)
	log.Info("whisper stopped")
	return nil
}

//
//
func (whisper *Whisper) HandlePeer(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
//
	whisperPeer := newPeer(whisper, peer, rw)

	whisper.peerMu.Lock()
	whisper.peers[whisperPeer] = struct{}{}
	whisper.peerMu.Unlock()

	defer func() {
		whisper.peerMu.Lock()
		delete(whisper.peers, whisperPeer)
		whisper.peerMu.Unlock()
	}()

//
	if err := whisperPeer.handshake(); err != nil {
		return err
	}
	whisperPeer.start()
	defer whisperPeer.stop()

	return whisper.runMessageLoop(whisperPeer, rw)
}

//
func (whisper *Whisper) runMessageLoop(p *Peer, rw p2p.MsgReadWriter) error {
	for {
//
		packet, err := rw.ReadMsg()
		if err != nil {
			log.Info("message loop", "peer", p.peer.ID(), "err", err)
			return err
		}
		if packet.Size > whisper.MaxMessageSize() {
			log.Warn("oversized message received", "peer", p.peer.ID())
			return errors.New("oversized message received")
		}

		switch packet.Code {
		case statusCode:
//
			log.Warn("unxepected status message received", "peer", p.peer.ID())
		case messagesCode:
//
			var envelopes []*Envelope
			if err := packet.Decode(&envelopes); err != nil {
				log.Warn("failed to decode envelopes, peer will be disconnected", "peer", p.peer.ID(), "err", err)
				return errors.New("invalid envelopes")
			}

			trouble := false
			for _, env := range envelopes {
				cached, err := whisper.add(env, whisper.lightClient)
				if err != nil {
					trouble = true
					log.Error("bad envelope received, peer will be disconnected", "peer", p.peer.ID(), "err", err)
				}
				if cached {
					p.mark(env)
				}
			}

			if trouble {
				return errors.New("invalid envelope")
			}
		case powRequirementCode:
			s := rlp.NewStream(packet.Payload, uint64(packet.Size))
			i, err := s.Uint()
			if err != nil {
				log.Warn("failed to decode powRequirementCode message, peer will be disconnected", "peer", p.peer.ID(), "err", err)
				return errors.New("invalid powRequirementCode message")
			}
			f := math.Float64frombits(i)
			if math.IsInf(f, 0) || math.IsNaN(f) || f < 0.0 {
				log.Warn("invalid value in powRequirementCode message, peer will be disconnected", "peer", p.peer.ID(), "err", err)
				return errors.New("invalid value in powRequirementCode message")
			}
			p.powRequirement = f
		case bloomFilterExCode:
			var bloom []byte
			err := packet.Decode(&bloom)
			if err == nil && len(bloom) != BloomFilterSize {
				err = fmt.Errorf("wrong bloom filter size %d", len(bloom))
			}

			if err != nil {
				log.Warn("failed to decode bloom filter exchange message, peer will be disconnected", "peer", p.peer.ID(), "err", err)
				return errors.New("invalid bloom filter exchange message")
			}
			p.setBloomFilter(bloom)
		case p2pMessageCode:
//
//
//
//
			if p.trusted {
				var envelope Envelope
				if err := packet.Decode(&envelope); err != nil {
					log.Warn("failed to decode direct message, peer will be disconnected", "peer", p.peer.ID(), "err", err)
					return errors.New("invalid direct message")
				}
				whisper.postEvent(&envelope, true)
			}
		case p2pRequestCode:
//
			if whisper.mailServer != nil {
				var request Envelope
				if err := packet.Decode(&request); err != nil {
					log.Warn("failed to decode p2p request message, peer will be disconnected", "peer", p.peer.ID(), "err", err)
					return errors.New("invalid p2p request")
				}
				whisper.mailServer.DeliverMail(p, &request)
			}
		default:
//
//
		}

		packet.Discard()
	}
}

//
//
//
//
func (whisper *Whisper) add(envelope *Envelope, isP2P bool) (bool, error) {
	now := uint32(time.Now().Unix())
	sent := envelope.Expiry - envelope.TTL

	if sent > now {
		if sent-DefaultSyncAllowance > now {
			return false, fmt.Errorf("envelope created in the future [%x]", envelope.Hash())
		}
//
		envelope.calculatePoW(sent - now + 1)
	}

	if envelope.Expiry < now {
		if envelope.Expiry+DefaultSyncAllowance*2 < now {
			return false, fmt.Errorf("very old message")
		}
		log.Debug("expired envelope dropped", "hash", envelope.Hash().Hex())
return false, nil //
	}

	if uint32(envelope.size()) > whisper.MaxMessageSize() {
		return false, fmt.Errorf("huge messages are not allowed [%x]", envelope.Hash())
	}

	if envelope.PoW() < whisper.MinPow() {
//
//
//
		if envelope.PoW() < whisper.MinPowTolerance() {
			return false, fmt.Errorf("envelope with low PoW received: PoW=%f, hash=[%v]", envelope.PoW(), envelope.Hash().Hex())
		}
	}

	if !BloomFilterMatch(whisper.BloomFilter(), envelope.Bloom()) {
//
//
//
		if !BloomFilterMatch(whisper.BloomFilterTolerance(), envelope.Bloom()) {
			return false, fmt.Errorf("envelope does not match bloom filter, hash=[%v], bloom: \n%x \n%x \n%x",
				envelope.Hash().Hex(), whisper.BloomFilter(), envelope.Bloom(), envelope.Topic)
		}
	}

	hash := envelope.Hash()

	whisper.poolMu.Lock()
	_, alreadyCached := whisper.envelopes[hash]
	if !alreadyCached {
		whisper.envelopes[hash] = envelope
		if whisper.expirations[envelope.Expiry] == nil {
			whisper.expirations[envelope.Expiry] = mapset.NewThreadUnsafeSet()
		}
		if !whisper.expirations[envelope.Expiry].Contains(hash) {
			whisper.expirations[envelope.Expiry].Add(hash)
		}
	}
	whisper.poolMu.Unlock()

	if alreadyCached {
		log.Trace("whisper envelope already cached", "hash", envelope.Hash().Hex())
	} else {
		log.Trace("cached whisper envelope", "hash", envelope.Hash().Hex())
		whisper.statsMu.Lock()
		whisper.stats.memoryUsed += envelope.size()
		whisper.statsMu.Unlock()
whisper.postEvent(envelope, isP2P) //
		if whisper.mailServer != nil {
			whisper.mailServer.Archive(envelope)
		}
	}
	return true, nil
}

//
func (whisper *Whisper) postEvent(envelope *Envelope, isP2P bool) {
	if isP2P {
		whisper.p2pMsgQueue <- envelope
	} else {
		whisper.checkOverflow()
		whisper.messageQueue <- envelope
	}
}

//
func (whisper *Whisper) checkOverflow() {
	queueSize := len(whisper.messageQueue)

	if queueSize == messageQueueLimit {
		if !whisper.Overflow() {
			whisper.settings.Store(overflowIdx, true)
			log.Warn("message queue overflow")
		}
	} else if queueSize <= messageQueueLimit/2 {
		if whisper.Overflow() {
			whisper.settings.Store(overflowIdx, false)
			log.Warn("message queue overflow fixed (back to normal)")
		}
	}
}

//
func (whisper *Whisper) processQueue() {
	var e *Envelope
	for {
		select {
		case <-whisper.quit:
			return

		case e = <-whisper.messageQueue:
			whisper.filters.NotifyWatchers(e, false)

		case e = <-whisper.p2pMsgQueue:
			whisper.filters.NotifyWatchers(e, true)
		}
	}
}

//
//
func (whisper *Whisper) update() {
//
	expire := time.NewTicker(expirationCycle)

//
	for {
		select {
		case <-expire.C:
			whisper.expire()

		case <-whisper.quit:
			return
		}
	}
}

//
//
func (whisper *Whisper) expire() {
	whisper.poolMu.Lock()
	defer whisper.poolMu.Unlock()

	whisper.statsMu.Lock()
	defer whisper.statsMu.Unlock()
	whisper.stats.reset()
	now := uint32(time.Now().Unix())
	for expiry, hashSet := range whisper.expirations {
		if expiry < now {
//
			hashSet.Each(func(v interface{}) bool {
				sz := whisper.envelopes[v.(common.Hash)].size()
				delete(whisper.envelopes, v.(common.Hash))
				whisper.stats.messagesCleared++
				whisper.stats.memoryCleared += sz
				whisper.stats.memoryUsed -= sz
				return true
			})
			whisper.expirations[expiry].Clear()
			delete(whisper.expirations, expiry)
		}
	}
}

//
func (whisper *Whisper) Stats() Statistics {
	whisper.statsMu.Lock()
	defer whisper.statsMu.Unlock()

	return whisper.stats
}

//
func (whisper *Whisper) Envelopes() []*Envelope {
	whisper.poolMu.RLock()
	defer whisper.poolMu.RUnlock()

	all := make([]*Envelope, 0, len(whisper.envelopes))
	for _, envelope := range whisper.envelopes {
		all = append(all, envelope)
	}
	return all
}

//
func (whisper *Whisper) isEnvelopeCached(hash common.Hash) bool {
	whisper.poolMu.Lock()
	defer whisper.poolMu.Unlock()

	_, exist := whisper.envelopes[hash]
	return exist
}

//
func (s *Statistics) reset() {
	s.cycles++
	s.totalMessagesCleared += s.messagesCleared

	s.memoryCleared = 0
	s.messagesCleared = 0
}

//
func ValidatePublicKey(k *ecdsa.PublicKey) bool {
	return k != nil && k.X != nil && k.Y != nil && k.X.Sign() != 0 && k.Y.Sign() != 0
}

//
func validatePrivateKey(k *ecdsa.PrivateKey) bool {
	if k == nil || k.D == nil || k.D.Sign() == 0 {
		return false
	}
	return ValidatePublicKey(&k.PublicKey)
}

//
//
func validateDataIntegrity(k []byte, expectedSize int) bool {
	if len(k) != expectedSize {
		return false
	}
	if expectedSize > 3 && containsOnlyZeros(k) {
		return false
	}
	return true
}

//
func containsOnlyZeros(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}

//
func bytesToUintLittleEndian(b []byte) (res uint64) {
	mul := uint64(1)
	for i := 0; i < len(b); i++ {
		res += uint64(b[i]) * mul
		mul *= 256
	}
	return res
}

//
func BytesToUintBigEndian(b []byte) (res uint64) {
	for i := 0; i < len(b); i++ {
		res *= 256
		res += uint64(b[i])
	}
	return res
}

//
func GenerateRandomID() (id string, err error) {
	buf, err := generateSecureRandomData(keyIDSize)
	if err != nil {
		return "", err
	}
	if !validateDataIntegrity(buf, keyIDSize) {
		return "", fmt.Errorf("error in generateRandomID: crypto/rand failed to generate random data")
	}
	id = common.Bytes2Hex(buf)
	return id, err
}

func isFullNode(bloom []byte) bool {
	if bloom == nil {
		return true
	}
	for _, b := range bloom {
		if b != 255 {
			return false
		}
	}
	return true
}

func BloomFilterMatch(filter, sample []byte) bool {
	if filter == nil {
		return true
	}

	for i := 0; i < BloomFilterSize; i++ {
		f := filter[i]
		s := sample[i]
		if (f | s) != f {
			return false
		}
	}

	return true
}

func addBloom(a, b []byte) []byte {
	c := make([]byte, BloomFilterSize)
	for i := 0; i < BloomFilterSize; i++ {
		c[i] = a[i] | b[i]
	}
	return c
}
