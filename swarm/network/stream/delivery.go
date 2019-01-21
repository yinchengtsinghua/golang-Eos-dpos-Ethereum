
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
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p/discover"
	cp "github.com/ethereum/go-ethereum/swarm/chunk"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/spancontext"
	"github.com/ethereum/go-ethereum/swarm/storage"
	opentracing "github.com/opentracing/opentracing-go"
)

const (
	swarmChunkServerStreamName = "RETRIEVE_REQUEST"
	deliveryCap                = 32
)

var (
	processReceivedChunksCount    = metrics.NewRegisteredCounter("network.stream.received_chunks.count", nil)
	handleRetrieveRequestMsgCount = metrics.NewRegisteredCounter("network.stream.handle_retrieve_request_msg.count", nil)

	requestFromPeersCount     = metrics.NewRegisteredCounter("network.stream.request_from_peers.count", nil)
	requestFromPeersEachCount = metrics.NewRegisteredCounter("network.stream.request_from_peers_each.count", nil)
)

type Delivery struct {
	db       *storage.DBAPI
	overlay  network.Overlay
	receiveC chan *ChunkDeliveryMsg
	getPeer  func(discover.NodeID) *Peer
}

func NewDelivery(overlay network.Overlay, db *storage.DBAPI) *Delivery {
	d := &Delivery{
		db:       db,
		overlay:  overlay,
		receiveC: make(chan *ChunkDeliveryMsg, deliveryCap),
	}

	go d.processReceivedChunks()
	return d
}

//
type SwarmChunkServer struct {
	deliveryC  chan []byte
	batchC     chan []byte
	db         *storage.DBAPI
	currentLen uint64
	quit       chan struct{}
}

//
func NewSwarmChunkServer(db *storage.DBAPI) *SwarmChunkServer {
	s := &SwarmChunkServer{
		deliveryC: make(chan []byte, deliveryCap),
		batchC:    make(chan []byte),
		db:        db,
		quit:      make(chan struct{}),
	}
	go s.processDeliveries()
	return s
}

//
func (s *SwarmChunkServer) processDeliveries() {
	var hashes []byte
	var batchC chan []byte
	for {
		select {
		case <-s.quit:
			return
		case hash := <-s.deliveryC:
			hashes = append(hashes, hash...)
			batchC = s.batchC
		case batchC <- hashes:
			hashes = nil
			batchC = nil
		}
	}
}

//
func (s *SwarmChunkServer) SetNextBatch(_, _ uint64) (hashes []byte, from uint64, to uint64, proof *HandoverProof, err error) {
	select {
	case hashes = <-s.batchC:
	case <-s.quit:
		return
	}

	from = s.currentLen
	s.currentLen += uint64(len(hashes))
	to = s.currentLen
	return
}

//
func (s *SwarmChunkServer) Close() {
	close(s.quit)
}

//
func (s *SwarmChunkServer) GetData(ctx context.Context, key []byte) ([]byte, error) {
	chunk, err := s.db.Get(ctx, storage.Address(key))
	if err == storage.ErrFetching {
		<-chunk.ReqC
	} else if err != nil {
		return nil, err
	}
	return chunk.SData, nil
}

//
type RetrieveRequestMsg struct {
	Addr      storage.Address
	SkipCheck bool
}

func (d *Delivery) handleRetrieveRequestMsg(ctx context.Context, sp *Peer, req *RetrieveRequestMsg) error {
	log.Trace("received request", "peer", sp.ID(), "hash", req.Addr)
	handleRetrieveRequestMsgCount.Inc(1)

	var osp opentracing.Span
	ctx, osp = spancontext.StartSpan(
		ctx,
		"retrieve.request")
	defer osp.Finish()

	s, err := sp.getServer(NewStream(swarmChunkServerStreamName, "", false))
	if err != nil {
		return err
	}
	streamer := s.Server.(*SwarmChunkServer)
	chunk, created := d.db.GetOrCreateRequest(ctx, req.Addr)
	if chunk.ReqC != nil {
		if created {
			if err := d.RequestFromPeers(ctx, chunk.Addr[:], true, sp.ID()); err != nil {
				log.Warn("unable to forward chunk request", "peer", sp.ID(), "key", chunk.Addr, "err", err)
				chunk.SetErrored(storage.ErrChunkForward)
				return nil
			}
		}
		go func() {
			var osp opentracing.Span
			ctx, osp = spancontext.StartSpan(
				ctx,
				"waiting.delivery")
			defer osp.Finish()

			t := time.NewTimer(10 * time.Minute)
			defer t.Stop()

			log.Debug("waiting delivery", "peer", sp.ID(), "hash", req.Addr, "node", common.Bytes2Hex(d.overlay.BaseAddr()), "created", created)
			start := time.Now()
			select {
			case <-chunk.ReqC:
				log.Debug("retrieve request ReqC closed", "peer", sp.ID(), "hash", req.Addr, "time", time.Since(start))
			case <-t.C:
				log.Debug("retrieve request timeout", "peer", sp.ID(), "hash", req.Addr)
				chunk.SetErrored(storage.ErrChunkTimeout)
				return
			}
			chunk.SetErrored(nil)

			if req.SkipCheck {
				err := sp.Deliver(ctx, chunk, s.priority)
				if err != nil {
					log.Warn("ERROR in handleRetrieveRequestMsg, DROPPING peer!", "err", err)
					sp.Drop(err)
				}
			}
			streamer.deliveryC <- chunk.Addr[:]
		}()
		return nil
	}
//
	if req.SkipCheck {
		log.Trace("deliver", "peer", sp.ID(), "hash", chunk.Addr)
		if length := len(chunk.SData); length < 9 {
			log.Error("Chunk.SData to deliver is too short", "len(chunk.SData)", length, "address", chunk.Addr)
		}
		return sp.Deliver(ctx, chunk, s.priority)
	}
	streamer.deliveryC <- chunk.Addr[:]
	return nil
}

type ChunkDeliveryMsg struct {
	Addr  storage.Address
SData []byte //
peer  *Peer  //
}

func (d *Delivery) handleChunkDeliveryMsg(ctx context.Context, sp *Peer, req *ChunkDeliveryMsg) error {
	var osp opentracing.Span
	ctx, osp = spancontext.StartSpan(
		ctx,
		"chunk.delivery")
	defer osp.Finish()

	req.peer = sp
	d.receiveC <- req
	return nil
}

func (d *Delivery) processReceivedChunks() {
R:
	for req := range d.receiveC {
		processReceivedChunksCount.Inc(1)

		if len(req.SData) > cp.DefaultSize+8 {
			log.Warn("received chunk is bigger than expected", "len", len(req.SData))
			continue R
		}

//
		chunk, err := d.db.Get(context.TODO(), req.Addr)
		if err == nil {
			continue R
		}
		if err != storage.ErrFetching {
			log.Error("processReceivedChunks db error", "addr", req.Addr, "err", err, "chunk", chunk)
			continue R
		}
		select {
		case <-chunk.ReqC:
			log.Error("someone else delivered?", "hash", chunk.Addr.Hex())
			continue R
		default:
		}

		chunk.SData = req.SData
		d.db.Put(context.TODO(), chunk)

		go func(req *ChunkDeliveryMsg) {
			err := chunk.WaitToStore()
			if err == storage.ErrChunkInvalid {
				req.peer.Drop(err)
			}
		}(req)
	}
}

//
func (d *Delivery) RequestFromPeers(ctx context.Context, hash []byte, skipCheck bool, peersToSkip ...discover.NodeID) error {
	var success bool
	var err error
	requestFromPeersCount.Inc(1)

	d.overlay.EachConn(hash, 255, func(p network.OverlayConn, po int, nn bool) bool {
		spId := p.(network.Peer).ID()
		for _, p := range peersToSkip {
			if p == spId {
				log.Trace("Delivery.RequestFromPeers: skip peer", "peer", spId)
				return true
			}
		}
		sp := d.getPeer(spId)
		if sp == nil {
			log.Warn("Delivery.RequestFromPeers: peer not found", "id", spId)
			return true
		}
		err = sp.SendPriority(ctx, &RetrieveRequestMsg{
			Addr:      hash,
			SkipCheck: skipCheck,
		}, Top)
		if err != nil {
			return true
		}
		requestFromPeersEachCount.Inc(1)
		success = true
		return false
	})
	if success {
		return nil
	}
	return errors.New("no peer found")
}
