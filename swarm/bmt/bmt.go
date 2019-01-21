
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

//
package bmt

import (
	"fmt"
	"hash"
	"strings"
	"sync"
	"sync/atomic"
)

/*

















  

  

  
 
 
 
 
*/


const (
//
//
	PoolSize = 8
)

//
//
type BaseHasherFunc func() hash.Hash

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
type Hasher struct {
pool *TreePool //
bmt  *tree     //
}

//
//
func New(p *TreePool) *Hasher {
	return &Hasher{
		pool: p,
	}
}

//
//
//
type TreePool struct {
	lock         sync.Mutex
c            chan *tree     //
hasher       BaseHasherFunc //
SegmentSize  int            //
SegmentCount int            //
Capacity     int            //
Depth        int            //
Size         int            //
count        int            //
zerohashes   [][]byte       //
}

//
//
func NewTreePool(hasher BaseHasherFunc, segmentCount, capacity int) *TreePool {
//
	depth := calculateDepthFor(segmentCount)
	segmentSize := hasher().Size()
	zerohashes := make([][]byte, depth+1)
	zeros := make([]byte, segmentSize)
	zerohashes[0] = zeros
	h := hasher()
	for i := 1; i < depth+1; i++ {
		zeros = doSum(h, nil, zeros, zeros)
		zerohashes[i] = zeros
	}
	return &TreePool{
		c:            make(chan *tree, capacity),
		hasher:       hasher,
		SegmentSize:  segmentSize,
		SegmentCount: segmentCount,
		Capacity:     capacity,
		Size:         segmentCount * segmentSize,
		Depth:        depth,
		zerohashes:   zerohashes,
	}
}

//
func (p *TreePool) Drain(n int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	for len(p.c) > n {
		<-p.c
		p.count--
	}
}

//
//
//
func (p *TreePool) reserve() *tree {
	p.lock.Lock()
	defer p.lock.Unlock()
	var t *tree
	if p.count == p.Capacity {
		return <-p.c
	}
	select {
	case t = <-p.c:
	default:
		t = newTree(p.SegmentSize, p.Depth, p.hasher)
		p.count++
	}
	return t
}

//
//
func (p *TreePool) release(t *tree) {
p.c <- t //
}

//
//
//
//
type tree struct {
leaves  []*node     //
cursor  int         //
offset  int         //
section []byte      //
result  chan []byte //
span    []byte      //
}

//
type node struct {
isLeft      bool      //
parent      *node     //
state       int32     //
left, right []byte    //
hasher      hash.Hash //
}

//
func newNode(index int, parent *node, hasher hash.Hash) *node {
	return &node{
		parent: parent,
		isLeft: index%2 == 0,
		hasher: hasher,
	}
}

//
func (t *tree) draw(hash []byte) string {
	var left, right []string
	var anc []*node
	for i, n := range t.leaves {
		left = append(left, fmt.Sprintf("%v", hashstr(n.left)))
		if i%2 == 0 {
			anc = append(anc, n.parent)
		}
		right = append(right, fmt.Sprintf("%v", hashstr(n.right)))
	}
	anc = t.leaves
	var hashes [][]string
	for l := 0; len(anc) > 0; l++ {
		var nodes []*node
		hash := []string{""}
		for i, n := range anc {
			hash = append(hash, fmt.Sprintf("%v|%v", hashstr(n.left), hashstr(n.right)))
			if i%2 == 0 && n.parent != nil {
				nodes = append(nodes, n.parent)
			}
		}
		hash = append(hash, "")
		hashes = append(hashes, hash)
		anc = nodes
	}
	hashes = append(hashes, []string{"", fmt.Sprintf("%v", hashstr(hash)), ""})
	total := 60
	del := "                             "
	var rows []string
	for i := len(hashes) - 1; i >= 0; i-- {
		var textlen int
		hash := hashes[i]
		for _, s := range hash {
			textlen += len(s)
		}
		if total < textlen {
			total = textlen + len(hash)
		}
		delsize := (total - textlen) / (len(hash) - 1)
		if delsize > len(del) {
			delsize = len(del)
		}
		row := fmt.Sprintf("%v: %v", len(hashes)-i-1, strings.Join(hash, del[:delsize]))
		rows = append(rows, row)

	}
	rows = append(rows, strings.Join(left, "  "))
	rows = append(rows, strings.Join(right, "  "))
	return strings.Join(rows, "\n") + "\n"
}

//
//
func newTree(segmentSize, depth int, hashfunc func() hash.Hash) *tree {
	n := newNode(0, nil, hashfunc())
	prevlevel := []*node{n}
//
//
	count := 2
	for level := depth - 2; level >= 0; level-- {
		nodes := make([]*node, count)
		for i := 0; i < count; i++ {
			parent := prevlevel[i/2]
			var hasher hash.Hash
			if level == 0 {
				hasher = hashfunc()
			}
			nodes[i] = newNode(i, parent, hasher)
		}
		prevlevel = nodes
		count *= 2
	}
//
	return &tree{
		leaves:  prevlevel,
		result:  make(chan []byte),
		section: make([]byte, 2*segmentSize),
	}
}

//

//
func (h *Hasher) Size() int {
	return h.pool.SegmentSize
}

//
func (h *Hasher) BlockSize() int {
	return 2 * h.pool.SegmentSize
}

//
//
//
//
//
func (h *Hasher) Sum(b []byte) (s []byte) {
	t := h.getTree()
//
	go h.writeSection(t.cursor, t.section, true, true)
//
	s = <-t.result
	span := t.span
//
	h.releaseTree()
//
	if len(span) == 0 {
		return append(b, s...)
	}
	return doSum(h.pool.hasher(), b, span, s)
}

//

//
//
func (h *Hasher) Write(b []byte) (int, error) {
	l := len(b)
	if l == 0 || l > h.pool.Size {
		return 0, nil
	}
	t := h.getTree()
	secsize := 2 * h.pool.SegmentSize
//
	smax := secsize - t.offset
//
	if t.offset < secsize {
//
		copy(t.section[t.offset:], b)
//
//
		if smax == 0 {
			smax = secsize
		}
		if l <= smax {
			t.offset += l
			return l, nil
		}
	} else {
//
		if t.cursor == h.pool.SegmentCount*2 {
			return 0, nil
		}
	}
//
	for smax < l {
//
		go h.writeSection(t.cursor, t.section, true, false)
//
		t.section = make([]byte, secsize)
//
		copy(t.section, b[smax:])
//
		t.cursor++
//
		smax += secsize
	}
	t.offset = l - smax + secsize
	return l, nil
}

//
func (h *Hasher) Reset() {
	h.releaseTree()
}

//

//
//
//
func (h *Hasher) ResetWithLength(span []byte) {
	h.Reset()
	h.getTree().span = span
}

//
//
func (h *Hasher) releaseTree() {
	t := h.bmt
	if t == nil {
		return
	}
	h.bmt = nil
	go func() {
		t.cursor = 0
		t.offset = 0
		t.span = nil
		t.section = make([]byte, h.pool.SegmentSize*2)
		select {
		case <-t.result:
		default:
		}
		h.pool.release(t)
	}()
}

//
func (h *Hasher) NewAsyncWriter(double bool) *AsyncHasher {
	secsize := h.pool.SegmentSize
	if double {
		secsize *= 2
	}
	write := func(i int, section []byte, final bool) {
		h.writeSection(i, section, double, final)
	}
	return &AsyncHasher{
		Hasher:  h,
		double:  double,
		secsize: secsize,
		write:   write,
	}
}

//
type SectionWriter interface {
Reset()                                       //
Write(index int, data []byte)                 //
Sum(b []byte, length int, span []byte) []byte //
SectionSize() int                             //
}

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
type AsyncHasher struct {
*Hasher            //
mtx     sync.Mutex //
double  bool       //
secsize int        //
	write   func(i int, section []byte, final bool)
}

//

//
func (sw *AsyncHasher) SectionSize() int {
	return sw.secsize
}

//
//
//
func (sw *AsyncHasher) Write(i int, section []byte) {
	sw.mtx.Lock()
	defer sw.mtx.Unlock()
	t := sw.getTree()
//
//
	if i < t.cursor {
//
		go sw.write(i, section, false)
		return
	}
//
	if t.offset > 0 {
		if i == t.cursor {
//
//
			t.section = make([]byte, sw.secsize)
			copy(t.section, section)
			go sw.write(i, t.section, true)
			return
		}
//
		go sw.write(t.cursor, t.section, false)
	}
//
//
	t.cursor = i
	t.offset = i*sw.secsize + 1
	t.section = make([]byte, sw.secsize)
	copy(t.section, section)
}

//
//
//
//
//
//
//
//
//
func (sw *AsyncHasher) Sum(b []byte, length int, meta []byte) (s []byte) {
	sw.mtx.Lock()
	t := sw.getTree()
	if length == 0 {
		sw.mtx.Unlock()
		s = sw.pool.zerohashes[sw.pool.Depth]
	} else {
//
//
		maxsec := (length - 1) / sw.secsize
		if t.offset > 0 {
			go sw.write(t.cursor, t.section, maxsec == t.cursor)
		}
//
		t.cursor = maxsec
		t.offset = length
		result := t.result
		sw.mtx.Unlock()
//
		s = <-result
	}
//
	sw.releaseTree()
//
	if len(meta) == 0 {
		return append(b, s...)
	}
//
	return doSum(sw.pool.hasher(), b, meta, s)
}

//
func (h *Hasher) writeSection(i int, section []byte, double bool, final bool) {
//
	var n *node
	var isLeft bool
	var hasher hash.Hash
	var level int
	t := h.getTree()
	if double {
		level++
		n = t.leaves[i]
		hasher = n.hasher
		isLeft = n.isLeft
		n = n.parent
//
		section = doSum(hasher, nil, section)
	} else {
		n = t.leaves[i/2]
		hasher = n.hasher
		isLeft = i%2 == 0
	}
//
	if final {
//
		h.writeFinalNode(level, n, hasher, isLeft, section)
	} else {
		h.writeNode(n, hasher, isLeft, section)
	}
}

//
//
//
//
//
func (h *Hasher) writeNode(n *node, bh hash.Hash, isLeft bool, s []byte) {
	level := 1
	for {
//
		if n == nil {
			h.getTree().result <- s
			return
		}
//
		if isLeft {
			n.left = s
		} else {
			n.right = s
		}
//
		if n.toggle() {
			return
		}
//
//
		s = doSum(bh, nil, n.left, n.right)
		isLeft = n.isLeft
		n = n.parent
		level++
	}
}

//
//
//
//
//
func (h *Hasher) writeFinalNode(level int, n *node, bh hash.Hash, isLeft bool, s []byte) {

	for {
//
		if n == nil {
			if s != nil {
				h.getTree().result <- s
			}
			return
		}
		var noHash bool
		if isLeft {
//
//
//
			n.right = h.pool.zerohashes[level]
			if s != nil {
				n.left = s
//
//
//
				noHash = false
			} else {
//
				noHash = n.toggle()
			}
		} else {
//
			if s != nil {
//
				n.right = s
//
				noHash = n.toggle()

			} else {
//
//
				noHash = true
			}
		}
//
//
//
		if noHash {
			s = nil
		} else {
			s = doSum(bh, nil, n.left, n.right)
		}
//
		isLeft = n.isLeft
		n = n.parent
		level++
	}
}

//
func (h *Hasher) getTree() *tree {
	if h.bmt != nil {
		return h.bmt
	}
	t := h.pool.reserve()
	h.bmt = t
	return t
}

//
//
//
func (n *node) toggle() bool {
	return atomic.AddInt32(&n.state, 1)%2 == 1
}

//
func doSum(h hash.Hash, b []byte, data ...[]byte) []byte {
	h.Reset()
	for _, v := range data {
		h.Write(v)
	}
	return h.Sum(b)
}

//
func hashstr(b []byte) string {
	end := len(b)
	if end > 4 {
		end = 4
	}
	return fmt.Sprintf("%x", b[:end])
}

//
func calculateDepthFor(n int) (d int) {
	c := 2
	for ; c < n; c *= 2 {
		d++
	}
	return d + 1
}
