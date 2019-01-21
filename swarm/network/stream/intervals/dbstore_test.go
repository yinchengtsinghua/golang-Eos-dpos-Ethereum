
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
	"io/ioutil"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/state"
)

//
func TestDBStore(t *testing.T) {
	dir, err := ioutil.TempDir("", "intervals_test_db_store")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	store, err := state.NewDBStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	testStore(t, store)
}
