
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
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/swarm/pot"
)

//

//
type discPeer struct {
	*BzzPeer
	overlay   Overlay
sentPeers bool //
	mtx       sync.RWMutex
peers     map[string]bool //
depth     uint8           //
}

//
func newDiscovery(p *BzzPeer, o Overlay) *discPeer {
	d := &discPeer{
		overlay: o,
		BzzPeer: p,
		peers:   make(map[string]bool),
	}
//
	d.seen(d)
	return d
}

//
func (d *discPeer) HandleMsg(ctx context.Context, msg interface{}) error {
	switch msg := msg.(type) {

	case *peersMsg:
		return d.handlePeersMsg(msg)

	case *subPeersMsg:
		return d.handleSubPeersMsg(msg)

	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
}

//
func NotifyDepth(depth uint8, h Overlay) {
	f := func(val OverlayConn, po int, _ bool) bool {
		dp, ok := val.(*discPeer)
		if ok {
			dp.NotifyDepth(depth)
		}
		return true
	}
	h.EachConn(nil, 255, f)
}

//
func NotifyPeer(p OverlayAddr, k Overlay) {
	f := func(val OverlayConn, po int, _ bool) bool {
		dp, ok := val.(*discPeer)
		if ok {
			dp.NotifyPeer(p, uint8(po))
		}
		return true
	}
	k.EachConn(p.Address(), 255, f)
}

//
//
//
//
func (d *discPeer) NotifyPeer(a OverlayAddr, po uint8) {
//
	if (po < d.getDepth() && pot.ProxCmp(d.localAddr, d, a) != 1) || d.seen(a) {
		return
	}
//
	resp := &peersMsg{
		Peers: []*BzzAddr{ToAddr(a)},
	}
	go d.Send(context.TODO(), resp)
}

//
//
func (d *discPeer) NotifyDepth(po uint8) {
//
	go d.Send(context.TODO(), &subPeersMsg{Depth: po})
}

/*













*/


//
//
//
type peersMsg struct {
	Peers []*BzzAddr
}

//
func (msg peersMsg) String() string {
	return fmt.Sprintf("%T: %v", msg, msg.Peers)
}

//
//
//
func (d *discPeer) handlePeersMsg(msg *peersMsg) error {
//
	if len(msg.Peers) == 0 {
		return nil
	}

	for _, a := range msg.Peers {
		d.seen(a)
		NotifyPeer(a, d.overlay)
	}
	return d.overlay.Register(toOverlayAddrs(msg.Peers...))
}

//
type subPeersMsg struct {
	Depth uint8
}

//
func (msg subPeersMsg) String() string {
	return fmt.Sprintf("%T: request peers > PO%02d. ", msg, msg.Depth)
}

func (d *discPeer) handleSubPeersMsg(msg *subPeersMsg) error {
	if !d.sentPeers {
		d.setDepth(msg.Depth)
		var peers []*BzzAddr
		d.overlay.EachConn(d.Over(), 255, func(p OverlayConn, po int, isproxbin bool) bool {
			if pob, _ := pof(d, d.localAddr, 0); pob > po {
				return false
			}
			if !d.seen(p) {
				peers = append(peers, ToAddr(p.Off()))
			}
			return true
		})
		if len(peers) > 0 {
//
			go d.Send(context.TODO(), &peersMsg{Peers: peers})
		}
	}
	d.sentPeers = true
	return nil
}

//
//
func (d *discPeer) seen(p OverlayPeer) bool {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	k := string(p.Address())
	if d.peers[k] {
		return true
	}
	d.peers[k] = true
	return false
}

func (d *discPeer) getDepth() uint8 {
	d.mtx.RLock()
	defer d.mtx.RUnlock()
	return d.depth
}
func (d *discPeer) setDepth(depth uint8) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	d.depth = depth
}
