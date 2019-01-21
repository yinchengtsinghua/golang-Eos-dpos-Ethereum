
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

package whisperv5

import (
	"fmt"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
)

//
type Peer struct {
	host    *Whisper
	peer    *p2p.Peer
	ws      p2p.MsgReadWriter
	trusted bool

known mapset.Set //

	quit chan struct{}
}

//
func newPeer(host *Whisper, remote *p2p.Peer, rw p2p.MsgReadWriter) *Peer {
	return &Peer{
		host:    host,
		peer:    remote,
		ws:      rw,
		trusted: false,
		known:   mapset.NewSet(),
		quit:    make(chan struct{}),
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
		errc <- p2p.Send(peer.ws, statusCode, ProtocolVersion)
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
	peerVersion, err := s.Uint()
	if err != nil {
		return fmt.Errorf("peer [%x] sent bad status message: %v", peer.ID(), err)
	}
	if peerVersion != ProtocolVersion {
		return fmt.Errorf("peer [%x]: protocol version mismatch %d != %d", peer.ID(), peerVersion, ProtocolVersion)
	}
//
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
	var cnt int
	envelopes := peer.host.Envelopes()
	for _, envelope := range envelopes {
		if !peer.marked(envelope) {
			err := p2p.Send(peer.ws, messagesCode, envelope)
			if err != nil {
				return err
			} else {
				peer.mark(envelope)
				cnt++
			}
		}
	}
	if cnt > 0 {
		log.Trace("broadcast", "num. messages", cnt)
	}
	return nil
}

func (peer *Peer) ID() []byte {
	id := peer.peer.ID()
	return id[:]
}
