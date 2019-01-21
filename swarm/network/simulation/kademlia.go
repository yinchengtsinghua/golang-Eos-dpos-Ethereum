
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
	"encoding/hex"
	"time"

	"github.com/ethereum/go-ethereum/p2p/discover"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/swarm/network"
)

//
//
var BucketKeyKademlia BucketKey = "kademlia"

//
//
func (s *Simulation) WaitTillHealthy(ctx context.Context, kadMinProxSize int) (ill map[discover.NodeID]*network.Kademlia, err error) {
//
	var ppmap map[string]*network.PeerPot
	kademlias := s.kademlias()
	addrs := make([][]byte, 0, len(kademlias))
	for _, k := range kademlias {
		addrs = append(addrs, k.BaseAddr())
	}
	ppmap = network.NewPeerPotMap(kadMinProxSize, addrs)

//
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	ill = make(map[discover.NodeID]*network.Kademlia)
	for {
		select {
		case <-ctx.Done():
			return ill, ctx.Err()
		case <-ticker.C:
			for k := range ill {
				delete(ill, k)
			}
			log.Debug("kademlia health check", "addr count", len(addrs))
			for id, k := range kademlias {
//
				addr := common.Bytes2Hex(k.BaseAddr())
				pp := ppmap[addr]
//
				h := k.Healthy(pp)
//
				log.Debug(k.String())
				log.Debug("kademlia", "empty bins", pp.EmptyBins, "gotNN", h.GotNN, "knowNN", h.KnowNN, "full", h.Full)
				log.Debug("kademlia", "health", h.GotNN && h.KnowNN && h.Full, "addr", hex.EncodeToString(k.BaseAddr()), "node", id)
				log.Debug("kademlia", "ill condition", !h.GotNN || !h.Full, "addr", hex.EncodeToString(k.BaseAddr()), "node", id)
				if !h.GotNN || !h.Full {
					ill[id] = k
				}
			}
			if len(ill) == 0 {
				return nil, nil
			}
		}
	}
}

//
//
func (s *Simulation) kademlias() (ks map[discover.NodeID]*network.Kademlia) {
	items := s.UpNodesItems(BucketKeyKademlia)
	ks = make(map[discover.NodeID]*network.Kademlia, len(items))
	for id, v := range items {
		k, ok := v.(*network.Kademlia)
		if !ok {
			continue
		}
		ks[id] = k
	}
	return ks
}
