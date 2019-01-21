
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

package protocols

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	p2ptest "github.com/ethereum/go-ethereum/p2p/testing"
)

//握手消息类型
type hs0 struct {
	C uint
}

//用nodeid终止/删除对等机的消息
type kill struct {
	C discover.NodeID
}

//断开连接的消息
type drop struct {
}

///protochandshake表示协议与模块无关的方面，并且
//第一个消息对等端作为初始交换的一部分发送和接收
type protoHandshake struct {
Version   uint   //本地和远程对等机应具有相同的版本
NetworkID string //本地和远程对等机应具有相同的网络ID
}

//检查协议握手验证本地和远程协议握手是否匹配
func checkProtoHandshake(testVersion uint, testNetworkID string) func(interface{}) error {
	return func(rhs interface{}) error {
		remote := rhs.(*protoHandshake)
		if remote.NetworkID != testNetworkID {
			return fmt.Errorf("%s (!= %s)", remote.NetworkID, testNetworkID)
		}

		if remote.Version != testVersion {
			return fmt.Errorf("%d (!= %d)", remote.Version, testVersion)
		}
		return nil
	}
}

//新协议设置协议
//这里的run函数演示了使用peerpool和handshake的典型协议
//以及注册到处理程序的消息
func newProtocol(pp *p2ptest.TestPeerPool) func(*p2p.Peer, p2p.MsgReadWriter) error {
	spec := &Spec{
		Name:       "test",
		Version:    42,
		MaxMsgSize: 10 * 1024,
		Messages: []interface{}{
			protoHandshake{},
			hs0{},
			kill{},
			drop{},
		},
	}
	return func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
		peer := NewPeer(p, rw, spec)

//启动一次性协议握手并检查有效性
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		phs := &protoHandshake{42, "420"}
		hsCheck := checkProtoHandshake(phs.Version, phs.NetworkID)
		_, err := peer.Handshake(ctx, phs, hsCheck)
		if err != nil {
			return err
		}

		lhs := &hs0{42}
//模块握手演示同一类型消息的简单可重复交换
		hs, err := peer.Handshake(ctx, lhs, nil)
		if err != nil {
			return err
		}

		if rmhs := hs.(*hs0); rmhs.C > lhs.C {
			return fmt.Errorf("handshake mismatch remote %v > local %v", rmhs.C, lhs.C)
		}

		handle := func(ctx context.Context, msg interface{}) error {
			switch msg := msg.(type) {

			case *protoHandshake:
				return errors.New("duplicate handshake")

			case *hs0:
				rhs := msg
				if rhs.C > lhs.C {
					return fmt.Errorf("handshake mismatch remote %v > local %v", rhs.C, lhs.C)
				}
				lhs.C += rhs.C
				return peer.Send(ctx, lhs)

			case *kill:
//演示使用对等池，终止另一个对等连接作为对消息的响应
				id := msg.C
				pp.Get(id).Drop(errors.New("killed"))
				return nil

			case *drop:
//对于测试，我们可以在接收到丢弃消息时触发自诱导断开。
				return errors.New("dropped")

			default:
				return fmt.Errorf("unknown message type: %T", msg)
			}
		}

		pp.Add(peer)
		defer pp.Remove(peer)
		return peer.Run(handle)
	}
}

func protocolTester(t *testing.T, pp *p2ptest.TestPeerPool) *p2ptest.ProtocolTester {
	conf := adapters.RandomNodeConfig()
	return p2ptest.NewProtocolTester(t, conf.ID, 2, newProtocol(pp))
}

func protoHandshakeExchange(id discover.NodeID, proto *protoHandshake) []p2ptest.Exchange {

	return []p2ptest.Exchange{
		{
			Expects: []p2ptest.Expect{
				{
					Code: 0,
					Msg:  &protoHandshake{42, "420"},
					Peer: id,
				},
			},
		},
		{
			Triggers: []p2ptest.Trigger{
				{
					Code: 0,
					Msg:  proto,
					Peer: id,
				},
			},
		},
	}
}

func runProtoHandshake(t *testing.T, proto *protoHandshake, errs ...error) {
	pp := p2ptest.NewTestPeerPool()
	s := protocolTester(t, pp)
//托多：多做一次握手
	id := s.IDs[0]
	if err := s.TestExchanges(protoHandshakeExchange(id, proto)...); err != nil {
		t.Fatal(err)
	}
	var disconnects []*p2ptest.Disconnect
	for i, err := range errs {
		disconnects = append(disconnects, &p2ptest.Disconnect{Peer: s.IDs[i], Error: err})
	}
	if err := s.TestDisconnected(disconnects...); err != nil {
		t.Fatal(err)
	}
}

func TestProtoHandshakeVersionMismatch(t *testing.T) {
	runProtoHandshake(t, &protoHandshake{41, "420"}, errorf(ErrHandshake, errorf(ErrHandler, "(msg code 0): 41 (!= 42)").Error()))
}

func TestProtoHandshakeNetworkIDMismatch(t *testing.T) {
	runProtoHandshake(t, &protoHandshake{42, "421"}, errorf(ErrHandshake, errorf(ErrHandler, "(msg code 0): 421 (!= 420)").Error()))
}

func TestProtoHandshakeSuccess(t *testing.T) {
	runProtoHandshake(t, &protoHandshake{42, "420"})
}

func moduleHandshakeExchange(id discover.NodeID, resp uint) []p2ptest.Exchange {

	return []p2ptest.Exchange{
		{
			Expects: []p2ptest.Expect{
				{
					Code: 1,
					Msg:  &hs0{42},
					Peer: id,
				},
			},
		},
		{
			Triggers: []p2ptest.Trigger{
				{
					Code: 1,
					Msg:  &hs0{resp},
					Peer: id,
				},
			},
		},
	}
}

func runModuleHandshake(t *testing.T, resp uint, errs ...error) {
	pp := p2ptest.NewTestPeerPool()
	s := protocolTester(t, pp)
	id := s.IDs[0]
	if err := s.TestExchanges(protoHandshakeExchange(id, &protoHandshake{42, "420"})...); err != nil {
		t.Fatal(err)
	}
	if err := s.TestExchanges(moduleHandshakeExchange(id, resp)...); err != nil {
		t.Fatal(err)
	}
	var disconnects []*p2ptest.Disconnect
	for i, err := range errs {
		disconnects = append(disconnects, &p2ptest.Disconnect{Peer: s.IDs[i], Error: err})
	}
	if err := s.TestDisconnected(disconnects...); err != nil {
		t.Fatal(err)
	}
}

func TestModuleHandshakeError(t *testing.T) {
	runModuleHandshake(t, 43, fmt.Errorf("handshake mismatch remote 43 > local 42"))
}

func TestModuleHandshakeSuccess(t *testing.T) {
	runModuleHandshake(t, 42)
}

//在多个对等点上测试复杂的交互、中继、丢弃
func testMultiPeerSetup(a, b discover.NodeID) []p2ptest.Exchange {

	return []p2ptest.Exchange{
		{
			Label: "primary handshake",
			Expects: []p2ptest.Expect{
				{
					Code: 0,
					Msg:  &protoHandshake{42, "420"},
					Peer: a,
				},
				{
					Code: 0,
					Msg:  &protoHandshake{42, "420"},
					Peer: b,
				},
			},
		},
		{
			Label: "module handshake",
			Triggers: []p2ptest.Trigger{
				{
					Code: 0,
					Msg:  &protoHandshake{42, "420"},
					Peer: a,
				},
				{
					Code: 0,
					Msg:  &protoHandshake{42, "420"},
					Peer: b,
				},
			},
			Expects: []p2ptest.Expect{
				{
					Code: 1,
					Msg:  &hs0{42},
					Peer: a,
				},
				{
					Code: 1,
					Msg:  &hs0{42},
					Peer: b,
				},
			},
		},

		{Label: "alternative module handshake", Triggers: []p2ptest.Trigger{{Code: 1, Msg: &hs0{41}, Peer: a},
			{Code: 1, Msg: &hs0{41}, Peer: b}}},
		{Label: "repeated module handshake", Triggers: []p2ptest.Trigger{{Code: 1, Msg: &hs0{1}, Peer: a}}},
		{Label: "receiving repeated module handshake", Expects: []p2ptest.Expect{{Code: 1, Msg: &hs0{43}, Peer: a}}}}
}

func runMultiplePeers(t *testing.T, peer int, errs ...error) {
	pp := p2ptest.NewTestPeerPool()
	s := protocolTester(t, pp)

	if err := s.TestExchanges(testMultiPeerSetup(s.IDs[0], s.IDs[1])...); err != nil {
		t.Fatal(err)
	}
//在一些消息交换之后，我们可以测试状态变化
//在这里，这只是由Peerpool演示的
//握手后，必须将对等方添加到池中
//睡眠时间（1）
	tick := time.NewTicker(10 * time.Millisecond)
	timeout := time.NewTimer(1 * time.Second)
WAIT:
	for {
		select {
		case <-tick.C:
			if pp.Has(s.IDs[0]) {
				break WAIT
			}
		case <-timeout.C:
			t.Fatal("timeout")
		}
	}
	if !pp.Has(s.IDs[1]) {
		t.Fatalf("missing peer test-1: %v (%v)", pp, s.IDs)
	}

//peer 0发送索引为peer的kill请求<peer>
	err := s.TestExchanges(p2ptest.Exchange{
		Triggers: []p2ptest.Trigger{
			{
				Code: 2,
				Msg:  &kill{s.IDs[peer]},
				Peer: s.IDs[0],
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

//未被杀死的对等机发送删除请求
	err = s.TestExchanges(p2ptest.Exchange{
		Triggers: []p2ptest.Trigger{
			{
				Code: 3,
				Msg:  &drop{},
				Peer: s.IDs[(peer+1)%2],
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

//检查各个对等机上的实际DiscConnect错误
	var disconnects []*p2ptest.Disconnect
	for i, err := range errs {
		disconnects = append(disconnects, &p2ptest.Disconnect{Peer: s.IDs[i], Error: err})
	}
	if err := s.TestDisconnected(disconnects...); err != nil {
		t.Fatal(err)
	}
//测试是否已从对等池中删除断开连接的对等机
	if pp.Has(s.IDs[peer]) {
		t.Fatalf("peer test-%v not dropped: %v (%v)", peer, pp, s.IDs)
	}

}
func XTestMultiplePeersDropSelf(t *testing.T) {
	runMultiplePeers(t, 0,
		fmt.Errorf("subprotocol error"),
		fmt.Errorf("Message handler error: (msg code 3): dropped"),
	)
}

func XTestMultiplePeersDropOther(t *testing.T) {
	runMultiplePeers(t, 1,
		fmt.Errorf("Message handler error: (msg code 3): dropped"),
		fmt.Errorf("subprotocol error"),
	)
}
