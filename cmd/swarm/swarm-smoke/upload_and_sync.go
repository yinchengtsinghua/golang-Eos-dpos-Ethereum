
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
//此文件是Go以太坊的一部分。
//
//Go以太坊是免费软件：您可以重新发布和/或修改它
//根据GNU通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊的分布希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU通用公共许可证了解更多详细信息。
//
//你应该已经收到一份GNU通用公共许可证的副本
//一起去以太坊吧。如果没有，请参见<http://www.gnu.org/licenses/>。

package main

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/pborman/uuid"

	cli "gopkg.in/urfave/cli.v1"
)

func generateEndpoints(scheme string, cluster string, from int, to int) {
	if cluster == "prod" {
		cluster = ""
	} else {
		cluster = cluster + "."
	}

	for port := from; port <= to; port++ {
endpoints = append(endpoints, fmt.Sprintf("%s://%v.%sswarm gateways.net“，方案，端口，群集）
	}

	if includeLocalhost {
endpoints = append(endpoints, "http://本地主机：8500“）
	}
}

func cliUploadAndSync(c *cli.Context) error {
	defer func(now time.Time) { log.Info("total time", "time", time.Since(now), "size", filesize) }(time.Now())

	generateEndpoints(scheme, cluster, from, to)

	log.Info("uploading to " + endpoints[0] + " and syncing")

	f, cleanup := generateRandomFile(filesize * 1000000)
	defer cleanup()

	hash, err := upload(f, endpoints[0])
	if err != nil {
		log.Error(err.Error())
		return err
	}

	fhash, err := digest(f)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	log.Info("uploaded successfully", "hash", hash, "digest", fmt.Sprintf("%x", fhash))

	if filesize < 10 {
		time.Sleep(35 * time.Second)
	} else {
		time.Sleep(15 * time.Second)
		time.Sleep(2 * time.Duration(filesize) * time.Second)
	}

	wg := sync.WaitGroup{}
	for _, endpoint := range endpoints {
		endpoint := endpoint
		ruid := uuid.New()[:8]
		wg.Add(1)
		go func(endpoint string, ruid string) {
			for {
				err := fetch(hash, endpoint, fhash, ruid)
				if err != nil {
					continue
				}

				wg.Done()
				return
			}
		}(endpoint, ruid)
	}
	wg.Wait()
	log.Info("all endpoints synced random file successfully")

	return nil
}

//
func fetch(hash string, endpoint string, original []byte, ruid string) error {
	log.Trace("sleeping", "ruid", ruid)
	time.Sleep(5 * time.Second)

	log.Trace("http get request", "ruid", ruid, "api", endpoint, "hash", hash)
	res, err := http.Get(endpoint + "/bzz:/" + hash + "/")
	if err != nil {
		log.Warn(err.Error(), "ruid", ruid)
		return err
	}
	log.Trace("http get response", "ruid", ruid, "api", endpoint, "hash", hash, "code", res.StatusCode, "len", res.ContentLength)

	if res.StatusCode != 200 {
		err := fmt.Errorf("expected status code %d, got %v", 200, res.StatusCode)
		log.Warn(err.Error(), "ruid", ruid)
		return err
	}

	defer res.Body.Close()

	rdigest, err := digest(res.Body)
	if err != nil {
		log.Warn(err.Error(), "ruid", ruid)
		return err
	}

	if !bytes.Equal(rdigest, original) {
		err := fmt.Errorf("downloaded imported file md5=%x is not the same as the generated one=%x", rdigest, original)
		log.Warn(err.Error(), "ruid", ruid)
		return err
	}

	log.Trace("downloaded file matches random file", "ruid", ruid, "len", res.ContentLength)

	return nil
}

//upload正在通过“swarm up”命令将文件“f”上载到“endpoint”
func upload(f *os.File, endpoint string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command("swarm", "--bzzapi", endpoint, "up", f.Name())
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	hash := strings.TrimRight(out.String(), "\r\n")
	return hash, nil
}

func digest(r io.Reader) ([]byte, error) {
	h := md5.New()
	_, err := io.Copy(h, r)
	if err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

//GenerateRandomFile正在创建具有请求字节大小的临时文件
func generateRandomFile(size int) (f *os.File, teardown func()) {
//创建tmp文件
	tmp, err := ioutil.TempFile("", "swarm-test")
	if err != nil {
		panic(err)
	}

//tmp文件清理回调
	teardown = func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}

	buf := make([]byte, size)
	_, err = rand.Read(buf)
	if err != nil {
		panic(err)
	}
	ioutil.WriteFile(tmp.Name(), buf, 0755)

	return tmp, teardown
}
