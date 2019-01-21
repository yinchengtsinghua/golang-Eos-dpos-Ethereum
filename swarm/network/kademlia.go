
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

package network

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/pot"
)

/*

根据相对于固定点X的接近顺序，将点分类为
将空间（n字节长的字节序列）放入容器中。每个项目位于
最远距离X的一半是前一个箱中的项目。给出了一个
均匀分布项（任意序列上的哈希函数）
接近比例映射到一系列子集上，基数为负
指数尺度。

它还具有属于同一存储箱的任何两个项目所在的属性。
最远的距离是彼此距离x的一半。

如果我们把垃圾箱中的随机样本看作是网络中的连接，
互联节点的相对邻近性可以作为局部
当任务要在两个路径之间查找路径时，用于图形遍历的决策
点。因为在每个跳跃中，有限距离的一半
达到一个跳数所需的保证不变的最大跳数限制。
节点。
**/


var pof = pot.DefaultPof(256)

//kadparams保存kademlia的配置参数
type KadParams struct {
//可调参数
MaxProxDisplay int   //表显示的行数
MinProxBinSize int   //最近邻核最小基数
MinBinSize     int   //一行中的最小对等数
MaxBinSize     int   //修剪前一行中的最大对等数
RetryInterval  int64 //对等机首次重新拨号前的初始间隔
RetryExponent  int   //用指数乘以重试间隔
MaxRetries     int   //重拨尝试的最大次数
//制裁或阻止建议同伴的职能
	Reachable func(OverlayAddr) bool
}

//newkadparams返回带有默认值的params结构
func NewKadParams() *KadParams {
	return &KadParams{
		MaxProxDisplay: 16,
		MinProxBinSize: 2,
		MinBinSize:     2,
		MaxBinSize:     4,
RetryInterval:  4200000000, //4.2秒
		MaxRetries:     42,
		RetryExponent:  2,
	}
}

//Kademlia是一个活动对等端表和一个已知对等端数据库（节点记录）
type Kademlia struct {
	lock       sync.RWMutex
*KadParams          //Kademlia配置参数
base       []byte   //表的不可变基址
addrs      *pot.Pot //用于已知对等地址的POTS容器
conns      *pot.Pot //用于实时对等连接的POTS容器
depth      uint8    //存储上一个当前饱和深度
nDepth     int      //存储上一个邻居深度
nDepthC    chan int //由depthc函数返回，用于信号邻域深度变化
addrCountC chan int //由addrcountc函数返回以指示对等计数更改
}

//newkademlia为基地址addr创建一个kademlia表
//参数与参数相同
//如果params为nil，则使用默认值
func NewKademlia(addr []byte, params *KadParams) *Kademlia {
	if params == nil {
		params = NewKadParams()
	}
	return &Kademlia{
		base:      addr,
		KadParams: params,
		addrs:     pot.NewPot(nil, 0),
		conns:     pot.NewPot(nil, 0),
	}
}

//覆盖对等接口从覆盖中捕获对等视图的公共方面
//拓扑驱动程序
type OverlayPeer interface {
	Address() []byte
}

//overlayconn表示连接的对等机
type OverlayConn interface {
	OverlayPeer
Drop(error)       //调用以指示应删除对等项
Off() OverlayAddr //调用以返回Persistant OverlayAddr
}

//overlayAddr表示Kademlia对等记录
type OverlayAddr interface {
	OverlayPeer
Update(OverlayAddr) OverlayAddr //返回原始版本的更新版本
}

//条目表示一个kademlia表条目（overlaypeer的扩展）
type entry struct {
	OverlayPeer
	seenAt  time.Time
	retries int
}

//NewEntry从重叠对等接口创建Kademlia对等
func newEntry(p OverlayPeer) *entry {
	return &entry{
		OverlayPeer: p,
		seenAt:      time.Now(),
	}
}

//bin是入口地址的二进制（位向量）序列化
func (e *entry) Bin() string {
	return pot.ToBin(e.addr().Address())
}

//label是调试条目的短标记
func Label(e *entry) string {
	return fmt.Sprintf("%s (%d)", e.Hex()[:4], e.retries)
}

//十六进制是入口地址的十六进制序列化
func (e *entry) Hex() string {
	return fmt.Sprintf("%x", e.addr().Address())
}

//字符串是条目的短标记
func (e *entry) String() string {
	return fmt.Sprintf("%s (%d)", e.Hex()[:8], e.retries)
}

//addr返回条目对应的kad对等记录（overlayaddr）
func (e *entry) addr() OverlayAddr {
	a, _ := e.OverlayPeer.(OverlayAddr)
	return a
}

//conn返回与条目相对应的已连接对等（overlaypeer）
func (e *entry) conn() OverlayConn {
	c, _ := e.OverlayPeer.(OverlayConn)
	return c
}

//寄存器将每个overlayaddr作为kademlia对等记录输入
//已知对等地址数据库
func (k *Kademlia) Register(peers []OverlayAddr) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	var known, size int
	for _, p := range peers {
//如果自我接收错误，对等方应该知道得更好。
//应该为此受到惩罚
		if bytes.Equal(p.Address(), k.base) {
			return fmt.Errorf("add peers: %x is self", k.base)
		}
		var found bool
		k.addrs, _, found, _ = pot.Swap(k.addrs, p, pof, func(v pot.Val) pot.Val {
//如果没有找到
			if v == nil {
//在conn中插入新的脱机对等
				return newEntry(p)
			}
//在已知的同龄人中发现，什么都不做
			return v
		})
		if found {
			known++
		}
		size++
	}
//仅当有新地址时才发送新地址计数值
	if k.addrCountC != nil && size-known > 0 {
		k.addrCountC <- k.addrs.Size()
	}
//log.trace（fmt.sprintf（“%x注册了%v个对等点，已知的%v个，总数：%v”，k.baseaddr（）[：4]，大小，已知的，k.addrs.size（）））

	k.sendNeighbourhoodDepthChange()
	return nil
}

//suggestpeer返回的最低接近箱的已知对等
//深度以下的最低料位计数
//当然，如果有一个空行，它将返回该行的对等方
func (k *Kademlia) SuggestPeer() (a OverlayAddr, o int, want bool) {
	k.lock.Lock()
	defer k.lock.Unlock()
	minsize := k.MinBinSize
	depth := k.neighbourhoodDepth()
//如果当前proxbin中存在可调用邻居，请连接
//这将确保最近的邻居集完全连接
	var ppo int
	k.addrs.EachNeighbour(k.base, pof, func(val pot.Val, po int) bool {
		if po < depth {
			return false
		}
		a = k.callable(val)
		ppo = po
		return a == nil
	})
	if a != nil {
		log.Trace(fmt.Sprintf("%08x candidate nearest neighbour found: %v (%v)", k.BaseAddr()[:4], a, ppo))
		return a, 0, false
	}
//log.trace（fmt.sprintf（“%08X没有要连接到的最近邻居（深度：%v，minproxySize:%v）%v”，k.baseAddr（）[：4]，深度，k.minproxyBinSize，a））。

	var bpo []int
	prev := -1
	k.conns.EachBin(k.base, pof, 0, func(po, size int, f func(func(val pot.Val, i int) bool) bool) bool {
		prev++
		for ; prev < po; prev++ {
			bpo = append(bpo, prev)
			minsize = 0
		}
		if size < minsize {
			bpo = append(bpo, po)
			minsize = size
		}
		return size > 0 && po < depth
	})
//所有桶都满了，即minSize==k.minBinSize
	if len(bpo) == 0 {
//log.debug（fmt.sprintf（“%08X:所有箱已饱和”，k.baseaddr（）[：4]））
		return nil, 0, false
	}
//只要我们有候选者可以联系到
//不要要求新同事（want=false）
//尝试选择一个候选对等
//找到第一个可调用的对等
	nxt := bpo[0]
	k.addrs.EachBin(k.base, pof, nxt, func(po, _ int, f func(func(pot.Val, int) bool) bool) bool {
//对于每个容器（直到深度），我们找到可调用的候选对等体
		if po >= depth {
			return false
		}
		return f(func(val pot.Val, _ int) bool {
			a = k.callable(val)
			return a == nil
		})
	})
//找到一个候选人
	if a != nil {
		return a, 0, false
	}
//找不到候选对等，请求短箱
	var changed bool
	if uint8(nxt) < k.depth {
		k.depth = uint8(nxt)
		changed = true
	}
	return a, nxt, changed
}

//在上，将对等机作为Kademlia对等机插入活动对等机
func (k *Kademlia) On(p OverlayConn) (uint8, bool) {
	k.lock.Lock()
	defer k.lock.Unlock()
	e := newEntry(p)
	var ins bool
	k.conns, _, _, _ = pot.Swap(k.conns, p, pof, func(v pot.Val) pot.Val {
//如果找不到现场
		if v == nil {
			ins = true
//在conns中插入新的在线对等点
			return e
		}
//在同龄人中找到，什么都不做
		return v
	})
	if ins {
//在加法器中插入新的在线对等点
		k.addrs, _, _, _ = pot.Swap(k.addrs, p, pof, func(v pot.Val) pot.Val {
			return e
		})
//仅当插入对等端时才发送新的地址计数值
		if k.addrCountC != nil {
			k.addrCountC <- k.addrs.Size()
		}
	}
	log.Trace(k.string())
//计算饱和深度是否改变
	depth := uint8(k.saturation(k.MinBinSize))
	var changed bool
	if depth != k.depth {
		changed = true
		k.depth = depth
	}
	k.sendNeighbourhoodDepthChange()
	return k.depth, changed
}

//Neighbourhooddepthc返回发送新Kademlia的频道
//每一次变化的邻里深度。
//不从返回通道接收将阻塞功能
//当邻近深度改变时。
func (k *Kademlia) NeighbourhoodDepthC() <-chan int {
	if k.nDepthC == nil {
		k.nDepthC = make(chan int)
	}
	return k.nDepthC
}

//sendnieghbourhooddepthchange向k.ndepth通道发送新的邻近深度
//如果已初始化。
func (k *Kademlia) sendNeighbourhoodDepthChange() {
//当调用neighbourhooddepthc并由其返回时，将初始化ndepthc。
//它提供了邻域深度变化的信号。
//如果满足这个条件，代码的这一部分将向ndepthc发送新的邻域深度。
	if k.nDepthC != nil {
		nDepth := k.neighbourhoodDepth()
		if nDepth != k.nDepth {
			k.nDepth = nDepth
			k.nDepthC <- nDepth
		}
	}
}

//addrCountc返回发送新的
//每次更改的地址计数值。
//不从返回通道接收将阻止寄存器功能
//地址计数值更改时。
func (k *Kademlia) AddrCountC() <-chan int {
	if k.addrCountC == nil {
		k.addrCountC = make(chan int)
	}
	return k.addrCountC
}

//关闭从活动对等中删除对等
func (k *Kademlia) Off(p OverlayConn) {
	k.lock.Lock()
	defer k.lock.Unlock()
	var del bool
	k.addrs, _, _, _ = pot.Swap(k.addrs, p, pof, func(v pot.Val) pot.Val {
//v不能为零，必须选中，否则我们将覆盖条目
		if v == nil {
			panic(fmt.Sprintf("connected peer not found %v", p))
		}
		del = true
		return newEntry(p.Off())
	})

	if del {
		k.conns, _, _, _ = pot.Swap(k.conns, p, pof, func(_ pot.Val) pot.Val {
//V不能为零，但不需要检查
			return nil
		})
//仅当对等端被删除时才发送新的地址计数值
		if k.addrCountC != nil {
			k.addrCountC <- k.addrs.Size()
		}
		k.sendNeighbourhoodDepthChange()
	}
}

func (k *Kademlia) EachBin(base []byte, pof pot.Pof, o int, eachBinFunc func(conn OverlayConn, po int) bool) {
	k.lock.RLock()
	defer k.lock.RUnlock()

	var startPo int
	var endPo int
	kadDepth := k.neighbourhoodDepth()

	k.conns.EachBin(base, pof, o, func(po, size int, f func(func(val pot.Val, i int) bool) bool) bool {
		if startPo > 0 && endPo != k.MaxProxDisplay {
			startPo = endPo + 1
		}
		if po < kadDepth {
			endPo = po
		} else {
			endPo = k.MaxProxDisplay
		}

		for bin := startPo; bin <= endPo; bin++ {
			f(func(val pot.Val, _ int) bool {
				return eachBinFunc(val.(*entry).conn(), bin)
			})
		}
		return true
	})
}

//eachconn是一个带有args（base、po、f）的迭代器，将f应用于每个活动对等端
//从基地测量，接近订单为po或更低
//如果基为零，则使用Kademlia基地址
func (k *Kademlia) EachConn(base []byte, o int, f func(OverlayConn, int, bool) bool) {
	k.lock.RLock()
	defer k.lock.RUnlock()
	k.eachConn(base, o, f)
}

func (k *Kademlia) eachConn(base []byte, o int, f func(OverlayConn, int, bool) bool) {
	if len(base) == 0 {
		base = k.base
	}
	depth := k.neighbourhoodDepth()
	k.conns.EachNeighbour(base, pof, func(val pot.Val, po int) bool {
		if po > o {
			return true
		}
		return f(val.(*entry).conn(), po, po >= depth)
	})
}

//用（base，po，f）调用的eachaddr是一个迭代器，将f应用于每个已知的对等端
//从基地测量，接近订单为po或更低
//如果基为零，则使用Kademlia基地址
func (k *Kademlia) EachAddr(base []byte, o int, f func(OverlayAddr, int, bool) bool) {
	k.lock.RLock()
	defer k.lock.RUnlock()
	k.eachAddr(base, o, f)
}

func (k *Kademlia) eachAddr(base []byte, o int, f func(OverlayAddr, int, bool) bool) {
	if len(base) == 0 {
		base = k.base
	}
	depth := k.neighbourhoodDepth()
	k.addrs.EachNeighbour(base, pof, func(val pot.Val, po int) bool {
		if po > o {
			return true
		}
		return f(val.(*entry).addr(), po, po >= depth)
	})
}

//neighbourhooddepth返回定义
//基数大于等于minProxBinSize的最近邻集
//如果总共小于minproxbinsize对等端，则返回0
//呼叫方必须持有锁
func (k *Kademlia) neighbourhoodDepth() (depth int) {
	if k.conns.Size() < k.MinProxBinSize {
		return 0
	}
	var size int
	f := func(v pot.Val, i int) bool {
		size++
		depth = i
		return size < k.MinProxBinSize
	}
	k.conns.EachNeighbour(k.base, pof, f)
	return depth
}

//使用val调用时可调用，
func (k *Kademlia) callable(val pot.Val) OverlayAddr {
	e := val.(*entry)
//如果对等方处于活动状态或超过了maxretries，则不可调用
	if e.conn() != nil || e.retries > k.MaxRetries {
		return nil
	}
//根据上次看到后经过的时间计算允许的重试次数
	timeAgo := int64(time.Since(e.seenAt))
	div := int64(k.RetryExponent)
	div += (150000 - rand.Int63n(300000)) * div / 1000000
	var retries int
	for delta := timeAgo; delta > k.RetryInterval; delta /= div {
		retries++
	}
//它从不并发调用，因此可以安全地递增
//可以再次重试对等机
	if retries < e.retries {
		log.Trace(fmt.Sprintf("%08x: %v long time since last try (at %v) needed before retry %v, wait only warrants %v", k.BaseAddr()[:4], e, timeAgo, e.retries, retries))
		return nil
	}
//制裁或阻止建议同伴的职能
	if k.Reachable != nil && !k.Reachable(e.addr()) {
		log.Trace(fmt.Sprintf("%08x: peer %v is temporarily not callable", k.BaseAddr()[:4], e))
		return nil
	}
	e.retries++
	log.Trace(fmt.Sprintf("%08x: peer %v is callable", k.BaseAddr()[:4], e))

	return e.addr()
}

//baseaddr返回kademlia基地址
func (k *Kademlia) BaseAddr() []byte {
	return k.base
}

//字符串返回用ASCII显示的kademlia表+kaddb表
func (k *Kademlia) String() string {
	k.lock.RLock()
	defer k.lock.RUnlock()
	return k.string()
}

//字符串返回用ASCII显示的kademlia表+kaddb表
func (k *Kademlia) string() string {
	wsrow := "                          "
	var rows []string

	rows = append(rows, "=========================================================================")
	rows = append(rows, fmt.Sprintf("%v KΛÐΞMLIΛ hive: queen's address: %x", time.Now().UTC().Format(time.UnixDate), k.BaseAddr()[:3]))
	rows = append(rows, fmt.Sprintf("population: %d (%d), MinProxBinSize: %d, MinBinSize: %d, MaxBinSize: %d", k.conns.Size(), k.addrs.Size(), k.MinProxBinSize, k.MinBinSize, k.MaxBinSize))

	liverows := make([]string, k.MaxProxDisplay)
	peersrows := make([]string, k.MaxProxDisplay)

	depth := k.neighbourhoodDepth()
	rest := k.conns.Size()
	k.conns.EachBin(k.base, pof, 0, func(po, size int, f func(func(val pot.Val, i int) bool) bool) bool {
		var rowlen int
		if po >= k.MaxProxDisplay {
			po = k.MaxProxDisplay - 1
		}
		row := []string{fmt.Sprintf("%2d", size)}
		rest -= size
		f(func(val pot.Val, vpo int) bool {
			e := val.(*entry)
			row = append(row, fmt.Sprintf("%x", e.Address()[:2]))
			rowlen++
			return rowlen < 4
		})
		r := strings.Join(row, " ")
		r = r + wsrow
		liverows[po] = r[:31]
		return true
	})

	k.addrs.EachBin(k.base, pof, 0, func(po, size int, f func(func(val pot.Val, i int) bool) bool) bool {
		var rowlen int
		if po >= k.MaxProxDisplay {
			po = k.MaxProxDisplay - 1
		}
		if size < 0 {
			panic("wtf")
		}
		row := []string{fmt.Sprintf("%2d", size)}
//我们也在现场展示同龄人
		f(func(val pot.Val, vpo int) bool {
			e := val.(*entry)
			row = append(row, Label(e))
			rowlen++
			return rowlen < 4
		})
		peersrows[po] = strings.Join(row, " ")
		return true
	})

	for i := 0; i < k.MaxProxDisplay; i++ {
		if i == depth {
			rows = append(rows, fmt.Sprintf("============ DEPTH: %d ==========================================", i))
		}
		left := liverows[i]
		right := peersrows[i]
		if len(left) == 0 {
			left = " 0                             "
		}
		if len(right) == 0 {
			right = " 0"
		}
		rows = append(rows, fmt.Sprintf("%03d %v | %v", i, left, right))
	}
	rows = append(rows, "=========================================================================")
	return "\n" + strings.Join(rows, "\n")
}

//Peerpot保存预期最近邻居和空箱子的信息
//仅用于测试
type PeerPot struct {
	NNSet     [][]byte
	EmptyBins []int
}

//newpeerpotmap用键创建overlayaddr的pot记录的映射
//作为地址的十六进制表示。
func NewPeerPotMap(kadMinProxSize int, addrs [][]byte) map[string]*PeerPot {
//为运行状况检查创建所有节点的表
	np := pot.NewPot(nil, 0)
	for _, addr := range addrs {
		np, _, _ = pot.Add(np, addr, pof)
	}
	ppmap := make(map[string]*PeerPot)

	for i, a := range addrs {
		pl := 256
		prev := 256
		var emptyBins []int
		var nns [][]byte
		np.EachNeighbour(addrs[i], pof, func(val pot.Val, po int) bool {
			a := val.([]byte)
			if po == 256 {
				return true
			}
			if pl == 256 || pl == po {
				nns = append(nns, a)
			}
			if pl == 256 && len(nns) >= kadMinProxSize {
				pl = po
				prev = po
			}
			if prev < pl {
				for j := prev; j > po; j-- {
					emptyBins = append(emptyBins, j)
				}
			}
			prev = po - 1
			return true
		})
		for j := prev; j >= 0; j-- {
			emptyBins = append(emptyBins, j)
		}
		log.Trace(fmt.Sprintf("%x NNS: %s", addrs[i][:4], LogAddrs(nns)))
		ppmap[common.Bytes2Hex(a)] = &PeerPot{nns, emptyBins}
	}
	return ppmap
}

//饱和返回该订单的箱的最低接近顺序
//具有少于n个对等点
func (k *Kademlia) saturation(n int) int {
	prev := -1
	k.addrs.EachBin(k.base, pof, 0, func(po, size int, f func(func(val pot.Val, i int) bool) bool) bool {
		prev++
		return prev == po && size >= n
	})
	depth := k.neighbourhoodDepth()
	if depth < prev {
		return depth
	}
	return prev
}

//如果所有必需的容器都连接了对等方，则full返回true。
//用于健康功能。
func (k *Kademlia) full(emptyBins []int) (full bool) {
	prev := 0
	e := len(emptyBins)
	ok := true
	depth := k.neighbourhoodDepth()
	k.conns.EachBin(k.base, pof, 0, func(po, _ int, _ func(func(val pot.Val, i int) bool) bool) bool {
		if prev == depth+1 {
			return true
		}
		for i := prev; i < po; i++ {
			e--
			if e < 0 {
				ok = false
				return false
			}
			if emptyBins[e] != i {
				log.Trace(fmt.Sprintf("%08x po: %d, i: %d, e: %d, emptybins: %v", k.BaseAddr()[:4], po, i, e, logEmptyBins(emptyBins)))
				if emptyBins[e] < i {
					panic("incorrect peerpot")
				}
				ok = false
				return false
			}
		}
		prev = po + 1
		return true
	})
	if !ok {
		return false
	}
	return e == 0
}

func (k *Kademlia) knowNearestNeighbours(peers [][]byte) bool {
	pm := make(map[string]bool)

	k.eachAddr(nil, 255, func(p OverlayAddr, po int, nn bool) bool {
		if !nn {
			return false
		}
		pk := fmt.Sprintf("%x", p.Address())
		pm[pk] = true
		return true
	})
	for _, p := range peers {
		pk := fmt.Sprintf("%x", p)
		if !pm[pk] {
			log.Trace(fmt.Sprintf("%08x: known nearest neighbour %s not found", k.BaseAddr()[:4], pk[:8]))
			return false
		}
	}
	return true
}

func (k *Kademlia) gotNearestNeighbours(peers [][]byte) (got bool, n int, missing [][]byte) {
	pm := make(map[string]bool)

	k.eachConn(nil, 255, func(p OverlayConn, po int, nn bool) bool {
		if !nn {
			return false
		}
		pk := fmt.Sprintf("%x", p.Address())
		pm[pk] = true
		return true
	})
	var gots int
	var culprits [][]byte
	for _, p := range peers {
		pk := fmt.Sprintf("%x", p)
		if pm[pk] {
			gots++
		} else {
			log.Trace(fmt.Sprintf("%08x: ExpNN: %s not found", k.BaseAddr()[:4], pk[:8]))
			culprits = append(culprits, p)
		}
	}
	return gots == len(peers), gots, culprits
}

//卡德米利亚的健康状况
type Health struct {
KnowNN     bool     //节点是否知道所有最近的邻居
GotNN      bool     //节点是否连接到其所有最近的邻居
CountNN    int      //连接到的最近邻居的数量
CulpritsNN [][]byte //哪些已知的nns丢失了
Full       bool     //节点在每个kademlia bin中是否有一个对等点（如果有这样的对等点）
	Hive       string
}

//健康报告Kademlia连接性的健康状态
//返回健康结构
func (k *Kademlia) Healthy(pp *PeerPot) *Health {
	k.lock.RLock()
	defer k.lock.RUnlock()
	gotnn, countnn, culpritsnn := k.gotNearestNeighbours(pp.NNSet)
	knownn := k.knowNearestNeighbours(pp.NNSet)
	full := k.full(pp.EmptyBins)
	log.Trace(fmt.Sprintf("%08x: healthy: knowNNs: %v, gotNNs: %v, full: %v\n", k.BaseAddr()[:4], knownn, gotnn, full))
	return &Health{knownn, gotnn, countnn, culpritsnn, full, k.string()}
}

func logEmptyBins(ebs []int) string {
	var ebss []string
	for _, eb := range ebs {
		ebss = append(ebss, fmt.Sprintf("%d", eb))
	}
	return strings.Join(ebss, ", ")
}
