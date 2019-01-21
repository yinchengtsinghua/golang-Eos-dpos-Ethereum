
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
	"math"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/stream/intervals"
	"github.com/ethereum/go-ethereum/swarm/pot"
	"github.com/ethereum/go-ethereum/swarm/spancontext"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
	opentracing "github.com/opentracing/opentracing-go"
)

const (
	Low uint8 = iota
	Mid
	High
	Top
PriorityQueue         //
PriorityQueueCap = 32 //
	HashSize         = 32
)

//
type Registry struct {
	api            *API
	addr           *network.BzzAddr
	skipCheck      bool
	clientMu       sync.RWMutex
	serverMu       sync.RWMutex
	peersMu        sync.RWMutex
	serverFuncs    map[string]func(*Peer, string, bool) (Server, error)
	clientFuncs    map[string]func(*Peer, string, bool) (Client, error)
	peers          map[discover.NodeID]*Peer
	delivery       *Delivery
	intervalsStore state.Store
	doRetrieve     bool
}

//
type RegistryOptions struct {
	SkipCheck       bool
	DoSync          bool
	DoRetrieve      bool
	SyncUpdateDelay time.Duration
}

//
func NewRegistry(addr *network.BzzAddr, delivery *Delivery, db *storage.DBAPI, intervalsStore state.Store, options *RegistryOptions) *Registry {
	if options == nil {
		options = &RegistryOptions{}
	}
	if options.SyncUpdateDelay <= 0 {
		options.SyncUpdateDelay = 15 * time.Second
	}
	streamer := &Registry{
		addr:           addr,
		skipCheck:      options.SkipCheck,
		serverFuncs:    make(map[string]func(*Peer, string, bool) (Server, error)),
		clientFuncs:    make(map[string]func(*Peer, string, bool) (Client, error)),
		peers:          make(map[discover.NodeID]*Peer),
		delivery:       delivery,
		intervalsStore: intervalsStore,
		doRetrieve:     options.DoRetrieve,
	}
	streamer.api = NewAPI(streamer)
	delivery.getPeer = streamer.getPeer
	streamer.RegisterServerFunc(swarmChunkServerStreamName, func(_ *Peer, _ string, _ bool) (Server, error) {
		return NewSwarmChunkServer(delivery.db), nil
	})
	streamer.RegisterClientFunc(swarmChunkServerStreamName, func(p *Peer, t string, live bool) (Client, error) {
		return NewSwarmSyncerClient(p, delivery.db, false, NewStream(swarmChunkServerStreamName, t, live))
	})
	RegisterSwarmSyncerServer(streamer, db)
	RegisterSwarmSyncerClient(streamer, db)

	if options.DoSync {
//
//
//
//
//
//
		latestIntC := func(in <-chan int) <-chan int {
			out := make(chan int, 1)

			go func() {
				defer close(out)

				for i := range in {
					select {
					case <-out:
					default:
					}
					out <- i
				}
			}()

			return out
		}

		go func() {
//
			time.Sleep(options.SyncUpdateDelay)

			kad := streamer.delivery.overlay.(*network.Kademlia)
			depthC := latestIntC(kad.NeighbourhoodDepthC())
			addressBookSizeC := latestIntC(kad.AddrCountC())

//
			streamer.updateSyncing()

			for depth := range depthC {
				log.Debug("Kademlia neighbourhood depth change", "depth", depth)

//
//
//
				timer := time.NewTimer(options.SyncUpdateDelay)
//
//
				maxTimer := time.NewTimer(3 * time.Minute)
			loop:
				for {
					select {
					case <-maxTimer.C:
//
						log.Trace("Sync subscriptions update on hard timeout")
//
						streamer.updateSyncing()
						break loop
					case <-timer.C:
//
//
						log.Trace("Sync subscriptions update")
//
						streamer.updateSyncing()
						break loop
					case size := <-addressBookSizeC:
						log.Trace("Kademlia address book size changed on depth change", "size", size)
//
//
						if !timer.Stop() {
							<-timer.C
						}
						timer.Reset(options.SyncUpdateDelay)
					}
				}
				timer.Stop()
				maxTimer.Stop()
			}
		}()
	}

	return streamer
}

//
func (r *Registry) RegisterClientFunc(stream string, f func(*Peer, string, bool) (Client, error)) {
	r.clientMu.Lock()
	defer r.clientMu.Unlock()

	r.clientFuncs[stream] = f
}

//
func (r *Registry) RegisterServerFunc(stream string, f func(*Peer, string, bool) (Server, error)) {
	r.serverMu.Lock()
	defer r.serverMu.Unlock()

	r.serverFuncs[stream] = f
}

//
func (r *Registry) GetClientFunc(stream string) (func(*Peer, string, bool) (Client, error), error) {
	r.clientMu.RLock()
	defer r.clientMu.RUnlock()

	f := r.clientFuncs[stream]
	if f == nil {
		return nil, fmt.Errorf("stream %v not registered", stream)
	}
	return f, nil
}

//
func (r *Registry) GetServerFunc(stream string) (func(*Peer, string, bool) (Server, error), error) {
	r.serverMu.RLock()
	defer r.serverMu.RUnlock()

	f := r.serverFuncs[stream]
	if f == nil {
		return nil, fmt.Errorf("stream %v not registered", stream)
	}
	return f, nil
}

func (r *Registry) RequestSubscription(peerId discover.NodeID, s Stream, h *Range, prio uint8) error {
//
	if _, err := r.GetServerFunc(s.Name); err != nil {
		return err
	}

	peer := r.getPeer(peerId)
	if peer == nil {
		return fmt.Errorf("peer not found %v", peerId)
	}

	if _, err := peer.getServer(s); err != nil {
		if e, ok := err.(*notFoundError); ok && e.t == "server" {
//
			log.Debug("RequestSubscription ", "peer", peerId, "stream", s, "history", h)
			return peer.Send(context.TODO(), &RequestSubscriptionMsg{
				Stream:   s,
				History:  h,
				Priority: prio,
			})
		}
		return err
	}
	log.Trace("RequestSubscription: already subscribed", "peer", peerId, "stream", s, "history", h)
	return nil
}

//
func (r *Registry) Subscribe(peerId discover.NodeID, s Stream, h *Range, priority uint8) error {
//
	if _, err := r.GetClientFunc(s.Name); err != nil {
		return err
	}

	peer := r.getPeer(peerId)
	if peer == nil {
		return fmt.Errorf("peer not found %v", peerId)
	}

	var to uint64
	if !s.Live && h != nil {
		to = h.To
	}

	err := peer.setClientParams(s, newClientParams(priority, to))
	if err != nil {
		return err
	}

	if s.Live && h != nil {
		if err := peer.setClientParams(
			getHistoryStream(s),
			newClientParams(getHistoryPriority(priority), h.To),
		); err != nil {
			return err
		}
	}

	msg := &SubscribeMsg{
		Stream:   s,
		History:  h,
		Priority: priority,
	}
	log.Debug("Subscribe ", "peer", peerId, "stream", s, "history", h)

	return peer.SendPriority(context.TODO(), msg, priority)
}

func (r *Registry) Unsubscribe(peerId discover.NodeID, s Stream) error {
	peer := r.getPeer(peerId)
	if peer == nil {
		return fmt.Errorf("peer not found %v", peerId)
	}

	msg := &UnsubscribeMsg{
		Stream: s,
	}
	log.Debug("Unsubscribe ", "peer", peerId, "stream", s)

	if err := peer.Send(context.TODO(), msg); err != nil {
		return err
	}
	return peer.removeClient(s)
}

//
//
func (r *Registry) Quit(peerId discover.NodeID, s Stream) error {
	peer := r.getPeer(peerId)
	if peer == nil {
		log.Debug("stream quit: peer not found", "peer", peerId, "stream", s)
//
		return nil
	}

	msg := &QuitMsg{
		Stream: s,
	}
	log.Debug("Quit ", "peer", peerId, "stream", s)

	return peer.Send(context.TODO(), msg)
}

func (r *Registry) Retrieve(ctx context.Context, chunk *storage.Chunk) error {
	var sp opentracing.Span
	ctx, sp = spancontext.StartSpan(
		ctx,
		"registry.retrieve")
	defer sp.Finish()

	return r.delivery.RequestFromPeers(ctx, chunk.Addr[:], r.skipCheck)
}

func (r *Registry) NodeInfo() interface{} {
	return nil
}

func (r *Registry) PeerInfo(id discover.NodeID) interface{} {
	return nil
}

func (r *Registry) Close() error {
	return r.intervalsStore.Close()
}

func (r *Registry) getPeer(peerId discover.NodeID) *Peer {
	r.peersMu.RLock()
	defer r.peersMu.RUnlock()

	return r.peers[peerId]
}

func (r *Registry) setPeer(peer *Peer) {
	r.peersMu.Lock()
	r.peers[peer.ID()] = peer
	metrics.GetOrRegisterGauge("registry.peers", nil).Update(int64(len(r.peers)))
	r.peersMu.Unlock()
}

func (r *Registry) deletePeer(peer *Peer) {
	r.peersMu.Lock()
	delete(r.peers, peer.ID())
	metrics.GetOrRegisterGauge("registry.peers", nil).Update(int64(len(r.peers)))
	r.peersMu.Unlock()
}

func (r *Registry) peersCount() (c int) {
	r.peersMu.Lock()
	c = len(r.peers)
	r.peersMu.Unlock()
	return
}

//
func (r *Registry) Run(p *network.BzzPeer) error {
	sp := NewPeer(p.Peer, r)
	r.setPeer(sp)
	defer r.deletePeer(sp)
	defer close(sp.quit)
	defer sp.close()

	if r.doRetrieve {
		err := r.Subscribe(p.ID(), NewStream(swarmChunkServerStreamName, "", false), nil, Top)
		if err != nil {
			return err
		}
	}

	return sp.Run(sp.HandleMsg)
}

//
//
//
//
func (r *Registry) updateSyncing() {
//
	kad := r.delivery.overlay.(*network.Kademlia)

//
//
//
	subs := make(map[discover.NodeID]map[Stream]struct{})
	r.peersMu.RLock()
	for id, peer := range r.peers {
		peer.serverMu.RLock()
		for stream := range peer.servers {
			if stream.Name == "SYNC" {
				if _, ok := subs[id]; !ok {
					subs[id] = make(map[Stream]struct{})
				}
				subs[id][stream] = struct{}{}
			}
		}
		peer.serverMu.RUnlock()
	}
	r.peersMu.RUnlock()

//
	kad.EachBin(r.addr.Over(), pot.DefaultPof(256), 0, func(conn network.OverlayConn, bin int) bool {
		p := conn.(network.Peer)
		log.Debug(fmt.Sprintf("Requesting subscription by: registry %s from peer %s for bin: %d", r.addr.ID(), p.ID(), bin))

//
		stream := NewStream("SYNC", FormatSyncBinKey(uint8(bin)), true)
		if streams, ok := subs[p.ID()]; ok {
//
			delete(streams, stream)
			delete(streams, getHistoryStream(stream))
		}
		err := r.RequestSubscription(p.ID(), stream, NewRange(0, 0), High)
		if err != nil {
			log.Debug("Request subscription", "err", err, "peer", p.ID(), "stream", stream)
			return false
		}
		return true
	})

//
	for id, streams := range subs {
		if len(streams) == 0 {
			continue
		}
		peer := r.getPeer(id)
		if peer == nil {
			continue
		}
		for stream := range streams {
			log.Debug("Remove sync server", "peer", id, "stream", stream)
			err := r.Quit(peer.ID(), stream)
			if err != nil && err != p2p.ErrShuttingDown {
				log.Error("quit", "err", err, "peer", peer.ID(), "stream", stream)
			}
		}
	}
}

func (r *Registry) runProtocol(p *p2p.Peer, rw p2p.MsgReadWriter) error {
	peer := protocols.NewPeer(p, rw, Spec)
	bzzPeer := network.NewBzzTestPeer(peer, r.addr)
	r.delivery.overlay.On(bzzPeer)
	defer r.delivery.overlay.Off(bzzPeer)
	return r.Run(bzzPeer)
}

//
func (p *Peer) HandleMsg(ctx context.Context, msg interface{}) error {
	switch msg := msg.(type) {

	case *SubscribeMsg:
		return p.handleSubscribeMsg(ctx, msg)

	case *SubscribeErrorMsg:
		return p.handleSubscribeErrorMsg(msg)

	case *UnsubscribeMsg:
		return p.handleUnsubscribeMsg(msg)

	case *OfferedHashesMsg:
		return p.handleOfferedHashesMsg(ctx, msg)

	case *TakeoverProofMsg:
		return p.handleTakeoverProofMsg(ctx, msg)

	case *WantedHashesMsg:
		return p.handleWantedHashesMsg(ctx, msg)

	case *ChunkDeliveryMsg:
		return p.streamer.delivery.handleChunkDeliveryMsg(ctx, p, msg)

	case *RetrieveRequestMsg:
		return p.streamer.delivery.handleRetrieveRequestMsg(ctx, p, msg)

	case *RequestSubscriptionMsg:
		return p.handleRequestSubscription(ctx, msg)

	case *QuitMsg:
		return p.handleQuitMsg(msg)

	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
}

type server struct {
	Server
	stream       Stream
	priority     uint8
	currentBatch []byte
}

//
type Server interface {
	SetNextBatch(uint64, uint64) (hashes []byte, from uint64, to uint64, proof *HandoverProof, err error)
	GetData(context.Context, []byte) ([]byte, error)
	Close()
}

type client struct {
	Client
	stream    Stream
	priority  uint8
	sessionAt uint64
	to        uint64
	next      chan error
	quit      chan struct{}

	intervalsKey   string
	intervalsStore state.Store
}

func peerStreamIntervalsKey(p *Peer, s Stream) string {
	return p.ID().String() + s.String()
}

func (c client) AddInterval(start, end uint64) (err error) {
	i := &intervals.Intervals{}
	err = c.intervalsStore.Get(c.intervalsKey, i)
	if err != nil {
		return err
	}
	i.Add(start, end)
	return c.intervalsStore.Put(c.intervalsKey, i)
}

func (c client) NextInterval() (start, end uint64, err error) {
	i := &intervals.Intervals{}
	err = c.intervalsStore.Get(c.intervalsKey, i)
	if err != nil {
		return 0, 0, err
	}
	start, end = i.Next()
	return start, end, nil
}

//
type Client interface {
	NeedData(context.Context, []byte) func()
	BatchDone(Stream, uint64, []byte, []byte) func() (*TakeoverProof, error)
	Close()
}

func (c *client) nextBatch(from uint64) (nextFrom uint64, nextTo uint64) {
	if c.to > 0 && from >= c.to {
		return 0, 0
	}
	if c.stream.Live {
		return from, 0
	} else if from >= c.sessionAt {
		if c.to > 0 {
			return from, c.to
		}
		return from, math.MaxUint64
	}
	nextFrom, nextTo, err := c.NextInterval()
	if err != nil {
		log.Error("next intervals", "stream", c.stream)
		return
	}
	if nextTo > c.to {
		nextTo = c.to
	}
	if nextTo == 0 {
		nextTo = c.sessionAt
	}
	return
}

func (c *client) batchDone(p *Peer, req *OfferedHashesMsg, hashes []byte) error {
	if tf := c.BatchDone(req.Stream, req.From, hashes, req.Root); tf != nil {
		tp, err := tf()
		if err != nil {
			return err
		}
		if err := p.SendPriority(context.TODO(), tp, c.priority); err != nil {
			return err
		}
		if c.to > 0 && tp.Takeover.End >= c.to {
			return p.streamer.Unsubscribe(p.Peer.ID(), req.Stream)
		}
		return nil
	}
//
	if err := c.AddInterval(req.From, req.To); err != nil {
		return err
	}
	return nil
}

func (c *client) close() {
	select {
	case <-c.quit:
	default:
		close(c.quit)
	}
	c.Close()
}

//
//
type clientParams struct {
	priority uint8
	to       uint64
//
	clientCreatedC chan struct{}
}

func newClientParams(priority uint8, to uint64) *clientParams {
	return &clientParams{
		priority:       priority,
		to:             to,
		clientCreatedC: make(chan struct{}),
	}
}

func (c *clientParams) waitClient(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.clientCreatedC:
		return nil
	}
}

func (c *clientParams) clientCreated() {
	close(c.clientCreatedC)
}

//
var Spec = &protocols.Spec{
	Name:       "stream",
	Version:    5,
	MaxMsgSize: 10 * 1024 * 1024,
	Messages: []interface{}{
		UnsubscribeMsg{},
		OfferedHashesMsg{},
		WantedHashesMsg{},
		TakeoverProofMsg{},
		SubscribeMsg{},
		RetrieveRequestMsg{},
		ChunkDeliveryMsg{},
		SubscribeErrorMsg{},
		RequestSubscriptionMsg{},
		QuitMsg{},
	},
}

func (r *Registry) Protocols() []p2p.Protocol {
	return []p2p.Protocol{
		{
			Name:    Spec.Name,
			Version: Spec.Version,
			Length:  Spec.Length(),
			Run:     r.runProtocol,
//
//
		},
	}
}

func (r *Registry) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: "stream",
			Version:   "3.0",
			Service:   r.api,
			Public:    true,
		},
	}
}

func (r *Registry) Start(server *p2p.Server) error {
	log.Info("Streamer started")
	return nil
}

func (r *Registry) Stop() error {
	return nil
}

type Range struct {
	From, To uint64
}

func NewRange(from, to uint64) *Range {
	return &Range{
		From: from,
		To:   to,
	}
}

func (r *Range) String() string {
	return fmt.Sprintf("%v-%v", r.From, r.To)
}

func getHistoryPriority(priority uint8) uint8 {
	if priority == 0 {
		return 0
	}
	return priority - 1
}

func getHistoryStream(s Stream) Stream {
	return NewStream(s.Name, s.Key, false)
}

type API struct {
	streamer *Registry
}

func NewAPI(r *Registry) *API {
	return &API{
		streamer: r,
	}
}

func (api *API) SubscribeStream(peerId discover.NodeID, s Stream, history *Range, priority uint8) error {
	return api.streamer.Subscribe(peerId, s, history, priority)
}

func (api *API) UnsubscribeStream(peerId discover.NodeID, s Stream) error {
	return api.streamer.Unsubscribe(peerId, s)
}
