
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
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/state"
)

var ErrNotFound = errors.New("not found")

//
func TestInmemoryStore(t *testing.T) {
	testStore(t, state.NewInmemoryStore())
}

//
func testStore(t *testing.T, s state.Store) {
	key1 := "key1"
	i1 := NewIntervals(0)
	i1.Add(10, 20)
	if err := s.Put(key1, i1); err != nil {
		t.Fatal(err)
	}
	i := &Intervals{}
	err := s.Get(key1, i)
	if err != nil {
		t.Fatal(err)
	}
	if i.String() != i1.String() {
		t.Errorf("expected interval %s, got %s", i1, i)
	}

	key2 := "key2"
	i2 := NewIntervals(0)
	i2.Add(10, 20)
	if err := s.Put(key2, i2); err != nil {
		t.Fatal(err)
	}
	err = s.Get(key2, i)
	if err != nil {
		t.Fatal(err)
	}
	if i.String() != i2.String() {
		t.Errorf("expected interval %s, got %s", i2, i)
	}

	if err := s.Delete(key1); err != nil {
		t.Fatal(err)
	}
	if err := s.Get(key1, i); err != state.ErrNotFound {
		t.Errorf("expected error %v, got %s", state.ErrNotFound, err)
	}
	if err := s.Get(key2, i); err != nil {
		t.Errorf("expected error %v, got %s", nil, err)
	}

	if err := s.Delete(key2); err != nil {
		t.Fatal(err)
	}
	if err := s.Get(key2, i); err != state.ErrNotFound {
		t.Errorf("expected error %v, got %s", state.ErrNotFound, err)
	}
}
