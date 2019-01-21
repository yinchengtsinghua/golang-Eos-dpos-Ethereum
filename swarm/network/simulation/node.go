
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
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
)

//
func (s *Simulation) NodeIDs() (ids []discover.NodeID) {
	nodes := s.Net.GetNodes()
	ids = make([]discover.NodeID, len(nodes))
	for i, node := range nodes {
		ids[i] = node.ID()
	}
	return ids
}

//
func (s *Simulation) UpNodeIDs() (ids []discover.NodeID) {
	nodes := s.Net.GetNodes()
	for _, node := range nodes {
		if node.Up {
			ids = append(ids, node.ID())
		}
	}
	return ids
}

//
func (s *Simulation) DownNodeIDs() (ids []discover.NodeID) {
	nodes := s.Net.GetNodes()
	for _, node := range nodes {
		if !node.Up {
			ids = append(ids, node.ID())
		}
	}
	return ids
}

//
//
type AddNodeOption func(*adapters.NodeConfig)

//
//
func AddNodeWithMsgEvents(enable bool) AddNodeOption {
	return func(o *adapters.NodeConfig) {
		o.EnableMsgEvents = enable
	}
}

//
//
//
//
func AddNodeWithService(serviceName string) AddNodeOption {
	return func(o *adapters.NodeConfig) {
		o.Services = append(o.Services, serviceName)
	}
}

//
//
//
//
func (s *Simulation) AddNode(opts ...AddNodeOption) (id discover.NodeID, err error) {
	conf := adapters.RandomNodeConfig()
	for _, o := range opts {
		o(conf)
	}
	if len(conf.Services) == 0 {
		conf.Services = s.serviceNames
	}
	node, err := s.Net.NewNodeWithConfig(conf)
	if err != nil {
		return id, err
	}
	return node.ID(), s.Net.Start(node.ID())
}

//
//
func (s *Simulation) AddNodes(count int, opts ...AddNodeOption) (ids []discover.NodeID, err error) {
	ids = make([]discover.NodeID, 0, count)
	for i := 0; i < count; i++ {
		id, err := s.AddNode(opts...)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

//
//
func (s *Simulation) AddNodesAndConnectFull(count int, opts ...AddNodeOption) (ids []discover.NodeID, err error) {
	if count < 2 {
		return nil, errors.New("count of nodes must be at least 2")
	}
	ids, err = s.AddNodes(count, opts...)
	if err != nil {
		return nil, err
	}
	err = s.ConnectNodesFull(ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

//
//
//
func (s *Simulation) AddNodesAndConnectChain(count int, opts ...AddNodeOption) (ids []discover.NodeID, err error) {
	if count < 2 {
		return nil, errors.New("count of nodes must be at least 2")
	}
	id, err := s.AddNode(opts...)
	if err != nil {
		return nil, err
	}
	err = s.ConnectToLastNode(id)
	if err != nil {
		return nil, err
	}
	ids, err = s.AddNodes(count-1, opts...)
	if err != nil {
		return nil, err
	}
	ids = append([]discover.NodeID{id}, ids...)
	err = s.ConnectNodesChain(ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

//
//
func (s *Simulation) AddNodesAndConnectRing(count int, opts ...AddNodeOption) (ids []discover.NodeID, err error) {
	if count < 2 {
		return nil, errors.New("count of nodes must be at least 2")
	}
	ids, err = s.AddNodes(count, opts...)
	if err != nil {
		return nil, err
	}
	err = s.ConnectNodesRing(ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

//
//
func (s *Simulation) AddNodesAndConnectStar(count int, opts ...AddNodeOption) (ids []discover.NodeID, err error) {
	if count < 2 {
		return nil, errors.New("count of nodes must be at least 2")
	}
	ids, err = s.AddNodes(count, opts...)
	if err != nil {
		return nil, err
	}
	err = s.ConnectNodesStar(ids[0], ids[1:])
	if err != nil {
		return nil, err
	}
	return ids, nil
}

//
//
//
func (s *Simulation) UploadSnapshot(snapshotFile string, opts ...AddNodeOption) error {
	f, err := os.Open(snapshotFile)
	if err != nil {
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Error("Error closing snapshot file", "err", err)
		}
	}()
	jsonbyte, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	var snap simulations.Snapshot
	err = json.Unmarshal(jsonbyte, &snap)
	if err != nil {
		return err
	}

//
//
//
	for _, n := range snap.Nodes {
		n.Node.Config.EnableMsgEvents = true
		n.Node.Config.Services = s.serviceNames
		for _, o := range opts {
			o(n.Node.Config)
		}
	}

	log.Info("Waiting for p2p connections to be established...")

//
	err = s.Net.Load(&snap)
	if err != nil {
		return err
	}
	log.Info("Snapshot loaded")
	return nil
}

//
//
//
//
//
func (s *Simulation) SetPivotNode(id discover.NodeID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pivotNodeID = &id
}

//
//
func (s *Simulation) PivotNodeID() (id *discover.NodeID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pivotNodeID
}

//
func (s *Simulation) StartNode(id discover.NodeID) (err error) {
	return s.Net.Start(id)
}

//
func (s *Simulation) StartRandomNode() (id discover.NodeID, err error) {
	n := s.randomDownNode()
	if n == nil {
		return id, ErrNodeNotFound
	}
	return n.ID, s.Net.Start(n.ID)
}

//
func (s *Simulation) StartRandomNodes(count int) (ids []discover.NodeID, err error) {
	ids = make([]discover.NodeID, 0, count)
	downIDs := s.DownNodeIDs()
	for i := 0; i < count; i++ {
		n := s.randomNode(downIDs, ids...)
		if n == nil {
			return nil, ErrNodeNotFound
		}
		err = s.Net.Start(n.ID)
		if err != nil {
			return nil, err
		}
		ids = append(ids, n.ID)
	}
	return ids, nil
}

//
func (s *Simulation) StopNode(id discover.NodeID) (err error) {
	return s.Net.Stop(id)
}

//
func (s *Simulation) StopRandomNode() (id discover.NodeID, err error) {
	n := s.RandomUpNode()
	if n == nil {
		return id, ErrNodeNotFound
	}
	return n.ID, s.Net.Stop(n.ID)
}

//
func (s *Simulation) StopRandomNodes(count int) (ids []discover.NodeID, err error) {
	ids = make([]discover.NodeID, 0, count)
	upIDs := s.UpNodeIDs()
	for i := 0; i < count; i++ {
		n := s.randomNode(upIDs, ids...)
		if n == nil {
			return nil, ErrNodeNotFound
		}
		err = s.Net.Stop(n.ID)
		if err != nil {
			return nil, err
		}
		ids = append(ids, n.ID)
	}
	return ids, nil
}

//
func init() {
	rand.Seed(time.Now().UnixNano())
}

//
//
func (s *Simulation) RandomUpNode(exclude ...discover.NodeID) *adapters.SimNode {
	return s.randomNode(s.UpNodeIDs(), exclude...)
}

//
func (s *Simulation) randomDownNode(exclude ...discover.NodeID) *adapters.SimNode {
	return s.randomNode(s.DownNodeIDs(), exclude...)
}

//
func (s *Simulation) randomNode(ids []discover.NodeID, exclude ...discover.NodeID) *adapters.SimNode {
	for _, e := range exclude {
		var i int
		for _, id := range ids {
			if id == e {
				ids = append(ids[:i], ids[i+1:]...)
			} else {
				i++
			}
		}
	}
	l := len(ids)
	if l == 0 {
		return nil
	}
	n := s.Net.GetNode(ids[rand.Intn(l)])
	node, _ := n.Node.(*adapters.SimNode)
	return node
}
