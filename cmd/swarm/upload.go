
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

//
package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/cmd/utils"
	swarm "github.com/ethereum/go-ethereum/swarm/api/client"
	"gopkg.in/urfave/cli.v1"
)

func upload(ctx *cli.Context) {

	args := ctx.Args()
	var (
		bzzapi       = strings.TrimRight(ctx.GlobalString(SwarmApiFlag.Name), "/")
		recursive    = ctx.GlobalBool(SwarmRecursiveFlag.Name)
		wantManifest = ctx.GlobalBoolT(SwarmWantManifestFlag.Name)
		defaultPath  = ctx.GlobalString(SwarmUploadDefaultPath.Name)
		fromStdin    = ctx.GlobalBool(SwarmUpFromStdinFlag.Name)
		mimeType     = ctx.GlobalString(SwarmUploadMimeType.Name)
		client       = swarm.NewClient(bzzapi)
		toEncrypt    = ctx.Bool(SwarmEncryptedFlag.Name)
		file         string
	)

	if len(args) != 1 {
		if fromStdin {
			tmp, err := ioutil.TempFile("", "swarm-stdin")
			if err != nil {
				utils.Fatalf("error create tempfile: %s", err)
			}
			defer os.Remove(tmp.Name())
			n, err := io.Copy(tmp, os.Stdin)
			if err != nil {
				utils.Fatalf("error copying stdin to tempfile: %s", err)
			} else if n == 0 {
				utils.Fatalf("error reading from stdin: zero length")
			}
			file = tmp.Name()
		} else {
			utils.Fatalf("Need filename as the first and only argument")
		}
	} else {
		file = expandPath(args[0])
	}

	if !wantManifest {
		f, err := swarm.Open(file)
		if err != nil {
			utils.Fatalf("Error opening file: %s", err)
		}
		defer f.Close()
		hash, err := client.UploadRaw(f, f.Size, toEncrypt)
		if err != nil {
			utils.Fatalf("Upload failed: %s", err)
		}
		fmt.Println(hash)
		return
	}

	stat, err := os.Stat(file)
	if err != nil {
		utils.Fatalf("Error opening file: %s", err)
	}

//
//根据上传文件的类型
	var doUpload func() (hash string, err error)
	if stat.IsDir() {
		doUpload = func() (string, error) {
			if !recursive {
				return "", errors.New("Argument is a directory and recursive upload is disabled")
			}
			if defaultPath != "" {
//构造绝对默认路径
				absDefaultPath, _ := filepath.Abs(defaultPath)
				absFile, _ := filepath.Abs(file)
//
//从绝对默认路径修剪它并获取相对默认路径
				absFile = strings.TrimRight(absFile, "/") + "/"
				if absDefaultPath != "" && absFile != "" && strings.HasPrefix(absDefaultPath, absFile) {
					defaultPath = strings.TrimPrefix(absDefaultPath, absFile)
				}
			}
			return client.UploadDirectory(file, defaultPath, "", toEncrypt)
		}
	} else {
		doUpload = func() (string, error) {
			f, err := swarm.Open(file)
			if err != nil {
				return "", fmt.Errorf("error opening file: %s", err)
			}
			defer f.Close()
			if mimeType == "" {
				mimeType = detectMimeType(file)
			}
			f.ContentType = mimeType
			return client.Upload(f, "", toEncrypt)
		}
	}
	hash, err := doUpload()
	if err != nil {
		utils.Fatalf("Upload failed: %s", err)
	}
	fmt.Println(hash)
}

//展开文件路径
//1。用用户主目录替换tilde
//2。扩展嵌入的环境变量
//三。清理路径，例如/a/b/。/c->/a/c
//注意，它有局限性，例如~someuser/tmp将不会扩展
func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		if home := homeDir(); home != "" {
			p = home + p[1:]
		}
	}
	return path.Clean(os.ExpandEnv(p))
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

func detectMimeType(file string) string {
	if ext := filepath.Ext(file); ext != "" {
		return mime.TypeByExtension(ext)
	}
	f, err := os.Open(file)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, 512)
	if n, _ := f.Read(buf); n > 0 {
		return http.DetectContentType(buf)
	}
	return ""
}
