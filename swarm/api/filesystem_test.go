
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
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

var testDownloadDir, _ = ioutil.TempDir(os.TempDir(), "bzz-test")

func testFileSystem(t *testing.T, f func(*FileSystem, bool)) {
	testAPI(t, func(api *API, toEncrypt bool) {
		f(NewFileSystem(api), toEncrypt)
	})
}

func readPath(t *testing.T, parts ...string) string {
	file := filepath.Join(parts...)
	content, err := ioutil.ReadFile(file)

	if err != nil {
		t.Fatalf("unexpected error reading '%v': %v", file, err)
	}
	return string(content)
}

func TestApiDirUpload0(t *testing.T) {
	testFileSystem(t, func(fs *FileSystem, toEncrypt bool) {
		api := fs.api
		bzzhash, err := fs.Upload(filepath.Join("testdata", "test0"), "", toEncrypt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content := readPath(t, "testdata", "test0", "index.html")
		resp := testGet(t, api, bzzhash, "index.html")
		exp := expResponse(content, "text/html; charset=utf-8", 0)
		checkResponse(t, resp, exp)

		content = readPath(t, "testdata", "test0", "index.css")
		resp = testGet(t, api, bzzhash, "index.css")
		exp = expResponse(content, "text/css", 0)
		checkResponse(t, resp, exp)

		addr := storage.Address(common.Hex2Bytes(bzzhash))
		_, _, _, _, err = api.Get(context.TODO(), NOOPDecrypt, addr, "")
		if err == nil {
			t.Fatalf("expected error: %v", err)
		}

		downloadDir := filepath.Join(testDownloadDir, "test0")
		defer os.RemoveAll(downloadDir)
		err = fs.Download(bzzhash, downloadDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		newbzzhash, err := fs.Upload(downloadDir, "", toEncrypt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
//
		if !toEncrypt && bzzhash != newbzzhash {
			t.Fatalf("download %v reuploaded has incorrect hash, expected %v, got %v", downloadDir, bzzhash, newbzzhash)
		}
	})
}

func TestApiDirUploadModify(t *testing.T) {
	testFileSystem(t, func(fs *FileSystem, toEncrypt bool) {
		api := fs.api
		bzzhash, err := fs.Upload(filepath.Join("testdata", "test0"), "", toEncrypt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		addr := storage.Address(common.Hex2Bytes(bzzhash))
		addr, err = api.Modify(context.TODO(), addr, "index.html", "", "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		index, err := ioutil.ReadFile(filepath.Join("testdata", "test0", "index.html"))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		ctx := context.TODO()
		hash, wait, err := api.Store(ctx, bytes.NewReader(index), int64(len(index)), toEncrypt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		err = wait(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		addr, err = api.Modify(context.TODO(), addr, "index2.html", hash.Hex(), "text/html; charset=utf-8")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		addr, err = api.Modify(context.TODO(), addr, "img/logo.png", hash.Hex(), "text/html; charset=utf-8")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		bzzhash = addr.Hex()

		content := readPath(t, "testdata", "test0", "index.html")
		resp := testGet(t, api, bzzhash, "index2.html")
		exp := expResponse(content, "text/html; charset=utf-8", 0)
		checkResponse(t, resp, exp)

		resp = testGet(t, api, bzzhash, "img/logo.png")
		exp = expResponse(content, "text/html; charset=utf-8", 0)
		checkResponse(t, resp, exp)

		content = readPath(t, "testdata", "test0", "index.css")
		resp = testGet(t, api, bzzhash, "index.css")
		exp = expResponse(content, "text/css", 0)
		checkResponse(t, resp, exp)

		_, _, _, _, err = api.Get(context.TODO(), nil, addr, "")
		if err == nil {
			t.Errorf("expected error: %v", err)
		}
	})
}

func TestApiDirUploadWithRootFile(t *testing.T) {
	testFileSystem(t, func(fs *FileSystem, toEncrypt bool) {
		api := fs.api
		bzzhash, err := fs.Upload(filepath.Join("testdata", "test0"), "index.html", toEncrypt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		content := readPath(t, "testdata", "test0", "index.html")
		resp := testGet(t, api, bzzhash, "")
		exp := expResponse(content, "text/html; charset=utf-8", 0)
		checkResponse(t, resp, exp)
	})
}

func TestApiFileUpload(t *testing.T) {
	testFileSystem(t, func(fs *FileSystem, toEncrypt bool) {
		api := fs.api
		bzzhash, err := fs.Upload(filepath.Join("testdata", "test0", "index.html"), "", toEncrypt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		content := readPath(t, "testdata", "test0", "index.html")
		resp := testGet(t, api, bzzhash, "index.html")
		exp := expResponse(content, "text/html; charset=utf-8", 0)
		checkResponse(t, resp, exp)
	})
}

func TestApiFileUploadWithRootFile(t *testing.T) {
	testFileSystem(t, func(fs *FileSystem, toEncrypt bool) {
		api := fs.api
		bzzhash, err := fs.Upload(filepath.Join("testdata", "test0", "index.html"), "index.html", toEncrypt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		content := readPath(t, "testdata", "test0", "index.html")
		resp := testGet(t, api, bzzhash, "")
		exp := expResponse(content, "text/html; charset=utf-8", 0)
		checkResponse(t, resp, exp)
	})
}
