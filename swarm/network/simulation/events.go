
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
	"sync"

	"github.com/ethereum/go-ethereum/p2p/discover"

	"github.com/ethereum/go-ethereum/p2p"
)

//
type PeerEvent struct {
//
	NodeID discover.NodeID
//
	Event *p2p.PeerEvent
//
	Error error
}

//
//
type PeerEventsFilter struct {
	t        *p2p.PeerEventType
	protocol *string
	msgCode  *uint64
}

//
func NewPeerEventsFilter() *PeerEventsFilter {
	return &PeerEventsFilter{}
}

//
func (f *PeerEventsFilter) Type(t p2p.PeerEventType) *PeerEventsFilter {
	f.t = &t
	return f
}

//
func (f *PeerEventsFilter) Protocol(p string) *PeerEventsFilter {
	f.protocol = &p
	return f
}

//
func (f *PeerEventsFilter) MsgCode(c uint64) *PeerEventsFilter {
	f.msgCode = &c
	return f
}

//
//
//
func (s *Simulation) PeerEvents(ctx context.Context, ids []discover.NodeID, filters ...*PeerEventsFilter) <-chan PeerEvent {
	eventC := make(chan PeerEvent)

//
//
	var subsWG sync.WaitGroup
	for _, id := range ids {
		s.shutdownWG.Add(1)
		subsWG.Add(1)
		go func(id discover.NodeID) {
			defer s.shutdownWG.Done()

			client, err := s.Net.GetNode(id).Client()
			if err != nil {
				subsWG.Done()
				eventC <- PeerEvent{NodeID: id, Error: err}
				return
			}
			events := make(chan *p2p.PeerEvent)
			sub, err := client.Subscribe(ctx, "admin", events, "peerEvents")
			if err != nil {
				subsWG.Done()
				eventC <- PeerEvent{NodeID: id, Error: err}
				return
			}
			defer sub.Unsubscribe()

			subsWG.Done()

			for {
				select {
				case <-ctx.Done():
					if err := ctx.Err(); err != nil {
						select {
						case eventC <- PeerEvent{NodeID: id, Error: err}:
						case <-s.Done():
						}
					}
					return
				case <-s.Done():
					return
				case e := <-events:
match := len(filters) == 0 //
					for _, f := range filters {
						if f.t != nil && *f.t != e.Type {
							continue
						}
						if f.protocol != nil && *f.protocol != e.Protocol {
							continue
						}
						if f.msgCode != nil && e.MsgCode != nil && *f.msgCode != *e.MsgCode {
							continue
						}
//
						match = true
						break
					}
					if match {
						select {
						case eventC <- PeerEvent{NodeID: id, Event: e}:
						case <-ctx.Done():
							if err := ctx.Err(); err != nil {
								select {
								case eventC <- PeerEvent{NodeID: id, Error: err}:
								case <-s.Done():
								}
							}
							return
						case <-s.Done():
							return
						}
					}
				case err := <-sub.Err():
					if err != nil {
						select {
						case eventC <- PeerEvent{NodeID: id, Error: err}:
						case <-ctx.Done():
							if err := ctx.Err(); err != nil {
								select {
								case eventC <- PeerEvent{NodeID: id, Error: err}:
								case <-s.Done():
								}
							}
							return
						case <-s.Done():
							return
						}
					}
				}
			}
		}(id)
	}

//
	subsWG.Wait()
	return eventC
}
