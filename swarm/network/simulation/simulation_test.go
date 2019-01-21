
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
	"errors"
	"flag"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/rpc"
	colorable "github.com/mattn/go-colorable"
)

var (
	loglevel = flag.Int("loglevel", 2, "verbosity of logs")
)

func init() {
	flag.Parse()
	log.PrintOrigins(true)
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(true))))
}

//
func TestRun(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	t.Run("call", func(t *testing.T) {
		expect := "something"
		var got string
		r := sim.Run(context.Background(), func(ctx context.Context, sim *Simulation) error {
			got = expect
			return nil
		})

		if r.Error != nil {
			t.Errorf("unexpected error: %v", r.Error)
		}
		if got != expect {
			t.Errorf("expected %q, got %q", expect, got)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		r := sim.Run(ctx, func(ctx context.Context, sim *Simulation) error {
			time.Sleep(time.Second)
			return nil
		})

		if r.Error != context.DeadlineExceeded {
			t.Errorf("unexpected error: %v", r.Error)
		}
	})

	t.Run("context value and duration", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "hey", "there")
		sleep := 50 * time.Millisecond

		r := sim.Run(ctx, func(ctx context.Context, sim *Simulation) error {
			if ctx.Value("hey") != "there" {
				return errors.New("expected context value not passed")
			}
			time.Sleep(sleep)
			return nil
		})

		if r.Error != nil {
			t.Errorf("unexpected error: %v", r.Error)
		}
		if r.Duration < sleep {
			t.Errorf("reported run duration less then expected: %s", r.Duration)
		}
	})
}

//
func TestClose(t *testing.T) {
	var mu sync.Mutex
	var cleanupCount int

	sleep := 50 * time.Millisecond

	sim := New(map[string]ServiceFunc{
		"noop": func(ctx *adapters.ServiceContext, b *sync.Map) (node.Service, func(), error) {
			return newNoopService(), func() {
				time.Sleep(sleep)
				mu.Lock()
				defer mu.Unlock()
				cleanupCount++
			}, nil
		},
	})

	nodeCount := 30

	_, err := sim.AddNodes(nodeCount)
	if err != nil {
		t.Fatal(err)
	}

	var upNodeCount int
	for _, n := range sim.Net.GetNodes() {
		if n.Up {
			upNodeCount++
		}
	}
	if upNodeCount != nodeCount {
		t.Errorf("all nodes should be up, insted only %v are up", upNodeCount)
	}

	sim.Close()

	if cleanupCount != nodeCount {
		t.Errorf("number of cleanups expected %v, got %v", nodeCount, cleanupCount)
	}

	upNodeCount = 0
	for _, n := range sim.Net.GetNodes() {
		if n.Up {
			upNodeCount++
		}
	}
	if upNodeCount != 0 {
		t.Errorf("all nodes should be down, insted %v are up", upNodeCount)
	}
}

//
func TestDone(t *testing.T) {
	sim := New(noopServiceFuncMap)
	sleep := 50 * time.Millisecond
	timeout := 2 * time.Second

	start := time.Now()
	go func() {
		time.Sleep(sleep)
		sim.Close()
	}()

	select {
	case <-time.After(timeout):
		t.Error("done channel closing timed out")
	case <-sim.Done():
		if d := time.Since(start); d < sleep {
			t.Errorf("done channel closed sooner then expected: %s", d)
		}
	}
}

//
var noopServiceFuncMap = map[string]ServiceFunc{
	"noop": noopServiceFunc,
}

//
func noopServiceFunc(ctx *adapters.ServiceContext, b *sync.Map) (node.Service, func(), error) {
	return newNoopService(), nil, nil
}

//
//
type noopService struct{}

func newNoopService() node.Service {
	return &noopService{}
}

func (t *noopService) Protocols() []p2p.Protocol {
	return []p2p.Protocol{}
}

func (t *noopService) APIs() []rpc.API {
	return []rpc.API{}
}

func (t *noopService) Start(server *p2p.Server) error {
	return nil
}

func (t *noopService) Stop() error {
	return nil
}
