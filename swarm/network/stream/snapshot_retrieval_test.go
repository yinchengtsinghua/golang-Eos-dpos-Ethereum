
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
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

//
const (
	minFileSize = 2
	maxFileSize = 40
)

//
//
//
//
//
func TestFileRetrieval(t *testing.T) {
	if *nodes != 0 {
		err := runFileRetrievalTest(*nodes)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		nodeCnt := []int{16}
//
//
		if *longrunning {
			nodeCnt = append(nodeCnt, 32, 64, 128)
		}
		for _, n := range nodeCnt {
			err := runFileRetrievalTest(n)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

//
//
//
//
//
//
func TestRetrieval(t *testing.T) {
//
//
	if *nodes != 0 && *chunks != 0 {
		err := runRetrievalTest(*chunks, *nodes)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		var nodeCnt []int
		var chnkCnt []int
//
//
		if *longrunning {
			nodeCnt = []int{16, 32, 128}
			chnkCnt = []int{4, 32, 256}
		} else {
//
			nodeCnt = []int{16}
			chnkCnt = []int{32}
		}
		for _, n := range nodeCnt {
			for _, c := range chnkCnt {
				err := runRetrievalTest(c, n)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

/*







*/

func runFileRetrievalTest(nodeCount int) error {
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

			fileStore := storage.NewFileStore(storage.NewNetStore(localStore, nil), storage.NewFileStoreParams())
			bucket.Store(bucketKeyFileStore, fileStore)

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
		return err
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
		var randomFiles []string
//
//
//

		conf.hashes, randomFiles, err = uploadFilesToNodes(sim)
		if err != nil {
			return err
		}
		if _, err := sim.WaitTillHealthy(ctx, 2); err != nil {
			return err
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
//
					item, ok := sim.NodeItem(id, bucketKeyFileStore)
					if !ok {
						return fmt.Errorf("No registry")
					}
					fileStore := item.(*storage.FileStore)
//
					for i, hash := range conf.hashes {
						reader, _ := fileStore.Retrieve(context.TODO(), hash)
//
						if s, err := reader.Size(ctx, nil); err != nil || s != int64(len(randomFiles[i])) {
							allSuccess = false
							log.Warn("Retrieve error", "err", err, "hash", hash, "nodeId", id)
						} else {
							log.Debug(fmt.Sprintf("File with root hash %x successfully retrieved", hash))
						}
					}
					if err != nil {
						log.Warn(fmt.Sprintf("Chunk %s NOT found for id %s", chunk, id))
						localSuccess = false
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

	return nil
}

/*








*/

func runRetrievalTest(chunkCount int, nodeCount int) error {
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
				SyncUpdateDelay: 0,
			})

			fileStore := storage.NewFileStore(storage.NewNetStore(localStore, nil), storage.NewFileStoreParams())
			bucketKeyFileStore = simulation.BucketKey("filestore")
			bucket.Store(bucketKeyFileStore, fileStore)

			return r, cleanup, nil

		},
	})
	defer sim.Close()

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

	ctx := context.Background()
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
		var randomFiles []string
//
		node := sim.RandomUpNode()
		item, ok := sim.NodeItem(node.ID, bucketKeyStore)
		if !ok {
			return fmt.Errorf("No localstore")
		}
		lstore := item.(*storage.LocalStore)
		conf.hashes, err = uploadFileToSingleNodeStore(node.ID, chunkCount, lstore)
		if err != nil {
			return err
		}
		if _, err := sim.WaitTillHealthy(ctx, 2); err != nil {
			return err
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
//
					item, ok := sim.NodeItem(id, bucketKeyFileStore)
					if !ok {
						return fmt.Errorf("No registry")
					}
					fileStore := item.(*storage.FileStore)
//
					for i, hash := range conf.hashes {
						reader, _ := fileStore.Retrieve(context.TODO(), hash)
//
						if s, err := reader.Size(ctx, nil); err != nil || s != int64(len(randomFiles[i])) {
							allSuccess = false
							log.Warn("Retrieve error", "err", err, "hash", hash, "nodeId", id)
						} else {
							log.Debug(fmt.Sprintf("File with root hash %x successfully retrieved", hash))
						}
					}
					if err != nil {
						log.Warn(fmt.Sprintf("Chunk %s NOT found for id %s", chunk, id))
						localSuccess = false
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

	return nil
}
