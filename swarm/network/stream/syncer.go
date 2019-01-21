
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
	"math"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

const (
//
	BatchSize = 128
)

//
//
//
//
type SwarmSyncerServer struct {
	po        uint8
	db        *storage.DBAPI
	sessionAt uint64
	start     uint64
	quit      chan struct{}
}

//
func NewSwarmSyncerServer(live bool, po uint8, db *storage.DBAPI) (*SwarmSyncerServer, error) {
	sessionAt := db.CurrentBucketStorageIndex(po)
	var start uint64
	if live {
		start = sessionAt
	}
	return &SwarmSyncerServer{
		po:        po,
		db:        db,
		sessionAt: sessionAt,
		start:     start,
		quit:      make(chan struct{}),
	}, nil
}

func RegisterSwarmSyncerServer(streamer *Registry, db *storage.DBAPI) {
	streamer.RegisterServerFunc("SYNC", func(p *Peer, t string, live bool) (Server, error) {
		po, err := ParseSyncBinKey(t)
		if err != nil {
			return nil, err
		}
		return NewSwarmSyncerServer(live, po, db)
	})
//
//
//
}

//
func (s *SwarmSyncerServer) Close() {
	close(s.quit)
}

//
func (s *SwarmSyncerServer) GetData(ctx context.Context, key []byte) ([]byte, error) {
	chunk, err := s.db.Get(ctx, storage.Address(key))
	if err == storage.ErrFetching {
		<-chunk.ReqC
	} else if err != nil {
		return nil, err
	}
	return chunk.SData, nil
}

//
func (s *SwarmSyncerServer) SetNextBatch(from, to uint64) ([]byte, uint64, uint64, *HandoverProof, error) {
	var batch []byte
	i := 0
	if from == 0 {
		from = s.start
	}
	if to <= from || from >= s.sessionAt {
		to = math.MaxUint64
	}
	var ticker *time.Ticker
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()
	var wait bool
	for {
		if wait {
			if ticker == nil {
				ticker = time.NewTicker(1000 * time.Millisecond)
			}
			select {
			case <-ticker.C:
			case <-s.quit:
				return nil, 0, 0, nil, nil
			}
		}

		metrics.GetOrRegisterCounter("syncer.setnextbatch.iterator", nil).Inc(1)
		err := s.db.Iterator(from, to, s.po, func(addr storage.Address, idx uint64) bool {
			batch = append(batch, addr[:]...)
			i++
			to = idx
			return i < BatchSize
		})
		if err != nil {
			return nil, 0, 0, nil, err
		}
		if len(batch) > 0 {
			break
		}
		wait = true
	}

	log.Trace("Swarm syncer offer batch", "po", s.po, "len", i, "from", from, "to", to, "current store count", s.db.CurrentBucketStorageIndex(s.po))
	return batch, from, to, nil, nil
}

//
type SwarmSyncerClient struct {
	sessionAt     uint64
	nextC         chan struct{}
	sessionRoot   storage.Address
	sessionReader storage.LazySectionReader
	retrieveC     chan *storage.Chunk
	storeC        chan *storage.Chunk
	db            *storage.DBAPI
//
	currentRoot           storage.Address
	requestFunc           func(chunk *storage.Chunk)
	end, start            uint64
	peer                  *Peer
	ignoreExistingRequest bool
	stream                Stream
}

//
func NewSwarmSyncerClient(p *Peer, db *storage.DBAPI, ignoreExistingRequest bool, stream Stream) (*SwarmSyncerClient, error) {
	return &SwarmSyncerClient{
		db:   db,
		peer: p,
		ignoreExistingRequest: ignoreExistingRequest,
		stream:                stream,
	}, nil
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
//
//
//
//
//
//

//
//
func RegisterSwarmSyncerClient(streamer *Registry, db *storage.DBAPI) {
	streamer.RegisterClientFunc("SYNC", func(p *Peer, t string, live bool) (Client, error) {
		return NewSwarmSyncerClient(p, db, true, NewStream("SYNC", t, live))
	})
}

//
func (s *SwarmSyncerClient) NeedData(ctx context.Context, key []byte) (wait func()) {
	chunk, _ := s.db.GetOrCreateRequest(ctx, key)
//

//
//
if chunk.ReqC == nil { //
		return nil
	}
//
	return func() {
		chunk.WaitToStore()
	}
}

//
func (s *SwarmSyncerClient) BatchDone(stream Stream, from uint64, hashes []byte, root []byte) func() (*TakeoverProof, error) {
//
//
//
//
	return nil
}

func (s *SwarmSyncerClient) TakeoverProof(stream Stream, from uint64, hashes []byte, root storage.Address) (*TakeoverProof, error) {
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
	s.end += uint64(len(hashes)) / HashSize
	takeover := &Takeover{
		Stream: stream,
		Start:  s.start,
		End:    s.end,
		Root:   root,
	}
//
	return &TakeoverProof{
		Takeover: takeover,
		Sig:      nil,
	}, nil
}

func (s *SwarmSyncerClient) Close() {}

//
//
const syncBinKeyBase = 36

//
//
func FormatSyncBinKey(bin uint8) string {
	return strconv.FormatUint(uint64(bin), syncBinKeyBase)
}

//
//
func ParseSyncBinKey(s string) (uint8, error) {
	bin, err := strconv.ParseUint(s, syncBinKeyBase, 8)
	if err != nil {
		return 0, err
	}
	return uint8(bin), nil
}
