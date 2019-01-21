
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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/api"
	"github.com/ethereum/go-ethereum/swarm/log"
)

var (
	errEmptyMountPoint      = errors.New("need non-empty mount point")
	errNoRelativeMountPoint = errors.New("invalid path for mount point (need absolute path)")
	errMaxMountCount        = errors.New("max FUSE mount count reached")
	errMountTimeout         = errors.New("mount timeout")
	errAlreadyMounted       = errors.New("mount point is already serving")
)

func isFUSEUnsupportedError(err error) bool {
	if perr, ok := err.(*os.PathError); ok {
		return perr.Op == "open" && perr.Path == "/dev/fuse"
	}
	return err == fuse.ErrOSXFUSENotFound
}

//
type MountInfo struct {
	MountPoint     string
	StartManifest  string
	LatestManifest string
	rootDir        *SwarmDir
	fuseConnection *fuse.Conn
	swarmApi       *api.API
	lock           *sync.RWMutex
	serveClose     chan struct{}
}

func NewMountInfo(mhash, mpoint string, sapi *api.API) *MountInfo {
	log.Debug("swarmfs NewMountInfo", "hash", mhash, "mount point", mpoint)
	newMountInfo := &MountInfo{
		MountPoint:     mpoint,
		StartManifest:  mhash,
		LatestManifest: mhash,
		rootDir:        nil,
		fuseConnection: nil,
		swarmApi:       sapi,
		lock:           &sync.RWMutex{},
		serveClose:     make(chan struct{}),
	}
	return newMountInfo
}

func (swarmfs *SwarmFS) Mount(mhash, mountpoint string) (*MountInfo, error) {
	log.Info("swarmfs", "mounting hash", mhash, "mount point", mountpoint)
	if mountpoint == "" {
		return nil, errEmptyMountPoint
	}
	if !strings.HasPrefix(mountpoint, "/") {
		return nil, errNoRelativeMountPoint
	}
	cleanedMountPoint, err := filepath.Abs(filepath.Clean(mountpoint))
	if err != nil {
		return nil, err
	}
	log.Trace("swarmfs mount", "cleanedMountPoint", cleanedMountPoint)

	swarmfs.swarmFsLock.Lock()
	defer swarmfs.swarmFsLock.Unlock()

	noOfActiveMounts := len(swarmfs.activeMounts)
	log.Debug("swarmfs mount", "# active mounts", noOfActiveMounts)
	if noOfActiveMounts >= maxFuseMounts {
		return nil, errMaxMountCount
	}

	if _, ok := swarmfs.activeMounts[cleanedMountPoint]; ok {
		return nil, errAlreadyMounted
	}

	log.Trace("swarmfs mount: getting manifest tree")
	_, manifestEntryMap, err := swarmfs.swarmApi.BuildDirectoryTree(context.TODO(), mhash, true)
	if err != nil {
		return nil, err
	}

	log.Trace("swarmfs mount: building mount info")
	mi := NewMountInfo(mhash, cleanedMountPoint, swarmfs.swarmApi)

	dirTree := map[string]*SwarmDir{}
	rootDir := NewSwarmDir("/", mi)
	log.Trace("swarmfs mount", "rootDir", rootDir)
	mi.rootDir = rootDir

	log.Trace("swarmfs mount: traversing manifest map")
	for suffix, entry := range manifestEntryMap {
if suffix == "" { //
			log.Warn("Manifest has an empty-path (default) entry which will be ignored in FUSE mount.")
			continue
		}
		addr := common.Hex2Bytes(entry.Hash)
		fullpath := "/" + suffix
		basepath := filepath.Dir(fullpath)
		parentDir := rootDir
		dirUntilNow := ""
		paths := strings.Split(basepath, "/")
		for i := range paths {
			if paths[i] != "" {
				thisDir := paths[i]
				dirUntilNow = dirUntilNow + "/" + thisDir

				if _, ok := dirTree[dirUntilNow]; !ok {
					dirTree[dirUntilNow] = NewSwarmDir(dirUntilNow, mi)
					parentDir.directories = append(parentDir.directories, dirTree[dirUntilNow])
					parentDir = dirTree[dirUntilNow]

				} else {
					parentDir = dirTree[dirUntilNow]
				}
			}
		}
		thisFile := NewSwarmFile(basepath, filepath.Base(fullpath), mi)
		thisFile.addr = addr

		parentDir.files = append(parentDir.files, thisFile)
	}

	fconn, err := fuse.Mount(cleanedMountPoint, fuse.FSName("swarmfs"), fuse.VolumeName(mhash))
	if isFUSEUnsupportedError(err) {
		log.Error("swarmfs error - FUSE not installed", "mountpoint", cleanedMountPoint, "err", err)
		return nil, err
	} else if err != nil {
		fuse.Unmount(cleanedMountPoint)
		log.Error("swarmfs error mounting swarm manifest", "mountpoint", cleanedMountPoint, "err", err)
		return nil, err
	}
	mi.fuseConnection = fconn

	serverr := make(chan error, 1)
	go func() {
		log.Info("swarmfs", "serving hash", mhash, "at", cleanedMountPoint)
		filesys := &SwarmRoot{root: rootDir}
//
		if err := fs.Serve(fconn, filesys); err != nil {
			log.Warn("swarmfs could not serve the requested hash", "error", err)
			serverr <- err
		}
		mi.serveClose <- struct{}{}
	}()

 /*
    
    
    
    

    
    
    

    
    
    

    
    

    
    
    
 */

	time.Sleep(2 * time.Second)

	timer := time.NewTimer(mountTimeout)
	defer timer.Stop()
//
	select {
	case <-timer.C:
		log.Warn("swarmfs timed out mounting over FUSE", "mountpoint", cleanedMountPoint, "err", err)
		err := fuse.Unmount(cleanedMountPoint)
		if err != nil {
			return nil, err
		}
		return nil, errMountTimeout
	case err := <-serverr:
		log.Warn("swarmfs error serving over FUSE", "mountpoint", cleanedMountPoint, "err", err)
		err = fuse.Unmount(cleanedMountPoint)
		return nil, err

	case <-fconn.Ready:
//
//
		if err := fconn.MountError; err != nil {
			log.Error("Mounting error from fuse driver: ", "err", err)
			return nil, err
		}
		log.Info("swarmfs now served over FUSE", "manifest", mhash, "mountpoint", cleanedMountPoint)
	}

	timer.Stop()
	swarmfs.activeMounts[cleanedMountPoint] = mi
	return mi, nil
}

func (swarmfs *SwarmFS) Unmount(mountpoint string) (*MountInfo, error) {
	swarmfs.swarmFsLock.Lock()
	defer swarmfs.swarmFsLock.Unlock()

	cleanedMountPoint, err := filepath.Abs(filepath.Clean(mountpoint))
	if err != nil {
		return nil, err
	}

	mountInfo := swarmfs.activeMounts[cleanedMountPoint]

	if mountInfo == nil || mountInfo.MountPoint != cleanedMountPoint {
		return nil, fmt.Errorf("swarmfs %s is not mounted", cleanedMountPoint)
	}
	err = fuse.Unmount(cleanedMountPoint)
	if err != nil {
		err1 := externalUnmount(cleanedMountPoint)
		if err1 != nil {
			errStr := fmt.Sprintf("swarmfs unmount error: %v", err)
			log.Warn(errStr)
			return nil, err1
		}
	}

	err = mountInfo.fuseConnection.Close()
	if err != nil {
		return nil, err
	}
	delete(swarmfs.activeMounts, cleanedMountPoint)

	<-mountInfo.serveClose

	succString := fmt.Sprintf("swarmfs unmounting %v succeeded", cleanedMountPoint)
	log.Info(succString)

	return mountInfo, nil
}

func (swarmfs *SwarmFS) Listmounts() []*MountInfo {
	swarmfs.swarmFsLock.RLock()
	defer swarmfs.swarmFsLock.RUnlock()
	rows := make([]*MountInfo, 0, len(swarmfs.activeMounts))
	for _, mi := range swarmfs.activeMounts {
		rows = append(rows, mi)
	}
	return rows
}

func (swarmfs *SwarmFS) Stop() bool {
	for mp := range swarmfs.activeMounts {
		mountInfo := swarmfs.activeMounts[mp]
		swarmfs.Unmount(mountInfo.MountPoint)
	}
	return true
}
