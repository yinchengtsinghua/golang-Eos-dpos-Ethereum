
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

package storage

//
//

import (
	"fmt"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const openFileLimit = 128

type LDBDatabase struct {
	db *leveldb.DB
}

func NewLDBDatabase(file string) (*LDBDatabase, error) {
//
	db, err := leveldb.OpenFile(file, &opt.Options{OpenFilesCacheCapacity: openFileLimit})
	if err != nil {
		return nil, err
	}

	database := &LDBDatabase{db: db}

	return database, nil
}

func (db *LDBDatabase) Put(key []byte, value []byte) {
	metrics.GetOrRegisterCounter("ldbdatabase.put", nil).Inc(1)

	err := db.db.Put(key, value, nil)
	if err != nil {
		fmt.Println("Error put", err)
	}
}

func (db *LDBDatabase) Get(key []byte) ([]byte, error) {
	metrics.GetOrRegisterCounter("ldbdatabase.get", nil).Inc(1)

	dat, err := db.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	return dat, nil
}

func (db *LDBDatabase) Delete(key []byte) error {
	return db.db.Delete(key, nil)
}

func (db *LDBDatabase) LastKnownTD() []byte {
	data, _ := db.Get([]byte("LTD"))

	if len(data) == 0 {
		data = []byte{0x0}
	}

	return data
}

func (db *LDBDatabase) NewIterator() iterator.Iterator {
	metrics.GetOrRegisterCounter("ldbdatabase.newiterator", nil).Inc(1)

	return db.db.NewIterator(nil, nil)
}

func (db *LDBDatabase) Write(batch *leveldb.Batch) error {
	metrics.GetOrRegisterCounter("ldbdatabase.write", nil).Inc(1)

	return db.db.Write(batch, nil)
}

func (db *LDBDatabase) Close() {
//
	db.db.Close()
}
