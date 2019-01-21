
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

package simulation

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/network"
)

func TestUpDownNodeIDs(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	ids, err := sim.AddNodes(10)
	if err != nil {
		t.Fatal(err)
	}

	gotIDs := sim.NodeIDs()

	if !equalNodeIDs(ids, gotIDs) {
		t.Error("returned nodes are not equal to added ones")
	}

	stoppedIDs, err := sim.StopRandomNodes(3)
	if err != nil {
		t.Fatal(err)
	}

	gotIDs = sim.UpNodeIDs()

	for _, id := range gotIDs {
		if !sim.Net.GetNode(id).Up {
			t.Errorf("node %s should not be down", id)
		}
	}

	if !equalNodeIDs(ids, append(gotIDs, stoppedIDs...)) {
		t.Error("returned nodes are not equal to added ones")
	}

	gotIDs = sim.DownNodeIDs()

	for _, id := range gotIDs {
		if sim.Net.GetNode(id).Up {
			t.Errorf("node %s should not be up", id)
		}
	}

	if !equalNodeIDs(stoppedIDs, gotIDs) {
		t.Error("returned nodes are not equal to the stopped ones")
	}
}

func equalNodeIDs(one, other []discover.NodeID) bool {
	if len(one) != len(other) {
		return false
	}
	var count int
	for _, a := range one {
		var found bool
		for _, b := range other {
			if a == b {
				found = true
				break
			}
		}
		if found {
			count++
		} else {
			return false
		}
	}
	return count == len(one)
}

func TestAddNode(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	id, err := sim.AddNode()
	if err != nil {
		t.Fatal(err)
	}

	n := sim.Net.GetNode(id)
	if n == nil {
		t.Fatal("node not found")
	}

	if !n.Up {
		t.Error("node not started")
	}
}

func TestAddNodeWithMsgEvents(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	id, err := sim.AddNode(AddNodeWithMsgEvents(true))
	if err != nil {
		t.Fatal(err)
	}

	if !sim.Net.GetNode(id).Config.EnableMsgEvents {
		t.Error("EnableMsgEvents is false")
	}

	id, err = sim.AddNode(AddNodeWithMsgEvents(false))
	if err != nil {
		t.Fatal(err)
	}

	if sim.Net.GetNode(id).Config.EnableMsgEvents {
		t.Error("EnableMsgEvents is true")
	}
}

func TestAddNodeWithService(t *testing.T) {
	sim := New(map[string]ServiceFunc{
		"noop1": noopServiceFunc,
		"noop2": noopServiceFunc,
	})
	defer sim.Close()

	id, err := sim.AddNode(AddNodeWithService("noop1"))
	if err != nil {
		t.Fatal(err)
	}

	n := sim.Net.GetNode(id).Node.(*adapters.SimNode)
	if n.Service("noop1") == nil {
		t.Error("service noop1 not found on node")
	}
	if n.Service("noop2") != nil {
		t.Error("service noop2 should not be found on node")
	}
}

func TestAddNodes(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	nodesCount := 12

	ids, err := sim.AddNodes(nodesCount)
	if err != nil {
		t.Fatal(err)
	}

	count := len(ids)
	if count != nodesCount {
		t.Errorf("expected %v nodes, got %v", nodesCount, count)
	}

	count = len(sim.Net.GetNodes())
	if count != nodesCount {
		t.Errorf("expected %v nodes, got %v", nodesCount, count)
	}
}

func TestAddNodesAndConnectFull(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	n := 12

	ids, err := sim.AddNodesAndConnectFull(n)
	if err != nil {
		t.Fatal(err)
	}

	testFull(t, sim, ids)
}

func TestAddNodesAndConnectChain(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	_, err := sim.AddNodesAndConnectChain(12)
	if err != nil {
		t.Fatal(err)
	}

//
//
	_, err = sim.AddNodesAndConnectChain(7)
	if err != nil {
		t.Fatal(err)
	}

	testChain(t, sim, sim.UpNodeIDs())
}

func TestAddNodesAndConnectRing(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	ids, err := sim.AddNodesAndConnectRing(12)
	if err != nil {
		t.Fatal(err)
	}

	testRing(t, sim, ids)
}

func TestAddNodesAndConnectStar(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	ids, err := sim.AddNodesAndConnectStar(12)
	if err != nil {
		t.Fatal(err)
	}

	testStar(t, sim, ids, 0)
}

//
func TestUploadSnapshot(t *testing.T) {
	log.Debug("Creating simulation")
	s := New(map[string]ServiceFunc{
		"bzz": func(ctx *adapters.ServiceContext, b *sync.Map) (node.Service, func(), error) {
			addr := network.NewAddrFromNodeID(ctx.Config.ID)
			hp := network.NewHiveParams()
			hp.Discovery = false
			config := &network.BzzConfig{
				OverlayAddr:  addr.Over(),
				UnderlayAddr: addr.Under(),
				HiveParams:   hp,
			}
			kad := network.NewKademlia(addr.Over(), network.NewKadParams())
			return network.NewBzz(config, kad, nil, nil, nil), nil, nil
		},
	})
	defer s.Close()

	nodeCount := 16
	log.Debug("Uploading snapshot")
	err := s.UploadSnapshot(fmt.Sprintf("../stream/testing/snapshot_%d.json", nodeCount))
	if err != nil {
		t.Fatalf("Error uploading snapshot to simulation network: %v", err)
	}

	ctx := context.Background()
	log.Debug("Starting simulation...")
	s.Run(ctx, func(ctx context.Context, sim *Simulation) error {
		log.Debug("Checking")
		nodes := sim.UpNodeIDs()
		if len(nodes) != nodeCount {
			t.Fatal("Simulation network node number doesn't match snapshot node number")
		}
		return nil
	})
	log.Debug("Done.")
}

func TestPivotNode(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	id, err := sim.AddNode()
	if err != nil {
		t.Fatal(err)
	}

	id2, err := sim.AddNode()
	if err != nil {
		t.Fatal(err)
	}

	if sim.PivotNodeID() != nil {
		t.Error("expected no pivot node")
	}

	sim.SetPivotNode(id)

	pid := sim.PivotNodeID()

	if pid == nil {
		t.Error("pivot node not set")
	} else if *pid != id {
		t.Errorf("expected pivot node %s, got %s", id, *pid)
	}

	sim.SetPivotNode(id2)

	pid = sim.PivotNodeID()

	if pid == nil {
		t.Error("pivot node not set")
	} else if *pid != id2 {
		t.Errorf("expected pivot node %s, got %s", id2, *pid)
	}
}

func TestStartStopNode(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	id, err := sim.AddNode()
	if err != nil {
		t.Fatal(err)
	}

	n := sim.Net.GetNode(id)
	if n == nil {
		t.Fatal("node not found")
	}
	if !n.Up {
		t.Error("node not started")
	}

	err = sim.StopNode(id)
	if err != nil {
		t.Fatal(err)
	}
	if n.Up {
		t.Error("node not stopped")
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
	time.Sleep(time.Second)

	err = sim.StartNode(id)
	if err != nil {
		t.Fatal(err)
	}
	if !n.Up {
		t.Error("node not started")
	}
}

func TestStartStopRandomNode(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	_, err := sim.AddNodes(3)
	if err != nil {
		t.Fatal(err)
	}

	id, err := sim.StopRandomNode()
	if err != nil {
		t.Fatal(err)
	}

	n := sim.Net.GetNode(id)
	if n == nil {
		t.Fatal("node not found")
	}
	if n.Up {
		t.Error("node not stopped")
	}

	id2, err := sim.StopRandomNode()
	if err != nil {
		t.Fatal(err)
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
	time.Sleep(time.Second)

	idStarted, err := sim.StartRandomNode()
	if err != nil {
		t.Fatal(err)
	}

	if idStarted != id && idStarted != id2 {
		t.Error("unexpected started node ID")
	}
}

func TestStartStopRandomNodes(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	_, err := sim.AddNodes(10)
	if err != nil {
		t.Fatal(err)
	}

	ids, err := sim.StopRandomNodes(3)
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range ids {
		n := sim.Net.GetNode(id)
		if n == nil {
			t.Fatal("node not found")
		}
		if n.Up {
			t.Error("node not stopped")
		}
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
	time.Sleep(time.Second)

	ids, err = sim.StartRandomNodes(2)
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range ids {
		n := sim.Net.GetNode(id)
		if n == nil {
			t.Fatal("node not found")
		}
		if !n.Up {
			t.Error("node not started")
		}
	}
}
