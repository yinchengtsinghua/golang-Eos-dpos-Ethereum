
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

//包发现实现了节点发现协议。
//
//节点发现协议提供了一种查找
//可以连接到。它使用一个类似kademlia的协议来维护
//所有监听的ID和端点的分布式数据库
//节点。
package discover

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/netutil"
)

const (
alpha           = 3  //Kademlia并发因子
bucketSize      = 16 //卡德米利亚水桶尺寸
maxReplacements = 10 //每个桶更换清单的尺寸

//我们把桶放在距离的1/15以上，因为
//我们不太可能遇到更近的节点。
	hashBits          = len(common.Hash{}) * 8
nBuckets          = hashBits / 15       //桶数
bucketMinDistance = hashBits - nBuckets //最近桶的对数距离

//IP地址限制。
bucketIPLimit, bucketSubnet = 2, 24 //最多2个地址来自同一个/24
	tableIPLimit, tableSubnet   = 10, 24

maxFindnodeFailures = 5 //将删除超过此限制的节点
	refreshInterval     = 30 * time.Minute
	revalidateInterval  = 10 * time.Second
	copyNodesInterval   = 30 * time.Second
	seedMinTableTime    = 5 * time.Minute
	seedCount           = 30
	seedMaxAge          = 5 * 24 * time.Hour
)

type Table struct {
mutex   sync.Mutex        //保护存储桶、存储桶内容、托儿所、兰特
buckets [nBuckets]*bucket //已知节点的距离索引
nursery []*Node           //引导节点
rand    *mrand.Rand       //随机性来源，定期重新播种
	ips     netutil.DistinctNetSet

db         *nodeDB //已知节点数据库
	refreshReq chan chan struct{}
	initDone   chan struct{}
	closeReq   chan struct{}
	closed     chan struct{}

nodeAddedHook func(*Node) //用于测试

	net  transport
self *Node //本地节点的元数据
}

//传输由UDP传输实现。
//它是一个接口，因此我们可以在不打开大量UDP的情况下进行测试
//不生成私钥的套接字。
type transport interface {
	ping(NodeID, *net.UDPAddr) error
	findnode(toid NodeID, addr *net.UDPAddr, target NodeID) ([]*Node, error)
	close()
}

//bucket包含按其上一个活动排序的节点。条目
//最近激活的元素是条目中的第一个元素。
type bucket struct {
entries      []*Node //实时条目，按上次联系时间排序
replacements []*Node //如果重新验证失败，则使用最近看到的节点
	ips          netutil.DistinctNetSet
}

func newTable(t transport, ourID NodeID, ourAddr *net.UDPAddr, nodeDBPath string, bootnodes []*Node) (*Table, error) {
//如果没有提供节点数据库，请使用内存中的数据库
	db, err := newNodeDB(nodeDBPath, nodeDBVersion, ourID)
	if err != nil {
		return nil, err
	}
	tab := &Table{
		net:        t,
		db:         db,
		self:       NewNode(ourID, ourAddr.IP, uint16(ourAddr.Port), uint16(ourAddr.Port)),
		refreshReq: make(chan chan struct{}),
		initDone:   make(chan struct{}),
		closeReq:   make(chan struct{}),
		closed:     make(chan struct{}),
		rand:       mrand.New(mrand.NewSource(0)),
		ips:        netutil.DistinctNetSet{Subnet: tableSubnet, Limit: tableIPLimit},
	}
	if err := tab.setFallbackNodes(bootnodes); err != nil {
		return nil, err
	}
	for i := range tab.buckets {
		tab.buckets[i] = &bucket{
			ips: netutil.DistinctNetSet{Subnet: bucketSubnet, Limit: bucketIPLimit},
		}
	}
	tab.seedRand()
	tab.loadSeedNodes()
//加载种子后启动后台过期goroutine，以便搜索
//种子节点还考虑将由
//到期。
	tab.db.ensureExpirer()
	go tab.loop()
	return tab, nil
}

func (tab *Table) seedRand() {
	var b [8]byte
	crand.Read(b[:])

	tab.mutex.Lock()
	tab.rand.Seed(int64(binary.BigEndian.Uint64(b[:])))
	tab.mutex.Unlock()
}

//self返回本地节点。
//调用方不应修改返回的节点。
func (tab *Table) Self() *Node {
	return tab.self
}

//readrandomnodes用来自
//表。它不会多次写入同一节点。节点
//切片是副本，可以由调用方修改。
func (tab *Table) ReadRandomNodes(buf []*Node) (n int) {
	if !tab.isInitDone() {
		return 0
	}
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

//找到所有非空桶，并从中获取新的部分。
	var buckets [][]*Node
	for _, b := range &tab.buckets {
		if len(b.entries) > 0 {
			buckets = append(buckets, b.entries[:])
		}
	}
	if len(buckets) == 0 {
		return 0
	}
//洗牌。
	for i := len(buckets) - 1; i > 0; i-- {
		j := tab.rand.Intn(len(buckets))
		buckets[i], buckets[j] = buckets[j], buckets[i]
	}
//将每个桶的头部移入buf，移除变空的桶。
	var i, j int
	for ; i < len(buf); i, j = i+1, (j+1)%len(buckets) {
		b := buckets[j]
		buf[i] = &(*b[0])
		buckets[j] = b[1:]
		if len(b) == 1 {
			buckets = append(buckets[:j], buckets[j+1:]...)
		}
		if len(buckets) == 0 {
			break
		}
	}
	return i + 1
}

//close终止网络侦听器并刷新节点数据库。
func (tab *Table) Close() {
	select {
	case <-tab.closed:
//已经关闭。
	case tab.closeReq <- struct{}{}:
<-tab.closed //等待RefreshLoop结束。
	}
}

//setFallbackNodes设置初始接触点。这些节点
//如果表为空，则用于连接到网络
//数据库中没有已知节点。
func (tab *Table) setFallbackNodes(nodes []*Node) error {
	for _, n := range nodes {
		if err := n.validateComplete(); err != nil {
			return fmt.Errorf("bad bootstrap/fallback node %q (%v)", n, err)
		}
	}
	tab.nursery = make([]*Node, 0, len(nodes))
	for _, n := range nodes {
		cpy := *n
//重新计算cpy.sha，因为节点可能没有
//由newnode或parsenode创建。
		cpy.sha = crypto.Keccak256Hash(n.ID[:])
		tab.nursery = append(tab.nursery, &cpy)
	}
	return nil
}

//IsInitDone返回表的初始种子设定过程是否已完成。
func (tab *Table) isInitDone() bool {
	select {
	case <-tab.initDone:
		return true
	default:
		return false
	}
}

//解析搜索具有给定ID的特定节点。
//如果找不到节点，则返回nil。
func (tab *Table) Resolve(targetID NodeID) *Node {
//如果节点存在于本地表中，则否
//需要网络交互。
	hash := crypto.Keccak256Hash(targetID[:])
	tab.mutex.Lock()
	cl := tab.closest(hash, 1)
	tab.mutex.Unlock()
	if len(cl.entries) > 0 && cl.entries[0].ID == targetID {
		return cl.entries[0]
	}
//否则，请执行网络查找。
	result := tab.Lookup(targetID)
	for _, n := range result {
		if n.ID == targetID {
			return n
		}
	}
	return nil
}

//查找对关闭的节点执行网络搜索
//目标。它通过查询接近目标
//在每次迭代中离它更近的节点。
//给定目标不需要是实际节点
//标识符。
func (tab *Table) Lookup(targetID NodeID) []*Node {
	return tab.lookup(targetID, true)
}

func (tab *Table) lookup(targetID NodeID, refreshIfEmpty bool) []*Node {
	var (
		target         = crypto.Keccak256Hash(targetID[:])
		asked          = make(map[NodeID]bool)
		seen           = make(map[NodeID]bool)
		reply          = make(chan []*Node, alpha)
		pendingQueries = 0
		result         *nodesByDistance
	)
//如果我们撞到自己，不要再问了。
//在实践中不太可能经常发生。
	asked[tab.self.ID] = true

	for {
		tab.mutex.Lock()
//生成初始结果集
		result = tab.closest(target, bucketSize)
		tab.mutex.Unlock()
		if len(result.entries) > 0 || !refreshIfEmpty {
			break
		}
//结果集为空，删除了所有节点，刷新。
//我们实际上在这里等待刷新完成。非常
//第一个查询将命中此情况并运行引导
//逻辑。
		<-tab.refresh()
		refreshIfEmpty = false
	}

	for {
//询问我们尚未询问的alpha最近的节点
		for i := 0; i < len(result.entries) && pendingQueries < alpha; i++ {
			n := result.entries[i]
			if !asked[n.ID] {
				asked[n.ID] = true
				pendingQueries++
				go tab.findnode(n, targetID, reply)
			}
		}
		if pendingQueries == 0 {
//我们要求所有最近的节点停止搜索
			break
		}
//等待下一个答复
		for _, n := range <-reply {
			if n != nil && !seen[n.ID] {
				seen[n.ID] = true
				result.push(n, bucketSize)
			}
		}
		pendingQueries--
	}
	return result.entries
}

func (tab *Table) findnode(n *Node, targetID NodeID, reply chan<- []*Node) {
	fails := tab.db.findFails(n.ID)
	r, err := tab.net.findnode(n.ID, n.addr(), targetID)
	if err != nil || len(r) == 0 {
		fails++
		tab.db.updateFindFails(n.ID, fails)
		log.Trace("Findnode failed", "id", n.ID, "failcount", fails, "err", err)
		if fails >= maxFindnodeFailures {
			log.Trace("Too many findnode failures, dropping", "id", n.ID, "failcount", fails)
			tab.delete(n)
		}
	} else if fails > 0 {
		tab.db.updateFindFails(n.ID, fails-1)
	}

//抓取尽可能多的节点。他们中的一些人可能已经不在了，但我们会
//在重新验证期间，只需再次移除这些。
	for _, n := range r {
		tab.add(n)
	}
	reply <- r
}

func (tab *Table) refresh() <-chan struct{} {
	done := make(chan struct{})
	select {
	case tab.refreshReq <- done:
	case <-tab.closed:
		close(done)
	}
	return done
}

//循环计划刷新、重新验证运行并协调关闭。
func (tab *Table) loop() {
	var (
		revalidate     = time.NewTimer(tab.nextRevalidateTime())
		refresh        = time.NewTicker(refreshInterval)
		copyNodes      = time.NewTicker(copyNodesInterval)
		revalidateDone = make(chan struct{})
refreshDone    = make(chan struct{})           //Dorefresh报告完成
waiting        = []chan struct{}{tab.initDone} //在DoRefresh运行时保留等待的呼叫者
	)
	defer refresh.Stop()
	defer revalidate.Stop()
	defer copyNodes.Stop()

//开始初始刷新。
	go tab.doRefresh(refreshDone)

loop:
	for {
		select {
		case <-refresh.C:
			tab.seedRand()
			if refreshDone == nil {
				refreshDone = make(chan struct{})
				go tab.doRefresh(refreshDone)
			}
		case req := <-tab.refreshReq:
			waiting = append(waiting, req)
			if refreshDone == nil {
				refreshDone = make(chan struct{})
				go tab.doRefresh(refreshDone)
			}
		case <-refreshDone:
			for _, ch := range waiting {
				close(ch)
			}
			waiting, refreshDone = nil, nil
		case <-revalidate.C:
			go tab.doRevalidate(revalidateDone)
		case <-revalidateDone:
			revalidate.Reset(tab.nextRevalidateTime())
		case <-copyNodes.C:
			go tab.copyLiveNodes()
		case <-tab.closeReq:
			break loop
		}
	}

	if tab.net != nil {
		tab.net.close()
	}
	if refreshDone != nil {
		<-refreshDone
	}
	for _, ch := range waiting {
		close(ch)
	}
	tab.db.close()
	close(tab.closed)
}

//DoRefresh执行查找随机目标以保留存储桶
//满的。如果表为空，则插入种子节点（初始
//引导或丢弃错误对等）。
func (tab *Table) doRefresh(done chan struct{}) {
	defer close(done)

//从数据库加载节点并插入
//他们。这将产生一些以前看到的节点，
//（希望）还活着。
	tab.loadSeedNodes()

//运行自我查找以发现新的邻居节点。
	tab.lookup(tab.self.ID, false)

//kademlia文件指定bucket刷新应该
//在最近使用最少的存储桶中执行查找。我们不能
//坚持这一点，因为findnode目标是512位值
//（不是哈希大小）并且不容易生成
//属于选定的桶中的sha3 preimage。
//我们用随机目标执行一些查找。
	for i := 0; i < 3; i++ {
		var target NodeID
		crand.Read(target[:])
		tab.lookup(target, false)
	}
}

func (tab *Table) loadSeedNodes() {
	seeds := tab.db.querySeeds(seedCount, seedMaxAge)
	seeds = append(seeds, tab.nursery...)
	for i := range seeds {
		seed := seeds[i]
		age := log.Lazy{Fn: func() interface{} { return time.Since(tab.db.lastPongReceived(seed.ID)) }}
		log.Debug("Found seed node in database", "id", seed.ID, "addr", seed.addr(), "age", age)
		tab.add(seed)
	}
}

//Dorevalidate检查随机存储桶中的最后一个节点是否仍然活动
//如果没有，则替换或删除节点。
func (tab *Table) doRevalidate(done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	last, bi := tab.nodeToRevalidate()
	if last == nil {
//找不到非空存储桶。
		return
	}

//ping所选节点并等待pong。
	err := tab.net.ping(last.ID, last.addr())

	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	b := tab.buckets[bi]
	if err == nil {
//节点响应，将其移到前面。
		log.Trace("Revalidated node", "b", bi, "id", last.ID)
		b.bump(last)
		return
	}
//未收到回复，请选择替换项或删除节点（如果没有）
//任何替代品。
	if r := tab.replace(b, last); r != nil {
		log.Trace("Replaced dead node", "b", bi, "id", last.ID, "ip", last.IP, "r", r.ID, "rip", r.IP)
	} else {
		log.Trace("Removed dead node", "b", bi, "id", last.ID, "ip", last.IP)
	}
}

//nodetorefalidate返回随机非空bucket中的最后一个节点。
func (tab *Table) nodeToRevalidate() (n *Node, bi int) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	for _, bi = range tab.rand.Perm(len(tab.buckets)) {
		b := tab.buckets[bi]
		if len(b.entries) > 0 {
			last := b.entries[len(b.entries)-1]
			return last, bi
		}
	}
	return nil, 0
}

func (tab *Table) nextRevalidateTime() time.Duration {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	return time.Duration(tab.rand.Int63n(int64(revalidateInterval)))
}

//CopyLiveNodes将表中的节点添加到数据库中（如果它们在表中）。
//比Mintable时间长。
func (tab *Table) copyLiveNodes() {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	now := time.Now()
	for _, b := range &tab.buckets {
		for _, n := range b.entries {
			if now.Sub(n.addedAt) >= seedMinTableTime {
				tab.db.updateNode(n)
			}
		}
	}
}

//最近返回表中最接近
//给定的ID。调用方必须保持tab.mutex。
func (tab *Table) closest(target common.Hash, nresults int) *nodesByDistance {
//这是一种非常浪费的查找最近节点的方法，但是
//显然是正确的。我相信以树为基础的桶可以
//这更容易有效地实施。
	close := &nodesByDistance{target: target}
	for _, b := range &tab.buckets {
		for _, n := range b.entries {
			close.push(n, nresults)
		}
	}
	return close
}

func (tab *Table) len() (n int) {
	for _, b := range &tab.buckets {
		n += len(b.entries)
	}
	return n
}

//bucket返回给定节点id散列的bucket。
func (tab *Table) bucket(sha common.Hash) *bucket {
	d := logdist(tab.self.sha, sha)
	if d <= bucketMinDistance {
		return tab.buckets[0]
	}
	return tab.buckets[d-bucketMinDistance-1]
}

//添加将给定节点添加到其相应存储桶的尝试。如果桶有空间
//可用，添加节点立即成功。否则，如果
//bucket中最近活动的节点不响应ping数据包。
//
//调用方不能持有tab.mutex。
func (tab *Table) add(n *Node) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	b := tab.bucket(n.sha)
	if !tab.bumpOrAdd(b, n) {
//节点不在表中。将其添加到替换列表中。
		tab.addReplacement(b, n)
	}
}

//addthroughping将给定节点添加到表中。与平原相比
//“添加”有一个附加的安全措施：如果表仍然存在
//未添加初始化节点。这可以防止攻击
//只需重复发送ping就可以填写表格。
//
//调用方不能持有tab.mutex。
func (tab *Table) addThroughPing(n *Node) {
	if !tab.isInitDone() {
		return
	}
	tab.add(n)
}

//stufacture将表中的节点添加到相应bucket的末尾
//如果桶没满。调用方不能持有tab.mutex。
func (tab *Table) stuff(nodes []*Node) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	for _, n := range nodes {
		if n.ID == tab.self.ID {
continue //不要增加自我
		}
		b := tab.bucket(n.sha)
		if len(b.entries) < bucketSize {
			tab.bumpOrAdd(b, n)
		}
	}
}

//删除从节点表中删除一个条目。用于疏散死节点。
func (tab *Table) delete(node *Node) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	tab.deleteInBucket(tab.bucket(node.sha), node)
}

func (tab *Table) addIP(b *bucket, ip net.IP) bool {
	if netutil.IsLAN(ip) {
		return true
	}
	if !tab.ips.Add(ip) {
		log.Debug("IP exceeds table limit", "ip", ip)
		return false
	}
	if !b.ips.Add(ip) {
		log.Debug("IP exceeds bucket limit", "ip", ip)
		tab.ips.Remove(ip)
		return false
	}
	return true
}

func (tab *Table) removeIP(b *bucket, ip net.IP) {
	if netutil.IsLAN(ip) {
		return
	}
	tab.ips.Remove(ip)
	b.ips.Remove(ip)
}

func (tab *Table) addReplacement(b *bucket, n *Node) {
	for _, e := range b.replacements {
		if e.ID == n.ID {
return //已列入清单
		}
	}
	if !tab.addIP(b, n.IP) {
		return
	}
	var removed *Node
	b.replacements, removed = pushNode(b.replacements, n, maxReplacements)
	if removed != nil {
		tab.removeIP(b, removed.IP)
	}
}

//replace从替换列表中删除n，如果“last”是
//桶中的最后一个条目。如果“last”不是最后一个条目，则它或已被替换
//和别人在一起或者变得活跃起来。
func (tab *Table) replace(b *bucket, last *Node) *Node {
	if len(b.entries) == 0 || b.entries[len(b.entries)-1].ID != last.ID {
//条目已移动，不要替换它。
		return nil
	}
//还是最后一个条目。
	if len(b.replacements) == 0 {
		tab.deleteInBucket(b, last)
		return nil
	}
	r := b.replacements[tab.rand.Intn(len(b.replacements))]
	b.replacements = deleteNode(b.replacements, r)
	b.entries[len(b.entries)-1] = r
	tab.removeIP(b, last.IP)
	return r
}

//bump将给定节点移动到bucket条目列表的前面
//如果它包含在那个列表中。
func (b *bucket) bump(n *Node) bool {
	for i := range b.entries {
		if b.entries[i].ID == n.ID {
//把它移到前面
			copy(b.entries[1:], b.entries[:i])
			b.entries[0] = n
			return true
		}
	}
	return false
}

//bumporadd将n移动到bucket条目列表的前面，或者如果该列表不在，则将其添加。
//满的。如果n在桶中，返回值为真。
func (tab *Table) bumpOrAdd(b *bucket, n *Node) bool {
	if b.bump(n) {
		return true
	}
	if len(b.entries) >= bucketSize || !tab.addIP(b, n.IP) {
		return false
	}
	b.entries, _ = pushNode(b.entries, n, bucketSize)
	b.replacements = deleteNode(b.replacements, n)
	n.addedAt = time.Now()
	if tab.nodeAddedHook != nil {
		tab.nodeAddedHook(n)
	}
	return true
}

func (tab *Table) deleteInBucket(b *bucket, n *Node) {
	b.entries = deleteNode(b.entries, n)
	tab.removeIP(b, n.IP)
}

//pushnode将n添加到列表的前面，最多保留max项。
func pushNode(list []*Node, n *Node, max int) ([]*Node, *Node) {
	if len(list) < max {
		list = append(list, nil)
	}
	removed := list[len(list)-1]
	copy(list[1:], list)
	list[0] = n
	return list, removed
}

//删除节点从列表中删除n。
func deleteNode(list []*Node, n *Node) []*Node {
	for i := range list {
		if list[i].ID == n.ID {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

//nodesByDistance是节点列表，按
//距离目标。
type nodesByDistance struct {
	entries []*Node
	target  common.Hash
}

//push将给定节点添加到列表中，使总大小保持在maxelems以下。
func (h *nodesByDistance) push(n *Node, maxElems int) {
	ix := sort.Search(len(h.entries), func(i int) bool {
		return distcmp(h.target, h.entries[i].sha, n.sha) > 0
	})
	if len(h.entries) < maxElems {
		h.entries = append(h.entries, n)
	}
	if ix == len(h.entries) {
//比我们现有的所有节点都要远。
//如果有空间，那么节点现在是最后一个元素。
	} else {
//向下滑动现有条目以腾出空间
//这将覆盖我们刚刚附加的条目。
		copy(h.entries[ix+1:], h.entries[ix:])
		h.entries[ix] = n
	}
}
