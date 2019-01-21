
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

package trie

import "testing"

func TestCanUnload(t *testing.T) {
	tests := []struct {
		flag                 nodeFlag
		cachegen, cachelimit uint16
		want                 bool
	}{
		{
			flag: nodeFlag{dirty: true, gen: 0},
			want: false,
		},
		{
			flag:     nodeFlag{dirty: false, gen: 0},
			cachegen: 0, cachelimit: 0,
			want: true,
		},
		{
			flag:     nodeFlag{dirty: false, gen: 65534},
			cachegen: 65535, cachelimit: 1,
			want: true,
		},
		{
			flag:     nodeFlag{dirty: false, gen: 65534},
			cachegen: 0, cachelimit: 1,
			want: true,
		},
		{
			flag:     nodeFlag{dirty: false, gen: 1},
			cachegen: 65535, cachelimit: 1,
			want: true,
		},
	}

	for _, test := range tests {
		if got := test.flag.canUnload(test.cachegen, test.cachelimit); got != test.want {
			t.Errorf("%+v\n   got %t, want %t", test, got, test.want)
		}
	}
}
