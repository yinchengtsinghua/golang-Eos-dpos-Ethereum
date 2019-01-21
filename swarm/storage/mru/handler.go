
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
package mru

import (
	"bytes"
	"context"
	"sync"
	"time"
	"unsafe"

	"github.com/ethereum/go-ethereum/swarm/chunk"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

type Handler struct {
	chunkStore      *storage.NetStore
	HashSize        int
	resources       map[uint64]*resource
	resourceLock    sync.RWMutex
	storeTimeout    time.Duration
	queryMaxPeriods uint32
}

//
//
type HandlerParams struct {
	QueryMaxPeriods uint32
}

//
var hashPool sync.Pool
var minimumChunkLength int

//
func init() {
	hashPool = sync.Pool{
		New: func() interface{} {
			return storage.MakeHashFunc(resourceHashAlgorithm)()
		},
	}
	if minimumMetadataLength < minimumUpdateDataLength {
		minimumChunkLength = minimumMetadataLength
	} else {
		minimumChunkLength = minimumUpdateDataLength
	}
}

//
func NewHandler(params *HandlerParams) *Handler {
	rh := &Handler{
		resources:       make(map[uint64]*resource),
		storeTimeout:    defaultStoreTimeout,
		queryMaxPeriods: params.QueryMaxPeriods,
	}

	for i := 0; i < hasherCount; i++ {
		hashfunc := storage.MakeHashFunc(resourceHashAlgorithm)()
		if rh.HashSize == 0 {
			rh.HashSize = hashfunc.Size()
		}
		hashPool.Put(hashfunc)
	}

	return rh
}

//
func (h *Handler) SetStore(store *storage.NetStore) {
	h.chunkStore = store
}

//
//
//
func (h *Handler) Validate(chunkAddr storage.Address, data []byte) bool {
	dataLength := len(data)
	if dataLength < minimumChunkLength || dataLength > chunk.DefaultSize+8 {
		return false
	}

//
	if data[0] == 0 && data[1] == 0 && dataLength >= minimumMetadataLength {
//
		rootAddr, _ := metadataHash(data)
		valid := bytes.Equal(chunkAddr, rootAddr)
		if !valid {
			log.Debug("Invalid root metadata chunk with address", "addr", chunkAddr.Hex())
		}
		return valid
	}

//
//
//

//
	var r SignedResourceUpdate
	if err := r.fromChunk(chunkAddr, data); err != nil {
		log.Debug("Invalid resource chunk", "addr", chunkAddr.Hex(), "err", err.Error())
		return false
	}

//
//
//
	if !bytes.Equal(chunkAddr, r.updateHeader.UpdateAddr()) {
		log.Debug("period,version,rootAddr contained in update chunk do not match updateAddr", "addr", chunkAddr.Hex())
		return false
	}

//
//
//
	if err := r.Verify(); err != nil {
		log.Debug("Invalid signature", "err", err)
		return false
	}

	return true
}

//
func (h *Handler) GetContent(rootAddr storage.Address) (storage.Address, []byte, error) {
	rsrc := h.get(rootAddr)
	if rsrc == nil || !rsrc.isSynced() {
		return nil, nil, NewError(ErrNotFound, " does not exist or is not synced")
	}
	return rsrc.lastKey, rsrc.data, nil
}

//
func (h *Handler) GetLastPeriod(rootAddr storage.Address) (uint32, error) {
	rsrc := h.get(rootAddr)
	if rsrc == nil {
		return 0, NewError(ErrNotFound, " does not exist")
	} else if !rsrc.isSynced() {
		return 0, NewError(ErrNotSynced, " is not synced")
	}
	return rsrc.period, nil
}

//
func (h *Handler) GetVersion(rootAddr storage.Address) (uint32, error) {
	rsrc := h.get(rootAddr)
	if rsrc == nil {
		return 0, NewError(ErrNotFound, " does not exist")
	} else if !rsrc.isSynced() {
		return 0, NewError(ErrNotSynced, " is not synced")
	}
	return rsrc.version, nil
}

//
func (h *Handler) New(ctx context.Context, request *Request) error {

//
	if request.metadata.Frequency == 0 {
		return NewError(ErrInvalidValue, "frequency cannot be 0 when creating a resource")
	}

//
	if request.metadata.Owner == zeroAddr {
		return NewError(ErrInvalidValue, "ownerAddr must be set to create a new metadata chunk")
	}

//
	chunk, metaHash, err := request.metadata.newChunk()
	if err != nil {
		return err
	}
	if request.metaHash != nil && !bytes.Equal(request.metaHash, metaHash) ||
		request.rootAddr != nil && !bytes.Equal(request.rootAddr, chunk.Addr) {
		return NewError(ErrInvalidValue, "metaHash in UpdateRequest does not match actual metadata")
	}

	request.metaHash = metaHash
	request.rootAddr = chunk.Addr

	h.chunkStore.Put(ctx, chunk)
	log.Debug("new resource", "name", request.metadata.Name, "startTime", request.metadata.StartTime, "frequency", request.metadata.Frequency, "owner", request.metadata.Owner)

//
	rsrc := &resource{
		resourceUpdate: resourceUpdate{
			updateHeader: updateHeader{
				UpdateLookup: UpdateLookup{
					rootAddr: chunk.Addr,
				},
			},
		},
		ResourceMetadata: request.metadata,
		updated:          time.Now(),
	}
	h.set(chunk.Addr, rsrc)

	return nil
}

//
//
//
func (h *Handler) NewUpdateRequest(ctx context.Context, rootAddr storage.Address) (updateRequest *Request, err error) {

	if rootAddr == nil {
		return nil, NewError(ErrInvalidValue, "rootAddr cannot be nil")
	}

//
	rsrc, err := h.Load(ctx, rootAddr)
	if err != nil {
		return nil, err
	}

	now := TimestampProvider.Now()

	updateRequest = new(Request)
	updateRequest.period, err = getNextPeriod(rsrc.StartTime.Time, now.Time, rsrc.Frequency)
	if err != nil {
		return nil, err
	}

	if _, err = h.lookup(rsrc, LookupLatestVersionInPeriod(rsrc.rootAddr, updateRequest.period)); err != nil {
		if err.(*Error).code != ErrNotFound {
			return nil, err
		}
//
//
	}

	updateRequest.multihash = rsrc.multihash
	updateRequest.rootAddr = rsrc.rootAddr
	updateRequest.metaHash = rsrc.metaHash
	updateRequest.metadata = rsrc.ResourceMetadata

//
//
	if h.hasUpdate(rootAddr, updateRequest.period) {
		updateRequest.version = rsrc.version + 1
	} else {
		updateRequest.version = 1
	}

	return updateRequest, nil
}

//
//
//
//
//
//
//
func (h *Handler) Lookup(ctx context.Context, params *LookupParams) (*resource, error) {

	rsrc := h.get(params.rootAddr)
	if rsrc == nil {
		return nil, NewError(ErrNothingToReturn, "resource not loaded")
	}
	return h.lookup(rsrc, params)
}

//
//
//
//
func (h *Handler) LookupPrevious(ctx context.Context, params *LookupParams) (*resource, error) {
	rsrc := h.get(params.rootAddr)
	if rsrc == nil {
		return nil, NewError(ErrNothingToReturn, "resource not loaded")
	}
	if !rsrc.isSynced() {
		return nil, NewError(ErrNotSynced, "LookupPrevious requires synced resource.")
	} else if rsrc.period == 0 {
		return nil, NewError(ErrNothingToReturn, " not found")
	}
	var version, period uint32
	if rsrc.version > 1 {
		version = rsrc.version - 1
		period = rsrc.period
	} else if rsrc.period == 1 {
		return nil, NewError(ErrNothingToReturn, "Current update is the oldest")
	} else {
		version = 0
		period = rsrc.period - 1
	}
	return h.lookup(rsrc, NewLookupParams(rsrc.rootAddr, period, version, params.Limit))
}

//
func (h *Handler) lookup(rsrc *resource, params *LookupParams) (*resource, error) {

	lp := *params
//
	if h.chunkStore == nil {
		return nil, NewError(ErrInit, "Call Handler.SetStore() before performing lookups")
	}

	var specificperiod bool
	if lp.period > 0 {
		specificperiod = true
	} else {
//
		now := TimestampProvider.Now()

		var period uint32
		period, err := getNextPeriod(rsrc.StartTime.Time, now.Time, rsrc.Frequency)
		if err != nil {
			return nil, err
		}
		lp.period = period
	}

//
//
//
	var specificversion bool
	if lp.version > 0 {
		specificversion = true
	} else {
		lp.version = 1
	}

	var hops uint32
	if lp.Limit == 0 {
		lp.Limit = h.queryMaxPeriods
	}
	log.Trace("resource lookup", "period", lp.period, "version", lp.version, "limit", lp.Limit)
	for lp.period > 0 {
		if lp.Limit != 0 && hops > lp.Limit {
			return nil, NewErrorf(ErrPeriodDepth, "Lookup exceeded max period hops (%d)", lp.Limit)
		}
		updateAddr := lp.UpdateAddr()
		chunk, err := h.chunkStore.GetWithTimeout(context.TODO(), updateAddr, defaultRetrieveTimeout)
		if err == nil {
			if specificversion {
				return h.updateIndex(rsrc, chunk)
			}
//
			log.Trace("rsrc update version 1 found, checking for version updates", "period", lp.period, "updateAddr", updateAddr)
			for {
				newversion := lp.version + 1
				updateAddr := lp.UpdateAddr()
				newchunk, err := h.chunkStore.GetWithTimeout(context.TODO(), updateAddr, defaultRetrieveTimeout)
				if err != nil {
					return h.updateIndex(rsrc, chunk)
				}
				chunk = newchunk
				lp.version = newversion
				log.Trace("version update found, checking next", "version", lp.version, "period", lp.period, "updateAddr", updateAddr)
			}
		}
		if specificperiod {
			break
		}
		log.Trace("rsrc update not found, checking previous period", "period", lp.period, "updateAddr", updateAddr)
		lp.period--
		hops++
	}
	return nil, NewError(ErrNotFound, "no updates found")
}

//
//
func (h *Handler) Load(ctx context.Context, rootAddr storage.Address) (*resource, error) {
	chunk, err := h.chunkStore.GetWithTimeout(ctx, rootAddr, defaultRetrieveTimeout)
	if err != nil {
		return nil, NewError(ErrNotFound, err.Error())
	}

//
	rsrc := &resource{}

if err := rsrc.ResourceMetadata.binaryGet(chunk.SData); err != nil { //
		return nil, err
	}

	rsrc.rootAddr, rsrc.metaHash = metadataHash(chunk.SData)
	if !bytes.Equal(rsrc.rootAddr, rootAddr) {
		return nil, NewError(ErrCorruptData, "Corrupt metadata chunk")
	}
	h.set(rootAddr, rsrc)
	log.Trace("resource index load", "rootkey", rootAddr, "name", rsrc.ResourceMetadata.Name, "starttime", rsrc.ResourceMetadata.StartTime, "frequency", rsrc.ResourceMetadata.Frequency)
	return rsrc, nil
}

//
func (h *Handler) updateIndex(rsrc *resource, chunk *storage.Chunk) (*resource, error) {

//
	var r SignedResourceUpdate
	if err := r.fromChunk(chunk.Addr, chunk.SData); err != nil {
		return nil, err
	}
	log.Trace("resource index update", "name", rsrc.ResourceMetadata.Name, "updatekey", chunk.Addr, "period", r.period, "version", r.version)

//
	rsrc.lastKey = chunk.Addr
	rsrc.period = r.period
	rsrc.version = r.version
	rsrc.updated = time.Now()
	rsrc.data = make([]byte, len(r.data))
	rsrc.multihash = r.multihash
	copy(rsrc.data, r.data)
	rsrc.Reader = bytes.NewReader(rsrc.data)
	log.Debug("resource synced", "name", rsrc.ResourceMetadata.Name, "updateAddr", chunk.Addr, "period", rsrc.period, "version", rsrc.version)
	h.set(chunk.Addr, rsrc)
	return rsrc, nil
}

//
//
//
//
//
//
func (h *Handler) Update(ctx context.Context, r *SignedResourceUpdate) (storage.Address, error) {
	return h.update(ctx, r)
}

//
func (h *Handler) update(ctx context.Context, r *SignedResourceUpdate) (updateAddr storage.Address, err error) {

//
	if h.chunkStore == nil {
		return nil, NewError(ErrInit, "Call Handler.SetStore() before updating")
	}

	rsrc := h.get(r.rootAddr)
if rsrc != nil && rsrc.period != 0 && rsrc.version != 0 && //
rsrc.period == r.period && rsrc.version >= r.version { //

		return nil, NewError(ErrInvalidValue, "A former update in this period is already known to exist")
	}

chunk, err := r.toChunk() //
	if err != nil {
		return nil, err
	}

//
	h.chunkStore.Put(ctx, chunk)
	log.Trace("resource update", "updateAddr", r.updateAddr, "lastperiod", r.period, "version", r.version, "data", chunk.SData, "multihash", r.multihash)

//
	if rsrc != nil && (r.period > rsrc.period || (rsrc.period == r.period && r.version > rsrc.version)) {
		rsrc.period = r.period
		rsrc.version = r.version
		rsrc.data = make([]byte, len(r.data))
		rsrc.updated = time.Now()
		rsrc.lastKey = r.updateAddr
		rsrc.multihash = r.multihash
		copy(rsrc.data, r.data)
		rsrc.Reader = bytes.NewReader(rsrc.data)
	}
	return r.updateAddr, nil
}

//
func (h *Handler) get(rootAddr storage.Address) *resource {
	if len(rootAddr) < storage.KeyLength {
		log.Warn("Handler.get with invalid rootAddr")
		return nil
	}
	hashKey := *(*uint64)(unsafe.Pointer(&rootAddr[0]))
	h.resourceLock.RLock()
	defer h.resourceLock.RUnlock()
	rsrc := h.resources[hashKey]
	return rsrc
}

//
func (h *Handler) set(rootAddr storage.Address, rsrc *resource) {
	if len(rootAddr) < storage.KeyLength {
		log.Warn("Handler.set with invalid rootAddr")
		return
	}
	hashKey := *(*uint64)(unsafe.Pointer(&rootAddr[0]))
	h.resourceLock.Lock()
	defer h.resourceLock.Unlock()
	h.resources[hashKey] = rsrc
}

//
func (h *Handler) hasUpdate(rootAddr storage.Address, period uint32) bool {
	rsrc := h.get(rootAddr)
	return rsrc != nil && rsrc.period == period
}
