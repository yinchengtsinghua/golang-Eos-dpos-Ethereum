
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
	"fmt"
	"math"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
)

//
type Peer struct {
	host *Whisper
	peer *p2p.Peer
	ws   p2p.MsgReadWriter

	trusted        bool
	powRequirement float64
	bloomMu        sync.Mutex
	bloomFilter    []byte
	fullNode       bool

known mapset.Set //

	quit chan struct{}
}

//
func newPeer(host *Whisper, remote *p2p.Peer, rw p2p.MsgReadWriter) *Peer {
	return &Peer{
		host:           host,
		peer:           remote,
		ws:             rw,
		trusted:        false,
		powRequirement: 0.0,
		known:          mapset.NewSet(),
		quit:           make(chan struct{}),
		bloomFilter:    MakeFullNodeBloom(),
		fullNode:       true,
	}
}

//
//
func (peer *Peer) start() {
	go peer.update()
	log.Trace("start", "peer", peer.ID())
}

//
func (peer *Peer) stop() {
	close(peer.quit)
	log.Trace("stop", "peer", peer.ID())
}

//
//
func (peer *Peer) handshake() error {
//
	errc := make(chan error, 1)
	go func() {
		pow := peer.host.MinPow()
		powConverted := math.Float64bits(pow)
		bloom := peer.host.BloomFilter()
		errc <- p2p.SendItems(peer.ws, statusCode, ProtocolVersion, powConverted, bloom)
	}()

//
	packet, err := peer.ws.ReadMsg()
	if err != nil {
		return err
	}
	if packet.Code != statusCode {
		return fmt.Errorf("peer [%x] sent packet %x before status packet", peer.ID(), packet.Code)
	}
	s := rlp.NewStream(packet.Payload, uint64(packet.Size))
	_, err = s.List()
	if err != nil {
		return fmt.Errorf("peer [%x] sent bad status message: %v", peer.ID(), err)
	}
	peerVersion, err := s.Uint()
	if err != nil {
		return fmt.Errorf("peer [%x] sent bad status message (unable to decode version): %v", peer.ID(), err)
	}
	if peerVersion != ProtocolVersion {
		return fmt.Errorf("peer [%x]: protocol version mismatch %d != %d", peer.ID(), peerVersion, ProtocolVersion)
	}

//
	powRaw, err := s.Uint()
	if err == nil {
		pow := math.Float64frombits(powRaw)
		if math.IsInf(pow, 0) || math.IsNaN(pow) || pow < 0.0 {
			return fmt.Errorf("peer [%x] sent bad status message: invalid pow", peer.ID())
		}
		peer.powRequirement = pow

		var bloom []byte
		err = s.Decode(&bloom)
		if err == nil {
			sz := len(bloom)
			if sz != BloomFilterSize && sz != 0 {
				return fmt.Errorf("peer [%x] sent bad status message: wrong bloom filter size %d", peer.ID(), sz)
			}
			peer.setBloomFilter(bloom)
		}
	}

	if err := <-errc; err != nil {
		return fmt.Errorf("peer [%x] failed to send status packet: %v", peer.ID(), err)
	}
	return nil
}

//
//
func (peer *Peer) update() {
//
	expire := time.NewTicker(expirationCycle)
	transmit := time.NewTicker(transmissionCycle)

//
	for {
		select {
		case <-expire.C:
			peer.expire()

		case <-transmit.C:
			if err := peer.broadcast(); err != nil {
				log.Trace("broadcast failed", "reason", err, "peer", peer.ID())
				return
			}

		case <-peer.quit:
			return
		}
	}
}

//
func (peer *Peer) mark(envelope *Envelope) {
	peer.known.Add(envelope.Hash())
}

//
func (peer *Peer) marked(envelope *Envelope) bool {
	return peer.known.Contains(envelope.Hash())
}

//
//
func (peer *Peer) expire() {
	unmark := make(map[common.Hash]struct{})
	peer.known.Each(func(v interface{}) bool {
		if !peer.host.isEnvelopeCached(v.(common.Hash)) {
			unmark[v.(common.Hash)] = struct{}{}
		}
		return true
	})
//
	for hash := range unmark {
		peer.known.Remove(hash)
	}
}

//
//
func (peer *Peer) broadcast() error {
	envelopes := peer.host.Envelopes()
	bundle := make([]*Envelope, 0, len(envelopes))
	for _, envelope := range envelopes {
		if !peer.marked(envelope) && envelope.PoW() >= peer.powRequirement && peer.bloomMatch(envelope) {
			bundle = append(bundle, envelope)
		}
	}

	if len(bundle) > 0 {
//
		if err := p2p.Send(peer.ws, messagesCode, bundle); err != nil {
			return err
		}

//
		for _, e := range bundle {
			peer.mark(e)
		}

		log.Trace("broadcast", "num. messages", len(bundle))
	}
	return nil
}

//
func (peer *Peer) ID() []byte {
	id := peer.peer.ID()
	return id[:]
}

func (peer *Peer) notifyAboutPowRequirementChange(pow float64) error {
	i := math.Float64bits(pow)
	return p2p.Send(peer.ws, powRequirementCode, i)
}

func (peer *Peer) notifyAboutBloomFilterChange(bloom []byte) error {
	return p2p.Send(peer.ws, bloomFilterExCode, bloom)
}

func (peer *Peer) bloomMatch(env *Envelope) bool {
	peer.bloomMu.Lock()
	defer peer.bloomMu.Unlock()
	return peer.fullNode || BloomFilterMatch(peer.bloomFilter, env.Bloom())
}

func (peer *Peer) setBloomFilter(bloom []byte) {
	peer.bloomMu.Lock()
	defer peer.bloomMu.Unlock()
	peer.bloomFilter = bloom
	peer.fullNode = isFullNode(bloom)
	if peer.fullNode && peer.bloomFilter == nil {
		peer.bloomFilter = MakeFullNodeBloom()
	}
}

func MakeFullNodeBloom() []byte {
	bloom := make([]byte, BloomFilterSize)
	for i := 0; i < BloomFilterSize; i++ {
		bloom[i] = 0xFF
	}
	return bloom
}
