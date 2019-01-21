
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

package trie

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	memcacheFlushTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/flush/time", nil)
	memcacheFlushNodesMeter = metrics.NewRegisteredMeter("trie/memcache/flush/nodes", nil)
	memcacheFlushSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/flush/size", nil)

	memcacheGCTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/gc/time", nil)
	memcacheGCNodesMeter = metrics.NewRegisteredMeter("trie/memcache/gc/nodes", nil)
	memcacheGCSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/gc/size", nil)

	memcacheCommitTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/commit/time", nil)
	memcacheCommitNodesMeter = metrics.NewRegisteredMeter("trie/memcache/commit/nodes", nil)
	memcacheCommitSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/commit/size", nil)
)

//
var secureKeyPrefix = []byte("secure-key-")

//
const secureKeyLength = 11 + 32

//
type DatabaseReader interface {
//
	Get(key []byte) (value []byte, err error)

//
	Has(key []byte) (bool, error)
}

//
//
//
type Database struct {
diskdb ethdb.Database //

nodes  map[common.Hash]*cachedNode //
oldest common.Hash                 //
newest common.Hash                 //

preimages map[common.Hash][]byte //
seckeybuf [secureKeyLength]byte  //

gctime  time.Duration      //
gcnodes uint64             //
gcsize  common.StorageSize //

flushtime  time.Duration      //
flushnodes uint64             //
flushsize  common.StorageSize //

nodesSize     common.StorageSize //
preimagesSize common.StorageSize //

	lock sync.RWMutex
}

//
//
//
type rawNode []byte

func (n rawNode) canUnload(uint16, uint16) bool { panic("this should never end up in a live trie") }
func (n rawNode) cache() (hashNode, bool)       { panic("this should never end up in a live trie") }
func (n rawNode) fstring(ind string) string     { panic("this should never end up in a live trie") }

//
//
//
type rawFullNode [17]node

func (n rawFullNode) canUnload(uint16, uint16) bool { panic("this should never end up in a live trie") }
func (n rawFullNode) cache() (hashNode, bool)       { panic("this should never end up in a live trie") }
func (n rawFullNode) fstring(ind string) string     { panic("this should never end up in a live trie") }

func (n rawFullNode) EncodeRLP(w io.Writer) error {
	var nodes [17]node

	for i, child := range n {
		if child != nil {
			nodes[i] = child
		} else {
			nodes[i] = nilValueNode
		}
	}
	return rlp.Encode(w, nodes)
}

//
//
//
type rawShortNode struct {
	Key []byte
	Val node
}

func (n rawShortNode) canUnload(uint16, uint16) bool { panic("this should never end up in a live trie") }
func (n rawShortNode) cache() (hashNode, bool)       { panic("this should never end up in a live trie") }
func (n rawShortNode) fstring(ind string) string     { panic("this should never end up in a live trie") }

//
//
type cachedNode struct {
node node   //
size uint16 //

parents  uint16                 //
children map[common.Hash]uint16 //

flushPrev common.Hash //
flushNext common.Hash //
}

//
//
func (n *cachedNode) rlp() []byte {
	if node, ok := n.node.(rawNode); ok {
		return node
	}
	blob, err := rlp.EncodeToBytes(n.node)
	if err != nil {
		panic(err)
	}
	return blob
}

//
//
func (n *cachedNode) obj(hash common.Hash, cachegen uint16) node {
	if node, ok := n.node.(rawNode); ok {
		return mustDecodeNode(hash[:], node, cachegen)
	}
	return expandNode(hash[:], n.node, cachegen)
}

//
//
func (n *cachedNode) childs() []common.Hash {
	children := make([]common.Hash, 0, 16)
	for child := range n.children {
		children = append(children, child)
	}
	if _, ok := n.node.(rawNode); !ok {
		gatherChildren(n.node, &children)
	}
	return children
}

//
//
func gatherChildren(n node, children *[]common.Hash) {
	switch n := n.(type) {
	case *rawShortNode:
		gatherChildren(n.Val, children)

	case rawFullNode:
		for i := 0; i < 16; i++ {
			gatherChildren(n[i], children)
		}
	case hashNode:
		*children = append(*children, common.BytesToHash(n))

	case valueNode, nil:

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

//
//
func simplifyNode(n node) node {
	switch n := n.(type) {
	case *shortNode:
//
		return &rawShortNode{Key: n.Key, Val: simplifyNode(n.Val)}

	case *fullNode:
//
		node := rawFullNode(n.Children)
		for i := 0; i < len(node); i++ {
			if node[i] != nil {
				node[i] = simplifyNode(node[i])
			}
		}
		return node

	case valueNode, hashNode, rawNode:
		return n

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

//
//
func expandNode(hash hashNode, n node, cachegen uint16) node {
	switch n := n.(type) {
	case *rawShortNode:
//
		return &shortNode{
			Key: compactToHex(n.Key),
			Val: expandNode(nil, n.Val, cachegen),
			flags: nodeFlag{
				hash: hash,
				gen:  cachegen,
			},
		}

	case rawFullNode:
//
		node := &fullNode{
			flags: nodeFlag{
				hash: hash,
				gen:  cachegen,
			},
		}
		for i := 0; i < len(node.Children); i++ {
			if n[i] != nil {
				node.Children[i] = expandNode(nil, n[i], cachegen)
			}
		}
		return node

	case valueNode, hashNode:
		return n

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

//
//
func NewDatabase(diskdb ethdb.Database) *Database {
	return &Database{
		diskdb:    diskdb,
		nodes:     map[common.Hash]*cachedNode{{}: {}},
		preimages: make(map[common.Hash][]byte),
	}
}

//
func (db *Database) DiskDB() DatabaseReader {
	return db.diskdb
}

//
//
//
//
func (db *Database) InsertBlob(hash common.Hash, blob []byte) {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.insert(hash, blob, rawNode(blob))
}

//
//
//
//
func (db *Database) insert(hash common.Hash, blob []byte, node node) {
//
	if _, ok := db.nodes[hash]; ok {
		return
	}
//
	entry := &cachedNode{
		node:      simplifyNode(node),
		size:      uint16(len(blob)),
		flushPrev: db.newest,
	}
	for _, child := range entry.childs() {
		if c := db.nodes[child]; c != nil {
			c.parents++
		}
	}
	db.nodes[hash] = entry

//
	if db.oldest == (common.Hash{}) {
		db.oldest, db.newest = hash, hash
	} else {
		db.nodes[db.newest].flushNext, db.newest = hash, hash
	}
	db.nodesSize += common.StorageSize(common.HashLength + entry.size)
}

//
//
//
//
func (db *Database) insertPreimage(hash common.Hash, preimage []byte) {
	if _, ok := db.preimages[hash]; ok {
		return
	}
	db.preimages[hash] = common.CopyBytes(preimage)
	db.preimagesSize += common.StorageSize(common.HashLength + len(preimage))
}

//
//
func (db *Database) node(hash common.Hash, cachegen uint16) node {
//
	db.lock.RLock()
	node := db.nodes[hash]
	db.lock.RUnlock()

	if node != nil {
		return node.obj(hash, cachegen)
	}
//
	enc, err := db.diskdb.Get(hash[:])
	if err != nil || enc == nil {
		return nil
	}
	return mustDecodeNode(hash[:], enc, cachegen)
}

//
//
func (db *Database) Node(hash common.Hash) ([]byte, error) {
//
	db.lock.RLock()
	node := db.nodes[hash]
	db.lock.RUnlock()

	if node != nil {
		return node.rlp(), nil
	}
//
	return db.diskdb.Get(hash[:])
}

//
//
func (db *Database) preimage(hash common.Hash) ([]byte, error) {
//
	db.lock.RLock()
	preimage := db.preimages[hash]
	db.lock.RUnlock()

	if preimage != nil {
		return preimage, nil
	}
//
	return db.diskdb.Get(db.secureKey(hash[:]))
}

//
//
//
func (db *Database) secureKey(key []byte) []byte {
	buf := append(db.seckeybuf[:0], secureKeyPrefix...)
	buf = append(buf, key...)
	return buf
}

//
//
//
func (db *Database) Nodes() []common.Hash {
	db.lock.RLock()
	defer db.lock.RUnlock()

	var hashes = make([]common.Hash, 0, len(db.nodes))
	for hash := range db.nodes {
if hash != (common.Hash{}) { //
			hashes = append(hashes, hash)
		}
	}
	return hashes
}

//
func (db *Database) Reference(child common.Hash, parent common.Hash) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	db.reference(child, parent)
}

//
func (db *Database) reference(child common.Hash, parent common.Hash) {
//
	node, ok := db.nodes[child]
	if !ok {
		return
	}
//
	if db.nodes[parent].children == nil {
		db.nodes[parent].children = make(map[common.Hash]uint16)
	} else if _, ok = db.nodes[parent].children[child]; ok && parent != (common.Hash{}) {
		return
	}
	node.parents++
	db.nodes[parent].children[child]++
}

//
func (db *Database) Dereference(root common.Hash) {
//
	if root == (common.Hash{}) {
		log.Error("Attempted to dereference the trie cache meta root")
		return
	}
	db.lock.Lock()
	defer db.lock.Unlock()

	nodes, storage, start := len(db.nodes), db.nodesSize, time.Now()
	db.dereference(root, common.Hash{})

	db.gcnodes += uint64(nodes - len(db.nodes))
	db.gcsize += storage - db.nodesSize
	db.gctime += time.Since(start)

	memcacheGCTimeTimer.Update(time.Since(start))
	memcacheGCSizeMeter.Mark(int64(storage - db.nodesSize))
	memcacheGCNodesMeter.Mark(int64(nodes - len(db.nodes)))

	log.Debug("Dereferenced trie from memory database", "nodes", nodes-len(db.nodes), "size", storage-db.nodesSize, "time", time.Since(start),
		"gcnodes", db.gcnodes, "gcsize", db.gcsize, "gctime", db.gctime, "livenodes", len(db.nodes), "livesize", db.nodesSize)
}

//
func (db *Database) dereference(child common.Hash, parent common.Hash) {
//
	node := db.nodes[parent]

	if node.children != nil && node.children[child] > 0 {
		node.children[child]--
		if node.children[child] == 0 {
			delete(node.children, child)
		}
	}
//
	node, ok := db.nodes[child]
	if !ok {
		return
	}
//
	if node.parents > 0 {
//
//
//
//
		node.parents--
	}
	if node.parents == 0 {
//
		switch child {
		case db.oldest:
			db.oldest = node.flushNext
			db.nodes[node.flushNext].flushPrev = common.Hash{}
		case db.newest:
			db.newest = node.flushPrev
			db.nodes[node.flushPrev].flushNext = common.Hash{}
		default:
			db.nodes[node.flushPrev].flushNext = node.flushNext
			db.nodes[node.flushNext].flushPrev = node.flushPrev
		}
//
		for _, hash := range node.childs() {
			db.dereference(hash, child)
		}
		delete(db.nodes, child)
		db.nodesSize -= common.StorageSize(common.HashLength + int(node.size))
	}
}

//
//
func (db *Database) Cap(limit common.StorageSize) error {
//
//
//
//
	db.lock.RLock()

	nodes, storage, start := len(db.nodes), db.nodesSize, time.Now()
	batch := db.diskdb.NewBatch()

//
//
//
	size := db.nodesSize + common.StorageSize((len(db.nodes)-1)*2*common.HashLength)

//
//
	flushPreimages := db.preimagesSize > 4*1024*1024
	if flushPreimages {
		for hash, preimage := range db.preimages {
			if err := batch.Put(db.secureKey(hash[:]), preimage); err != nil {
				log.Error("Failed to commit preimage from trie database", "err", err)
				db.lock.RUnlock()
				return err
			}
			if batch.ValueSize() > ethdb.IdealBatchSize {
				if err := batch.Write(); err != nil {
					db.lock.RUnlock()
					return err
				}
				batch.Reset()
			}
		}
	}
//
	oldest := db.oldest
	for size > limit && oldest != (common.Hash{}) {
//
		node := db.nodes[oldest]
		if err := batch.Put(oldest[:], node.rlp()); err != nil {
			db.lock.RUnlock()
			return err
		}
//
		if batch.ValueSize() >= ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				log.Error("Failed to write flush list to disk", "err", err)
				db.lock.RUnlock()
				return err
			}
			batch.Reset()
		}
//
//
//
//
		size -= common.StorageSize(3*common.HashLength + int(node.size))
		oldest = node.flushNext
	}
//
	if err := batch.Write(); err != nil {
		log.Error("Failed to write flush list to disk", "err", err)
		db.lock.RUnlock()
		return err
	}
	db.lock.RUnlock()

//
	db.lock.Lock()
	defer db.lock.Unlock()

	if flushPreimages {
		db.preimages = make(map[common.Hash][]byte)
		db.preimagesSize = 0
	}
	for db.oldest != oldest {
		node := db.nodes[db.oldest]
		delete(db.nodes, db.oldest)
		db.oldest = node.flushNext

		db.nodesSize -= common.StorageSize(common.HashLength + int(node.size))
	}
	if db.oldest != (common.Hash{}) {
		db.nodes[db.oldest].flushPrev = common.Hash{}
	}
	db.flushnodes += uint64(nodes - len(db.nodes))
	db.flushsize += storage - db.nodesSize
	db.flushtime += time.Since(start)

	memcacheFlushTimeTimer.Update(time.Since(start))
	memcacheFlushSizeMeter.Mark(int64(storage - db.nodesSize))
	memcacheFlushNodesMeter.Mark(int64(nodes - len(db.nodes)))

	log.Debug("Persisted nodes from memory database", "nodes", nodes-len(db.nodes), "size", storage-db.nodesSize, "time", time.Since(start),
		"flushnodes", db.flushnodes, "flushsize", db.flushsize, "flushtime", db.flushtime, "livenodes", len(db.nodes), "livesize", db.nodesSize)

	return nil
}

//
//
//
//
func (db *Database) Commit(node common.Hash, report bool) error {
//
//
//
//
	db.lock.RLock()

	start := time.Now()
	batch := db.diskdb.NewBatch()

//
	for hash, preimage := range db.preimages {
		if err := batch.Put(db.secureKey(hash[:]), preimage); err != nil {
			log.Error("Failed to commit preimage from trie database", "err", err)
			db.lock.RUnlock()
			return err
		}
		if batch.ValueSize() > ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				return err
			}
			batch.Reset()
		}
	}
//
	nodes, storage := len(db.nodes), db.nodesSize
	if err := db.commit(node, batch); err != nil {
		log.Error("Failed to commit trie from trie database", "err", err)
		db.lock.RUnlock()
		return err
	}
//
	if err := batch.Write(); err != nil {
		log.Error("Failed to write trie to disk", "err", err)
		db.lock.RUnlock()
		return err
	}
	db.lock.RUnlock()

//
	db.lock.Lock()
	defer db.lock.Unlock()

	db.preimages = make(map[common.Hash][]byte)
	db.preimagesSize = 0

	db.uncache(node)

	memcacheCommitTimeTimer.Update(time.Since(start))
	memcacheCommitSizeMeter.Mark(int64(storage - db.nodesSize))
	memcacheCommitNodesMeter.Mark(int64(nodes - len(db.nodes)))

	logger := log.Info
	if !report {
		logger = log.Debug
	}
	logger("Persisted trie from memory database", "nodes", nodes-len(db.nodes)+int(db.flushnodes), "size", storage-db.nodesSize+db.flushsize, "time", time.Since(start)+db.flushtime,
		"gcnodes", db.gcnodes, "gcsize", db.gcsize, "gctime", db.gctime, "livenodes", len(db.nodes), "livesize", db.nodesSize)

//
	db.gcnodes, db.gcsize, db.gctime = 0, 0, 0
	db.flushnodes, db.flushsize, db.flushtime = 0, 0, 0

	return nil
}

//
func (db *Database) commit(hash common.Hash, batch ethdb.Batch) error {
//
	node, ok := db.nodes[hash]
	if !ok {
		return nil
	}
	for _, child := range node.childs() {
		if err := db.commit(child, batch); err != nil {
			return err
		}
	}
	if err := batch.Put(hash[:], node.rlp()); err != nil {
		return err
	}
//
	if batch.ValueSize() >= ethdb.IdealBatchSize {
		if err := batch.Write(); err != nil {
			return err
		}
		batch.Reset()
	}
	return nil
}

//
//
//
//
func (db *Database) uncache(hash common.Hash) {
//
	node, ok := db.nodes[hash]
	if !ok {
		return
	}
//
	switch hash {
	case db.oldest:
		db.oldest = node.flushNext
		db.nodes[node.flushNext].flushPrev = common.Hash{}
	case db.newest:
		db.newest = node.flushPrev
		db.nodes[node.flushPrev].flushNext = common.Hash{}
	default:
		db.nodes[node.flushPrev].flushNext = node.flushNext
		db.nodes[node.flushNext].flushPrev = node.flushPrev
	}
//
	for _, child := range node.childs() {
		db.uncache(child)
	}
	delete(db.nodes, hash)
	db.nodesSize -= common.StorageSize(common.HashLength + int(node.size))
}

//
//
func (db *Database) Size() (common.StorageSize, common.StorageSize) {
	db.lock.RLock()
	defer db.lock.RUnlock()

//
//
//
	var flushlistSize = common.StorageSize((len(db.nodes) - 1) * 2 * common.HashLength)
	return db.nodesSize + flushlistSize, db.preimagesSize
}

//
//
//
//
//
//
func (db *Database) verifyIntegrity() {
//
	reachable := map[common.Hash]struct{}{{}: {}}

	for child := range db.nodes[common.Hash{}].children {
		db.accumulate(child, reachable)
	}
//
	unreachable := []string{}
	for hash, node := range db.nodes {
		if _, ok := reachable[hash]; !ok {
			unreachable = append(unreachable, fmt.Sprintf("%x: {Node: %v, Parents: %d, Prev: %x, Next: %x}",
				hash, node.node, node.parents, node.flushPrev, node.flushNext))
		}
	}
	if len(unreachable) != 0 {
		panic(fmt.Sprintf("trie cache memory leak: %v", unreachable))
	}
}

//
//
func (db *Database) accumulate(hash common.Hash, reachable map[common.Hash]struct{}) {
//
	node, ok := db.nodes[hash]
	if !ok {
		return
	}
	reachable[hash] = struct{}{}

//
	for _, child := range node.childs() {
		db.accumulate(child, reachable)
	}
}
