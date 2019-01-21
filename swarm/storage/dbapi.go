
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

import "context"

//
type DBAPI struct {
	db  *LDBStore
	loc *LocalStore
}

func NewDBAPI(loc *LocalStore) *DBAPI {
	return &DBAPI{loc.DbStore, loc}
}

//
func (d *DBAPI) Get(ctx context.Context, addr Address) (*Chunk, error) {
	return d.loc.Get(ctx, addr)
}

//
func (d *DBAPI) CurrentBucketStorageIndex(po uint8) uint64 {
	return d.db.CurrentBucketStorageIndex(po)
}

//
func (d *DBAPI) Iterator(from uint64, to uint64, po uint8, f func(Address, uint64) bool) error {
	return d.db.SyncIterator(from, to, po, f)
}

//
func (d *DBAPI) GetOrCreateRequest(ctx context.Context, addr Address) (*Chunk, bool) {
	return d.loc.GetOrCreateRequest(ctx, addr)
}

//
func (d *DBAPI) Put(ctx context.Context, chunk *Chunk) {
	d.loc.Put(ctx, chunk)
}
