
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

//包含网络层使用的仪表和计时器。

package p2p

import (
	"net"

	"github.com/ethereum/go-ethereum/metrics"
)

var (
	ingressConnectMeter = metrics.NewRegisteredMeter("p2p/InboundConnects", nil)
	ingressTrafficMeter = metrics.NewRegisteredMeter("p2p/InboundTraffic", nil)
	egressConnectMeter  = metrics.NewRegisteredMeter("p2p/OutboundConnects", nil)
	egressTrafficMeter  = metrics.NewRegisteredMeter("p2p/OutboundTraffic", nil)
)

//meteredconn是一个围绕net.conn的包装器，用于测量
//入站和出站网络流量。
type meteredConn struct {
net.Conn //与计量包装的网络连接
}

//NewMeteredConn创建了一个新的Metered连接，也会影响入口或
//出口连接仪表。如果禁用度量系统，则此函数
//返回原始对象。
func newMeteredConn(conn net.Conn, ingress bool) net.Conn {
//如果禁用度量值，则短路
	if !metrics.Enabled {
		return conn
	}
//否则，请触发连接计数器并包装连接
	if ingress {
		ingressConnectMeter.Mark(1)
	} else {
		egressConnectMeter.Mark(1)
	}
	return &meteredConn{Conn: conn}
}

//读取将网络读取委派给基础连接，从而阻止进入
//一路上的交通表。
func (c *meteredConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	ingressTrafficMeter.Mark(int64(n))
	return
}

//写入将网络写入委托给基础连接，将
//一路上的出口流量表。
func (c *meteredConn) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	egressTrafficMeter.Mark(int64(n))
	return
}
