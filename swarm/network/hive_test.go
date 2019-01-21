
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

package network

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	p2ptest "github.com/ethereum/go-ethereum/p2p/testing"
	"github.com/ethereum/go-ethereum/swarm/state"
)

func newHiveTester(t *testing.T, params *HiveParams, n int, store state.Store) (*bzzTester, *Hive) {
//设置
addr := RandomAddr() //测试的对等地址
	to := NewKademlia(addr.OAddr, NewKadParams())
pp := NewHive(params, to, store) //蜂箱

	return newBzzBaseTester(t, n, addr, DiscoverySpec, pp.Run), pp
}

func TestRegisterAndConnect(t *testing.T) {
	params := NewHiveParams()
	s, pp := newHiveTester(t, params, 1, nil)

	id := s.IDs[0]
	raddr := NewAddrFromNodeID(id)
	pp.Register([]OverlayAddr{OverlayAddr(raddr)})

//启动配置单元并等待连接
	err := pp.Start(s.Server)
	if err != nil {
		t.Fatal(err)
	}
	defer pp.Stop()
//检索和广播
	err = s.TestDisconnected(&p2ptest.Disconnect{
		Peer:  s.IDs[0],
		Error: nil,
	})

	if err == nil || err.Error() != "timed out waiting for peers to disconnect" {
		t.Fatalf("expected peer to connect")
	}
}

func TestHiveStatePersistance(t *testing.T) {
	log.SetOutput(os.Stdout)

	dir, err := ioutil.TempDir("", "hive_test_store")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

store, err := state.NewDBStore(dir) //用空的dbstore启动配置单元

	params := NewHiveParams()
	s, pp := newHiveTester(t, params, 5, store)

	peers := make(map[string]bool)
	for _, id := range s.IDs {
		raddr := NewAddrFromNodeID(id)
		pp.Register([]OverlayAddr{OverlayAddr(raddr)})
		peers[raddr.String()] = true
	}

//启动配置单元并等待连接
	err = pp.Start(s.Server)
	if err != nil {
		t.Fatal(err)
	}
	pp.Stop()
	store.Close()

persistedStore, err := state.NewDBStore(dir) //用空的dbstore启动配置单元

	s1, pp := newHiveTester(t, params, 1, persistedStore)

//启动配置单元并等待连接

	pp.Start(s1.Server)
	i := 0
	pp.Overlay.EachAddr(nil, 256, func(addr OverlayAddr, po int, nn bool) bool {
		delete(peers, addr.(*BzzAddr).String())
		i++
		return true
	})
	if len(peers) != 0 || i != 5 {
		t.Fatalf("invalid peers loaded")
	}
}
