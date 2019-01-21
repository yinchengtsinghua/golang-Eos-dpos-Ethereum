
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
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	p2ptest "github.com/ethereum/go-ethereum/p2p/testing"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
	"github.com/ethereum/go-ethereum/swarm/pot"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
	mockdb "github.com/ethereum/go-ethereum/swarm/storage/mock/db"
	colorable "github.com/mattn/go-colorable"
)

var (
	loglevel     = flag.Int("loglevel", 2, "verbosity of logs")
	nodes        = flag.Int("nodes", 0, "number of nodes")
	chunks       = flag.Int("chunks", 0, "number of chunks")
	useMockStore = flag.Bool("mockstore", false, "disabled mock store (default: enabled)")
	longrunning  = flag.Bool("longrunning", false, "do run long-running tests")

	bucketKeyDB        = simulation.BucketKey("db")
	bucketKeyStore     = simulation.BucketKey("store")
	bucketKeyFileStore = simulation.BucketKey("filestore")
	bucketKeyNetStore  = simulation.BucketKey("netstore")
	bucketKeyDelivery  = simulation.BucketKey("delivery")
	bucketKeyRegistry  = simulation.BucketKey("registry")

	chunkSize = 4096
	pof       = pot.DefaultPof(256)
)

func init() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	log.PrintOrigins(true)
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(true))))
}

func createGlobalStore() (string, *mockdb.GlobalStore, error) {
	var globalStore *mockdb.GlobalStore
	globalStoreDir, err := ioutil.TempDir("", "global.store")
	if err != nil {
		log.Error("Error initiating global store temp directory!", "err", err)
		return "", nil, err
	}
	globalStore, err = mockdb.NewGlobalStore(globalStoreDir)
	if err != nil {
		log.Error("Error initiating global store!", "err", err)
		return "", nil, err
	}
	return globalStoreDir, globalStore, nil
}

func newStreamerTester(t *testing.T) (*p2ptest.ProtocolTester, *Registry, *storage.LocalStore, func(), error) {
//
addr := network.RandomAddr() //
	to := network.NewKademlia(addr.OAddr, network.NewKadParams())

//
	datadir, err := ioutil.TempDir("", "streamer")
	if err != nil {
		return nil, nil, nil, func() {}, err
	}
	removeDataDir := func() {
		os.RemoveAll(datadir)
	}

	params := storage.NewDefaultLocalStoreParams()
	params.Init(datadir)
	params.BaseKey = addr.Over()

	localStore, err := storage.NewTestLocalStoreForAddr(params)
	if err != nil {
		return nil, nil, nil, removeDataDir, err
	}

	db := storage.NewDBAPI(localStore)
	delivery := NewDelivery(to, db)
	streamer := NewRegistry(addr, delivery, db, state.NewInmemoryStore(), nil)
	teardown := func() {
		streamer.Close()
		removeDataDir()
	}
	protocolTester := p2ptest.NewProtocolTester(t, network.NewNodeIDFromAddr(addr), 1, streamer.runProtocol)

	err = waitForPeers(streamer, 1*time.Second, 1)
	if err != nil {
		return nil, nil, nil, nil, errors.New("timeout: peer is not created")
	}

	return protocolTester, streamer, localStore, teardown, nil
}

func waitForPeers(streamer *Registry, timeout time.Duration, expectedPeers int) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	timeoutTimer := time.NewTimer(timeout)
	for {
		select {
		case <-ticker.C:
			if streamer.peersCount() >= expectedPeers {
				return nil
			}
		case <-timeoutTimer.C:
			return errors.New("timeout")
		}
	}
}

type roundRobinStore struct {
	index  uint32
	stores []storage.ChunkStore
}

func newRoundRobinStore(stores ...storage.ChunkStore) *roundRobinStore {
	return &roundRobinStore{
		stores: stores,
	}
}

func (rrs *roundRobinStore) Get(ctx context.Context, addr storage.Address) (*storage.Chunk, error) {
	return nil, errors.New("get not well defined on round robin store")
}

func (rrs *roundRobinStore) Put(ctx context.Context, chunk *storage.Chunk) {
	i := atomic.AddUint32(&rrs.index, 1)
	idx := int(i) % len(rrs.stores)
	rrs.stores[idx].Put(ctx, chunk)
}

func (rrs *roundRobinStore) Close() {
	for _, store := range rrs.stores {
		store.Close()
	}
}

func readAll(fileStore *storage.FileStore, hash []byte) (int64, error) {
	r, _ := fileStore.Retrieve(context.TODO(), hash)
	buf := make([]byte, 1024)
	var n int
	var total int64
	var err error
	for (total == 0 || n > 0) && err == nil {
		n, err = r.ReadAt(buf, total)
		total += int64(n)
	}
	if err != nil && err != io.EOF {
		return total, err
	}
	return total, nil
}

func uploadFilesToNodes(sim *simulation.Simulation) ([]storage.Address, []string, error) {
	nodes := sim.UpNodeIDs()
	nodeCnt := len(nodes)
	log.Debug(fmt.Sprintf("Uploading %d files to nodes", nodeCnt))
//
	rfiles := make([]string, nodeCnt)
//
	rootAddrs := make([]storage.Address, nodeCnt)

	var err error
//
	for i, id := range nodes {
		item, ok := sim.NodeItem(id, bucketKeyFileStore)
		if !ok {
			return nil, nil, fmt.Errorf("Error accessing localstore")
		}
		fileStore := item.(*storage.FileStore)
//
		rfiles[i], err = generateRandomFile()
		if err != nil {
			return nil, nil, err
		}
//
		ctx := context.TODO()
		rk, wait, err := fileStore.Store(ctx, strings.NewReader(rfiles[i]), int64(len(rfiles[i])), false)
		log.Debug("Uploaded random string file to node")
		if err != nil {
			return nil, nil, err
		}
		err = wait(ctx)
		if err != nil {
			return nil, nil, err
		}
		rootAddrs[i] = rk
	}
	return rootAddrs, rfiles, nil
}

//
func generateRandomFile() (string, error) {
//
	fileSize := rand.Intn(maxFileSize-minFileSize) + minFileSize
	log.Debug(fmt.Sprintf("Generated file with filesize %d kB", fileSize))
	b := make([]byte, fileSize*1024)
	_, err := crand.Read(b)
	if err != nil {
		log.Error("Error generating random file.", "err", err)
		return "", err
	}
	return string(b), nil
}

//
func createTestLocalStorageForID(id discover.NodeID, addr *network.BzzAddr) (storage.ChunkStore, string, error) {
	var datadir string
	var err error
	datadir, err = ioutil.TempDir("", fmt.Sprintf("syncer-test-%s", id.TerminalString()))
	if err != nil {
		return nil, "", err
	}
	var store storage.ChunkStore
	params := storage.NewDefaultLocalStoreParams()
	params.ChunkDbPath = datadir
	params.BaseKey = addr.Over()
	store, err = storage.NewTestLocalStoreForAddr(params)
	if err != nil {
		os.RemoveAll(datadir)
		return nil, "", err
	}
	return store, datadir, nil
}
