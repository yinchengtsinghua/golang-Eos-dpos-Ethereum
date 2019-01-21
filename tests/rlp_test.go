
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

package tests

import (
	"testing"
)

func TestRLP(t *testing.T) {
	t.Parallel()
	tm := new(testMatcher)
	tm.walk(t, rlpTestDir, func(t *testing.T, name string, test *RLPTest) {
		if err := tm.checkFailure(t, name, test.Run()); err != nil {
			t.Error(err)
		}
	})
}
