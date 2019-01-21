
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

package simulation_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
)

//
//
//
func ExampleSimulation_WaitTillHealthy() {
	sim := simulation.New(map[string]simulation.ServiceFunc{
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
//
//
			b.Store(simulation.BucketKeyKademlia, kad)
			return network.NewBzz(config, kad, nil, nil, nil), nil, nil
		},
	})
	defer sim.Close()

	_, err := sim.AddNodesAndConnectRing(10)
	if err != nil {
//
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ill, err := sim.WaitTillHealthy(ctx, 2)
	if err != nil {
//
		for id, kad := range ill {
			fmt.Println("Node", id)
			fmt.Println(kad.String())
		}
//
	}

//
}

//
func ExampleSimulation_PeerEvents() {
	sim := simulation.New(nil)
	defer sim.Close()

	events := sim.PeerEvents(context.Background(), sim.NodeIDs())

	go func() {
		for e := range events {
			if e.Error != nil {
				log.Error("peer event", "err", e.Error)
				continue
			}
			log.Info("peer event", "node", e.NodeID, "peer", e.Event.Peer, "msgcode", e.Event.MsgCode)
		}
	}()
}

//
func ExampleSimulation_PeerEvents_disconnections() {
	sim := simulation.New(nil)
	defer sim.Close()

	disconnections := sim.PeerEvents(
		context.Background(),
		sim.NodeIDs(),
		simulation.NewPeerEventsFilter().Type(p2p.PeerEventTypeDrop),
	)

	go func() {
		for d := range disconnections {
			if d.Error != nil {
				log.Error("peer drop", "err", d.Error)
				continue
			}
			log.Warn("peer drop", "node", d.NodeID, "peer", d.Event.Peer)
		}
	}()
}

//
//
func ExampleSimulation_PeerEvents_multipleFilters() {
	sim := simulation.New(nil)
	defer sim.Close()

	msgs := sim.PeerEvents(
		context.Background(),
		sim.NodeIDs(),
//
		simulation.NewPeerEventsFilter().Type(p2p.PeerEventTypeMsgRecv).Protocol("bzz").MsgCode(1),
		simulation.NewPeerEventsFilter().Type(p2p.PeerEventTypeMsgRecv).Protocol("bzz").MsgCode(4),
	)

	go func() {
		for m := range msgs {
			if m.Error != nil {
				log.Error("bzz message", "err", m.Error)
				continue
			}
			log.Info("bzz message", "node", m.NodeID, "peer", m.Event.Peer)
		}
	}()
}
