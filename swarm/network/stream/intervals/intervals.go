
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

package intervals

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"
)

//
//
//
//
//
type Intervals struct {
	start  uint64
	ranges [][2]uint64
	mu     sync.RWMutex
}

//
//
//
//
//
//
//
func NewIntervals(start uint64) *Intervals {
	return &Intervals{
		start: start,
	}
}

//
//
func (i *Intervals) Add(start, end uint64) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.add(start, end)
}

func (i *Intervals) add(start, end uint64) {
	if start < i.start {
		start = i.start
	}
	if end < i.start {
		return
	}
	minStartJ := -1
	maxEndJ := -1
	j := 0
	for ; j < len(i.ranges); j++ {
		if minStartJ < 0 {
			if (start <= i.ranges[j][0] && end+1 >= i.ranges[j][0]) || (start <= i.ranges[j][1]+1 && end+1 >= i.ranges[j][1]) {
				if i.ranges[j][0] < start {
					start = i.ranges[j][0]
				}
				minStartJ = j
			}
		}
		if (start <= i.ranges[j][1] && end+1 >= i.ranges[j][1]) || (start <= i.ranges[j][0] && end+1 >= i.ranges[j][0]) {
			if i.ranges[j][1] > end {
				end = i.ranges[j][1]
			}
			maxEndJ = j
		}
		if end+1 <= i.ranges[j][0] {
			break
		}
	}
	if minStartJ < 0 && maxEndJ < 0 {
		i.ranges = append(i.ranges[:j], append([][2]uint64{{start, end}}, i.ranges[j:]...)...)
		return
	}
	if minStartJ >= 0 {
		i.ranges[minStartJ][0] = start
	}
	if maxEndJ >= 0 {
		i.ranges[maxEndJ][1] = end
	}
	if minStartJ >= 0 && maxEndJ >= 0 && minStartJ != maxEndJ {
		i.ranges[maxEndJ][0] = start
		i.ranges = append(i.ranges[:minStartJ], i.ranges[maxEndJ:]...)
	}
}

//
func (i *Intervals) Merge(m *Intervals) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	i.mu.Lock()
	defer i.mu.Unlock()

	for _, r := range m.ranges {
		i.add(r[0], r[1])
	}
}

//
//
//
//
//
//
//
func (i *Intervals) Next() (start, end uint64) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	l := len(i.ranges)
	if l == 0 {
		return i.start, 0
	}
	if i.ranges[0][0] != i.start {
		return i.start, i.ranges[0][0] - 1
	}
	if l == 1 {
		return i.ranges[0][1] + 1, 0
	}
	return i.ranges[0][1] + 1, i.ranges[1][0] - 1
}

//
func (i *Intervals) Last() (end uint64) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	l := len(i.ranges)
	if l == 0 {
		return 0
	}
	return i.ranges[l-1][1]
}

//
//
func (i *Intervals) String() string {
	return fmt.Sprint(i.ranges)
}

//
//
//
func (i *Intervals) MarshalBinary() (data []byte, err error) {
	d := make([][]byte, len(i.ranges)+1)
	d[0] = []byte(strconv.FormatUint(i.start, 36))
	for j := range i.ranges {
		r := i.ranges[j]
		d[j+1] = []byte(strconv.FormatUint(r[0], 36) + "," + strconv.FormatUint(r[1], 36))
	}
	return bytes.Join(d, []byte(";")), nil
}

//
func (i *Intervals) UnmarshalBinary(data []byte) (err error) {
	d := bytes.Split(data, []byte(";"))
	l := len(d)
	if l == 0 {
		return nil
	}
	if l >= 1 {
		i.start, err = strconv.ParseUint(string(d[0]), 36, 64)
		if err != nil {
			return err
		}
	}
	if l == 1 {
		return nil
	}

	i.ranges = make([][2]uint64, 0, l-1)
	for j := 1; j < l; j++ {
		r := bytes.SplitN(d[j], []byte(","), 2)
		if len(r) < 2 {
			return fmt.Errorf("range %d has less then 2 elements", j)
		}
		start, err := strconv.ParseUint(string(r[0]), 36, 64)
		if err != nil {
			return fmt.Errorf("parsing the first element in range %d: %v", j, err)
		}
		end, err := strconv.ParseUint(string(r[1]), 36, 64)
		if err != nil {
			return fmt.Errorf("parsing the second element in range %d: %v", j, err)
		}
		i.ranges = append(i.ranges, [2]uint64{start, end})
	}

	return nil
}
