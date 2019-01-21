
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

package simulation

import (
	"testing"

	"github.com/ethereum/go-ethereum/p2p/discover"
)

func TestConnectToPivotNode(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	pid, err := sim.AddNode()
	if err != nil {
		t.Fatal(err)
	}

	sim.SetPivotNode(pid)

	id, err := sim.AddNode()
	if err != nil {
		t.Fatal(err)
	}

	if len(sim.Net.Conns) > 0 {
		t.Fatal("no connections should exist after just adding nodes")
	}

	err = sim.ConnectToPivotNode(id)
	if err != nil {
		t.Fatal(err)
	}

	if sim.Net.GetConn(id, pid) == nil {
		t.Error("node did not connect to pivot node")
	}
}

func TestConnectToLastNode(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	n := 10

	ids, err := sim.AddNodes(n)
	if err != nil {
		t.Fatal(err)
	}

	id, err := sim.AddNode()
	if err != nil {
		t.Fatal(err)
	}

	if len(sim.Net.Conns) > 0 {
		t.Fatal("no connections should exist after just adding nodes")
	}

	err = sim.ConnectToLastNode(id)
	if err != nil {
		t.Fatal(err)
	}

	for _, i := range ids[:n-2] {
		if sim.Net.GetConn(id, i) != nil {
			t.Error("node connected to the node that is not the last")
		}
	}

	if sim.Net.GetConn(id, ids[n-1]) == nil {
		t.Error("node did not connect to the last node")
	}
}

func TestConnectToRandomNode(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	n := 10

	ids, err := sim.AddNodes(n)
	if err != nil {
		t.Fatal(err)
	}

	if len(sim.Net.Conns) > 0 {
		t.Fatal("no connections should exist after just adding nodes")
	}

	err = sim.ConnectToRandomNode(ids[0])
	if err != nil {
		t.Fatal(err)
	}

	var cc int
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if sim.Net.GetConn(ids[i], ids[j]) != nil {
				cc++
			}
		}
	}

	if cc != 1 {
		t.Errorf("expected one connection, got %v", cc)
	}
}

func TestConnectNodesFull(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	ids, err := sim.AddNodes(12)
	if err != nil {
		t.Fatal(err)
	}

	if len(sim.Net.Conns) > 0 {
		t.Fatal("no connections should exist after just adding nodes")
	}

	err = sim.ConnectNodesFull(ids)
	if err != nil {
		t.Fatal(err)
	}

	testFull(t, sim, ids)
}

func testFull(t *testing.T, sim *Simulation, ids []discover.NodeID) {
	n := len(ids)
	var cc int
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if sim.Net.GetConn(ids[i], ids[j]) != nil {
				cc++
			}
		}
	}

	want := n * (n - 1) / 2

	if cc != want {
		t.Errorf("expected %v connection, got %v", want, cc)
	}
}

func TestConnectNodesChain(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	ids, err := sim.AddNodes(10)
	if err != nil {
		t.Fatal(err)
	}

	if len(sim.Net.Conns) > 0 {
		t.Fatal("no connections should exist after just adding nodes")
	}

	err = sim.ConnectNodesChain(ids)
	if err != nil {
		t.Fatal(err)
	}

	testChain(t, sim, ids)
}

func testChain(t *testing.T, sim *Simulation, ids []discover.NodeID) {
	n := len(ids)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			c := sim.Net.GetConn(ids[i], ids[j])
			if i == j-1 {
				if c == nil {
					t.Errorf("nodes %v and %v are not connected, but they should be", i, j)
				}
			} else {
				if c != nil {
					t.Errorf("nodes %v and %v are connected, but they should not be", i, j)
				}
			}
		}
	}
}

func TestConnectNodesRing(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	ids, err := sim.AddNodes(10)
	if err != nil {
		t.Fatal(err)
	}

	if len(sim.Net.Conns) > 0 {
		t.Fatal("no connections should exist after just adding nodes")
	}

	err = sim.ConnectNodesRing(ids)
	if err != nil {
		t.Fatal(err)
	}

	testRing(t, sim, ids)
}

func testRing(t *testing.T, sim *Simulation, ids []discover.NodeID) {
	n := len(ids)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			c := sim.Net.GetConn(ids[i], ids[j])
			if i == j-1 || (i == 0 && j == n-1) {
				if c == nil {
					t.Errorf("nodes %v and %v are not connected, but they should be", i, j)
				}
			} else {
				if c != nil {
					t.Errorf("nodes %v and %v are connected, but they should not be", i, j)
				}
			}
		}
	}
}

func TestConnectToNodesStar(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	ids, err := sim.AddNodes(10)
	if err != nil {
		t.Fatal(err)
	}

	if len(sim.Net.Conns) > 0 {
		t.Fatal("no connections should exist after just adding nodes")
	}

	centerIndex := 2

	err = sim.ConnectNodesStar(ids[centerIndex], ids)
	if err != nil {
		t.Fatal(err)
	}

	testStar(t, sim, ids, centerIndex)
}

func testStar(t *testing.T, sim *Simulation, ids []discover.NodeID, centerIndex int) {
	n := len(ids)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			c := sim.Net.GetConn(ids[i], ids[j])
			if i == centerIndex || j == centerIndex {
				if c == nil {
					t.Errorf("nodes %v and %v are not connected, but they should be", i, j)
				}
			} else {
				if c != nil {
					t.Errorf("nodes %v and %v are connected, but they should not be", i, j)
				}
			}
		}
	}
}

func TestConnectToNodesStarPivot(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	ids, err := sim.AddNodes(10)
	if err != nil {
		t.Fatal(err)
	}

	if len(sim.Net.Conns) > 0 {
		t.Fatal("no connections should exist after just adding nodes")
	}

	pivotIndex := 4

	sim.SetPivotNode(ids[pivotIndex])

	err = sim.ConnectNodesStarPivot(ids)
	if err != nil {
		t.Fatal(err)
	}

	testStar(t, sim, ids, pivotIndex)
}
