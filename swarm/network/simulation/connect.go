
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
	"strings"

	"github.com/ethereum/go-ethereum/p2p/discover"
)

//ConnectToPivotNode将节点与提供的节点ID连接起来
//到透视节点，已由simulation.setPivotNode方法设置。
//它在构建星型网络拓扑时很有用
//当模拟动态添加和删除节点时。
func (s *Simulation) ConnectToPivotNode(id discover.NodeID) (err error) {
	pid := s.PivotNodeID()
	if pid == nil {
		return ErrNoPivotNode
	}
	return s.connect(*pid, id)
}

//ConnectToLastNode将节点与提供的节点ID连接起来
//到上一个节点，并避免连接到自身。
//它在构建链网络拓扑结构时很有用
//当模拟动态添加和删除节点时。
func (s *Simulation) ConnectToLastNode(id discover.NodeID) (err error) {
	ids := s.UpNodeIDs()
	l := len(ids)
	if l < 2 {
		return nil
	}
	lid := ids[l-1]
	if lid == id {
		lid = ids[l-2]
	}
	return s.connect(lid, id)
}

//connecttorandomnode将节点与provided nodeid连接起来
//向上的随机节点发送。
func (s *Simulation) ConnectToRandomNode(id discover.NodeID) (err error) {
	n := s.RandomUpNode(id)
	if n == nil {
		return ErrNodeNotFound
	}
	return s.connect(n.ID, id)
}

//ConnectNodesFull将所有节点连接到另一个。
//它在网络中提供了完整的连接
//这应该是很少需要的。
func (s *Simulation) ConnectNodesFull(ids []discover.NodeID) (err error) {
	if ids == nil {
		ids = s.UpNodeIDs()
	}
	l := len(ids)
	for i := 0; i < l; i++ {
		for j := i + 1; j < l; j++ {
			err = s.connect(ids[i], ids[j])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

//connectnodeschain连接链拓扑中的所有节点。
//如果ids参数为nil，则所有打开的节点都将被连接。
func (s *Simulation) ConnectNodesChain(ids []discover.NodeID) (err error) {
	if ids == nil {
		ids = s.UpNodeIDs()
	}
	l := len(ids)
	for i := 0; i < l-1; i++ {
		err = s.connect(ids[i], ids[i+1])
		if err != nil {
			return err
		}
	}
	return nil
}

//ConnectNodesRing连接环拓扑中的所有节点。
//如果ids参数为nil，则所有打开的节点都将被连接。
func (s *Simulation) ConnectNodesRing(ids []discover.NodeID) (err error) {
	if ids == nil {
		ids = s.UpNodeIDs()
	}
	l := len(ids)
	if l < 2 {
		return nil
	}
	for i := 0; i < l-1; i++ {
		err = s.connect(ids[i], ids[i+1])
		if err != nil {
			return err
		}
	}
	return s.connect(ids[l-1], ids[0])
}

//connectnodestar连接星形拓扑中的所有节点
//中心位于提供的节点ID。
//如果ids参数为nil，则所有打开的节点都将被连接。
func (s *Simulation) ConnectNodesStar(id discover.NodeID, ids []discover.NodeID) (err error) {
	if ids == nil {
		ids = s.UpNodeIDs()
	}
	l := len(ids)
	for i := 0; i < l; i++ {
		if id == ids[i] {
			continue
		}
		err = s.connect(id, ids[i])
		if err != nil {
			return err
		}
	}
	return nil
}

//ConnectNodessTarPivot连接星形拓扑中的所有节点
//中心位于已设置的轴节点。
//如果ids参数为nil，则所有打开的节点都将被连接。
func (s *Simulation) ConnectNodesStarPivot(ids []discover.NodeID) (err error) {
	id := s.PivotNodeID()
	if id == nil {
		return ErrNoPivotNode
	}
	return s.ConnectNodesStar(*id, ids)
}

//连接连接两个节点，但忽略已连接的错误。
func (s *Simulation) connect(oneID, otherID discover.NodeID) error {
	return ignoreAlreadyConnectedErr(s.Net.Connect(oneID, otherID))
}

func ignoreAlreadyConnectedErr(err error) error {
	if err == nil || strings.Contains(err.Error(), "already connected") {
		return nil
	}
	return err
}
