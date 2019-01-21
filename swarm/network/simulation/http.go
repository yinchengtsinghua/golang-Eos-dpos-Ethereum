
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
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/simulations"
)

//
var (
	DefaultHTTPSimAddr = ":8888"
)

//
//
func (s *Simulation) WithServer(addr string) *Simulation {
//
	if addr == "" {
		addr = DefaultHTTPSimAddr
	}
	log.Info(fmt.Sprintf("Initializing simulation server on %s...", addr))
//
	s.handler = simulations.NewServer(s.Net)
	s.runC = make(chan struct{})
//
	s.addSimulationRoutes()
	s.httpSrv = &http.Server{
		Addr:    addr,
		Handler: s.handler,
	}
	go func() {
		err := s.httpSrv.ListenAndServe()
		if err != nil {
			log.Error("Error starting the HTTP server", "error", err)
		}
	}()
	return s
}

//
func (s *Simulation) addSimulationRoutes() {
	s.handler.POST("/runsim", s.RunSimulation)
}

//
func (s *Simulation) RunSimulation(w http.ResponseWriter, req *http.Request) {
	log.Debug("RunSimulation endpoint running")
	s.runC <- struct{}{}
	w.WriteHeader(http.StatusOK)
}
