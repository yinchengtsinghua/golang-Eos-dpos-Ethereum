
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
	"testing"

	p2ptest "github.com/ethereum/go-ethereum/p2p/testing"
)

/*
 
 
 
 */

func TestDiscovery(t *testing.T) {
	params := NewHiveParams()
	s, pp := newHiveTester(t, params, 1, nil)

	id := s.IDs[0]
	raddr := NewAddrFromNodeID(id)
	pp.Register([]OverlayAddr{OverlayAddr(raddr)})

//
	pp.Start(s.Server)
	defer pp.Stop()

//
	err := s.TestExchanges(p2ptest.Exchange{
		Label: "outgoing subPeersMsg",
		Expects: []p2ptest.Expect{
			{
				Code: 1,
				Msg:  &subPeersMsg{Depth: 0},
				Peer: id,
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}
}
