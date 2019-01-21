
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package adapters

import (
	"bufio"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
)

//wsaddrpattern是一个regex，用于从节点的
//日志
var wsAddrPattern = regexp.MustCompile(`ws://[\D::] +）

func matchWSAddr(str string) (string, bool) {
	if !strings.Contains(str, "WebSocket endpoint opened") {
		return "", false
	}

	return wsAddrPattern.FindString(str), true
}

//findwsaddr通过读卡器r扫描，查找
//WebSocket地址信息。
func findWSAddr(r io.Reader, timeout time.Duration) (string, error) {
	ch := make(chan string)

	go func() {
		s := bufio.NewScanner(r)
		for s.Scan() {
			addr, ok := matchWSAddr(s.Text())
			if ok {
				ch <- addr
			}
		}
		close(ch)
	}()

	var wsAddr string
	select {
	case wsAddr = <-ch:
		if wsAddr == "" {
			return "", errors.New("empty result")
		}
	case <-time.After(timeout):
		return "", errors.New("timed out")
	}

	return wsAddr, nil
}
