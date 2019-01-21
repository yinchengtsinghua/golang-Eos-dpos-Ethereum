
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

package api

import (
	"context"
	"testing"
)

func testStorage(t *testing.T, f func(*Storage, bool)) {
	testAPI(t, func(api *API, toEncrypt bool) {
		f(NewStorage(api), toEncrypt)
	})
}

func TestStoragePutGet(t *testing.T) {
	testStorage(t, func(api *Storage, toEncrypt bool) {
		content := "hello"
		exp := expResponse(content, "text/plain", 0)
//
		ctx := context.TODO()
		bzzkey, wait, err := api.Put(ctx, content, exp.MimeType, toEncrypt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		err = wait(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		bzzhash := bzzkey.Hex()
//
		resp0 := testGet(t, api.api, bzzhash, "")
		checkResponse(t, resp0, exp)

//
		resp, err := api.Get(context.TODO(), bzzhash)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		checkResponse(t, &testResponse{nil, resp}, exp)
	})
}
