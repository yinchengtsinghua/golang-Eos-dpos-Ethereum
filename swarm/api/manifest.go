
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

const (
	ManifestType        = "application/bzz-manifest+json"
	ResourceContentType = "application/bzz-resource"

	manifestSizeLimit = 5 * 1024 * 1024
)

//
type Manifest struct {
	Entries []ManifestEntry `json:"entries,omitempty"`
}

//
type ManifestEntry struct {
	Hash        string       `json:"hash,omitempty"`
	Path        string       `json:"path,omitempty"`
	ContentType string       `json:"contentType,omitempty"`
	Mode        int64        `json:"mode,omitempty"`
	Size        int64        `json:"size,omitempty"`
	ModTime     time.Time    `json:"mod_time,omitempty"`
	Status      int          `json:"status,omitempty"`
	Access      *AccessEntry `json:"access,omitempty"`
}

//
type ManifestList struct {
	CommonPrefixes []string         `json:"common_prefixes,omitempty"`
	Entries        []*ManifestEntry `json:"entries,omitempty"`
}

//
func (a *API) NewManifest(ctx context.Context, toEncrypt bool) (storage.Address, error) {
	var manifest Manifest
	data, err := json.Marshal(&manifest)
	if err != nil {
		return nil, err
	}
	key, wait, err := a.Store(ctx, bytes.NewReader(data), int64(len(data)), toEncrypt)
	wait(ctx)
	return key, err
}

//
//
func (a *API) NewResourceManifest(ctx context.Context, resourceAddr string) (storage.Address, error) {
	var manifest Manifest
	entry := ManifestEntry{
		Hash:        resourceAddr,
		ContentType: ResourceContentType,
	}
	manifest.Entries = append(manifest.Entries, entry)
	data, err := json.Marshal(&manifest)
	if err != nil {
		return nil, err
	}
	key, _, err := a.Store(ctx, bytes.NewReader(data), int64(len(data)), false)
	return key, err
}

//
type ManifestWriter struct {
	api   *API
	trie  *manifestTrie
	quitC chan bool
}

func (a *API) NewManifestWriter(ctx context.Context, addr storage.Address, quitC chan bool) (*ManifestWriter, error) {
	trie, err := loadManifest(ctx, a.fileStore, addr, quitC, NOOPDecrypt)
	if err != nil {
		return nil, fmt.Errorf("error loading manifest %s: %s", addr, err)
	}
	return &ManifestWriter{a, trie, quitC}, nil
}

//
func (m *ManifestWriter) AddEntry(ctx context.Context, data io.Reader, e *ManifestEntry) (key storage.Address, err error) {
	entry := newManifestTrieEntry(e, nil)
	if data != nil {
		key, _, err = m.api.Store(ctx, data, e.Size, m.trie.encrypted)
		if err != nil {
			return nil, err
		}
		entry.Hash = key.Hex()
	}
	if entry.Hash == "" {
		return key, errors.New("missing entry hash")
	}
	m.trie.addEntry(entry, m.quitC)
	return key, nil
}

//
func (m *ManifestWriter) RemoveEntry(path string) error {
	m.trie.deleteEntry(path, m.quitC)
	return nil
}

//
func (m *ManifestWriter) Store() (storage.Address, error) {
	return m.trie.ref, m.trie.recalcAndStore()
}

//
//
type ManifestWalker struct {
	api   *API
	trie  *manifestTrie
	quitC chan bool
}

func (a *API) NewManifestWalker(ctx context.Context, addr storage.Address, decrypt DecryptFunc, quitC chan bool) (*ManifestWalker, error) {
	trie, err := loadManifest(ctx, a.fileStore, addr, quitC, decrypt)
	if err != nil {
		return nil, fmt.Errorf("error loading manifest %s: %s", addr, err)
	}
	return &ManifestWalker{a, trie, quitC}, nil
}

//
//
var ErrSkipManifest = errors.New("skip this manifest")

//
//
type WalkFn func(entry *ManifestEntry) error

//
//
func (m *ManifestWalker) Walk(walkFn WalkFn) error {
	return m.walk(m.trie, "", walkFn)
}

func (m *ManifestWalker) walk(trie *manifestTrie, prefix string, walkFn WalkFn) error {
	for _, entry := range &trie.entries {
		if entry == nil {
			continue
		}
		entry.Path = prefix + entry.Path
		err := walkFn(&entry.ManifestEntry)
		if err != nil {
			if entry.ContentType == ManifestType && err == ErrSkipManifest {
				continue
			}
			return err
		}
		if entry.ContentType != ManifestType {
			continue
		}
		if err := trie.loadSubTrie(entry, nil); err != nil {
			return err
		}
		if err := m.walk(entry.subtrie, entry.Path, walkFn); err != nil {
			return err
		}
	}
	return nil
}

type manifestTrie struct {
	fileStore *storage.FileStore
entries   [257]*manifestTrieEntry //
ref       storage.Address         //
	encrypted bool
	decrypt   DecryptFunc
}

func newManifestTrieEntry(entry *ManifestEntry, subtrie *manifestTrie) *manifestTrieEntry {
	return &manifestTrieEntry{
		ManifestEntry: *entry,
		subtrie:       subtrie,
	}
}

type manifestTrieEntry struct {
	ManifestEntry

	subtrie *manifestTrie
}

func loadManifest(ctx context.Context, fileStore *storage.FileStore, hash storage.Address, quitC chan bool, decrypt DecryptFunc) (trie *manifestTrie, err error) { //
	log.Trace("manifest lookup", "key", hash)
//
	manifestReader, isEncrypted := fileStore.Retrieve(ctx, hash)
	log.Trace("reader retrieved", "key", hash)
	return readManifest(manifestReader, hash, fileStore, isEncrypted, quitC, decrypt)
}

func readManifest(mr storage.LazySectionReader, hash storage.Address, fileStore *storage.FileStore, isEncrypted bool, quitC chan bool, decrypt DecryptFunc) (trie *manifestTrie, err error) { //

//
	size, err := mr.Size(mr.Context(), quitC)
if err != nil { //
//
		log.Trace("manifest not found", "key", hash)
		err = fmt.Errorf("Manifest not Found")
		return
	}
	if size > manifestSizeLimit {
		log.Warn("manifest exceeds size limit", "key", hash, "size", size, "limit", manifestSizeLimit)
		err = fmt.Errorf("Manifest size of %v bytes exceeds the %v byte limit", size, manifestSizeLimit)
		return
	}
	manifestData := make([]byte, size)
	read, err := mr.Read(manifestData)
	if int64(read) < size {
		log.Trace("manifest not found", "key", hash)
		if err == nil {
			err = fmt.Errorf("Manifest retrieval cut short: read %v, expect %v", read, size)
		}
		return
	}

	log.Debug("manifest retrieved", "key", hash)
	var man struct {
		Entries []*manifestTrieEntry `json:"entries"`
	}
	err = json.Unmarshal(manifestData, &man)
	if err != nil {
		err = fmt.Errorf("Manifest %v is malformed: %v", hash.Log(), err)
		log.Trace("malformed manifest", "key", hash)
		return
	}

	log.Trace("manifest entries", "key", hash, "len", len(man.Entries))

	trie = &manifestTrie{
		fileStore: fileStore,
		encrypted: isEncrypted,
		decrypt:   decrypt,
	}
	for _, entry := range man.Entries {
		err = trie.addEntry(entry, quitC)
		if err != nil {
			return
		}
	}
	return
}

func (mt *manifestTrie) addEntry(entry *manifestTrieEntry, quitC chan bool) error {
mt.ref = nil //

	if entry.ManifestEntry.Access != nil {
		if mt.decrypt == nil {
			return errors.New("dont have decryptor")
		}

		err := mt.decrypt(&entry.ManifestEntry)
		if err != nil {
			return err
		}
	}

	if len(entry.Path) == 0 {
		mt.entries[256] = entry
		return nil
	}

	b := entry.Path[0]
	oldentry := mt.entries[b]
	if (oldentry == nil) || (oldentry.Path == entry.Path && oldentry.ContentType != ManifestType) {
		mt.entries[b] = entry
		return nil
	}

	cpl := 0
	for (len(entry.Path) > cpl) && (len(oldentry.Path) > cpl) && (entry.Path[cpl] == oldentry.Path[cpl]) {
		cpl++
	}

	if (oldentry.ContentType == ManifestType) && (cpl == len(oldentry.Path)) {
		if mt.loadSubTrie(oldentry, quitC) != nil {
			return nil
		}
		entry.Path = entry.Path[cpl:]
		oldentry.subtrie.addEntry(entry, quitC)
		oldentry.Hash = ""
		return nil
	}

	commonPrefix := entry.Path[:cpl]

	subtrie := &manifestTrie{
		fileStore: mt.fileStore,
		encrypted: mt.encrypted,
	}
	entry.Path = entry.Path[cpl:]
	oldentry.Path = oldentry.Path[cpl:]
	subtrie.addEntry(entry, quitC)
	subtrie.addEntry(oldentry, quitC)

	mt.entries[b] = newManifestTrieEntry(&ManifestEntry{
		Path:        commonPrefix,
		ContentType: ManifestType,
	}, subtrie)
	return nil
}

func (mt *manifestTrie) getCountLast() (cnt int, entry *manifestTrieEntry) {
	for _, e := range &mt.entries {
		if e != nil {
			cnt++
			entry = e
		}
	}
	return
}

func (mt *manifestTrie) deleteEntry(path string, quitC chan bool) {
mt.ref = nil //

	if len(path) == 0 {
		mt.entries[256] = nil
		return
	}

	b := path[0]
	entry := mt.entries[b]
	if entry == nil {
		return
	}
	if entry.Path == path {
		mt.entries[b] = nil
		return
	}

	epl := len(entry.Path)
	if (entry.ContentType == ManifestType) && (len(path) >= epl) && (path[:epl] == entry.Path) {
		if mt.loadSubTrie(entry, quitC) != nil {
			return
		}
		entry.subtrie.deleteEntry(path[epl:], quitC)
		entry.Hash = ""
//
		cnt, lastentry := entry.subtrie.getCountLast()
		if cnt < 2 {
			if lastentry != nil {
				lastentry.Path = entry.Path + lastentry.Path
			}
			mt.entries[b] = lastentry
		}
	}
}

func (mt *manifestTrie) recalcAndStore() error {
	if mt.ref != nil {
		return nil
	}

	var buffer bytes.Buffer
	buffer.WriteString(`{"entries":[`)

	list := &Manifest{}
	for _, entry := range &mt.entries {
		if entry != nil {
if entry.Hash == "" { //
				err := entry.subtrie.recalcAndStore()
				if err != nil {
					return err
				}
				entry.Hash = entry.subtrie.ref.Hex()
			}
			list.Entries = append(list.Entries, entry.ManifestEntry)
		}

	}

	manifest, err := json.Marshal(list)
	if err != nil {
		return err
	}

	sr := bytes.NewReader(manifest)
	ctx := context.TODO()
	key, wait, err2 := mt.fileStore.Store(ctx, sr, int64(len(manifest)), mt.encrypted)
	if err2 != nil {
		return err2
	}
	err2 = wait(ctx)
	mt.ref = key
	return err2
}

func (mt *manifestTrie) loadSubTrie(entry *manifestTrieEntry, quitC chan bool) (err error) {
	if entry.ManifestEntry.Access != nil {
		if mt.decrypt == nil {
			return errors.New("dont have decryptor")
		}

		err := mt.decrypt(&entry.ManifestEntry)
		if err != nil {
			return err
		}
	}

	if entry.subtrie == nil {
		hash := common.Hex2Bytes(entry.Hash)
		entry.subtrie, err = loadManifest(context.TODO(), mt.fileStore, hash, quitC, mt.decrypt)
entry.Hash = "" //
	}
	return
}

func (mt *manifestTrie) listWithPrefixInt(prefix, rp string, quitC chan bool, cb func(entry *manifestTrieEntry, suffix string)) error {
	plen := len(prefix)
	var start, stop int
	if plen == 0 {
		start = 0
		stop = 256
	} else {
		start = int(prefix[0])
		stop = start
	}

	for i := start; i <= stop; i++ {
		select {
		case <-quitC:
			return fmt.Errorf("aborted")
		default:
		}
		entry := mt.entries[i]
		if entry != nil {
			epl := len(entry.Path)
			if entry.ContentType == ManifestType {
				l := plen
				if epl < l {
					l = epl
				}
				if prefix[:l] == entry.Path[:l] {
					err := mt.loadSubTrie(entry, quitC)
					if err != nil {
						return err
					}
					err = entry.subtrie.listWithPrefixInt(prefix[l:], rp+entry.Path[l:], quitC, cb)
					if err != nil {
						return err
					}
				}
			} else {
				if (epl >= plen) && (prefix == entry.Path[:plen]) {
					cb(entry, rp+entry.Path[plen:])
				}
			}
		}
	}
	return nil
}

func (mt *manifestTrie) listWithPrefix(prefix string, quitC chan bool, cb func(entry *manifestTrieEntry, suffix string)) (err error) {
	return mt.listWithPrefixInt(prefix, "", quitC, cb)
}

func (mt *manifestTrie) findPrefixOf(path string, quitC chan bool) (entry *manifestTrieEntry, pos int) {
	log.Trace(fmt.Sprintf("findPrefixOf(%s)", path))

	if len(path) == 0 {
		return mt.entries[256], 0
	}

//
	b := path[0]
	entry = mt.entries[b]
	if entry == nil {
		return mt.entries[256], 0
	}

	epl := len(entry.Path)
	log.Trace(fmt.Sprintf("path = %v  entry.Path = %v  epl = %v", path, entry.Path, epl))
	if len(path) <= epl {
		if entry.Path[:len(path)] == path {
			if entry.ContentType == ManifestType {
				err := mt.loadSubTrie(entry, quitC)
				if err == nil && entry.subtrie != nil {
					subentries := entry.subtrie.entries
					for i := 0; i < len(subentries); i++ {
						sub := subentries[i]
						if sub != nil && sub.Path == "" {
							return sub, len(path)
						}
					}
				}
				entry.Status = http.StatusMultipleChoices
			}
			pos = len(path)
			return
		}
		return nil, 0
	}
	if path[:epl] == entry.Path {
		log.Trace(fmt.Sprintf("entry.ContentType = %v", entry.ContentType))
//
		if entry.ContentType == ManifestType && (strings.Contains(entry.Path, path) || strings.Contains(path, entry.Path)) {
			err := mt.loadSubTrie(entry, quitC)
			if err != nil {
				return nil, 0
			}
			sub, pos := entry.subtrie.findPrefixOf(path[epl:], quitC)
			if sub != nil {
				entry = sub
				pos += epl
				return sub, pos
			} else if path == entry.Path {
				entry.Status = http.StatusMultipleChoices
			}

		} else {
//
			if path != entry.Path {
				return nil, 0
			}
			pos = epl
		}
	}
	return nil, 0
}

//
//
func RegularSlashes(path string) (res string) {
	for i := 0; i < len(path); i++ {
		if (path[i] != '/') || ((i > 0) && (path[i-1] != '/')) {
			res = res + path[i:i+1]
		}
	}
	if (len(res) > 0) && (res[len(res)-1] == '/') {
		res = res[:len(res)-1]
	}
	return
}

func (mt *manifestTrie) getEntry(spath string) (entry *manifestTrieEntry, fullpath string) {
	path := RegularSlashes(spath)
	var pos int
	quitC := make(chan bool)
	entry, pos = mt.findPrefixOf(path, quitC)
	return entry, path[:pos]
}
