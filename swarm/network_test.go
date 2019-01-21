
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

package swarm

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/api"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
	"github.com/ethereum/go-ethereum/swarm/storage"
	colorable "github.com/mattn/go-colorable"
)

var (
	loglevel     = flag.Int("loglevel", 2, "verbosity of logs")
	longrunning  = flag.Bool("longrunning", false, "do run long-running tests")
	waitKademlia = flag.Bool("waitkademlia", false, "wait for healthy kademlia before checking files availability")
)

func init() {
	rand.Seed(time.Now().UnixNano())

	flag.Parse()

	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(true))))
}

//
//
//
func TestSwarmNetwork(t *testing.T) {
	for _, tc := range []struct {
		name     string
		steps    []testSwarmNetworkStep
		options  *testSwarmNetworkOptions
		disabled bool
	}{
		{
			name: "10_nodes",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 10,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout: 45 * time.Second,
			},
		},
		{
			name: "10_nodes_skip_check",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 10,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout:   45 * time.Second,
				SkipCheck: true,
			},
		},
		{
			name: "100_nodes",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 100,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout: 3 * time.Minute,
			},
			disabled: !*longrunning,
		},
		{
			name: "100_nodes_skip_check",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 100,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout:   3 * time.Minute,
				SkipCheck: true,
			},
			disabled: !*longrunning,
		},
		{
			name: "inc_node_count",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 2,
				},
				{
					nodeCount: 5,
				},
				{
					nodeCount: 10,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout: 90 * time.Second,
			},
			disabled: !*longrunning,
		},
		{
			name: "dec_node_count",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 10,
				},
				{
					nodeCount: 6,
				},
				{
					nodeCount: 3,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout: 90 * time.Second,
			},
			disabled: !*longrunning,
		},
		{
			name: "dec_inc_node_count",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 5,
				},
				{
					nodeCount: 3,
				},
				{
					nodeCount: 10,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout: 90 * time.Second,
			},
		},
		{
			name: "inc_dec_node_count",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 3,
				},
				{
					nodeCount: 5,
				},
				{
					nodeCount: 25,
				},
				{
					nodeCount: 10,
				},
				{
					nodeCount: 4,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout: 5 * time.Minute,
			},
			disabled: !*longrunning,
		},
		{
			name: "inc_dec_node_count_skip_check",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 3,
				},
				{
					nodeCount: 5,
				},
				{
					nodeCount: 25,
				},
				{
					nodeCount: 10,
				},
				{
					nodeCount: 4,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout:   5 * time.Minute,
				SkipCheck: true,
			},
			disabled: !*longrunning,
		},
	} {
		if tc.disabled {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			testSwarmNetwork(t, tc.options, tc.steps...)
		})
	}
}

//
//
type testSwarmNetworkStep struct {
//
	nodeCount int
}

//
type file struct {
	addr   storage.Address
	data   string
	nodeID discover.NodeID
}

//
//
type check struct {
	key    string
	nodeID discover.NodeID
}

//
//
type testSwarmNetworkOptions struct {
	Timeout   time.Duration
	SkipCheck bool
}

//
//
//
//
//
//
//
func testSwarmNetwork(t *testing.T, o *testSwarmNetworkOptions, steps ...testSwarmNetworkStep) {
	if o == nil {
		o = new(testSwarmNetworkOptions)
	}

	sim := simulation.New(map[string]simulation.ServiceFunc{
		"swarm": func(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error) {
			config := api.NewConfig()

			dir, err := ioutil.TempDir("", "swarm-network-test-node")
			if err != nil {
				return nil, nil, err
			}
			cleanup = func() {
				err := os.RemoveAll(dir)
				if err != nil {
					log.Error("cleaning up swarm temp dir", "err", err)
				}
			}

			config.Path = dir

			privkey, err := crypto.GenerateKey()
			if err != nil {
				return nil, cleanup, err
			}

			config.Init(privkey)
			config.DeliverySkipCheck = o.SkipCheck

			swarm, err := NewSwarm(config, nil)
			if err != nil {
				return nil, cleanup, err
			}
			bucket.Store(simulation.BucketKeyKademlia, swarm.bzz.Hive.Overlay.(*network.Kademlia))
			log.Info("new swarm", "bzzKey", config.BzzKey, "baseAddr", fmt.Sprintf("%x", swarm.bzz.BaseAddr()))
			return swarm, cleanup, nil
		},
	})
	defer sim.Close()

	ctx := context.Background()
	if o.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer cancel()
	}

	files := make([]file, 0)

	for i, step := range steps {
		log.Debug("test sync step", "n", i+1, "nodes", step.nodeCount)

		change := step.nodeCount - len(sim.UpNodeIDs())

		if change > 0 {
			_, err := sim.AddNodesAndConnectChain(change)
			if err != nil {
				t.Fatal(err)
			}
		} else if change < 0 {
			_, err := sim.StopRandomNodes(-change)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			t.Logf("step %v: no change in nodes", i)
			continue
		}

		var checkStatusM sync.Map
		var nodeStatusM sync.Map
		var totalFoundCount uint64

		result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) error {
			nodeIDs := sim.UpNodeIDs()
			shuffle(len(nodeIDs), func(i, j int) {
				nodeIDs[i], nodeIDs[j] = nodeIDs[j], nodeIDs[i]
			})
			for _, id := range nodeIDs {
				key, data, err := uploadFile(sim.Service("swarm", id).(*Swarm))
				if err != nil {
					return err
				}
				log.Trace("file uploaded", "node", id, "key", key.String())
				files = append(files, file{
					addr:   key,
					data:   data,
					nodeID: id,
				})
			}

			if *waitKademlia {
				if _, err := sim.WaitTillHealthy(ctx, 2); err != nil {
					return err
				}
			}

//
//
			for {
				if retrieve(sim, files, &checkStatusM, &nodeStatusM, &totalFoundCount) == 0 {
					return nil
				}
			}
		})

		if result.Error != nil {
			t.Fatal(result.Error)
		}
		log.Debug("done: test sync step", "n", i+1, "nodes", step.nodeCount)
	}
}

//
//
func uploadFile(swarm *Swarm) (storage.Address, string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return nil, "", err
	}
//
//
	data := fmt.Sprintf("test content %s %x", time.Now().Round(0), b)
	ctx := context.TODO()
	k, wait, err := swarm.api.Put(ctx, data, "text/plain", false)
	if err != nil {
		return nil, "", err
	}
	if wait != nil {
		err = wait(ctx)
	}
	return k, data, err
}

//
//
func retrieve(
	sim *simulation.Simulation,
	files []file,
	checkStatusM *sync.Map,
	nodeStatusM *sync.Map,
	totalFoundCount *uint64,
) (missing uint64) {
	shuffle(len(files), func(i, j int) {
		files[i], files[j] = files[j], files[i]
	})

	var totalWg sync.WaitGroup
	errc := make(chan error)

	nodeIDs := sim.UpNodeIDs()

	totalCheckCount := len(nodeIDs) * len(files)

	for _, id := range nodeIDs {
		if _, ok := nodeStatusM.Load(id); ok {
			continue
		}
		start := time.Now()
		var checkCount uint64
		var foundCount uint64

		totalWg.Add(1)

		var wg sync.WaitGroup

		swarm := sim.Service("swarm", id).(*Swarm)
		for _, f := range files {

			checkKey := check{
				key:    f.addr.String(),
				nodeID: id,
			}
			if n, ok := checkStatusM.Load(checkKey); ok && n.(int) == 0 {
				continue
			}

			checkCount++
			wg.Add(1)
			go func(f file, id discover.NodeID) {
				defer wg.Done()

				log.Debug("api get: check file", "node", id.String(), "key", f.addr.String(), "total files found", atomic.LoadUint64(totalFoundCount))

				r, _, _, _, err := swarm.api.Get(context.TODO(), api.NOOPDecrypt, f.addr, "/")
				if err != nil {
					errc <- fmt.Errorf("api get: node %s, key %s, kademlia %s: %v", id, f.addr, swarm.bzz.Hive, err)
					return
				}
				d, err := ioutil.ReadAll(r)
				if err != nil {
					errc <- fmt.Errorf("api get: read response: node %s, key %s: kademlia %s: %v", id, f.addr, swarm.bzz.Hive, err)
					return
				}
				data := string(d)
				if data != f.data {
					errc <- fmt.Errorf("file contend missmatch: node %s, key %s, expected %q, got %q", id, f.addr, f.data, data)
					return
				}
				checkStatusM.Store(checkKey, 0)
				atomic.AddUint64(&foundCount, 1)
				log.Info("api get: file found", "node", id.String(), "key", f.addr.String(), "content", data, "files found", atomic.LoadUint64(&foundCount))
			}(f, id)
		}

		go func(id discover.NodeID) {
			defer totalWg.Done()
			wg.Wait()

			atomic.AddUint64(totalFoundCount, foundCount)

			if foundCount == checkCount {
				log.Info("all files are found for node", "id", id.String(), "duration", time.Since(start))
				nodeStatusM.Store(id, 0)
				return
			}
			log.Debug("files missing for node", "id", id.String(), "check", checkCount, "found", foundCount)
		}(id)

	}

	go func() {
		totalWg.Wait()
		close(errc)
	}()

	var errCount int
	for err := range errc {
		if err != nil {
			errCount++
		}
		log.Warn(err.Error())
	}

	log.Info("check stats", "total check count", totalCheckCount, "total files found", atomic.LoadUint64(totalFoundCount), "total errors", errCount)

	return uint64(totalCheckCount) - atomic.LoadUint64(totalFoundCount)
}

//
//
//
//
//
//
//
func shuffle(n int, swap func(i, j int)) {
	if n < 0 {
		panic("invalid argument to Shuffle")
	}

//
//
//
//
//
//
	i := n - 1
	for ; i > 1<<31-1-1; i-- {
		j := int(rand.Int63n(int64(i + 1)))
		swap(i, j)
	}
	for ; i > 0; i-- {
		j := int(rand.Int31n(int32(i + 1)))
		swap(i, j)
	}
}
