
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
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
	"github.com/ethereum/go-ethereum/swarm/pot"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
	mockdb "github.com/ethereum/go-ethereum/swarm/storage/mock/db"
)

const testMinProxBinSize = 2
const MaxTimeout = 600

type synctestConfig struct {
	addrs            [][]byte
	hashes           []storage.Address
	idToChunksMap    map[discover.NodeID][]int
	chunksToNodesMap map[string][]int
	addrToIDMap      map[string]discover.NodeID
}

//
//
//
//
//
//
//
func TestSyncingViaGlobalSync(t *testing.T) {
//
//
	if *nodes != 0 && *chunks != 0 {
		log.Info(fmt.Sprintf("Running test with %d chunks and %d nodes...", *chunks, *nodes))
		testSyncingViaGlobalSync(t, *chunks, *nodes)
	} else {
		var nodeCnt []int
		var chnkCnt []int
//
//
		if *longrunning {
			chnkCnt = []int{1, 8, 32, 256, 1024}
			nodeCnt = []int{16, 32, 64, 128, 256}
		} else {
//
			chnkCnt = []int{4, 32}
			nodeCnt = []int{32, 16}
		}
		for _, chnk := range chnkCnt {
			for _, n := range nodeCnt {
				log.Info(fmt.Sprintf("Long running test with %d chunks and %d nodes...", chnk, n))
				testSyncingViaGlobalSync(t, chnk, n)
			}
		}
	}
}

func TestSyncingViaDirectSubscribe(t *testing.T) {
//
//
	if *nodes != 0 && *chunks != 0 {
		log.Info(fmt.Sprintf("Running test with %d chunks and %d nodes...", *chunks, *nodes))
		err := testSyncingViaDirectSubscribe(*chunks, *nodes)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		var nodeCnt []int
		var chnkCnt []int
//
//
		if *longrunning {
			chnkCnt = []int{1, 8, 32, 256, 1024}
			nodeCnt = []int{32, 16}
		} else {
//
			chnkCnt = []int{4, 32}
			nodeCnt = []int{32, 16}
		}
		for _, chnk := range chnkCnt {
			for _, n := range nodeCnt {
				log.Info(fmt.Sprintf("Long running test with %d chunks and %d nodes...", chnk, n))
				err := testSyncingViaDirectSubscribe(chnk, n)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func testSyncingViaGlobalSync(t *testing.T, chunkCount int, nodeCount int) {
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
				DoSync:          true,
				SyncUpdateDelay: 3 * time.Second,
			})
			bucket.Store(bucketKeyRegistry, r)

			return r, cleanup, nil

		},
	})
	defer sim.Close()

	log.Info("Initializing test config")

	conf := &synctestConfig{}
//
	conf.idToChunksMap = make(map[discover.NodeID][]int)
//
	conf.addrToIDMap = make(map[string]discover.NodeID)
//
	conf.hashes = make([]storage.Address, 0)

	err := sim.UploadSnapshot(fmt.Sprintf("testing/snapshot_%d.json", nodeCount))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancelSimRun := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelSimRun()

	result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) error {
		nodeIDs := sim.UpNodeIDs()
		for _, n := range nodeIDs {
//
			a := network.ToOverlayAddr(n.Bytes())
//
			conf.addrs = append(conf.addrs, a)
//
//
//
			conf.addrToIDMap[string(a)] = n
		}

//
//
		node := sim.RandomUpNode()
		item, ok := sim.NodeItem(node.ID, bucketKeyStore)
		if !ok {
			return fmt.Errorf("No localstore")
		}
		lstore := item.(*storage.LocalStore)
		hashes, err := uploadFileToSingleNodeStore(node.ID, chunkCount, lstore)
		if err != nil {
			return err
		}
		conf.hashes = append(conf.hashes, hashes...)
		mapKeysToNodes(conf)

		if _, err := sim.WaitTillHealthy(ctx, 2); err != nil {
			return err
		}

//
//
		allSuccess := false
		var gDir string
		var globalStore *mockdb.GlobalStore
		if *useMockStore {
			gDir, globalStore, err = createGlobalStore()
			if err != nil {
				return fmt.Errorf("Something went wrong; using mockStore enabled but globalStore is nil")
			}
			defer func() {
				os.RemoveAll(gDir)
				err := globalStore.Close()
				if err != nil {
					log.Error("Error closing global store! %v", "err", err)
				}
			}()
		}
		for !allSuccess {
			for _, id := range nodeIDs {
//
				localChunks := conf.idToChunksMap[id]
				localSuccess := true
				for _, ch := range localChunks {
//
					chunk := conf.hashes[ch]
					log.Trace(fmt.Sprintf("node has chunk: %s:", chunk))
//
					var err error
					if *useMockStore {
//
//
						_, err = globalStore.Get(common.BytesToAddress(id.Bytes()), chunk)
					} else {
//
						item, ok := sim.NodeItem(id, bucketKeyStore)
						if !ok {
							return fmt.Errorf("Error accessing localstore")
						}
						lstore := item.(*storage.LocalStore)
						_, err = lstore.Get(ctx, chunk)
					}
					if err != nil {
						log.Warn(fmt.Sprintf("Chunk %s NOT found for id %s", chunk, id))
						localSuccess = false
//
						time.Sleep(500 * time.Millisecond)
					} else {
						log.Debug(fmt.Sprintf("Chunk %s IS FOUND for id %s", chunk, id))
					}
				}
				allSuccess = localSuccess
			}
		}
		if !allSuccess {
			return fmt.Errorf("Not all chunks succeeded!")
		}
		return nil
	})

	if result.Error != nil {
		t.Fatal(result.Error)
	}
}

/*









*/

func testSyncingViaDirectSubscribe(chunkCount int, nodeCount int) error {
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

			r := NewRegistry(addr, delivery, db, state.NewInmemoryStore(), nil)
			bucket.Store(bucketKeyRegistry, r)

			fileStore := storage.NewFileStore(storage.NewNetStore(localStore, nil), storage.NewFileStoreParams())
			bucket.Store(bucketKeyFileStore, fileStore)

			return r, cleanup, nil

		},
	})
	defer sim.Close()

	ctx, cancelSimRun := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelSimRun()

	conf := &synctestConfig{}
//
	conf.idToChunksMap = make(map[discover.NodeID][]int)
//
	conf.addrToIDMap = make(map[string]discover.NodeID)
//
	conf.hashes = make([]storage.Address, 0)

	err := sim.UploadSnapshot(fmt.Sprintf("testing/snapshot_%d.json", nodeCount))
	if err != nil {
		return err
	}

	result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) error {
		nodeIDs := sim.UpNodeIDs()
		for _, n := range nodeIDs {
//
			a := network.ToOverlayAddr(n.Bytes())
//
			conf.addrs = append(conf.addrs, a)
//
//
//
			conf.addrToIDMap[string(a)] = n
		}

		var subscriptionCount int

		filter := simulation.NewPeerEventsFilter().Type(p2p.PeerEventTypeMsgRecv).Protocol("stream").MsgCode(4)
		eventC := sim.PeerEvents(ctx, nodeIDs, filter)

		for j, node := range nodeIDs {
			log.Trace(fmt.Sprintf("Start syncing subscriptions: %d", j))
//
			item, ok := sim.NodeItem(node, bucketKeyRegistry)
			if !ok {
				return fmt.Errorf("No registry")
			}
			registry := item.(*Registry)

			var cnt int
			cnt, err = startSyncing(registry, conf)
			if err != nil {
				return err
			}
//
//
			subscriptionCount += cnt
		}

		for e := range eventC {
			if e.Error != nil {
				return e.Error
			}
			subscriptionCount--
			if subscriptionCount == 0 {
				break
			}
		}
//
		node := sim.RandomUpNode()
		item, ok := sim.NodeItem(node.ID, bucketKeyStore)
		if !ok {
			return fmt.Errorf("No localstore")
		}
		lstore := item.(*storage.LocalStore)
		hashes, err := uploadFileToSingleNodeStore(node.ID, chunkCount, lstore)
		if err != nil {
			return err
		}
		conf.hashes = append(conf.hashes, hashes...)
		mapKeysToNodes(conf)

		if _, err := sim.WaitTillHealthy(ctx, 2); err != nil {
			return err
		}

		var gDir string
		var globalStore *mockdb.GlobalStore
		if *useMockStore {
			gDir, globalStore, err = createGlobalStore()
			if err != nil {
				return fmt.Errorf("Something went wrong; using mockStore enabled but globalStore is nil")
			}
			defer os.RemoveAll(gDir)
		}
//
//
		allSuccess := false
		for !allSuccess {
			for _, id := range nodeIDs {
//
				localChunks := conf.idToChunksMap[id]
				localSuccess := true
				for _, ch := range localChunks {
//
					chunk := conf.hashes[ch]
					log.Trace(fmt.Sprintf("node has chunk: %s:", chunk))
//
					var err error
					if *useMockStore {
//
//
						_, err = globalStore.Get(common.BytesToAddress(id.Bytes()), chunk)
					} else {
//
						item, ok := sim.NodeItem(id, bucketKeyStore)
						if !ok {
							return fmt.Errorf("Error accessing localstore")
						}
						lstore := item.(*storage.LocalStore)
						_, err = lstore.Get(ctx, chunk)
					}
					if err != nil {
						log.Warn(fmt.Sprintf("Chunk %s NOT found for id %s", chunk, id))
						localSuccess = false
//
						time.Sleep(500 * time.Millisecond)
					} else {
						log.Debug(fmt.Sprintf("Chunk %s IS FOUND for id %s", chunk, id))
					}
				}
				allSuccess = localSuccess
			}
		}
		if !allSuccess {
			return fmt.Errorf("Not all chunks succeeded!")
		}
		return nil
	})

	if result.Error != nil {
		return result.Error
	}

	log.Info("Simulation terminated")
	return nil
}

//
//
//
//
func startSyncing(r *Registry, conf *synctestConfig) (int, error) {
	var err error

	kad, ok := r.delivery.overlay.(*network.Kademlia)
	if !ok {
		return 0, fmt.Errorf("Not a Kademlia!")
	}

	subCnt := 0
//
	kad.EachBin(r.addr.Over(), pof, 0, func(conn network.OverlayConn, po int) bool {
//
		histRange := &Range{}

		subCnt++
		err = r.RequestSubscription(conf.addrToIDMap[string(conn.Address())], NewStream("SYNC", FormatSyncBinKey(uint8(po)), true), histRange, Top)
		if err != nil {
			log.Error(fmt.Sprintf("Error in RequestSubsciption! %v", err))
			return false
		}
		return true

	})
	return subCnt, nil
}

//
func mapKeysToNodes(conf *synctestConfig) {
	kmap := make(map[string][]int)
	nodemap := make(map[string][]int)
//
	np := pot.NewPot(nil, 0)
	indexmap := make(map[string]int)
	for i, a := range conf.addrs {
		indexmap[string(a)] = i
		np, _, _ = pot.Add(np, a, pof)
	}
//
	log.Trace(fmt.Sprintf("Generated hash chunk(s): %v", conf.hashes))
	for i := 0; i < len(conf.hashes); i++ {
pl := 256 //
		var nns []int
		np.EachNeighbour([]byte(conf.hashes[i]), pof, func(val pot.Val, po int) bool {
			a := val.([]byte)
			if pl < 256 && pl != po {
				return false
			}
			if pl == 256 || pl == po {
				log.Trace(fmt.Sprintf("appending %s", conf.addrToIDMap[string(a)]))
				nns = append(nns, indexmap[string(a)])
				nodemap[string(a)] = append(nodemap[string(a)], i)
			}
			if pl == 256 && len(nns) >= testMinProxBinSize {
//
//
				pl = po
			}
			return true
		})
		kmap[string(conf.hashes[i])] = nns
	}
	for addr, chunks := range nodemap {
//
		conf.idToChunksMap[conf.addrToIDMap[addr]] = chunks
	}
	log.Debug(fmt.Sprintf("Map of expected chunks by ID: %v", conf.idToChunksMap))
	conf.chunksToNodesMap = kmap
}

//
func uploadFileToSingleNodeStore(id discover.NodeID, chunkCount int, lstore *storage.LocalStore) ([]storage.Address, error) {
	log.Debug(fmt.Sprintf("Uploading to node id: %s", id))
	fileStore := storage.NewFileStore(lstore, storage.NewFileStoreParams())
	size := chunkSize
	var rootAddrs []storage.Address
	for i := 0; i < chunkCount; i++ {
		rk, wait, err := fileStore.Store(context.TODO(), io.LimitReader(crand.Reader, int64(size)), int64(size), false)
		if err != nil {
			return nil, err
		}
		err = wait(context.TODO())
		if err != nil {
			return nil, err
		}
		rootAddrs = append(rootAddrs, (rk))
	}

	return rootAddrs, nil
}
