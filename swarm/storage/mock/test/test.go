
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
//
package test

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/mock"
)

//
//
//
//
func MockStore(t *testing.T, globalStore mock.GlobalStorer, n int) {
	t.Run("GlobalStore", func(t *testing.T) {
		addrs := make([]common.Address, n)
		for i := 0; i < n; i++ {
			addrs[i] = common.HexToAddress(strconv.FormatInt(int64(i)+1, 16))
		}

		for i, addr := range addrs {
			chunkAddr := storage.Address(append(addr[:], []byte(strconv.FormatInt(int64(i)+1, 16))...))
			data := []byte(strconv.FormatInt(int64(i)+1, 16))
			data = append(data, make([]byte, 4096-len(data))...)
			globalStore.Put(addr, chunkAddr, data)

			for _, cAddr := range addrs {
				cData, err := globalStore.Get(cAddr, chunkAddr)
				if cAddr == addr {
					if err != nil {
						t.Fatalf("get data from store %s key %s: %v", cAddr.Hex(), chunkAddr.Hex(), err)
					}
					if !bytes.Equal(data, cData) {
						t.Fatalf("data on store %s: expected %x, got %x", cAddr.Hex(), data, cData)
					}
					if !globalStore.HasKey(cAddr, chunkAddr) {
						t.Fatalf("expected key %s on global store for node %s, but it was not found", chunkAddr.Hex(), cAddr.Hex())
					}
				} else {
					if err != mock.ErrNotFound {
						t.Fatalf("expected error from store %s: %v, got %v", cAddr.Hex(), mock.ErrNotFound, err)
					}
					if len(cData) > 0 {
						t.Fatalf("data on store %s: expected nil, got %x", cAddr.Hex(), cData)
					}
					if globalStore.HasKey(cAddr, chunkAddr) {
						t.Fatalf("not expected key %s on global store for node %s, but it was found", chunkAddr.Hex(), cAddr.Hex())
					}
				}
			}
		}
	})

	t.Run("NodeStore", func(t *testing.T) {
		nodes := make(map[common.Address]*mock.NodeStore)
		for i := 0; i < n; i++ {
			addr := common.HexToAddress(strconv.FormatInt(int64(i)+1, 16))
			nodes[addr] = globalStore.NewNodeStore(addr)
		}

		i := 0
		for addr, store := range nodes {
			i++
			chunkAddr := storage.Address(append(addr[:], []byte(fmt.Sprintf("%x", i))...))
			data := []byte(strconv.FormatInt(int64(i)+1, 16))
			data = append(data, make([]byte, 4096-len(data))...)
			store.Put(chunkAddr, data)

			for cAddr, cStore := range nodes {
				cData, err := cStore.Get(chunkAddr)
				if cAddr == addr {
					if err != nil {
						t.Fatalf("get data from store %s key %s: %v", cAddr.Hex(), chunkAddr.Hex(), err)
					}
					if !bytes.Equal(data, cData) {
						t.Fatalf("data on store %s: expected %x, got %x", cAddr.Hex(), data, cData)
					}
					if !globalStore.HasKey(cAddr, chunkAddr) {
						t.Fatalf("expected key %s on global store for node %s, but it was not found", chunkAddr.Hex(), cAddr.Hex())
					}
				} else {
					if err != mock.ErrNotFound {
						t.Fatalf("expected error from store %s: %v, got %v", cAddr.Hex(), mock.ErrNotFound, err)
					}
					if len(cData) > 0 {
						t.Fatalf("data on store %s: expected nil, got %x", cAddr.Hex(), cData)
					}
					if globalStore.HasKey(cAddr, chunkAddr) {
						t.Fatalf("not expected key %s on global store for node %s, but it was found", chunkAddr.Hex(), cAddr.Hex())
					}
				}
			}
		}
	})
}

//
//
func ImportExport(t *testing.T, outStore, inStore mock.GlobalStorer, n int) {
	exporter, ok := outStore.(mock.Exporter)
	if !ok {
		t.Fatal("outStore does not implement mock.Exporter")
	}
	importer, ok := inStore.(mock.Importer)
	if !ok {
		t.Fatal("inStore does not implement mock.Importer")
	}
	addrs := make([]common.Address, n)
	for i := 0; i < n; i++ {
		addrs[i] = common.HexToAddress(strconv.FormatInt(int64(i)+1, 16))
	}

	for i, addr := range addrs {
		chunkAddr := storage.Address(append(addr[:], []byte(strconv.FormatInt(int64(i)+1, 16))...))
		data := []byte(strconv.FormatInt(int64(i)+1, 16))
		data = append(data, make([]byte, 4096-len(data))...)
		outStore.Put(addr, chunkAddr, data)
	}

	r, w := io.Pipe()
	defer r.Close()

	go func() {
		defer w.Close()
		if _, err := exporter.Export(w); err != nil {
			t.Fatalf("export: %v", err)
		}
	}()

	if _, err := importer.Import(r); err != nil {
		t.Fatalf("import: %v", err)
	}

	for i, addr := range addrs {
		chunkAddr := storage.Address(append(addr[:], []byte(strconv.FormatInt(int64(i)+1, 16))...))
		data := []byte(strconv.FormatInt(int64(i)+1, 16))
		data = append(data, make([]byte, 4096-len(data))...)
		for _, cAddr := range addrs {
			cData, err := inStore.Get(cAddr, chunkAddr)
			if cAddr == addr {
				if err != nil {
					t.Fatalf("get data from store %s key %s: %v", cAddr.Hex(), chunkAddr.Hex(), err)
				}
				if !bytes.Equal(data, cData) {
					t.Fatalf("data on store %s: expected %x, got %x", cAddr.Hex(), data, cData)
				}
				if !inStore.HasKey(cAddr, chunkAddr) {
					t.Fatalf("expected key %s on global store for node %s, but it was not found", chunkAddr.Hex(), cAddr.Hex())
				}
			} else {
				if err != mock.ErrNotFound {
					t.Fatalf("expected error from store %s: %v, got %v", cAddr.Hex(), mock.ErrNotFound, err)
				}
				if len(cData) > 0 {
					t.Fatalf("data on store %s: expected nil, got %x", cAddr.Hex(), cData)
				}
				if inStore.HasKey(cAddr, chunkAddr) {
					t.Fatalf("not expected key %s on global store for node %s, but it was found", chunkAddr.Hex(), cAddr.Hex())
				}
			}
		}
	}
}
