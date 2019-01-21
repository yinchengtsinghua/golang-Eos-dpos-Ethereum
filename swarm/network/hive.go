
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

package network

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/state"
)

/*





*/


//
type Overlay interface {
//
	SuggestPeer() (OverlayAddr, int, bool)
//
	On(OverlayConn) (depth uint8, changed bool)
	Off(OverlayConn)
//
	Register([]OverlayAddr) error
//
	EachConn([]byte, int, func(OverlayConn, int, bool) bool)
//
	EachAddr([]byte, int, func(OverlayAddr, int, bool) bool)
//
	String() string
//
	BaseAddr() []byte
//
	Healthy(*PeerPot) *Health
}

//
type HiveParams struct {
Discovery             bool  //
PeersBroadcastSetSize uint8 //
MaxPeersPerRequest    uint8 //
	KeepAliveInterval     time.Duration
}

//
func NewHiveParams() *HiveParams {
	return &HiveParams{
		Discovery:             true,
		PeersBroadcastSetSize: 3,
		MaxPeersPerRequest:    5,
		KeepAliveInterval:     500 * time.Millisecond,
	}
}

//
type Hive struct {
*HiveParams                      //
Overlay                          //
Store       state.Store          //
addPeer     func(*discover.Node) //
//
	lock   sync.Mutex
	ticker *time.Ticker
}

//
//
//
//
func NewHive(params *HiveParams, overlay Overlay, store state.Store) *Hive {
	return &Hive{
		HiveParams: params,
		Overlay:    overlay,
		Store:      store,
	}
}

//
//
//
func (h *Hive) Start(server *p2p.Server) error {
	log.Info("Starting hive", "baseaddr", fmt.Sprintf("%x", h.BaseAddr()[:4]))
//
	if h.Store != nil {
		log.Info("Detected an existing store. trying to load peers")
		if err := h.loadPeers(); err != nil {
			log.Error(fmt.Sprintf("%08x hive encoutered an error trying to load peers", h.BaseAddr()[:4]))
			return err
		}
	}
//
	h.addPeer = server.AddPeer
//
	h.ticker = time.NewTicker(h.KeepAliveInterval)
//
	go h.connect()
	return nil
}

//
func (h *Hive) Stop() error {
	log.Info(fmt.Sprintf("%08x hive stopping, saving peers", h.BaseAddr()[:4]))
	h.ticker.Stop()
	if h.Store != nil {
		if err := h.savePeers(); err != nil {
			return fmt.Errorf("could not save peers to persistence store: %v", err)
		}
		if err := h.Store.Close(); err != nil {
			return fmt.Errorf("could not close file handle to persistence store: %v", err)
		}
	}
	log.Info(fmt.Sprintf("%08x hive stopped, dropping peers", h.BaseAddr()[:4]))
	h.EachConn(nil, 255, func(p OverlayConn, _ int, _ bool) bool {
		log.Info(fmt.Sprintf("%08x dropping peer %08x", h.BaseAddr()[:4], p.Address()[:4]))
		p.Drop(nil)
		return true
	})

	log.Info(fmt.Sprintf("%08x all peers dropped", h.BaseAddr()[:4]))
	return nil
}

//
//
//
func (h *Hive) connect() {
	for range h.ticker.C {

		addr, depth, changed := h.SuggestPeer()
		if h.Discovery && changed {
			NotifyDepth(uint8(depth), h)
		}
		if addr == nil {
			continue
		}

		log.Trace(fmt.Sprintf("%08x hive connect() suggested %08x", h.BaseAddr()[:4], addr.Address()[:4]))
		under, err := discover.ParseNode(string(addr.(Addr).Under()))
		if err != nil {
			log.Warn(fmt.Sprintf("%08x unable to connect to bee %08x: invalid node URL: %v", h.BaseAddr()[:4], addr.Address()[:4], err))
			continue
		}
		log.Trace(fmt.Sprintf("%08x attempt to connect to bee %08x", h.BaseAddr()[:4], addr.Address()[:4]))
		h.addPeer(under)
	}
}

//
func (h *Hive) Run(p *BzzPeer) error {
	dp := newDiscovery(p, h)
	depth, changed := h.On(dp)
//
	if h.Discovery {
		if changed {
//
			NotifyDepth(depth, h)
		} else {
//
			dp.NotifyDepth(depth)
		}
	}
	NotifyPeer(p.Off(), h)
	defer h.Off(dp)
	return dp.Run(dp.HandleMsg)
}

//
//
func (h *Hive) NodeInfo() interface{} {
	return h.String()
}

//
//
func (h *Hive) PeerInfo(id discover.NodeID) interface{} {
	addr := NewAddrFromNodeID(id)
	return struct {
		OAddr hexutil.Bytes
		UAddr hexutil.Bytes
	}{
		OAddr: addr.OAddr,
		UAddr: addr.UAddr,
	}
}

//
func ToAddr(pa OverlayPeer) *BzzAddr {
	if addr, ok := pa.(*BzzAddr); ok {
		return addr
	}
	if p, ok := pa.(*discPeer); ok {
		return p.BzzAddr
	}
	return pa.(*BzzPeer).BzzAddr
}

//
func (h *Hive) loadPeers() error {
	var as []*BzzAddr
	err := h.Store.Get("peers", &as)
	if err != nil {
		if err == state.ErrNotFound {
			log.Info(fmt.Sprintf("hive %08x: no persisted peers found", h.BaseAddr()[:4]))
			return nil
		}
		return err
	}
	log.Info(fmt.Sprintf("hive %08x: peers loaded", h.BaseAddr()[:4]))

	return h.Register(toOverlayAddrs(as...))
}

//
func toOverlayAddrs(as ...*BzzAddr) (oas []OverlayAddr) {
	for _, a := range as {
		oas = append(oas, OverlayAddr(a))
	}
	return
}

//
func (h *Hive) savePeers() error {
	var peers []*BzzAddr
	h.Overlay.EachAddr(nil, 256, func(pa OverlayAddr, i int, _ bool) bool {
		if pa == nil {
			log.Warn(fmt.Sprintf("empty addr: %v", i))
			return true
		}
		apa := ToAddr(pa)
		log.Trace("saving peer", "peer", apa)
		peers = append(peers, apa)
		return true
	})
	if err := h.Store.Put("peers", peers); err != nil {
		return fmt.Errorf("could not save peers: %v", err)
	}
	return nil
}
