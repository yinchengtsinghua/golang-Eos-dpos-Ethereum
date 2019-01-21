
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

package simulations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
)

//TestNetworkSimulation使用每个节点创建多节点仿真网络
//在环形拓扑中连接，检查所有节点是否成功握手
//彼此之间，快照完全代表所需的拓扑
func TestNetworkSimulation(t *testing.T) {
//使用20个testservice节点创建模拟网络
	adapter := adapters.NewSimAdapter(adapters.Services{
		"test": newTestService,
	})
	network := NewNetwork(adapter, &NetworkConfig{
		DefaultService: "test",
	})
	defer network.Shutdown()
	nodeCount := 20
	ids := make([]discover.NodeID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		conf := adapters.RandomNodeConfig()
		node, err := network.NewNodeWithConfig(conf)
		if err != nil {
			t.Fatalf("error creating node: %s", err)
		}
		if err := network.Start(node.ID()); err != nil {
			t.Fatalf("error starting node: %s", err)
		}
		ids[i] = node.ID()
	}

//执行连接环中节点的检查（因此每个节点
//然后检查所有节点
//通过检查他们的对等计数进行了两次握手
	action := func(_ context.Context) error {
		for i, id := range ids {
			peerID := ids[(i+1)%len(ids)]
			if err := network.Connect(id, peerID); err != nil {
				return err
			}
		}
		return nil
	}
	check := func(ctx context.Context, id discover.NodeID) (bool, error) {
//检查一下我们的时间没有用完
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

//获取节点
		node := network.GetNode(id)
		if node == nil {
			return false, fmt.Errorf("unknown node: %s", id)
		}

//检查它是否有两个同龄人
		client, err := node.Client()
		if err != nil {
			return false, err
		}
		var peerCount int64
		if err := client.CallContext(ctx, &peerCount, "test_peerCount"); err != nil {
			return false, err
		}
		switch {
		case peerCount < 2:
			return false, nil
		case peerCount == 2:
			return true, nil
		default:
			return false, fmt.Errorf("unexpected peerCount: %d", peerCount)
		}
	}

	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

//每100毫秒触发一次检查
	trigger := make(chan discover.NodeID)
	go triggerChecks(ctx, ids, trigger, 100*time.Millisecond)

	result := NewSimulation(network).Run(ctx, &Step{
		Action:  action,
		Trigger: trigger,
		Expect: &Expectation{
			Nodes: ids,
			Check: check,
		},
	})
	if result.Error != nil {
		t.Fatalf("simulation failed: %s", result.Error)
	}

//获取网络快照并检查它是否包含正确的拓扑
	snap, err := network.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Nodes) != nodeCount {
		t.Fatalf("expected snapshot to contain %d nodes, got %d", nodeCount, len(snap.Nodes))
	}
	if len(snap.Conns) != nodeCount {
		t.Fatalf("expected snapshot to contain %d connections, got %d", nodeCount, len(snap.Conns))
	}
	for i, id := range ids {
		conn := snap.Conns[i]
		if conn.One != id {
			t.Fatalf("expected conn[%d].One to be %s, got %s", i, id, conn.One)
		}
		peerID := ids[(i+1)%len(ids)]
		if conn.Other != peerID {
			t.Fatalf("expected conn[%d].Other to be %s, got %s", i, peerID, conn.Other)
		}
	}
}

func triggerChecks(ctx context.Context, ids []discover.NodeID, trigger chan discover.NodeID, interval time.Duration) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			for _, id := range ids {
				select {
				case trigger <- id:
				case <-ctx.Done():
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
