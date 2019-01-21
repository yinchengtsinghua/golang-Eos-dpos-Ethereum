
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

package state

import (
	"encoding"
	"encoding/json"
	"sync"
)

//
//
type InmemoryStore struct {
	db map[string][]byte
	mu sync.RWMutex
}

//
func NewInmemoryStore() *InmemoryStore {
	return &InmemoryStore{
		db: make(map[string][]byte),
	}
}

//
//
func (s *InmemoryStore) Get(key string, i interface{}) (err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bytes, ok := s.db[key]
	if !ok {
		return ErrNotFound
	}

	unmarshaler, ok := i.(encoding.BinaryUnmarshaler)
	if !ok {
		return json.Unmarshal(bytes, i)
	}

	return unmarshaler.UnmarshalBinary(bytes)
}

//
func (s *InmemoryStore) Put(key string, i interface{}) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bytes := []byte{}

	marshaler, ok := i.(encoding.BinaryMarshaler)
	if !ok {
		if bytes, err = json.Marshal(i); err != nil {
			return err
		}
	} else {
		if bytes, err = marshaler.MarshalBinary(); err != nil {
			return err
		}
	}

	s.db[key] = bytes
	return nil
}

//
func (s *InmemoryStore) Delete(key string) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.db[key]; !ok {
		return ErrNotFound
	}
	delete(s.db, key)
	return nil
}

//
func (s *InmemoryStore) Close() error {
	return nil
}
