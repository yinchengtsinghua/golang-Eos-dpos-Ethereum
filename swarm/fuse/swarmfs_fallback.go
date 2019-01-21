
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

//

package fuse

import (
	"errors"
)

var errNoFUSE = errors.New("FUSE is not supported on this platform")

func isFUSEUnsupportedError(err error) bool {
	return err == errNoFUSE
}

type MountInfo struct {
	MountPoint     string
	StartManifest  string
	LatestManifest string
}

func (self *SwarmFS) Mount(mhash, mountpoint string) (*MountInfo, error) {
	return nil, errNoFUSE
}

func (self *SwarmFS) Unmount(mountpoint string) (bool, error) {
	return false, errNoFUSE
}

func (self *SwarmFS) Listmounts() ([]*MountInfo, error) {
	return nil, errNoFUSE
}

func (self *SwarmFS) Stop() error {
	return nil
}
