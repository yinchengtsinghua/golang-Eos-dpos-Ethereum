
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
	"sync"
	"testing"
	"time"
)

//
//
//
//
func TestPeerEvents(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	_, err := sim.AddNodes(2)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events := sim.PeerEvents(ctx, sim.NodeIDs())

//
	expectedEventCount := 2

	var wg sync.WaitGroup
	wg.Add(expectedEventCount)

	go func() {
		for e := range events {
			if e.Error != nil {
				if e.Error == context.Canceled {
					return
				}
				t.Error(e.Error)
				continue
			}
			wg.Done()
		}
	}()

	err = sim.ConnectNodesChain(sim.NodeIDs())
	if err != nil {
		t.Fatal(err)
	}

	wg.Wait()
}

func TestPeerEventsTimeout(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	_, err := sim.AddNodes(2)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	events := sim.PeerEvents(ctx, sim.NodeIDs())

	done := make(chan struct{})
	go func() {
		for e := range events {
			if e.Error == context.Canceled {
				return
			}
			if e.Error == context.DeadlineExceeded {
				close(done)
				return
			} else {
				t.Fatal(e.Error)
			}
		}
	}()

	select {
	case <-time.After(time.Second):
		t.Error("no context deadline received")
	case <-done:
//
	}
}
