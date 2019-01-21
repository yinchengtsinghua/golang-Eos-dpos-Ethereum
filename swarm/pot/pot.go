
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
package pot

import (
	"fmt"
	"sync"
)

const (
	maxkeylen = 256
)

//
type Pot struct {
	pin  Val
	bins []*Pot
	size int
	po   int
}

//
type Val interface{}

//
type Pof func(Val, Val, int) (int, bool)

//
//
//
func NewPot(v Val, po int) *Pot {
	var size int
	if v != nil {
		size++
	}
	return &Pot{
		pin:  v,
		po:   po,
		size: size,
	}
}

//
func (t *Pot) Pin() Val {
	return t.pin
}

//
func (t *Pot) Size() int {
	if t == nil {
		return 0
	}
	return t.size
}

//
//
//
//
//
//
//
func Add(t *Pot, val Val, pof Pof) (*Pot, int, bool) {
	return add(t, val, pof)
}

func (t *Pot) clone() *Pot {
	return &Pot{
		pin:  t.pin,
		size: t.size,
		po:   t.po,
		bins: t.bins,
	}
}

func add(t *Pot, val Val, pof Pof) (*Pot, int, bool) {
	var r *Pot
	if t == nil || t.pin == nil {
		r = t.clone()
		r.pin = val
		r.size++
		return r, 0, false
	}
	po, found := pof(t.pin, val, t.po)
	if found {
		r = t.clone()
		r.pin = val
		return r, po, true
	}

	var p *Pot
	var i, j int
	size := t.size
	for i < len(t.bins) {
		n := t.bins[i]
		if n.po == po {
			p, _, found = add(n, val, pof)
			if !found {
				size++
			}
			j++
			break
		}
		if n.po > po {
			break
		}
		i++
		j++
	}
	if p == nil {
		size++
		p = &Pot{
			pin:  val,
			size: 1,
			po:   po,
		}
	}

	bins := append([]*Pot{}, t.bins[:i]...)
	bins = append(bins, p)
	bins = append(bins, t.bins[j:]...)
	r = &Pot{
		pin:  t.pin,
		size: size,
		po:   t.po,
		bins: bins,
	}

	return r, po, found
}

//
//
//
//
//
//
//
func Remove(t *Pot, v Val, pof Pof) (*Pot, int, bool) {
	return remove(t, v, pof)
}

func remove(t *Pot, val Val, pof Pof) (r *Pot, po int, found bool) {
	size := t.size
	po, found = pof(t.pin, val, t.po)
	if found {
		size--
		if size == 0 {
			r = &Pot{
				po: t.po,
			}
			return r, po, true
		}
		i := len(t.bins) - 1
		last := t.bins[i]
		r = &Pot{
			pin:  last.pin,
			bins: append(t.bins[:i], last.bins...),
			size: size,
			po:   t.po,
		}
		return r, t.po, true
	}

	var p *Pot
	var i, j int
	for i < len(t.bins) {
		n := t.bins[i]
		if n.po == po {
			p, po, found = remove(n, val, pof)
			if found {
				size--
			}
			j++
			break
		}
		if n.po > po {
			return t, po, false
		}
		i++
		j++
	}
	bins := t.bins[:i]
	if p != nil && p.pin != nil {
		bins = append(bins, p)
	}
	bins = append(bins, t.bins[j:]...)
	r = &Pot{
		pin:  val,
		size: size,
		po:   t.po,
		bins: bins,
	}
	return r, po, found
}

//
//
//
//
//
//
func Swap(t *Pot, k Val, pof Pof, f func(v Val) Val) (r *Pot, po int, found bool, change bool) {
	var val Val
	if t.pin == nil {
		val = f(nil)
		if val == nil {
			return nil, 0, false, false
		}
		return NewPot(val, t.po), 0, false, true
	}
	size := t.size
	po, found = pof(k, t.pin, t.po)
	if found {
		val = f(t.pin)
//
		if val == nil {
			size--
			if size == 0 {
				r = &Pot{
					po: t.po,
				}
//
				return r, po, true, true
			}
//
			i := len(t.bins) - 1
			last := t.bins[i]
			r = &Pot{
				pin:  last.pin,
				bins: append(t.bins[:i], last.bins...),
				size: size,
				po:   t.po,
			}
			return r, po, true, true
		}
//
		if val == t.pin {
			return t, po, true, false
		}
//
		r = t.clone()
		r.pin = val
		return r, po, true, true
	}

//
	var p *Pot
	n, i := t.getPos(po)
	if n != nil {
		p, po, found, change = Swap(n, k, pof, f)
//
		if !change {
			return t, po, found, false
		}
//
		bins := append([]*Pot{}, t.bins[:i]...)
		if p.size == 0 {
			size--
		} else {
			size += p.size - n.size
			bins = append(bins, p)
		}
		i++
		if i < len(t.bins) {
			bins = append(bins, t.bins[i:]...)
		}
		r = t.clone()
		r.bins = bins
		r.size = size
		return r, po, found, true
	}
//
	val = f(nil)
	if val == nil {
//
		return t, po, false, false
	}
//
	if _, eq := pof(val, k, po); !eq {
		panic("invalid value")
	}
//
	size++
	p = &Pot{
		pin:  val,
		size: 1,
		po:   po,
	}

	bins := append([]*Pot{}, t.bins[:i]...)
	bins = append(bins, p)
	if i < len(t.bins) {
		bins = append(bins, t.bins[i:]...)
	}
	r = t.clone()
	r.bins = bins
	r.size = size
	return r, po, found, true
}

//
//
//
func Union(t0, t1 *Pot, pof Pof) (*Pot, int) {
	return union(t0, t1, pof)
}

func union(t0, t1 *Pot, pof Pof) (*Pot, int) {
	if t0 == nil || t0.size == 0 {
		return t1, 0
	}
	if t1 == nil || t1.size == 0 {
		return t0, 0
	}
	var pin Val
	var bins []*Pot
	var mis []int
	wg := &sync.WaitGroup{}
	wg.Add(1)
	pin0 := t0.pin
	pin1 := t1.pin
	bins0 := t0.bins
	bins1 := t1.bins
	var i0, i1 int
	var common int

	po, eq := pof(pin0, pin1, 0)

	for {
		l0 := len(bins0)
		l1 := len(bins1)
		var n0, n1 *Pot
		var p0, p1 int
		var a0, a1 bool

		for {

			if !a0 && i0 < l0 && bins0[i0] != nil && bins0[i0].po <= po {
				n0 = bins0[i0]
				p0 = n0.po
				a0 = p0 == po
			} else {
				a0 = true
			}

			if !a1 && i1 < l1 && bins1[i1] != nil && bins1[i1].po <= po {
				n1 = bins1[i1]
				p1 = n1.po
				a1 = p1 == po
			} else {
				a1 = true
			}
			if a0 && a1 {
				break
			}

			switch {
			case (p0 < p1 || a1) && !a0:
				bins = append(bins, n0)
				i0++
				n0 = nil
			case (p1 < p0 || a0) && !a1:
				bins = append(bins, n1)
				i1++
				n1 = nil
			case p1 < po:
				bl := len(bins)
				bins = append(bins, nil)
				ml := len(mis)
				mis = append(mis, 0)
//
//
//
//
//
				bins[bl], mis[ml] = union(n0, n1, pof)
				i0++
				i1++
				n0 = nil
				n1 = nil
			}
		}

		if eq {
			common++
			pin = pin1
			break
		}

		i := i0
		if len(bins0) > i && bins0[i].po == po {
			i++
		}
		var size0 int
		for _, n := range bins0[i:] {
			size0 += n.size
		}
		np := &Pot{
			pin:  pin0,
			bins: bins0[i:],
			size: size0 + 1,
			po:   po,
		}

		bins2 := []*Pot{np}
		if n0 == nil {
			pin0 = pin1
			po = maxkeylen + 1
			eq = true
			common--

		} else {
			bins2 = append(bins2, n0.bins...)
			pin0 = pin1
			pin1 = n0.pin
			po, eq = pof(pin0, pin1, n0.po)

		}
		bins0 = bins1
		bins1 = bins2
		i0 = i1
		i1 = 0

	}

	wg.Done()
	wg.Wait()
	for _, c := range mis {
		common += c
	}
	n := &Pot{
		pin:  pin,
		bins: bins,
		size: t0.size + t1.size - common,
		po:   t0.po,
	}
	return n, common
}

//
//
//
func (t *Pot) Each(f func(Val, int) bool) bool {
	return t.each(f)
}

func (t *Pot) each(f func(Val, int) bool) bool {
	var next bool
	for _, n := range t.bins {
		if n == nil {
			return true
		}
		next = n.each(f)
		if !next {
			return false
		}
	}
	if t.size == 0 {
		return false
	}
	return f(t.pin, t.po)
}

//
//
//
//
//
//
//
//
func (t *Pot) EachFrom(f func(Val, int) bool, po int) bool {
	return t.eachFrom(f, po)
}

func (t *Pot) eachFrom(f func(Val, int) bool, po int) bool {
	var next bool
	_, lim := t.getPos(po)
	for i := lim; i < len(t.bins); i++ {
		n := t.bins[i]
		next = n.each(f)
		if !next {
			return false
		}
	}
	return f(t.pin, t.po)
}

//
//
//
//
func (t *Pot) EachBin(val Val, pof Pof, po int, f func(int, int, func(func(val Val, i int) bool) bool) bool) {
	t.eachBin(val, pof, po, f)
}

func (t *Pot) eachBin(val Val, pof Pof, po int, f func(int, int, func(func(val Val, i int) bool) bool) bool) {
	if t == nil || t.size == 0 {
		return
	}
	spr, _ := pof(t.pin, val, t.po)
	_, lim := t.getPos(spr)
	var size int
	var n *Pot
	for i := 0; i < lim; i++ {
		n = t.bins[i]
		size += n.size
		if n.po < po {
			continue
		}
		if !f(n.po, n.size, n.each) {
			return
		}
	}
	if lim == len(t.bins) {
		if spr >= po {
			f(spr, 1, func(g func(Val, int) bool) bool {
				return g(t.pin, spr)
			})
		}
		return
	}

	n = t.bins[lim]

	spo := spr
	if n.po == spr {
		spo++
		size += n.size
	}
	if spr >= po {
		if !f(spr, t.size-size, func(g func(Val, int) bool) bool {
			return t.eachFrom(func(v Val, j int) bool {
				return g(v, spr)
			}, spo)
		}) {
			return
		}
	}
	if n.po == spr {
		n.eachBin(val, pof, po, f)
	}

}

//
//
//
func (t *Pot) EachNeighbour(val Val, pof Pof, f func(Val, int) bool) bool {
	return t.eachNeighbour(val, pof, f)
}

func (t *Pot) eachNeighbour(val Val, pof Pof, f func(Val, int) bool) bool {
	if t == nil || t.size == 0 {
		return false
	}
	var next bool
	l := len(t.bins)
	var n *Pot
	ir := l
	il := l
	po, eq := pof(t.pin, val, t.po)
	if !eq {
		n, il = t.getPos(po)
		if n != nil {
			next = n.eachNeighbour(val, pof, f)
			if !next {
				return false
			}
			ir = il
		} else {
			ir = il - 1
		}
	}

	next = f(t.pin, po)
	if !next {
		return false
	}

	for i := l - 1; i > ir; i-- {
		next = t.bins[i].each(func(v Val, _ int) bool {
			return f(v, po)
		})
		if !next {
			return false
		}
	}

	for i := il - 1; i >= 0; i-- {
		n := t.bins[i]
		next = n.each(func(v Val, _ int) bool {
			return f(v, n.po)
		})
		if !next {
			return false
		}
	}
	return true
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
func (t *Pot) EachNeighbourAsync(val Val, pof Pof, max int, maxPos int, f func(Val, int), wait bool) {
	if max > t.size {
		max = t.size
	}
	var wg *sync.WaitGroup
	if wait {
		wg = &sync.WaitGroup{}
	}
	t.eachNeighbourAsync(val, pof, max, maxPos, f, wg)
	if wait {
		wg.Wait()
	}
}

func (t *Pot) eachNeighbourAsync(val Val, pof Pof, max int, maxPos int, f func(Val, int), wg *sync.WaitGroup) (extra int) {
	l := len(t.bins)

	po, eq := pof(t.pin, val, t.po)

//
	pom := po
	if pom > maxPos {
		pom = maxPos
	}
	n, il := t.getPos(pom)
	ir := il
//
	if pom == po {
		if n != nil {

			m := n.size
			if max < m {
				m = max
			}
			max -= m

			extra = n.eachNeighbourAsync(val, pof, m, maxPos, f, wg)

		} else {
			if !eq {
				ir--
			}
		}
	} else {
		extra++
		max--
		if n != nil {
			il++
		}
//
//
		for i := l - 1; i >= il; i-- {
			s := t.bins[i]
			m := s.size
			if max < m {
				m = max
			}
			max -= m
			extra += m
		}
	}

	var m int
	if pom == po {

		m, max, extra = need(1, max, extra)
		if m <= 0 {
			return
		}

		if wg != nil {
			wg.Add(1)
		}
		go func() {
			if wg != nil {
				defer wg.Done()
			}
			f(t.pin, po)
		}()

//
		for i := l - 1; i > ir; i-- {
			n := t.bins[i]

			m, max, extra = need(n.size, max, extra)
			if m <= 0 {
				return
			}

			if wg != nil {
				wg.Add(m)
			}
			go func(pn *Pot, pm int) {
				pn.each(func(v Val, _ int) bool {
					if wg != nil {
						defer wg.Done()
					}
					f(v, po)
					pm--
					return pm > 0
				})
			}(n, m)

		}
	}

//
	for i := il - 1; i >= 0; i-- {
		n := t.bins[i]
//
//
		m, max, extra = need(n.size, max, extra)
		if m <= 0 {
			return
		}

		if wg != nil {
			wg.Add(m)
		}
		go func(pn *Pot, pm int) {
			pn.each(func(v Val, _ int) bool {
				if wg != nil {
					defer wg.Done()
				}
				f(v, pn.po)
				pm--
				return pm > 0
			})
		}(n, m)

	}
	return max + extra
}

//
//
//
func (t *Pot) getPos(po int) (n *Pot, i int) {
	for i, n = range t.bins {
		if po > n.po {
			continue
		}
		if po < n.po {
			return nil, i
		}
		return n, i
	}
	return nil, len(t.bins)
}

//
//
func need(m, max, extra int) (int, int, int) {
	if m <= extra {
		return m, max, extra - m
	}
	max += extra - m
	if max <= 0 {
		return m + max, 0, 0
	}
	return m, max, 0
}

func (t *Pot) String() string {
	return t.sstring("")
}

func (t *Pot) sstring(indent string) string {
	if t == nil {
		return "<nil>"
	}
	var s string
	indent += "  "
	s += fmt.Sprintf("%v%v (%v) %v \n", indent, t.pin, t.po, t.size)
	for _, n := range t.bins {
		s += fmt.Sprintf("%v%v\n", indent, n.sstring(indent))
	}
	return s
}
