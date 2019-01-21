
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
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
)

//
//
func (s *Simulation) Service(name string, id discover.NodeID) node.Service {
	simNode, ok := s.Net.GetNode(id).Node.(*adapters.SimNode)
	if !ok {
		return nil
	}
	services := simNode.ServiceMap()
	if len(services) == 0 {
		return nil
	}
	return services[name]
}

//
//
func (s *Simulation) RandomService(name string) node.Service {
	n := s.RandomUpNode()
	if n == nil {
		return nil
	}
	return n.Service(name)
}

//
//
func (s *Simulation) Services(name string) (services map[discover.NodeID]node.Service) {
	nodes := s.Net.GetNodes()
	services = make(map[discover.NodeID]node.Service)
	for _, node := range nodes {
		if !node.Up {
			continue
		}
		simNode, ok := node.Node.(*adapters.SimNode)
		if !ok {
			continue
		}
		services[node.ID()] = simNode.Service(name)
	}
	return services
}
