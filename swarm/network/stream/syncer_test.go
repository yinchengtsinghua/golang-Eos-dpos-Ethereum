
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
	crand "crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
	mockdb "github.com/ethereum/go-ethereum/swarm/storage/mock/db"
)

const dataChunkCount = 200

func TestSyncerSimulation(t *testing.T) {
	testSyncBetweenNodes(t, 2, 1, dataChunkCount, true, 1)
	testSyncBetweenNodes(t, 4, 1, dataChunkCount, true, 1)
	testSyncBetweenNodes(t, 8, 1, dataChunkCount, true, 1)
	testSyncBetweenNodes(t, 16, 1, dataChunkCount, true, 1)
}

func createMockStore(globalStore *mockdb.GlobalStore, id discover.NodeID, addr *network.BzzAddr) (lstore storage.ChunkStore, datadir string, err error) {
	address := common.BytesToAddress(id.Bytes())
	mockStore := globalStore.NewNodeStore(address)
	params := storage.NewDefaultLocalStoreParams()

	datadir, err = ioutil.TempDir("", "localMockStore-"+id.TerminalString())
	if err != nil {
		return nil, "", err
	}
	params.Init(datadir)
	params.BaseKey = addr.Over()
	lstore, err = storage.NewLocalStore(params, mockStore)
	return lstore, datadir, nil
}

func testSyncBetweenNodes(t *testing.T, nodes, conns, chunkCount int, skipCheck bool, po uint8) {
	sim := simulation.New(map[string]simulation.ServiceFunc{
		"streamer": func(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error) {
			var store storage.ChunkStore
			var globalStore *mockdb.GlobalStore
			var gDir, datadir string

			id := ctx.Config.ID
			addr := network.NewAddrFromNodeID(id)
//
			addr.OAddr[0] = byte(0)

			if *useMockStore {
				gDir, globalStore, err = createGlobalStore()
				if err != nil {
					return nil, nil, fmt.Errorf("Something went wrong; using mockStore enabled but globalStore is nil")
				}
				store, datadir, err = createMockStore(globalStore, id, addr)
			} else {
				store, datadir, err = createTestLocalStorageForID(id, addr)
			}
			if err != nil {
				return nil, nil, err
			}
			bucket.Store(bucketKeyStore, store)
			cleanup = func() {
				store.Close()
				os.RemoveAll(datadir)
				if *useMockStore {
					err := globalStore.Close()
					if err != nil {
						log.Error("Error closing global store! %v", "err", err)
					}
					os.RemoveAll(gDir)
				}
			}
			localStore := store.(*storage.LocalStore)
			db := storage.NewDBAPI(localStore)
			bucket.Store(bucketKeyDB, db)
			kad := network.NewKademlia(addr.Over(), network.NewKadParams())
			delivery := NewDelivery(kad, db)
			bucket.Store(bucketKeyDelivery, delivery)

			r := NewRegistry(addr, delivery, db, state.NewInmemoryStore(), &RegistryOptions{
				SkipCheck: skipCheck,
			})

			fileStore := storage.NewFileStore(storage.NewNetStore(localStore, nil), storage.NewFileStoreParams())
			bucket.Store(bucketKeyFileStore, fileStore)

			return r, cleanup, nil

		},
	})
	defer sim.Close()

//
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
//
	defer cancel()

	_, err := sim.AddNodesAndConnectChain(nodes)
	if err != nil {
		t.Fatal(err)
	}
	result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) error {
		nodeIDs := sim.UpNodeIDs()

		nodeIndex := make(map[discover.NodeID]int)
		for i, id := range nodeIDs {
			nodeIndex[id] = i
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
					t.Fatal(d.Error)
				}
			}
		}()

//
		for j := 0; j < nodes-1; j++ {
			id := nodeIDs[j]
			client, err := sim.Net.GetNode(id).Client()
			if err != nil {
				t.Fatal(err)
			}
			sid := nodeIDs[j+1]
			client.CallContext(ctx, nil, "stream_subscribeStream", sid, NewStream("SYNC", FormatSyncBinKey(1), false), NewRange(0, 0), Top)
			if err != nil {
				return err
			}
			if j > 0 || nodes == 2 {
				item, ok := sim.NodeItem(nodeIDs[j], bucketKeyFileStore)
				if !ok {
					return fmt.Errorf("No filestore")
				}
				fileStore := item.(*storage.FileStore)
				size := chunkCount * chunkSize
				_, wait, err := fileStore.Store(ctx, io.LimitReader(crand.Reader, int64(size)), int64(size), false)
				if err != nil {
					t.Fatal(err.Error())
				}
				wait(ctx)
			}
		}
//
		if _, err := sim.WaitTillHealthy(ctx, 2); err != nil {
			return err
		}

//
		hashes := make([][]storage.Address, nodes)
		totalHashes := 0
		hashCounts := make([]int, nodes)
		for i := nodes - 1; i >= 0; i-- {
			if i < nodes-1 {
				hashCounts[i] = hashCounts[i+1]
			}
			item, ok := sim.NodeItem(nodeIDs[i], bucketKeyDB)
			if !ok {
				return fmt.Errorf("No DB")
			}
			db := item.(*storage.DBAPI)
			db.Iterator(0, math.MaxUint64, po, func(addr storage.Address, index uint64) bool {
				hashes[i] = append(hashes[i], addr)
				totalHashes++
				hashCounts[i]++
				return true
			})
		}
		var total, found int
		for _, node := range nodeIDs {
			i := nodeIndex[node]

			for j := i; j < nodes; j++ {
				total += len(hashes[j])
				for _, key := range hashes[j] {
					item, ok := sim.NodeItem(nodeIDs[j], bucketKeyDB)
					if !ok {
						return fmt.Errorf("No DB")
					}
					db := item.(*storage.DBAPI)
					chunk, err := db.Get(ctx, key)
					if err == storage.ErrFetching {
						<-chunk.ReqC
					} else if err != nil {
						continue
					}
//
//
					found++
				}
			}
			log.Debug("sync check", "node", node, "index", i, "bin", po, "found", found, "total", total)
		}
		if total == found && total > 0 {
			return nil
		}
		return fmt.Errorf("Total not equallying found: total is %d", total)
	})

	if result.Error != nil {
		t.Fatal(result.Error)
	}
}
