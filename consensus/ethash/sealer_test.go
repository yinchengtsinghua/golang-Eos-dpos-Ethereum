
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package ethash

import (
	"encoding/json"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

//测试是否正确通知远程HTTP服务器新工作。
func TestRemoteNotify(t *testing.T) {
//启动简单的Web服务器以捕获通知
	sink := make(chan [3]string)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			blob, err := ioutil.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read miner notification: %v", err)
			}
			var work [3]string
			if err := json.Unmarshal(blob, &work); err != nil {
				t.Fatalf("failed to unmarshal miner notification: %v", err)
			}
			sink <- work
		}),
	}
//打开自定义侦听器以提取其本地地址
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to open notification server: %v", err)
	}
	defer listener.Close()

	go server.Serve(listener)

//创建自定义ethash引擎
ethash := NewTester([]string{"http://“+listener.addr（）.string（））
	defer ethash.Close()

//流式处理工作任务并确保通知冒泡
	header := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(100)}
	block := types.NewBlockWithHeader(header)

	ethash.Seal(nil, block, nil)
	select {
	case work := <-sink:
		if want := header.HashNoNonce().Hex(); work[0] != want {
			t.Errorf("work packet hash mismatch: have %s, want %s", work[0], want)
		}
		if want := common.BytesToHash(SeedHash(header.Number.Uint64())).Hex(); work[1] != want {
			t.Errorf("work packet seed mismatch: have %s, want %s", work[1], want)
		}
		target := new(big.Int).Div(new(big.Int).Lsh(big.NewInt(1), 256), header.Difficulty)
		if want := common.BytesToHash(target.Bytes()).Hex(); work[2] != want {
			t.Errorf("work packet target mismatch: have %s, want %s", work[2], want)
		}
	case <-time.After(time.Second):
		t.Fatalf("notification timed out")
	}
}

//将工作包快速推送到矿工身上的测试不会导致任何DAA竞赛
//通知中的问题。
func TestRemoteMultiNotify(t *testing.T) {
//启动简单的Web服务器以捕获通知
	sink := make(chan [3]string, 64)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			blob, err := ioutil.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read miner notification: %v", err)
			}
			var work [3]string
			if err := json.Unmarshal(blob, &work); err != nil {
				t.Fatalf("failed to unmarshal miner notification: %v", err)
			}
			sink <- work
		}),
	}
//打开自定义侦听器以提取其本地地址
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to open notification server: %v", err)
	}
	defer listener.Close()

	go server.Serve(listener)

//创建自定义ethash引擎
ethash := NewTester([]string{"http://“+listener.addr（）.string（））
	defer ethash.Close()

//流式处理大量工作任务并确保所有通知都冒泡出来
	for i := 0; i < cap(sink); i++ {
		header := &types.Header{Number: big.NewInt(int64(i)), Difficulty: big.NewInt(100)}
		block := types.NewBlockWithHeader(header)

		ethash.Seal(nil, block, nil)
	}
	for i := 0; i < cap(sink); i++ {
		select {
		case <-sink:
		case <-time.After(250 * time.Millisecond):
			t.Fatalf("notification %d timed out", i)
		}
	}
}
