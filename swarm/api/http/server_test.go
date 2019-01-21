
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

package http

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/swarm/api"
	swarm "github.com/ethereum/go-ethereum/swarm/api/client"
	"github.com/ethereum/go-ethereum/swarm/multihash"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/mru"
	"github.com/ethereum/go-ethereum/swarm/testutil"
)

func init() {
	loglevel := flag.Int("loglevel", 2, "loglevel")
	flag.Parse()
	log.Root().SetHandler(log.CallerFileHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(os.Stderr, log.TerminalFormat(true)))))
}

func TestResourcePostMode(t *testing.T) {
	path := ""
	errstr := "resourcePostMode for '%s' should be raw %v frequency %d, was raw %v, frequency %d"
	r, f, err := resourcePostMode(path)
	if err != nil {
		t.Fatal(err)
	} else if r || f != 0 {
		t.Fatalf(errstr, path, false, 0, r, f)
	}

	path = "raw"
	r, f, err = resourcePostMode(path)
	if err != nil {
		t.Fatal(err)
	} else if !r || f != 0 {
		t.Fatalf(errstr, path, true, 0, r, f)
	}

	path = "13"
	r, f, err = resourcePostMode(path)
	if err != nil {
		t.Fatal(err)
	} else if r || f == 0 {
		t.Fatalf(errstr, path, false, 13, r, f)
	}

	path = "raw/13"
	r, f, err = resourcePostMode(path)
	if err != nil {
		t.Fatal(err)
	} else if !r || f == 0 {
		t.Fatalf(errstr, path, true, 13, r, f)
	}

	path = "foo/13"
	r, f, err = resourcePostMode(path)
	if err == nil {
		t.Fatal("resourcePostMode for 'foo/13' should fail, returned error nil")
	}
}

func serverFunc(api *api.API) testutil.TestServer {
	return NewServer(api, "")
}

func newTestSigner() (*mru.GenericSigner, error) {
	privKey, err := crypto.HexToECDSA("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err != nil {
		return nil, err
	}
	return mru.NewGenericSigner(privKey), nil
}

//
//
//
//
//
func TestBzzResourceMultihash(t *testing.T) {

	signer, _ := newTestSigner()

	srv := testutil.NewTestSwarmServer(t, serverFunc)
	defer srv.Close()

//
	databytes := "bar"
	url := fmt.Sprintf("%s/bzz:/", srv.URL)
	resp, err := http.Post(url, "text/plain", bytes.NewReader([]byte(databytes)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		t.Fatal(err)
	}
	s := common.FromHex(string(b))
	mh := multihash.ToMultihash(s)

	log.Info("added data", "manifest", string(b), "data", common.ToHex(mh))

//
	keybytes := "foo.eth"

	updateRequest, err := mru.NewCreateUpdateRequest(&mru.ResourceMetadata{
		Name:      keybytes,
		Frequency: 13,
		StartTime: srv.GetCurrentTime(),
		Owner:     signer.Address(),
	})
	if err != nil {
		t.Fatal(err)
	}
	updateRequest.SetData(mh, true)

	if err := updateRequest.Sign(signer); err != nil {
		t.Fatal(err)
	}
	log.Info("added data", "manifest", string(b), "data", common.ToHex(mh))

	body, err := updateRequest.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

//
	url = fmt.Sprintf("%s/bzz-resource:/", srv.URL)
	resp, err = http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	rsrcResp := &storage.Address{}
	err = json.Unmarshal(b, rsrcResp)
	if err != nil {
		t.Fatalf("data %s could not be unmarshaled: %v", b, err)
	}

	correctManifestAddrHex := "6d3bc4664c97d8b821cb74bcae43f592494fb46d2d9cd31e69f3c7c802bbbd8e"
	if rsrcResp.Hex() != correctManifestAddrHex {
		t.Fatalf("Response resource key mismatch, expected '%s', got '%s'", correctManifestAddrHex, rsrcResp.Hex())
	}

//
	url = fmt.Sprintf("%s/bzz:/%s", srv.URL, rsrcResp)
	resp, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, []byte(databytes)) {
		t.Fatalf("retrieved data mismatch, expected %x, got %x", databytes, b)
	}
}

//
func TestBzzResource(t *testing.T) {
	srv := testutil.NewTestSwarmServer(t, serverFunc)
	signer, _ := newTestSigner()

	defer srv.Close()

//
	keybytes := "foo.eth"

//
	databytes := make([]byte, 666)
	_, err := rand.Read(databytes)
	if err != nil {
		t.Fatal(err)
	}

	updateRequest, err := mru.NewCreateUpdateRequest(&mru.ResourceMetadata{
		Name:      keybytes,
		Frequency: 13,
		StartTime: srv.GetCurrentTime(),
		Owner:     signer.Address(),
	})
	if err != nil {
		t.Fatal(err)
	}
	updateRequest.SetData(databytes, false)

	if err := updateRequest.Sign(signer); err != nil {
		t.Fatal(err)
	}

	body, err := updateRequest.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

//
	url := fmt.Sprintf("%s/bzz-resource:/", srv.URL)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	rsrcResp := &storage.Address{}
	err = json.Unmarshal(b, rsrcResp)
	if err != nil {
		t.Fatalf("data %s could not be unmarshaled: %v", b, err)
	}

	correctManifestAddrHex := "6d3bc4664c97d8b821cb74bcae43f592494fb46d2d9cd31e69f3c7c802bbbd8e"
	if rsrcResp.Hex() != correctManifestAddrHex {
		t.Fatalf("Response resource key mismatch, expected '%s', got '%s'", correctManifestAddrHex, rsrcResp.Hex())
	}

//
	url = fmt.Sprintf("%s/bzz-raw:/%s", srv.URL, rsrcResp)
	resp, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	manifest := &api.Manifest{}
	err = json.Unmarshal(b, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entries) != 1 {
		t.Fatalf("Manifest has %d entries", len(manifest.Entries))
	}
	correctRootKeyHex := "68f7ba07ac8867a4c841a4d4320e3cdc549df23702dc7285fcb6acf65df48562"
	if manifest.Entries[0].Hash != correctRootKeyHex {
		t.Fatalf("Expected manifest path '%s', got '%s'", correctRootKeyHex, manifest.Entries[0].Hash)
	}

//
	url = fmt.Sprintf("%s/bzz:/%s", srv.URL, rsrcResp)
	resp, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

//
	url = fmt.Sprintf("%s/bzz-resource:/bar", srv.URL)
	resp, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected get non-existent resource to fail with StatusNotFound (404), got %d", resp.StatusCode)
	}

	resp.Body.Close()

//
	log.Info("get update latest = 1.1", "addr", correctManifestAddrHex)
	url = fmt.Sprintf("%s/bzz-resource:/%s", srv.URL, correctManifestAddrHex)
	resp, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(databytes, b) {
		t.Fatalf("Expected body '%x', got '%x'", databytes, b)
	}

//
	log.Info("update 2")

//
	url = fmt.Sprintf("%s/bzz-resource:/%s/", srv.URL, correctManifestAddrHex)
	resp, err = http.Get(url + "meta")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Get resource metadata returned %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	updateRequest = &mru.Request{}
	if err = updateRequest.UnmarshalJSON(b); err != nil {
		t.Fatalf("Error decoding resource metadata: %s", err)
	}
	data := []byte("foo")
	updateRequest.SetData(data, false)
	if err = updateRequest.Sign(signer); err != nil {
		t.Fatal(err)
	}
	body, err = updateRequest.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	resp, err = http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Update returned %s", resp.Status)
	}

//
	log.Info("get update 1.2")
	url = fmt.Sprintf("%s/bzz-resource:/%s", srv.URL, correctManifestAddrHex)
	resp, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, b) {
		t.Fatalf("Expected body '%x', got '%x'", data, b)
	}

//
	log.Info("get update latest = 1.2")
	url = fmt.Sprintf("%s/bzz-resource:/%s/1", srv.URL, correctManifestAddrHex)
	resp, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, b) {
		t.Fatalf("Expected body '%x', got '%x'", data, b)
	}

//
	log.Info("get first update 1.1")
	url = fmt.Sprintf("%s/bzz-resource:/%s/1/1", srv.URL, correctManifestAddrHex)
	resp, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(databytes, b) {
		t.Fatalf("Expected body '%x', got '%x'", databytes, b)
	}
}

func TestBzzGetPath(t *testing.T) {
	testBzzGetPath(false, t)
	testBzzGetPath(true, t)
}

func testBzzGetPath(encrypted bool, t *testing.T) {
	var err error

	testmanifest := []string{
		`{"entries":[{"path":"b","hash":"011b4d03dd8c01f1049143cf9c4c817e4b167f1d1b83e5c6f0f10d89ba1e7bce","contentType":"","status":0},{"path":"c","hash":"011b4d03dd8c01f1049143cf9c4c817e4b167f1d1b83e5c6f0f10d89ba1e7bce","contentType":"","status":0}]}`,
		`{"entries":[{"path":"a","hash":"011b4d03dd8c01f1049143cf9c4c817e4b167f1d1b83e5c6f0f10d89ba1e7bce","contentType":"","status":0},{"path":"b/","hash":"<key0>","contentType":"application/bzz-manifest+json","status":0}]}`,
		`{"entries":[{"path":"a/","hash":"<key1>","contentType":"application/bzz-manifest+json","status":0}]}`,
	}

	testrequests := make(map[string]int)
	testrequests["/"] = 2
	testrequests["/a/"] = 1
	testrequests["/a/b/"] = 0
	testrequests["/x"] = 0
	testrequests[""] = 0

	expectedfailrequests := []string{"", "/x"}

	reader := [3]*bytes.Reader{}

	addr := [3]storage.Address{}

	srv := testutil.NewTestSwarmServer(t, serverFunc)
	defer srv.Close()

	for i, mf := range testmanifest {
		reader[i] = bytes.NewReader([]byte(mf))
		var wait func(context.Context) error
		ctx := context.TODO()
		addr[i], wait, err = srv.FileStore.Store(ctx, reader[i], int64(len(mf)), encrypted)
		for j := i + 1; j < len(testmanifest); j++ {
			testmanifest[j] = strings.Replace(testmanifest[j], fmt.Sprintf("<key%v>", i), addr[i].Hex(), -1)
		}
		if err != nil {
			t.Fatal(err)
		}
		err = wait(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	rootRef := addr[2].Hex()

	_, err = http.Get(srv.URL + "/bzz-raw:/" + rootRef + "/a")
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}

	for k, v := range testrequests {
		var resp *http.Response
		var respbody []byte

		url := srv.URL + "/bzz-raw:/"
		if k[:] != "" {
			url += rootRef + "/" + k[1:] + "?content_type=text/plain"
		}
		resp, err = http.Get(url)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()
		respbody, err = ioutil.ReadAll(resp.Body)

		if string(respbody) != testmanifest[v] {
			isexpectedfailrequest := false

			for _, r := range expectedfailrequests {
				if k[:] == r {
					isexpectedfailrequest = true
				}
			}
			if !isexpectedfailrequest {
				t.Fatalf("Response body does not match, expected: %v, got %v", testmanifest[v], string(respbody))
			}
		}
	}

	for k, v := range testrequests {
		var resp *http.Response
		var respbody []byte

		url := srv.URL + "/bzz-hash:/"
		if k[:] != "" {
			url += rootRef + "/" + k[1:]
		}
		resp, err = http.Get(url)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()
		respbody, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Read request body: %v", err)
		}

		if string(respbody) != addr[v].Hex() {
			isexpectedfailrequest := false

			for _, r := range expectedfailrequests {
				if k[:] == r {
					isexpectedfailrequest = true
				}
			}
			if !isexpectedfailrequest {
				t.Fatalf("Response body does not match, expected: %v, got %v", addr[v], string(respbody))
			}
		}
	}

	ref := addr[2].Hex()

	for _, c := range []struct {
		path          string
		json          string
		pageFragments []string
	}{
		{
			path: "/",
			json: `{"common_prefixes":["a/"]}`,
			pageFragments: []string{
				fmt.Sprintf("Swarm index of bzz:/%s/", ref),
				`<a class="normal-link" href="a/">a/</a>`,
			},
		},
		{
			path: "/a/",
			json: `{"common_prefixes":["a/b/"],"entries":[{"hash":"011b4d03dd8c01f1049143cf9c4c817e4b167f1d1b83e5c6f0f10d89ba1e7bce","path":"a/a","mod_time":"0001-01-01T00:00:00Z"}]}`,
			pageFragments: []string{
				fmt.Sprintf("Swarm index of bzz:/%s/a/", ref),
				`<a class="normal-link" href="b/">b/</a>`,
				fmt.Sprintf(`<a class="normal-link" href="/bzz:/%s/a/a">a</a>`, ref),
			},
		},
		{
			path: "/a/b/",
			json: `{"entries":[{"hash":"011b4d03dd8c01f1049143cf9c4c817e4b167f1d1b83e5c6f0f10d89ba1e7bce","path":"a/b/b","mod_time":"0001-01-01T00:00:00Z"},{"hash":"011b4d03dd8c01f1049143cf9c4c817e4b167f1d1b83e5c6f0f10d89ba1e7bce","path":"a/b/c","mod_time":"0001-01-01T00:00:00Z"}]}`,
			pageFragments: []string{
				fmt.Sprintf("Swarm index of bzz:/%s/a/b/", ref),
				fmt.Sprintf(`<a class="normal-link" href="/bzz:/%s/a/b/b">b</a>`, ref),
				fmt.Sprintf(`<a class="normal-link" href="/bzz:/%s/a/b/c">c</a>`, ref),
			},
		},
		{
			path: "/x",
		},
		{
			path: "",
		},
	} {
		k := c.path
		url := srv.URL + "/bzz-list:/"
		if k[:] != "" {
			url += rootRef + "/" + k[1:]
		}
		t.Run("json list "+c.path, func(t *testing.T) {
			resp, err := http.Get(url)
			if err != nil {
				t.Fatalf("HTTP request: %v", err)
			}
			defer resp.Body.Close()
			respbody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Read response body: %v", err)
			}

			body := strings.TrimSpace(string(respbody))
			if body != c.json {
				isexpectedfailrequest := false

				for _, r := range expectedfailrequests {
					if k[:] == r {
						isexpectedfailrequest = true
					}
				}
				if !isexpectedfailrequest {
					t.Errorf("Response list body %q does not match, expected: %v, got %v", k, c.json, body)
				}
			}
		})
		t.Run("html list "+c.path, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, url, nil)
			if err != nil {
				t.Fatalf("New request: %v", err)
			}
			req.Header.Set("Accept", "text/html")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("HTTP request: %v", err)
			}
			defer resp.Body.Close()
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Read response body: %v", err)
			}

			body := string(b)

			for _, f := range c.pageFragments {
				if !strings.Contains(body, f) {
					isexpectedfailrequest := false

					for _, r := range expectedfailrequests {
						if k[:] == r {
							isexpectedfailrequest = true
						}
					}
					if !isexpectedfailrequest {
						t.Errorf("Response list body %q does not contain %q: body %q", k, f, body)
					}
				}
			}
		})
	}

	nonhashtests := []string{
		srv.URL + "/bzz:/name",
		srv.URL + "/bzz-immutable:/nonhash",
		srv.URL + "/bzz-raw:/nonhash",
		srv.URL + "/bzz-list:/nonhash",
		srv.URL + "/bzz-hash:/nonhash",
	}

	nonhashresponses := []string{
		`cannot resolve name: no DNS to resolve name: "name"`,
		`cannot resolve nonhash: immutable address not a content hash: "nonhash"`,
		`cannot resolve nonhash: no DNS to resolve name: "nonhash"`,
		`cannot resolve nonhash: no DNS to resolve name: "nonhash"`,
		`cannot resolve nonhash: no DNS to resolve name: "nonhash"`,
	}

	for i, url := range nonhashtests {
		var resp *http.Response
		var respbody []byte

		resp, err = http.Get(url)

		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()
		respbody, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}
		if !strings.Contains(string(respbody), nonhashresponses[i]) {
			t.Fatalf("Non-Hash response body does not match, expected: %v, got: %v", nonhashresponses[i], string(respbody))
		}
	}
}

func TestBzzTar(t *testing.T) {
	testBzzTar(false, t)
	testBzzTar(true, t)
}

func testBzzTar(encrypted bool, t *testing.T) {
	srv := testutil.NewTestSwarmServer(t, serverFunc)
	defer srv.Close()
	fileNames := []string{"tmp1.txt", "tmp2.lock", "tmp3.rtf"}
	fileContents := []string{"tmp1textfilevalue", "tmp2lockfilelocked", "tmp3isjustaplaintextfile"}

	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	defer tw.Close()

	for i, v := range fileNames {
		size := int64(len(fileContents[i]))
		hdr := &tar.Header{
			Name:    v,
			Mode:    0644,
			Size:    size,
			ModTime: time.Now(),
			Xattrs: map[string]string{
				"user.swarm.content-type": "text/plain",
			},
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}

//
		n, err := io.Copy(tw, bytes.NewBufferString(fileContents[i]))
		if err != nil {
			t.Fatal(err)
		} else if n != size {
			t.Fatal("size mismatch")
		}
	}

//
	url := srv.URL + "/bzz:/"
	if encrypted {
		url = url + "encrypt"
	}
	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-tar")
	client := &http.Client{}
	resp2, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp2.Status)
	}
	swarmHash, err := ioutil.ReadAll(resp2.Body)
	resp2.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

//
	req, err = http.NewRequest("GET", fmt.Sprintf(srv.URL+"/bzz:/%s", string(swarmHash)), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Accept", "application/x-tar")
	resp2, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	file, err := ioutil.TempFile("", "swarm-downloaded-tarball")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())
	_, err = io.Copy(file, resp2.Body)
	if err != nil {
		t.Fatalf("error getting tarball: %v", err)
	}
	file.Sync()
	file.Close()

	tarFileHandle, err := os.Open(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(tarFileHandle)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("error reading tar stream: %s", err)
		}
		bb := make([]byte, hdr.Size)
		_, err = tr.Read(bb)
		if err != nil && err != io.EOF {
			t.Fatal(err)
		}
		passed := false
		for i, v := range fileNames {
			if v == hdr.Name {
				if string(bb) == fileContents[i] {
					passed = true
					break
				}
			}
		}
		if !passed {
			t.Fatalf("file %s did not pass content assertion", hdr.Name)
		}
	}
}

//
//
//
func TestBzzRootRedirect(t *testing.T) {
	testBzzRootRedirect(false, t)
}
func TestBzzRootRedirectEncrypted(t *testing.T) {
	testBzzRootRedirect(true, t)
}

func testBzzRootRedirect(toEncrypt bool, t *testing.T) {
	srv := testutil.NewTestSwarmServer(t, serverFunc)
	defer srv.Close()

//
	client := swarm.NewClient(srv.URL)
	data := []byte("data")
	file := &swarm.File{
		ReadCloser: ioutil.NopCloser(bytes.NewReader(data)),
		ManifestEntry: api.ManifestEntry{
			Path:        "",
			ContentType: "text/plain",
			Size:        int64(len(data)),
		},
	}
	hash, err := client.Upload(file, "", toEncrypt)
	if err != nil {
		t.Fatal(err)
	}

//
//
	redirected := false
	httpClient := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if redirected {
				return errors.New("too many redirects")
			}
			redirected = true
			expectedPath := "/bzz:/" + hash + "/"
			if req.URL.Path != expectedPath {
				return fmt.Errorf("expected redirect to %q, got %q", expectedPath, req.URL.Path)
			}
			return nil
		},
	}

//
	res, err := httpClient.Get(srv.URL + "/bzz:/" + hash)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if !redirected {
		t.Fatal("expected GET /bzz:/<hash> to redirect to /bzz:/<hash>/ but it didn't")
	}
	gotData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotData, data) {
		t.Fatalf("expected response to equal %q, got %q", data, gotData)
	}
}

func TestMethodsNotAllowed(t *testing.T) {
	srv := testutil.NewTestSwarmServer(t, serverFunc)
	defer srv.Close()
	databytes := "bar"
	for _, c := range []struct {
		url  string
		code int
	}{
		{
			url:  fmt.Sprintf("%s/bzz-list:/", srv.URL),
			code: 405,
		}, {
			url:  fmt.Sprintf("%s/bzz-hash:/", srv.URL),
			code: 405,
		},
		{
			url:  fmt.Sprintf("%s/bzz-immutable:/", srv.URL),
			code: 405,
		},
	} {
		res, _ := http.Post(c.url, "text/plain", bytes.NewReader([]byte(databytes)))
		if res.StatusCode != c.code {
			t.Fatalf("should have failed. requested url: %s, expected code %d, got %d", c.url, c.code, res.StatusCode)
		}
	}

}

func httpDo(httpMethod string, url string, reqBody io.Reader, headers map[string]string, verbose bool, t *testing.T) (*http.Response, string) {
//
	req, err := http.NewRequest(httpMethod, url, reqBody)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if verbose {
		t.Log(req.Method, req.URL, req.Header, req.Body)
	}

//
	httpClient := &http.Client{}
	res, err := httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

//
	buffer, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body := string(buffer)

	return res, body
}

func TestGet(t *testing.T) {
	srv := testutil.NewTestSwarmServer(t, serverFunc)
	defer srv.Close()

	for _, testCase := range []struct {
		uri                string
		method             string
		headers            map[string]string
		expectedStatusCode int
		assertResponseBody string
		verbose            bool
	}{
		{
			uri:                fmt.Sprintf("%s/", srv.URL),
			method:             "GET",
			headers:            map[string]string{"Accept": "text/html"},
			expectedStatusCode: 200,
			assertResponseBody: "Swarm: Serverless Hosting Incentivised Peer-To-Peer Storage And Content Distribution",
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/", srv.URL),
			method:             "GET",
			headers:            map[string]string{"Accept": "application/json"},
			expectedStatusCode: 200,
			assertResponseBody: "Swarm: Please request a valid ENS or swarm hash with the appropriate bzz scheme",
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/robots.txt", srv.URL),
			method:             "GET",
			headers:            map[string]string{"Accept": "text/html"},
			expectedStatusCode: 200,
			assertResponseBody: "User-agent: *\nDisallow: /",
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/nonexistent_path", srv.URL),
			method:             "GET",
			headers:            map[string]string{},
			expectedStatusCode: 404,
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/bzz:asdf/", srv.URL),
			method:             "GET",
			headers:            map[string]string{},
			expectedStatusCode: 404,
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/tbz2/", srv.URL),
			method:             "GET",
			headers:            map[string]string{},
			expectedStatusCode: 404,
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/bzz-rack:/", srv.URL),
			method:             "GET",
			headers:            map[string]string{},
			expectedStatusCode: 404,
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/bzz-ls", srv.URL),
			method:             "GET",
			headers:            map[string]string{},
			expectedStatusCode: 404,
			verbose:            false,
		},
	} {
		t.Run("GET "+testCase.uri, func(t *testing.T) {
			res, body := httpDo(testCase.method, testCase.uri, nil, testCase.headers, testCase.verbose, t)
			if res.StatusCode != testCase.expectedStatusCode {
				t.Fatalf("expected status code %d but got %d", testCase.expectedStatusCode, res.StatusCode)
			}
			if testCase.assertResponseBody != "" && !strings.Contains(body, testCase.assertResponseBody) {
				t.Fatalf("expected response to be: %s but got: %s", testCase.assertResponseBody, body)
			}
		})
	}
}

func TestModify(t *testing.T) {
	srv := testutil.NewTestSwarmServer(t, serverFunc)
	defer srv.Close()

	swarmClient := swarm.NewClient(srv.URL)
	data := []byte("data")
	file := &swarm.File{
		ReadCloser: ioutil.NopCloser(bytes.NewReader(data)),
		ManifestEntry: api.ManifestEntry{
			Path:        "",
			ContentType: "text/plain",
			Size:        int64(len(data)),
		},
	}

	hash, err := swarmClient.Upload(file, "", false)
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range []struct {
		uri                   string
		method                string
		headers               map[string]string
		requestBody           []byte
		expectedStatusCode    int
		assertResponseBody    string
		assertResponseHeaders map[string]string
		verbose               bool
	}{
		{
			uri:                fmt.Sprintf("%s/bzz:/%s", srv.URL, hash),
			method:             "DELETE",
			headers:            map[string]string{},
			expectedStatusCode: 200,
			assertResponseBody: "8b634aea26eec353ac0ecbec20c94f44d6f8d11f38d4578a4c207a84c74ef731",
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/bzz:/%s", srv.URL, hash),
			method:             "PUT",
			headers:            map[string]string{},
			expectedStatusCode: 405,
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/bzz-raw:/%s", srv.URL, hash),
			method:             "PUT",
			headers:            map[string]string{},
			expectedStatusCode: 405,
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/bzz:/%s", srv.URL, hash),
			method:             "PATCH",
			headers:            map[string]string{},
			expectedStatusCode: 405,
			verbose:            false,
		},
		{
			uri:                   fmt.Sprintf("%s/bzz-raw:/", srv.URL),
			method:                "POST",
			headers:               map[string]string{},
			requestBody:           []byte("POSTdata"),
			expectedStatusCode:    200,
			assertResponseHeaders: map[string]string{"Content-Length": "64"},
			verbose:               false,
		},
		{
			uri:                   fmt.Sprintf("%s/bzz-raw:/encrypt", srv.URL),
			method:                "POST",
			headers:               map[string]string{},
			requestBody:           []byte("POSTdata"),
			expectedStatusCode:    200,
			assertResponseHeaders: map[string]string{"Content-Length": "128"},
			verbose:               false,
		},
	} {
		t.Run(testCase.method+" "+testCase.uri, func(t *testing.T) {
			reqBody := bytes.NewReader(testCase.requestBody)
			res, body := httpDo(testCase.method, testCase.uri, reqBody, testCase.headers, testCase.verbose, t)

			if res.StatusCode != testCase.expectedStatusCode {
				t.Fatalf("expected status code %d but got %d", testCase.expectedStatusCode, res.StatusCode)
			}
			if testCase.assertResponseBody != "" && !strings.Contains(body, testCase.assertResponseBody) {
				t.Log(body)
				t.Fatalf("expected response %s but got %s", testCase.assertResponseBody, body)
			}
			for key, value := range testCase.assertResponseHeaders {
				if res.Header.Get(key) != value {
					t.Logf("expected %s=%s in HTTP response header but got %s", key, value, res.Header.Get(key))
				}
			}
		})
	}
}

func TestMultiPartUpload(t *testing.T) {
//
	verbose := false
//
	srv := testutil.NewTestSwarmServer(t, serverFunc)
	defer srv.Close()

	url := fmt.Sprintf("%s/bzz:/", srv.URL)

	buf := new(bytes.Buffer)
	form := multipart.NewWriter(buf)
	form.WriteField("name", "John Doe")
	file1, _ := form.CreateFormFile("cv", "cv.txt")
	file1.Write([]byte("John Doe's Credentials"))
	file2, _ := form.CreateFormFile("profile_picture", "profile.jpg")
	file2.Write([]byte("imaginethisisjpegdata"))
	form.Close()

	headers := map[string]string{
		"Content-Type":   form.FormDataContentType(),
		"Content-Length": strconv.Itoa(buf.Len()),
	}
	res, body := httpDo("POST", url, buf, headers, verbose, t)

	if res.StatusCode != 200 {
		t.Fatalf("expected POST multipart/form-data to return 200, but it returned %d", res.StatusCode)
	}
	if len(body) != 64 {
		t.Fatalf("expected POST multipart/form-data to return a 64 char manifest but the answer was %d chars long", len(body))
	}
}
