
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
package db

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/storage/mock"
)

//
//
//
//
type GlobalStore struct {
	db *leveldb.DB
}

//
func NewGlobalStore(path string) (s *GlobalStore, err error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	return &GlobalStore{
		db: db,
	}, nil
}

//
func (s *GlobalStore) Close() error {
	return s.db.Close()
}

//
//
func (s *GlobalStore) NewNodeStore(addr common.Address) *mock.NodeStore {
	return mock.NewNodeStore(addr, s)
}

//
//
func (s *GlobalStore) Get(addr common.Address, key []byte) (data []byte, err error) {
	has, err := s.db.Has(nodeDBKey(addr, key), nil)
	if err != nil {
		return nil, mock.ErrNotFound
	}
	if !has {
		return nil, mock.ErrNotFound
	}
	data, err = s.db.Get(dataDBKey(key), nil)
	if err == leveldb.ErrNotFound {
		err = mock.ErrNotFound
	}
	return
}

//
func (s *GlobalStore) Put(addr common.Address, key []byte, data []byte) error {
	batch := new(leveldb.Batch)
	batch.Put(nodeDBKey(addr, key), nil)
	batch.Put(dataDBKey(key), data)
	return s.db.Write(batch, nil)
}

//
func (s *GlobalStore) HasKey(addr common.Address, key []byte) bool {
	has, err := s.db.Has(nodeDBKey(addr, key), nil)
	if err != nil {
		has = false
	}
	return has
}

//
//
func (s *GlobalStore) Import(r io.Reader) (n int, err error) {
	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return n, err
		}

		data, err := ioutil.ReadAll(tr)
		if err != nil {
			return n, err
		}

		var c mock.ExportedChunk
		if err = json.Unmarshal(data, &c); err != nil {
			return n, err
		}

		batch := new(leveldb.Batch)
		for _, addr := range c.Addrs {
			batch.Put(nodeDBKeyHex(addr, hdr.Name), nil)
		}

		batch.Put(dataDBKey(common.Hex2Bytes(hdr.Name)), c.Data)
		if err = s.db.Write(batch, nil); err != nil {
			return n, err
		}

		n++
	}
	return n, err
}

//
//
func (s *GlobalStore) Export(w io.Writer) (n int, err error) {
	tw := tar.NewWriter(w)
	defer tw.Close()

	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	encoder := json.NewEncoder(buf)

	iter := s.db.NewIterator(util.BytesPrefix(nodeKeyPrefix), nil)
	defer iter.Release()

	var currentKey string
	var addrs []common.Address

	saveChunk := func(hexKey string) error {
		key := common.Hex2Bytes(hexKey)

		data, err := s.db.Get(dataDBKey(key), nil)
		if err != nil {
			return err
		}

		buf.Reset()
		if err = encoder.Encode(mock.ExportedChunk{
			Addrs: addrs,
			Data:  data,
		}); err != nil {
			return err
		}

		d := buf.Bytes()
		hdr := &tar.Header{
			Name: hexKey,
			Mode: 0644,
			Size: int64(len(d)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write(d); err != nil {
			return err
		}
		n++
		return nil
	}

	for iter.Next() {
		k := bytes.TrimPrefix(iter.Key(), nodeKeyPrefix)
		i := bytes.Index(k, []byte("-"))
		if i < 0 {
			continue
		}
		hexKey := string(k[:i])

		if currentKey == "" {
			currentKey = hexKey
		}

		if hexKey != currentKey {
			if err = saveChunk(currentKey); err != nil {
				return n, err
			}

			addrs = addrs[:0]
		}

		currentKey = hexKey
		addrs = append(addrs, common.BytesToAddress(k[i:]))
	}

	if len(addrs) > 0 {
		if err = saveChunk(currentKey); err != nil {
			return n, err
		}
	}

	return n, err
}

var (
	nodeKeyPrefix = []byte("node-")
	dataKeyPrefix = []byte("data-")
)

//
func nodeDBKey(addr common.Address, key []byte) []byte {
	return nodeDBKeyHex(addr, common.Bytes2Hex(key))
}

//
//
func nodeDBKeyHex(addr common.Address, hexKey string) []byte {
	return append(append(nodeKeyPrefix, []byte(hexKey+"-")...), addr[:]...)
}

//
func dataDBKey(key []byte) []byte {
	return append(dataKeyPrefix, key...)
}
