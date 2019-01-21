
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

package network

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	p2ptest "github.com/ethereum/go-ethereum/p2p/testing"
)

const (
	TestProtocolVersion   = 6
	TestProtocolNetworkID = 3
)

var (
	loglevel = flag.Int("loglevel", 2, "verbosity of logs")
)

func init() {
	flag.Parse()
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(os.Stderr, log.TerminalFormat(true))))
}

type testStore struct {
	sync.Mutex

	values map[string][]byte
}

func newTestStore() *testStore {
	return &testStore{values: make(map[string][]byte)}
}

func (t *testStore) Load(key string) ([]byte, error) {
	t.Lock()
	defer t.Unlock()
	v, ok := t.values[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return v, nil
}

func (t *testStore) Save(key string, v []byte) error {
	t.Lock()
	defer t.Unlock()
	t.values[key] = v
	return nil
}

func HandshakeMsgExchange(lhs, rhs *HandshakeMsg, id discover.NodeID) []p2ptest.Exchange {

	return []p2ptest.Exchange{
		{
			Expects: []p2ptest.Expect{
				{
					Code: 0,
					Msg:  lhs,
					Peer: id,
				},
			},
		},
		{
			Triggers: []p2ptest.Trigger{
				{
					Code: 0,
					Msg:  rhs,
					Peer: id,
				},
			},
		},
	}
}

func newBzzBaseTester(t *testing.T, n int, addr *BzzAddr, spec *protocols.Spec, run func(*BzzPeer) error) *bzzTester {
	cs := make(map[string]chan bool)

	srv := func(p *BzzPeer) error {
		defer func() {
			if cs[p.ID().String()] != nil {
				close(cs[p.ID().String()])
			}
		}()
		return run(p)
	}

	protocol := func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
		return srv(&BzzPeer{
			Peer:      protocols.NewPeer(p, rw, spec),
			localAddr: addr,
			BzzAddr:   NewAddrFromNodeID(p.ID()),
		})
	}

	s := p2ptest.NewProtocolTester(t, NewNodeIDFromAddr(addr), n, protocol)

	for _, id := range s.IDs {
		cs[id.String()] = make(chan bool)
	}

	return &bzzTester{
		addr:           addr,
		ProtocolTester: s,
		cs:             cs,
	}
}

type bzzTester struct {
	*p2ptest.ProtocolTester
	addr *BzzAddr
	cs   map[string]chan bool
	bzz  *Bzz
}

func newBzz(addr *BzzAddr, lightNode bool) *Bzz {
	config := &BzzConfig{
		OverlayAddr:  addr.Over(),
		UnderlayAddr: addr.Under(),
		HiveParams:   NewHiveParams(),
		NetworkID:    DefaultNetworkID,
		LightNode:    lightNode,
	}
	kad := NewKademlia(addr.OAddr, NewKadParams())
	bzz := NewBzz(config, kad, nil, nil, nil)
	return bzz
}

func newBzzHandshakeTester(t *testing.T, n int, addr *BzzAddr, lightNode bool) *bzzTester {
	bzz := newBzz(addr, lightNode)
	pt := p2ptest.NewProtocolTester(t, NewNodeIDFromAddr(addr), n, bzz.runBzz)

	return &bzzTester{
		addr:           addr,
		ProtocolTester: pt,
		bzz:            bzz,
	}
}

//应该在一次交换中测试握手吗？并行化
func (s *bzzTester) testHandshake(lhs, rhs *HandshakeMsg, disconnects ...*p2ptest.Disconnect) error {
	var peers []discover.NodeID
	id := NewNodeIDFromAddr(rhs.Addr)
	if len(disconnects) > 0 {
		for _, d := range disconnects {
			peers = append(peers, d.Peer)
		}
	} else {
		peers = []discover.NodeID{id}
	}

	if err := s.TestExchanges(HandshakeMsgExchange(lhs, rhs, id)...); err != nil {
		return err
	}

	if len(disconnects) > 0 {
		return s.TestDisconnected(disconnects...)
	}

//如果我们不期望断开连接，请确保对等端保持连接
	err := s.TestDisconnected(&p2ptest.Disconnect{
		Peer:  s.IDs[0],
		Error: nil,
	})

	if err == nil {
		return fmt.Errorf("Unexpected peer disconnect")
	}

	if err.Error() != "timed out waiting for peers to disconnect" {
		return err
	}

	return nil
}

func correctBzzHandshake(addr *BzzAddr, lightNode bool) *HandshakeMsg {
	return &HandshakeMsg{
		Version:   TestProtocolVersion,
		NetworkID: TestProtocolNetworkID,
		Addr:      addr,
		LightNode: lightNode,
	}
}

func TestBzzHandshakeNetworkIDMismatch(t *testing.T) {
	lightNode := false
	addr := RandomAddr()
	s := newBzzHandshakeTester(t, 1, addr, lightNode)
	id := s.IDs[0]

	err := s.testHandshake(
		correctBzzHandshake(addr, lightNode),
		&HandshakeMsg{Version: TestProtocolVersion, NetworkID: 321, Addr: NewAddrFromNodeID(id)},
		&p2ptest.Disconnect{Peer: id, Error: fmt.Errorf("Handshake error: Message handler error: (msg code 0): network id mismatch 321 (!= 3)")},
	)

	if err != nil {
		t.Fatal(err)
	}
}

func TestBzzHandshakeVersionMismatch(t *testing.T) {
	lightNode := false
	addr := RandomAddr()
	s := newBzzHandshakeTester(t, 1, addr, lightNode)
	id := s.IDs[0]

	err := s.testHandshake(
		correctBzzHandshake(addr, lightNode),
		&HandshakeMsg{Version: 0, NetworkID: TestProtocolNetworkID, Addr: NewAddrFromNodeID(id)},
		&p2ptest.Disconnect{Peer: id, Error: fmt.Errorf("Handshake error: Message handler error: (msg code 0): version mismatch 0 (!= %d)", TestProtocolVersion)},
	)

	if err != nil {
		t.Fatal(err)
	}
}

func TestBzzHandshakeSuccess(t *testing.T) {
	lightNode := false
	addr := RandomAddr()
	s := newBzzHandshakeTester(t, 1, addr, lightNode)
	id := s.IDs[0]

	err := s.testHandshake(
		correctBzzHandshake(addr, lightNode),
		&HandshakeMsg{Version: TestProtocolVersion, NetworkID: TestProtocolNetworkID, Addr: NewAddrFromNodeID(id)},
	)

	if err != nil {
		t.Fatal(err)
	}
}

func TestBzzHandshakeLightNode(t *testing.T) {
	var lightNodeTests = []struct {
		name      string
		lightNode bool
	}{
		{"on", true},
		{"off", false},
	}

	for _, test := range lightNodeTests {
		t.Run(test.name, func(t *testing.T) {
			randomAddr := RandomAddr()
			pt := newBzzHandshakeTester(t, 1, randomAddr, false)
			id := pt.IDs[0]
			addr := NewAddrFromNodeID(id)

			err := pt.testHandshake(
				correctBzzHandshake(randomAddr, false),
				&HandshakeMsg{Version: TestProtocolVersion, NetworkID: TestProtocolNetworkID, Addr: addr, LightNode: test.lightNode},
			)

			if err != nil {
				t.Fatal(err)
			}

			if pt.bzz.handshakes[id].LightNode != test.lightNode {
				t.Fatalf("peer LightNode flag is %v, should be %v", pt.bzz.handshakes[id].LightNode, test.lightNode)
			}
		})
	}
}
