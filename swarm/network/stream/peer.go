
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
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/swarm/log"
	pq "github.com/ethereum/go-ethereum/swarm/network/priorityqueue"
	"github.com/ethereum/go-ethereum/swarm/network/stream/intervals"
	"github.com/ethereum/go-ethereum/swarm/spancontext"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
	opentracing "github.com/opentracing/opentracing-go"
)

var sendTimeout = 30 * time.Second

type notFoundError struct {
	t string
	s Stream
}

func newNotFoundError(t string, s Stream) *notFoundError {
	return &notFoundError{t: t, s: s}
}

func (e *notFoundError) Error() string {
	return fmt.Sprintf("%s not found for stream %q", e.t, e.s)
}

//
type Peer struct {
	*protocols.Peer
	streamer *Registry
	pq       *pq.PriorityQueue
	serverMu sync.RWMutex
clientMu sync.RWMutex //
	servers  map[Stream]*server
	clients  map[Stream]*client
//
//
//
	clientParams map[Stream]*clientParams
	quit         chan struct{}
}

type WrappedPriorityMsg struct {
	Context context.Context
	Msg     interface{}
}

//
func NewPeer(peer *protocols.Peer, streamer *Registry) *Peer {
	p := &Peer{
		Peer:         peer,
		pq:           pq.New(int(PriorityQueue), PriorityQueueCap),
		streamer:     streamer,
		servers:      make(map[Stream]*server),
		clients:      make(map[Stream]*client),
		clientParams: make(map[Stream]*clientParams),
		quit:         make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	go p.pq.Run(ctx, func(i interface{}) {
		wmsg := i.(WrappedPriorityMsg)
		p.Send(wmsg.Context, wmsg.Msg)
	})
	go func() {
		<-p.quit
		cancel()
	}()
	return p
}

//
func (p *Peer) Deliver(ctx context.Context, chunk *storage.Chunk, priority uint8) error {
	var sp opentracing.Span
	ctx, sp = spancontext.StartSpan(
		ctx,
		"send.chunk.delivery")
	defer sp.Finish()

	msg := &ChunkDeliveryMsg{
		Addr:  chunk.Addr,
		SData: chunk.SData,
	}
	return p.SendPriority(ctx, msg, priority)
}

//
func (p *Peer) SendPriority(ctx context.Context, msg interface{}, priority uint8) error {
	defer metrics.GetOrRegisterResettingTimer(fmt.Sprintf("peer.sendpriority_t.%d", priority), nil).UpdateSince(time.Now())
	metrics.GetOrRegisterCounter(fmt.Sprintf("peer.sendpriority.%d", priority), nil).Inc(1)
	cctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	wmsg := WrappedPriorityMsg{
		Context: ctx,
		Msg:     msg,
	}
	return p.pq.Push(cctx, wmsg, int(priority))
}

//
func (p *Peer) SendOfferedHashes(s *server, f, t uint64) error {
	var sp opentracing.Span
	ctx, sp := spancontext.StartSpan(
		context.TODO(),
		"send.offered.hashes")
	defer sp.Finish()

	hashes, from, to, proof, err := s.SetNextBatch(f, t)
	if err != nil {
		return err
	}
//
	if len(hashes) == 0 {
		return nil
	}
	if proof == nil {
		proof = &HandoverProof{
			Handover: &Handover{},
		}
	}
	s.currentBatch = hashes
	msg := &OfferedHashesMsg{
		HandoverProof: proof,
		Hashes:        hashes,
		From:          from,
		To:            to,
		Stream:        s.stream,
	}
	log.Trace("Swarm syncer offer batch", "peer", p.ID(), "stream", s.stream, "len", len(hashes), "from", from, "to", to)
	return p.SendPriority(ctx, msg, s.priority)
}

func (p *Peer) getServer(s Stream) (*server, error) {
	p.serverMu.RLock()
	defer p.serverMu.RUnlock()

	server := p.servers[s]
	if server == nil {
		return nil, newNotFoundError("server", s)
	}
	return server, nil
}

func (p *Peer) setServer(s Stream, o Server, priority uint8) (*server, error) {
	p.serverMu.Lock()
	defer p.serverMu.Unlock()

	if p.servers[s] != nil {
		return nil, fmt.Errorf("server %s already registered", s)
	}
	os := &server{
		Server:   o,
		stream:   s,
		priority: priority,
	}
	p.servers[s] = os
	return os, nil
}

func (p *Peer) removeServer(s Stream) error {
	p.serverMu.Lock()
	defer p.serverMu.Unlock()

	server, ok := p.servers[s]
	if !ok {
		return newNotFoundError("server", s)
	}
	server.Close()
	delete(p.servers, s)
	return nil
}

func (p *Peer) getClient(ctx context.Context, s Stream) (c *client, err error) {
	var params *clientParams
	func() {
		p.clientMu.RLock()
		defer p.clientMu.RUnlock()

		c = p.clients[s]
		if c != nil {
			return
		}
		params = p.clientParams[s]
	}()
	if c != nil {
		return c, nil
	}

	if params != nil {
//
		if err := params.waitClient(ctx); err != nil {
			return nil, err
		}
	}

	p.clientMu.RLock()
	defer p.clientMu.RUnlock()

	c = p.clients[s]
	if c != nil {
		return c, nil
	}
	return nil, newNotFoundError("client", s)
}

func (p *Peer) getOrSetClient(s Stream, from, to uint64) (c *client, created bool, err error) {
	p.clientMu.Lock()
	defer p.clientMu.Unlock()

	c = p.clients[s]
	if c != nil {
		return c, false, nil
	}

	f, err := p.streamer.GetClientFunc(s.Name)
	if err != nil {
		return nil, false, err
	}

	is, err := f(p, s.Key, s.Live)
	if err != nil {
		return nil, false, err
	}

	cp, err := p.getClientParams(s)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		if err == nil {
			if err := p.removeClientParams(s); err != nil {
				log.Error("stream set client: remove client params", "stream", s, "peer", p, "err", err)
			}
		}
	}()

	intervalsKey := peerStreamIntervalsKey(p, s)
	if s.Live {
//
		historyKey := peerStreamIntervalsKey(p, NewStream(s.Name, s.Key, false))
		historyIntervals := &intervals.Intervals{}
		err := p.streamer.intervalsStore.Get(historyKey, historyIntervals)
		switch err {
		case nil:
			liveIntervals := &intervals.Intervals{}
			err := p.streamer.intervalsStore.Get(intervalsKey, liveIntervals)
			switch err {
			case nil:
				historyIntervals.Merge(liveIntervals)
				if err := p.streamer.intervalsStore.Put(historyKey, historyIntervals); err != nil {
					log.Error("stream set client: put history intervals", "stream", s, "peer", p, "err", err)
				}
			case state.ErrNotFound:
			default:
				log.Error("stream set client: get live intervals", "stream", s, "peer", p, "err", err)
			}
		case state.ErrNotFound:
		default:
			log.Error("stream set client: get history intervals", "stream", s, "peer", p, "err", err)
		}
	}

	if err := p.streamer.intervalsStore.Put(intervalsKey, intervals.NewIntervals(from)); err != nil {
		return nil, false, err
	}

	next := make(chan error, 1)
	c = &client{
		Client:         is,
		stream:         s,
		priority:       cp.priority,
		to:             cp.to,
		next:           next,
		quit:           make(chan struct{}),
		intervalsStore: p.streamer.intervalsStore,
		intervalsKey:   intervalsKey,
	}
	p.clients[s] = c
cp.clientCreated() //
next <- nil        //
	return c, true, nil
}

func (p *Peer) removeClient(s Stream) error {
	p.clientMu.Lock()
	defer p.clientMu.Unlock()

	client, ok := p.clients[s]
	if !ok {
		return newNotFoundError("client", s)
	}
	client.close()
	return nil
}

func (p *Peer) setClientParams(s Stream, params *clientParams) error {
	p.clientMu.Lock()
	defer p.clientMu.Unlock()

	if p.clients[s] != nil {
		return fmt.Errorf("client %s already exists", s)
	}
	if p.clientParams[s] != nil {
		return fmt.Errorf("client params %s already set", s)
	}
	p.clientParams[s] = params
	return nil
}

func (p *Peer) getClientParams(s Stream) (*clientParams, error) {
	params := p.clientParams[s]
	if params == nil {
		return nil, fmt.Errorf("client params '%v' not provided to peer %v", s, p.ID())
	}
	return params, nil
}

func (p *Peer) removeClientParams(s Stream) error {
	_, ok := p.clientParams[s]
	if !ok {
		return newNotFoundError("client params", s)
	}
	delete(p.clientParams, s)
	return nil
}

func (p *Peer) close() {
	for _, s := range p.servers {
		s.Close()
	}
}
