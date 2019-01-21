
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

package stream

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/log"
	bv "github.com/ethereum/go-ethereum/swarm/network/bitvector"
	"github.com/ethereum/go-ethereum/swarm/spancontext"
	"github.com/ethereum/go-ethereum/swarm/storage"
	opentracing "github.com/opentracing/opentracing-go"
)

//
type Stream struct {
//
	Name string
//
	Key string
//
//
	Live bool
}

func NewStream(name string, key string, live bool) Stream {
	return Stream{
		Name: name,
		Key:  key,
		Live: live,
	}
}

//
func (s Stream) String() string {
	t := "h"
	if s.Live {
		t = "l"
	}
	return fmt.Sprintf("%s|%s|%s", s.Name, s.Key, t)
}

//
type SubscribeMsg struct {
	Stream   Stream
	History  *Range `rlp:"nil"`
Priority uint8  //
}

//
//
type RequestSubscriptionMsg struct {
	Stream   Stream
	History  *Range `rlp:"nil"`
Priority uint8  //
}

func (p *Peer) handleRequestSubscription(ctx context.Context, req *RequestSubscriptionMsg) (err error) {
	log.Debug(fmt.Sprintf("handleRequestSubscription: streamer %s to subscribe to %s with stream %s", p.streamer.addr.ID(), p.ID(), req.Stream))
	return p.streamer.Subscribe(p.ID(), req.Stream, req.History, req.Priority)
}

func (p *Peer) handleSubscribeMsg(ctx context.Context, req *SubscribeMsg) (err error) {
	metrics.GetOrRegisterCounter("peer.handlesubscribemsg", nil).Inc(1)

	defer func() {
		if err != nil {
			if e := p.Send(context.TODO(), SubscribeErrorMsg{
				Error: err.Error(),
			}); e != nil {
				log.Error("send stream subscribe error message", "err", err)
			}
		}
	}()

	log.Debug("received subscription", "from", p.streamer.addr.ID(), "peer", p.ID(), "stream", req.Stream, "history", req.History)

	f, err := p.streamer.GetServerFunc(req.Stream.Name)
	if err != nil {
		return err
	}

	s, err := f(p, req.Stream.Key, req.Stream.Live)
	if err != nil {
		return err
	}
	os, err := p.setServer(req.Stream, s, req.Priority)
	if err != nil {
		return err
	}

	var from uint64
	var to uint64
	if !req.Stream.Live && req.History != nil {
		from = req.History.From
		to = req.History.To
	}

	go func() {
		if err := p.SendOfferedHashes(os, from, to); err != nil {
			log.Warn("SendOfferedHashes dropping peer", "err", err)
			p.Drop(err)
		}
	}()

	if req.Stream.Live && req.History != nil {
//
		s, err := f(p, req.Stream.Key, false)
		if err != nil {
			return err
		}

		os, err := p.setServer(getHistoryStream(req.Stream), s, getHistoryPriority(req.Priority))
		if err != nil {
			return err
		}
		go func() {
			if err := p.SendOfferedHashes(os, req.History.From, req.History.To); err != nil {
				log.Warn("SendOfferedHashes dropping peer", "err", err)
				p.Drop(err)
			}
		}()
	}

	return nil
}

type SubscribeErrorMsg struct {
	Error string
}

func (p *Peer) handleSubscribeErrorMsg(req *SubscribeErrorMsg) (err error) {
	return fmt.Errorf("subscribe to peer %s: %v", p.ID(), req.Error)
}

type UnsubscribeMsg struct {
	Stream Stream
}

func (p *Peer) handleUnsubscribeMsg(req *UnsubscribeMsg) error {
	return p.removeServer(req.Stream)
}

type QuitMsg struct {
	Stream Stream
}

func (p *Peer) handleQuitMsg(req *QuitMsg) error {
	return p.removeClient(req.Stream)
}

//
//
type OfferedHashesMsg struct {
Stream         Stream //
From, To       uint64 //
Hashes         []byte //
*HandoverProof        //
}

//
func (m OfferedHashesMsg) String() string {
	return fmt.Sprintf("Stream '%v' [%v-%v] (%v)", m.Stream, m.From, m.To, len(m.Hashes)/HashSize)
}

//
//
func (p *Peer) handleOfferedHashesMsg(ctx context.Context, req *OfferedHashesMsg) error {
	metrics.GetOrRegisterCounter("peer.handleofferedhashes", nil).Inc(1)

	var sp opentracing.Span
	ctx, sp = spancontext.StartSpan(
		ctx,
		"handle.offered.hashes")
	defer sp.Finish()

	c, _, err := p.getOrSetClient(req.Stream, req.From, req.To)
	if err != nil {
		return err
	}
	hashes := req.Hashes
	want, err := bv.New(len(hashes) / HashSize)
	if err != nil {
		return fmt.Errorf("error initiaising bitvector of length %v: %v", len(hashes)/HashSize, err)
	}
	wg := sync.WaitGroup{}
	for i := 0; i < len(hashes); i += HashSize {
		hash := hashes[i : i+HashSize]

		if wait := c.NeedData(ctx, hash); wait != nil {
			want.Set(i/HashSize, true)
			wg.Add(1)
//
			go func(w func()) {
				w()
				wg.Done()
			}(wait)
		}
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
//
	go func() {
		wg.Wait()
		select {
		case c.next <- c.batchDone(p, req, hashes):
		case <-c.quit:
		}
	}()
//
//
	if c.stream.Live {
		c.sessionAt = req.From
	}
	from, to := c.nextBatch(req.To + 1)
	log.Trace("received offered batch", "peer", p.ID(), "stream", req.Stream, "from", req.From, "to", req.To)
	if from == to {
		return nil
	}

	msg := &WantedHashesMsg{
		Stream: req.Stream,
		Want:   want.Bytes(),
		From:   from,
		To:     to,
	}
	go func() {
		select {
		case <-time.After(120 * time.Second):
			log.Warn("handleOfferedHashesMsg timeout, so dropping peer")
			p.Drop(errors.New("handle offered hashes timeout"))
			return
		case err := <-c.next:
			if err != nil {
				log.Warn("c.next dropping peer", "err", err)
				p.Drop(err)
				return
			}
		case <-c.quit:
			return
		}
		log.Trace("sending want batch", "peer", p.ID(), "stream", msg.Stream, "from", msg.From, "to", msg.To)
		err := p.SendPriority(ctx, msg, c.priority)
		if err != nil {
			log.Warn("SendPriority err, so dropping peer", "err", err)
			p.Drop(err)
		}
	}()
	return nil
}

//
//
type WantedHashesMsg struct {
	Stream   Stream
Want     []byte //
From, To uint64 //
}

//
func (m WantedHashesMsg) String() string {
	return fmt.Sprintf("Stream '%v', Want: %x, Next: [%v-%v]", m.Stream, m.Want, m.From, m.To)
}

//
//
//
func (p *Peer) handleWantedHashesMsg(ctx context.Context, req *WantedHashesMsg) error {
	metrics.GetOrRegisterCounter("peer.handlewantedhashesmsg", nil).Inc(1)

	log.Trace("received wanted batch", "peer", p.ID(), "stream", req.Stream, "from", req.From, "to", req.To)
	s, err := p.getServer(req.Stream)
	if err != nil {
		return err
	}
	hashes := s.currentBatch
//
	go func() {
		if err := p.SendOfferedHashes(s, req.From, req.To); err != nil {
			log.Warn("SendOfferedHashes dropping peer", "err", err)
			p.Drop(err)
		}
	}()
//
	l := len(hashes) / HashSize

	log.Trace("wanted batch length", "peer", p.ID(), "stream", req.Stream, "from", req.From, "to", req.To, "lenhashes", len(hashes), "l", l)
	want, err := bv.NewFromBytes(req.Want, l)
	if err != nil {
		return fmt.Errorf("error initiaising bitvector of length %v: %v", l, err)
	}
	for i := 0; i < l; i++ {
		if want.Get(i) {
			metrics.GetOrRegisterCounter("peer.handlewantedhashesmsg.actualget", nil).Inc(1)

			hash := hashes[i*HashSize : (i+1)*HashSize]
			data, err := s.GetData(ctx, hash)
			if err != nil {
				return fmt.Errorf("handleWantedHashesMsg get data %x: %v", hash, err)
			}
			chunk := storage.NewChunk(hash, nil)
			chunk.SData = data
			if length := len(chunk.SData); length < 9 {
				log.Error("Chunk.SData to sync is too short", "len(chunk.SData)", length, "address", chunk.Addr)
			}
			if err := p.Deliver(ctx, chunk, s.priority); err != nil {
				return err
			}
		}
	}
	return nil
}

//
type Handover struct {
Stream     Stream //
Start, End uint64 //
Root       []byte //
}

//
type HandoverProof struct {
Sig []byte //
	*Handover
}

//
//
type Takeover Handover

//
//
type TakeoverProof struct {
Sig []byte //
	*Takeover
}

//
type TakeoverProofMsg TakeoverProof

//
func (m TakeoverProofMsg) String() string {
	return fmt.Sprintf("Stream: '%v' [%v-%v], Root: %x, Sig: %x", m.Stream, m.Start, m.End, m.Root, m.Sig)
}

func (p *Peer) handleTakeoverProofMsg(ctx context.Context, req *TakeoverProofMsg) error {
	_, err := p.getServer(req.Stream)
//
	return err
}
