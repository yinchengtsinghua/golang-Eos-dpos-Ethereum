
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
	"errors"

	"github.com/syndtr/goleveldb/leveldb"
)

//
var ErrNotFound = errors.New("ErrorNotFound")

//
var ErrInvalidArgument = errors.New("ErrorInvalidArgument")

//
type DBStore struct {
	db *leveldb.DB
}

//
func NewDBStore(path string) (s *DBStore, err error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	return &DBStore{
		db: db,
	}, nil
}

//
//
//
func (s *DBStore) Get(key string, i interface{}) (err error) {
	has, err := s.db.Has([]byte(key), nil)
	if err != nil || !has {
		return ErrNotFound
	}

	data, err := s.db.Get([]byte(key), nil)
	if err == leveldb.ErrNotFound {
		return ErrNotFound
	}

	unmarshaler, ok := i.(encoding.BinaryUnmarshaler)
	if !ok {
		return json.Unmarshal(data, i)
	}
	return unmarshaler.UnmarshalBinary(data)
}

//
func (s *DBStore) Put(key string, i interface{}) (err error) {
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

	return s.db.Put([]byte(key), bytes, nil)
}

//
func (s *DBStore) Delete(key string) (err error) {
	return s.db.Delete([]byte(key), nil)
}

//
func (s *DBStore) Close() error {
	return s.db.Close()
}
