
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

package client

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/swarm/api"
	swarmhttp "github.com/ethereum/go-ethereum/swarm/api/http"
	"github.com/ethereum/go-ethereum/swarm/multihash"
	"github.com/ethereum/go-ethereum/swarm/storage/mru"
	"github.com/ethereum/go-ethereum/swarm/testutil"
)

func serverFunc(api *api.API) testutil.TestServer {
	return swarmhttp.NewServer(api, "")
}

//测试客户端上传下载原始测试上传和下载原始数据到Swarm
func TestClientUploadDownloadRaw(t *testing.T) {
	testClientUploadDownloadRaw(false, t)
}
func TestClientUploadDownloadRawEncrypted(t *testing.T) {
	testClientUploadDownloadRaw(true, t)
}

func testClientUploadDownloadRaw(toEncrypt bool, t *testing.T) {
	srv := testutil.NewTestSwarmServer(t, serverFunc)
	defer srv.Close()

	client := NewClient(srv.URL)

//上传一些原始数据
	data := []byte("foo123")
	hash, err := client.UploadRaw(bytes.NewReader(data), int64(len(data)), toEncrypt)
	if err != nil {
		t.Fatal(err)
	}

//检查我们是否可以下载相同的数据
	res, isEncrypted, err := client.DownloadRaw(hash)
	if err != nil {
		t.Fatal(err)
	}
	if isEncrypted != toEncrypt {
		t.Fatalf("Expected encyption status %v got %v", toEncrypt, isEncrypted)
	}
	defer res.Close()
	gotData, err := ioutil.ReadAll(res)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotData, data) {
		t.Fatalf("expected downloaded data to be %q, got %q", data, gotData)
	}
}

//测试客户端上传下载文件测试上传和下载文件到Swarm
//清单
func TestClientUploadDownloadFiles(t *testing.T) {
	testClientUploadDownloadFiles(false, t)
}

func TestClientUploadDownloadFilesEncrypted(t *testing.T) {
	testClientUploadDownloadFiles(true, t)
}

func testClientUploadDownloadFiles(toEncrypt bool, t *testing.T) {
	srv := testutil.NewTestSwarmServer(t, serverFunc)
	defer srv.Close()

	client := NewClient(srv.URL)
	upload := func(manifest, path string, data []byte) string {
		file := &File{
			ReadCloser: ioutil.NopCloser(bytes.NewReader(data)),
			ManifestEntry: api.ManifestEntry{
				Path:        path,
				ContentType: "text/plain",
				Size:        int64(len(data)),
			},
		}
		hash, err := client.Upload(file, manifest, toEncrypt)
		if err != nil {
			t.Fatal(err)
		}
		return hash
	}
	checkDownload := func(manifest, path string, expected []byte) {
		file, err := client.Download(manifest, path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if file.Size != int64(len(expected)) {
			t.Fatalf("expected downloaded file to be %d bytes, got %d", len(expected), file.Size)
		}
		if file.ContentType != "text/plain" {
			t.Fatalf("expected downloaded file to have type %q, got %q", "text/plain", file.ContentType)
		}
		data, err := ioutil.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, expected) {
			t.Fatalf("expected downloaded data to be %q, got %q", expected, data)
		}
	}

//将文件上载到清单的根目录
	rootData := []byte("some-data")
	rootHash := upload("", "", rootData)

//检查我们是否可以下载根文件
	checkDownload(rootHash, "", rootData)

//将另一个文件上载到同一清单
	otherData := []byte("some-other-data")
	newHash := upload(rootHash, "some/other/path", otherData)

//检查我们可以从新清单下载这两个文件
	checkDownload(newHash, "", rootData)
	checkDownload(newHash, "some/other/path", otherData)

//用不同的数据替换根文件
	newHash = upload(newHash, "", otherData)

//检查两个文件是否都有其他数据
	checkDownload(newHash, "", otherData)
	checkDownload(newHash, "some/other/path", otherData)
}

var testDirFiles = []string{
	"file1.txt",
	"file2.txt",
	"dir1/file3.txt",
	"dir1/file4.txt",
	"dir2/file5.txt",
	"dir2/dir3/file6.txt",
	"dir2/dir4/file7.txt",
	"dir2/dir4/file8.txt",
}

func newTestDirectory(t *testing.T) string {
	dir, err := ioutil.TempDir("", "swarm-client-test")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range testDirFiles {
		path := filepath.Join(dir, file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			os.RemoveAll(dir)
			t.Fatalf("error creating dir for %s: %s", path, err)
		}
		if err := ioutil.WriteFile(path, []byte(file), 0644); err != nil {
			os.RemoveAll(dir)
			t.Fatalf("error writing file %s: %s", path, err)
		}
	}

	return dir
}

//测试客户端上载下载目录测试上载和下载
//Swarm清单的文件目录
func TestClientUploadDownloadDirectory(t *testing.T) {
	srv := testutil.NewTestSwarmServer(t, serverFunc)
	defer srv.Close()

	dir := newTestDirectory(t)
	defer os.RemoveAll(dir)

//上传目录
	client := NewClient(srv.URL)
	defaultPath := testDirFiles[0]
	hash, err := client.UploadDirectory(dir, defaultPath, "", false)
	if err != nil {
		t.Fatalf("error uploading directory: %s", err)
	}

//检查我们是否可以下载单独的文件
	checkDownloadFile := func(path string, expected []byte) {
		file, err := client.Download(hash, path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, err := ioutil.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, expected) {
			t.Fatalf("expected data to be %q, got %q", expected, data)
		}
	}
	for _, file := range testDirFiles {
		checkDownloadFile(file, []byte(file))
	}

//检查我们是否可以下载默认路径
	checkDownloadFile("", []byte(testDirFiles[0]))

//检查我们可以下载目录
	tmp, err := ioutil.TempDir("", "swarm-client-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	if err := client.DownloadDirectory(hash, "", tmp, ""); err != nil {
		t.Fatal(err)
	}
	for _, file := range testDirFiles {
		data, err := ioutil.ReadFile(filepath.Join(tmp, file))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, []byte(file)) {
			t.Fatalf("expected data to be %q, got %q", file, data)
		}
	}
}

//testclientfilelist在swarm清单中列出文件的测试
func TestClientFileList(t *testing.T) {
	testClientFileList(false, t)
}

func TestClientFileListEncrypted(t *testing.T) {
	testClientFileList(true, t)
}

func testClientFileList(toEncrypt bool, t *testing.T) {
	srv := testutil.NewTestSwarmServer(t, serverFunc)
	defer srv.Close()

	dir := newTestDirectory(t)
	defer os.RemoveAll(dir)

	client := NewClient(srv.URL)
	hash, err := client.UploadDirectory(dir, "", "", toEncrypt)
	if err != nil {
		t.Fatalf("error uploading directory: %s", err)
	}

	ls := func(prefix string) []string {
		list, err := client.List(hash, prefix, "")
		if err != nil {
			t.Fatal(err)
		}
		paths := make([]string, 0, len(list.CommonPrefixes)+len(list.Entries))
		paths = append(paths, list.CommonPrefixes...)
		for _, entry := range list.Entries {
			paths = append(paths, entry.Path)
		}
		sort.Strings(paths)
		return paths
	}

	tests := map[string][]string{
		"":                    {"dir1/", "dir2/", "file1.txt", "file2.txt"},
		"file":                {"file1.txt", "file2.txt"},
		"file1":               {"file1.txt"},
		"file2.txt":           {"file2.txt"},
		"file12":              {},
		"dir":                 {"dir1/", "dir2/"},
		"dir1":                {"dir1/"},
		"dir1/":               {"dir1/file3.txt", "dir1/file4.txt"},
		"dir1/file":           {"dir1/file3.txt", "dir1/file4.txt"},
		"dir1/file3.txt":      {"dir1/file3.txt"},
		"dir1/file34":         {},
		"dir2/":               {"dir2/dir3/", "dir2/dir4/", "dir2/file5.txt"},
		"dir2/file":           {"dir2/file5.txt"},
		"dir2/dir":            {"dir2/dir3/", "dir2/dir4/"},
		"dir2/dir3/":          {"dir2/dir3/file6.txt"},
		"dir2/dir4/":          {"dir2/dir4/file7.txt", "dir2/dir4/file8.txt"},
		"dir2/dir4/file":      {"dir2/dir4/file7.txt", "dir2/dir4/file8.txt"},
		"dir2/dir4/file7.txt": {"dir2/dir4/file7.txt"},
		"dir2/dir4/file78":    {},
	}
	for prefix, expected := range tests {
		actual := ls(prefix)
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("expected prefix %q to return %v, got %v", prefix, expected, actual)
		}
	}
}

//testclientmultipartupload测试使用多部分将文件上载到swarm
//上传
func TestClientMultipartUpload(t *testing.T) {
	srv := testutil.NewTestSwarmServer(t, serverFunc)
	defer srv.Close()

//定义上载程序，该上载程序使用某些数据上载testdir文件
	data := []byte("some-data")
	uploader := UploaderFunc(func(upload UploadFn) error {
		for _, name := range testDirFiles {
			file := &File{
				ReadCloser: ioutil.NopCloser(bytes.NewReader(data)),
				ManifestEntry: api.ManifestEntry{
					Path:        name,
					ContentType: "text/plain",
					Size:        int64(len(data)),
				},
			}
			if err := upload(file); err != nil {
				return err
			}
		}
		return nil
	})

//以多部分上载方式上载文件
	client := NewClient(srv.URL)
	hash, err := client.MultipartUpload("", uploader)
	if err != nil {
		t.Fatal(err)
	}

//检查我们是否可以下载单独的文件
	checkDownloadFile := func(path string) {
		file, err := client.Download(hash, path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		gotData, err := ioutil.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(gotData, data) {
			t.Fatalf("expected data to be %q, got %q", data, gotData)
		}
	}
	for _, file := range testDirFiles {
		checkDownloadFile(file)
	}
}

func newTestSigner() (*mru.GenericSigner, error) {
	privKey, err := crypto.HexToECDSA("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err != nil {
		return nil, err
	}
	return mru.NewGenericSigner(privKey), nil
}

//使用bzz://scheme测试多哈希资源类型的透明解析
//
//首先上载数据，并将多哈希存储到资源更新中的结果清单中
//使用multihash检索更新时，应返回直接指向数据的清单。
//对散列的原始检索应该返回数据
func TestClientCreateResourceMultihash(t *testing.T) {

	signer, _ := newTestSigner()

	srv := testutil.NewTestSwarmServer(t, serverFunc)
	client := NewClient(srv.URL)
	defer srv.Close()

//添加我们的多哈希别名清单将指向的数据
	databytes := []byte("bar")

	swarmHash, err := client.UploadRaw(bytes.NewReader(databytes), int64(len(databytes)), false)
	if err != nil {
		t.Fatalf("Error uploading raw test data: %s", err)
	}

	s := common.FromHex(swarmHash)
	mh := multihash.ToMultihash(s)

//我们的可变资源“名称”
	resourceName := "foo.eth"

	createRequest, err := mru.NewCreateUpdateRequest(&mru.ResourceMetadata{
		Name:      resourceName,
		Frequency: 13,
		StartTime: srv.GetCurrentTime(),
		Owner:     signer.Address(),
	})
	if err != nil {
		t.Fatal(err)
	}
	createRequest.SetData(mh, true)
	if err := createRequest.Sign(signer); err != nil {
		t.Fatalf("Error signing update: %s", err)
	}

	resourceManifestHash, err := client.CreateResource(createRequest)

	if err != nil {
		t.Fatalf("Error creating resource: %s", err)
	}

	correctManifestAddrHex := "6d3bc4664c97d8b821cb74bcae43f592494fb46d2d9cd31e69f3c7c802bbbd8e"
	if resourceManifestHash != correctManifestAddrHex {
		t.Fatalf("Response resource key mismatch, expected '%s', got '%s'", correctManifestAddrHex, resourceManifestHash)
	}

	reader, err := client.GetResource(correctManifestAddrHex)
	if err != nil {
		t.Fatalf("Error retrieving resource: %s", err)
	}
	defer reader.Close()
	gotData, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mh, gotData) {
		t.Fatalf("Expected: %v, got %v", mh, gotData)
	}

}

//TestClientCreateUpdateResource将检查是否可以通过HTTP客户端创建和更新可变资源。
func TestClientCreateUpdateResource(t *testing.T) {

	signer, _ := newTestSigner()

	srv := testutil.NewTestSwarmServer(t, serverFunc)
	client := NewClient(srv.URL)
	defer srv.Close()

//设置资源的原始数据
	databytes := []byte("En un lugar de La Mancha, de cuyo nombre no quiero acordarme...")

//我们的可变资源名称
	resourceName := "El Quijote"

	createRequest, err := mru.NewCreateUpdateRequest(&mru.ResourceMetadata{
		Name:      resourceName,
		Frequency: 13,
		StartTime: srv.GetCurrentTime(),
		Owner:     signer.Address(),
	})
	if err != nil {
		t.Fatal(err)
	}
	createRequest.SetData(databytes, false)
	if err := createRequest.Sign(signer); err != nil {
		t.Fatalf("Error signing update: %s", err)
	}

	resourceManifestHash, err := client.CreateResource(createRequest)

	correctManifestAddrHex := "cc7904c17b49f9679e2d8006fe25e87e3f5c2072c2b49cab50f15e544471b30a"
	if resourceManifestHash != correctManifestAddrHex {
		t.Fatalf("Response resource key mismatch, expected '%s', got '%s'", correctManifestAddrHex, resourceManifestHash)
	}

	reader, err := client.GetResource(correctManifestAddrHex)
	if err != nil {
		t.Fatalf("Error retrieving resource: %s", err)
	}
	defer reader.Close()
	gotData, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(databytes, gotData) {
		t.Fatalf("Expected: %v, got %v", databytes, gotData)
	}

//定义不同的数据
	databytes = []byte("... no ha mucho tiempo que vivía un hidalgo de los de lanza en astillero ...")

	updateRequest, err := client.GetResourceMetadata(correctManifestAddrHex)
	if err != nil {
		t.Fatalf("Error retrieving update request template: %s", err)
	}

	updateRequest.SetData(databytes, false)
	if err := updateRequest.Sign(signer); err != nil {
		t.Fatalf("Error signing update: %s", err)
	}

	if err = client.UpdateResource(updateRequest); err != nil {
		t.Fatalf("Error updating resource: %s", err)
	}

	reader, err = client.GetResource(correctManifestAddrHex)
	if err != nil {
		t.Fatalf("Error retrieving resource: %s", err)
	}
	defer reader.Close()
	gotData, err = ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(databytes, gotData) {
		t.Fatalf("Expected: %v, got %v", databytes, gotData)
	}

}
