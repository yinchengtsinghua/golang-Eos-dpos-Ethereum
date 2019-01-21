
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
	"bytes"
	"context"
	crand "crypto/rand"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	p2ptest "github.com/ethereum/go-ethereum/p2p/testing"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

func TestStreamerRetrieveRequest(t *testing.T) {
	tester, streamer, _, teardown, err := newStreamerTester(t)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	peerID := tester.IDs[0]

	streamer.delivery.RequestFromPeers(context.TODO(), hash0[:], true)

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "RetrieveRequestMsg",
		Expects: []p2ptest.Expect{
			{
				Code: 5,
				Msg: &RetrieveRequestMsg{
					Addr:      hash0[:],
					SkipCheck: true,
				},
				Peer: peerID,
			},
		},
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestStreamerUpstreamRetrieveRequestMsgExchangeWithoutStore(t *testing.T) {
	tester, streamer, _, teardown, err := newStreamerTester(t)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	peerID := tester.IDs[0]

	chunk := storage.NewChunk(storage.Address(hash0[:]), nil)

	peer := streamer.getPeer(peerID)

	peer.handleSubscribeMsg(context.TODO(), &SubscribeMsg{
		Stream:   NewStream(swarmChunkServerStreamName, "", false),
		History:  nil,
		Priority: Top,
	})

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "RetrieveRequestMsg",
		Triggers: []p2ptest.Trigger{
			{
				Code: 5,
				Msg: &RetrieveRequestMsg{
					Addr: chunk.Addr[:],
				},
				Peer: peerID,
			},
		},
		Expects: []p2ptest.Expect{
			{
				Code: 1,
				Msg: &OfferedHashesMsg{
					HandoverProof: nil,
					Hashes:        nil,
					From:          0,
					To:            0,
				},
				Peer: peerID,
			},
		},
	})

	expectedError := `exchange #0 "RetrieveRequestMsg": timed out`
	if err == nil || err.Error() != expectedError {
		t.Fatalf("Expected error %v, got %v", expectedError, err)
	}
}

//
//
func TestStreamerUpstreamRetrieveRequestMsgExchange(t *testing.T) {
	tester, streamer, localStore, teardown, err := newStreamerTester(t)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	peerID := tester.IDs[0]
	peer := streamer.getPeer(peerID)

	stream := NewStream(swarmChunkServerStreamName, "", false)

	peer.handleSubscribeMsg(context.TODO(), &SubscribeMsg{
		Stream:   stream,
		History:  nil,
		Priority: Top,
	})

	hash := storage.Address(hash0[:])
	chunk := storage.NewChunk(hash, nil)
	chunk.SData = hash
	localStore.Put(context.TODO(), chunk)
	chunk.WaitToStore()

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "RetrieveRequestMsg",
		Triggers: []p2ptest.Trigger{
			{
				Code: 5,
				Msg: &RetrieveRequestMsg{
					Addr: hash,
				},
				Peer: peerID,
			},
		},
		Expects: []p2ptest.Expect{
			{
				Code: 1,
				Msg: &OfferedHashesMsg{
					HandoverProof: &HandoverProof{
						Handover: &Handover{},
					},
					Hashes: hash,
					From:   0,
//
					To:     32,
					Stream: stream,
				},
				Peer: peerID,
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	hash = storage.Address(hash1[:])
	chunk = storage.NewChunk(hash, nil)
	chunk.SData = hash1[:]
	localStore.Put(context.TODO(), chunk)
	chunk.WaitToStore()

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "RetrieveRequestMsg",
		Triggers: []p2ptest.Trigger{
			{
				Code: 5,
				Msg: &RetrieveRequestMsg{
					Addr:      hash,
					SkipCheck: true,
				},
				Peer: peerID,
			},
		},
		Expects: []p2ptest.Expect{
			{
				Code: 6,
				Msg: &ChunkDeliveryMsg{
					Addr:  hash,
					SData: hash,
				},
				Peer: peerID,
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestStreamerDownstreamChunkDeliveryMsgExchange(t *testing.T) {
	tester, streamer, localStore, teardown, err := newStreamerTester(t)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	streamer.RegisterClientFunc("foo", func(p *Peer, t string, live bool) (Client, error) {
		return &testClient{
			t: t,
		}, nil
	})

	peerID := tester.IDs[0]

	stream := NewStream("foo", "", true)
	err = streamer.Subscribe(peerID, stream, NewRange(5, 8), Top)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	chunkKey := hash0[:]
	chunkData := hash1[:]
	chunk, created := localStore.GetOrCreateRequest(context.TODO(), chunkKey)

	if !created {
		t.Fatal("chunk already exists")
	}
	select {
	case <-chunk.ReqC:
		t.Fatal("chunk is already received")
	default:
	}

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "Subscribe message",
		Expects: []p2ptest.Expect{
			{
				Code: 4,
				Msg: &SubscribeMsg{
					Stream:   stream,
					History:  NewRange(5, 8),
					Priority: Top,
				},
				Peer: peerID,
			},
		},
	},
		p2ptest.Exchange{
			Label: "ChunkDeliveryRequest message",
			Triggers: []p2ptest.Trigger{
				{
					Code: 6,
					Msg: &ChunkDeliveryMsg{
						Addr:  chunkKey,
						SData: chunkData,
					},
					Peer: peerID,
				},
			},
		})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	timeout := time.NewTimer(1 * time.Second)

	select {
	case <-timeout.C:
		t.Fatal("timeout receiving chunk")
	case <-chunk.ReqC:
	}

	storedChunk, err := localStore.Get(context.TODO(), chunkKey)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !bytes.Equal(storedChunk.SData, chunkData) {
		t.Fatal("Retrieved chunk has different data than original")
	}

}

func TestDeliveryFromNodes(t *testing.T) {
	testDeliveryFromNodes(t, 2, 1, dataChunkCount, true)
	testDeliveryFromNodes(t, 2, 1, dataChunkCount, false)
	testDeliveryFromNodes(t, 4, 1, dataChunkCount, true)
	testDeliveryFromNodes(t, 4, 1, dataChunkCount, false)
	testDeliveryFromNodes(t, 8, 1, dataChunkCount, true)
	testDeliveryFromNodes(t, 8, 1, dataChunkCount, false)
	testDeliveryFromNodes(t, 16, 1, dataChunkCount, true)
	testDeliveryFromNodes(t, 16, 1, dataChunkCount, false)
}

func testDeliveryFromNodes(t *testing.T, nodes, conns, chunkCount int, skipCheck bool) {
	sim := simulation.New(map[string]simulation.ServiceFunc{
		"streamer": func(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error) {

			id := ctx.Config.ID
			addr := network.NewAddrFromNodeID(id)
			store, datadir, err := createTestLocalStorageForID(id, addr)
			if err != nil {
				return nil, nil, err
			}
			bucket.Store(bucketKeyStore, store)
			cleanup = func() {
				os.RemoveAll(datadir)
				store.Close()
			}
			localStore := store.(*storage.LocalStore)
			db := storage.NewDBAPI(localStore)
			kad := network.NewKademlia(addr.Over(), network.NewKadParams())
			delivery := NewDelivery(kad, db)

			r := NewRegistry(addr, delivery, db, state.NewInmemoryStore(), &RegistryOptions{
				SkipCheck: skipCheck,
			})
			bucket.Store(bucketKeyRegistry, r)

			retrieveFunc := func(ctx context.Context, chunk *storage.Chunk) error {
				return delivery.RequestFromPeers(ctx, chunk.Addr[:], skipCheck)
			}
			netStore := storage.NewNetStore(localStore, retrieveFunc)
			fileStore := storage.NewFileStore(netStore, storage.NewFileStoreParams())
			bucket.Store(bucketKeyFileStore, fileStore)

			return r, cleanup, nil

		},
	})
	defer sim.Close()

	log.Info("Adding nodes to simulation")
	_, err := sim.AddNodesAndConnectChain(nodes)
	if err != nil {
		t.Fatal(err)
	}

	log.Info("Starting simulation")
	ctx := context.Background()
	result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) error {
		nodeIDs := sim.UpNodeIDs()
//
		sim.SetPivotNode(nodeIDs[0])
//
//
//
//
		log.Debug("Writing file to round-robin file store")
//
		stores := make([]storage.ChunkStore, len(nodeIDs)-1)
//
		lStores := sim.NodesItems(bucketKeyStore)
		i := 0
//
		for id, bucketVal := range lStores {
//
			if id == *sim.PivotNodeID() {
				continue
			}
//
			stores[i] = bucketVal.(storage.ChunkStore)
			i++
		}
//
		roundRobinFileStore := storage.NewFileStore(newRoundRobinStore(stores...), storage.NewFileStoreParams())
//
		size := chunkCount * chunkSize
		log.Debug("Storing data to file store")
		fileHash, wait, err := roundRobinFileStore.Store(ctx, io.LimitReader(crand.Reader, int64(size)), int64(size), false)
//
		if err != nil {
			return err
		}
		err = wait(ctx)
		if err != nil {
			return err
		}

		log.Debug("Waiting for kademlia")
		if _, err := sim.WaitTillHealthy(ctx, 2); err != nil {
			return err
		}

//
		for j, node := range nodeIDs[0 : nodes-1] {
			sid := nodeIDs[j+1]
			item, ok := sim.NodeItem(node, bucketKeyRegistry)
			if !ok {
				return fmt.Errorf("No registry")
			}
			registry := item.(*Registry)
			err = registry.Subscribe(sid, NewStream(swarmChunkServerStreamName, "", false), NewRange(0, 0), Top)
			if err != nil {
				return err
			}
		}

//
		item, ok := sim.NodeItem(*sim.PivotNodeID(), bucketKeyFileStore)
		if !ok {
			return fmt.Errorf("No filestore")
		}
		pivotFileStore := item.(*storage.FileStore)
		log.Debug("Starting retrieval routine")
		go func() {
//
//
			n, err := readAll(pivotFileStore, fileHash)
			log.Info(fmt.Sprintf("retrieved %v", fileHash), "read", n, "err", err)
			if err != nil {
				t.Fatalf("requesting chunks action error: %v", err)
			}
		}()

		log.Debug("Watching for disconnections")
		disconnections := sim.PeerEvents(
			context.Background(),
			sim.NodeIDs(),
			simulation.NewPeerEventsFilter().Type(p2p.PeerEventTypeDrop),
		)

		go func() {
			for d := range disconnections {
				if d.Error != nil {
					log.Error("peer drop", "node", d.NodeID, "peer", d.Event.Peer)
					t.Fatal(d.Error)
				}
			}
		}()

//
		log.Debug("Check retrieval")
		success := true
		var total int64
		total, err = readAll(pivotFileStore, fileHash)
		if err != nil {
			return err
		}
		log.Info(fmt.Sprintf("check if %08x is available locally: number of bytes read %v/%v (error: %v)", fileHash, total, size, err))
		if err != nil || total != int64(size) {
			success = false
		}

		if !success {
			return fmt.Errorf("Test failed, chunks not available on all nodes")
		}
		log.Debug("Test terminated successfully")
		return nil
	})
	if result.Error != nil {
		t.Fatal(result.Error)
	}
}

func BenchmarkDeliveryFromNodesWithoutCheck(b *testing.B) {
	for chunks := 32; chunks <= 128; chunks *= 2 {
		for i := 2; i < 32; i *= 2 {
			b.Run(
				fmt.Sprintf("nodes=%v,chunks=%v", i, chunks),
				func(b *testing.B) {
					benchmarkDeliveryFromNodes(b, i, 1, chunks, true)
				},
			)
		}
	}
}

func BenchmarkDeliveryFromNodesWithCheck(b *testing.B) {
	for chunks := 32; chunks <= 128; chunks *= 2 {
		for i := 2; i < 32; i *= 2 {
			b.Run(
				fmt.Sprintf("nodes=%v,chunks=%v", i, chunks),
				func(b *testing.B) {
					benchmarkDeliveryFromNodes(b, i, 1, chunks, false)
				},
			)
		}
	}
}

func benchmarkDeliveryFromNodes(b *testing.B, nodes, conns, chunkCount int, skipCheck bool) {
	sim := simulation.New(map[string]simulation.ServiceFunc{
		"streamer": func(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error) {

			id := ctx.Config.ID
			addr := network.NewAddrFromNodeID(id)
			store, datadir, err := createTestLocalStorageForID(id, addr)
			if err != nil {
				return nil, nil, err
			}
			bucket.Store(bucketKeyStore, store)
			cleanup = func() {
				os.RemoveAll(datadir)
				store.Close()
			}
			localStore := store.(*storage.LocalStore)
			db := storage.NewDBAPI(localStore)
			kad := network.NewKademlia(addr.Over(), network.NewKadParams())
			delivery := NewDelivery(kad, db)

			r := NewRegistry(addr, delivery, db, state.NewInmemoryStore(), &RegistryOptions{
				SkipCheck:       skipCheck,
				DoSync:          true,
				SyncUpdateDelay: 0,
			})

			retrieveFunc := func(ctx context.Context, chunk *storage.Chunk) error {
				return delivery.RequestFromPeers(ctx, chunk.Addr[:], skipCheck)
			}
			netStore := storage.NewNetStore(localStore, retrieveFunc)
			fileStore := storage.NewFileStore(netStore, storage.NewFileStoreParams())
			bucket.Store(bucketKeyFileStore, fileStore)

			return r, cleanup, nil

		},
	})
	defer sim.Close()

	log.Info("Initializing test config")
	_, err := sim.AddNodesAndConnectChain(nodes)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) error {
		nodeIDs := sim.UpNodeIDs()
		node := nodeIDs[len(nodeIDs)-1]

		item, ok := sim.NodeItem(node, bucketKeyFileStore)
		if !ok {
			b.Fatal("No filestore")
		}
		remoteFileStore := item.(*storage.FileStore)

		pivotNode := nodeIDs[0]
		item, ok = sim.NodeItem(pivotNode, bucketKeyNetStore)
		if !ok {
			b.Fatal("No filestore")
		}
		netStore := item.(*storage.NetStore)

		if _, err := sim.WaitTillHealthy(ctx, 2); err != nil {
			return err
		}

		disconnections := sim.PeerEvents(
			context.Background(),
			sim.NodeIDs(),
			simulation.NewPeerEventsFilter().Type(p2p.PeerEventTypeDrop),
		)

		go func() {
			for d := range disconnections {
				if d.Error != nil {
					log.Error("peer drop", "node", d.NodeID, "peer", d.Event.Peer)
					b.Fatal(d.Error)
				}
			}
		}()
//
		b.ResetTimer()
		b.StopTimer()
	Loop:
		for i := 0; i < b.N; i++ {
//
			hashes := make([]storage.Address, chunkCount)
			for i := 0; i < chunkCount; i++ {
//
				ctx := context.TODO()
				hash, wait, err := remoteFileStore.Store(ctx, io.LimitReader(crand.Reader, int64(chunkSize)), int64(chunkSize), false)
				if err != nil {
					b.Fatalf("expected no error. got %v", err)
				}
//
				err = wait(ctx)
				if err != nil {
					b.Fatalf("expected no error. got %v", err)
				}
//
				hashes[i] = hash
			}
//
//
			b.StartTimer()
			errs := make(chan error)
			for _, hash := range hashes {
				go func(h storage.Address) {
					_, err := netStore.Get(ctx, h)
					log.Warn("test check netstore get", "hash", h, "err", err)
					errs <- err
				}(hash)
			}
//
//
			var total, misses int
			for err := range errs {
				if err != nil {
					log.Warn(err.Error())
					misses++
				}
				total++
				if total == chunkCount {
					break
				}
			}
			b.StopTimer()

			if misses > 0 {
				err = fmt.Errorf("%v chunk not found out of %v", misses, total)
				break Loop
			}
		}
		if err != nil {
			b.Fatal(err)
		}
		return nil
	})
	if result.Error != nil {
		b.Fatal(result.Error)
	}

}
