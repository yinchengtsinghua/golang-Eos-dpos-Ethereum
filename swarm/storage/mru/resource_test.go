
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

package mru

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"flag"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/contracts/ens"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/swarm/chunk"
	"github.com/ethereum/go-ethereum/swarm/multihash"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

var (
	loglevel   = flag.Int("loglevel", 3, "loglevel")
	testHasher = storage.MakeHashFunc(resourceHashAlgorithm)()
	startTime  = Timestamp{
		Time: uint64(4200),
	}
	resourceFrequency = uint64(42)
	cleanF            func()
	resourceName      = "føø.bar"
	hashfunc          = storage.MakeHashFunc(storage.DefaultHash)
)

func init() {
	flag.Parse()
	log.Root().SetHandler(log.CallerFileHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(os.Stderr, log.TerminalFormat(true)))))
}

//
type fakeTimeProvider struct {
	currentTime uint64
}

func (f *fakeTimeProvider) Tick() {
	f.currentTime++
}

func (f *fakeTimeProvider) Now() Timestamp {
	return Timestamp{
		Time: f.currentTime,
	}
}

func TestUpdateChunkSerializationErrorChecking(t *testing.T) {

//
	var r SignedResourceUpdate
	if err := r.fromChunk(storage.ZeroAddr, make([]byte, minimumUpdateDataLength-1)); err == nil {
		t.Fatalf("Expected parseUpdate to fail when chunkData contains less than %d bytes", minimumUpdateDataLength)
	}

	r = SignedResourceUpdate{}
//
	fakeChunk := make([]byte, 150)
	binary.LittleEndian.PutUint16(fakeChunk, 44)
	if err := r.fromChunk(storage.ZeroAddr, fakeChunk); err == nil {
		t.Fatal("Expected parseUpdate to fail when the header length does not match the actual data array passed in")
	}

	r = SignedResourceUpdate{
		resourceUpdate: resourceUpdate{
			updateHeader: updateHeader{
				UpdateLookup: UpdateLookup{

rootAddr: make([]byte, 79), //
				},
				metaHash:  nil,
				multihash: false,
			},
		},
	}
	_, err := r.toChunk()
	if err == nil {
		t.Fatal("Expected newUpdateChunk to fail when rootAddr or metaHash have the wrong length")
	}
	r.rootAddr = make([]byte, storage.KeyLength)
	r.metaHash = make([]byte, storage.KeyLength)
	_, err = r.toChunk()
	if err == nil {
		t.Fatal("Expected newUpdateChunk to fail when there is no data")
	}
r.data = make([]byte, 79) //
	_, err = r.toChunk()
	if err == nil {
		t.Fatal("expected newUpdateChunk to fail when there is no signature", err)
	}

	alice := newAliceSigner()
	if err := r.Sign(alice); err != nil {
		t.Fatalf("error signing:%s", err)

	}
	_, err = r.toChunk()
	if err != nil {
		t.Fatalf("error creating update chunk:%s", err)
	}

	r.multihash = true
r.data[1] = 79 //
	if err := r.Sign(alice); err == nil {
		t.Fatal("expected Sign() to fail when an invalid multihash is in data and multihash=true", err)
	}
}

//
func TestReverse(t *testing.T) {

	period := uint32(4)
	version := uint32(2)

//
	timeProvider := &fakeTimeProvider{
		currentTime: startTime.Time,
	}

//
	signer := newAliceSigner()

//
	_, _, teardownTest, err := setupTest(timeProvider, signer)
	if err != nil {
		t.Fatal(err)
	}
	defer teardownTest()

	metadata := ResourceMetadata{
		Name:      resourceName,
		StartTime: startTime,
		Frequency: resourceFrequency,
		Owner:     signer.Address(),
	}

	rootAddr, metaHash, _, err := metadata.serializeAndHash()
	if err != nil {
		t.Fatal(err)
	}

//
	data := make([]byte, 8)
	_, err = rand.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	testHasher.Reset()
	testHasher.Write(data)

	update := &SignedResourceUpdate{
		resourceUpdate: resourceUpdate{
			updateHeader: updateHeader{
				UpdateLookup: UpdateLookup{
					period:   period,
					version:  version,
					rootAddr: rootAddr,
				},
				metaHash: metaHash,
			},
			data: data,
		},
	}
//
	key := update.UpdateAddr()

	if err = update.Sign(signer); err != nil {
		t.Fatal(err)
	}

	chunk, err := update.toChunk()
	if err != nil {
		t.Fatal(err)
	}

//
	var checkUpdate SignedResourceUpdate
	if err := checkUpdate.fromChunk(chunk.Addr, chunk.SData); err != nil {
		t.Fatal(err)
	}
	checkdigest, err := checkUpdate.GetDigest()
	if err != nil {
		t.Fatal(err)
	}
	recoveredaddress, err := getOwner(checkdigest, *checkUpdate.signature)
	if err != nil {
		t.Fatalf("Retrieve address from signature fail: %v", err)
	}
	originaladdress := crypto.PubkeyToAddress(signer.PrivKey.PublicKey)

//
	if recoveredaddress != originaladdress {
		t.Fatalf("addresses dont match: %x != %x", originaladdress, recoveredaddress)
	}

	if !bytes.Equal(key[:], chunk.Addr[:]) {
		t.Fatalf("Expected chunk key '%x', was '%x'", key, chunk.Addr)
	}
	if period != checkUpdate.period {
		t.Fatalf("Expected period '%d', was '%d'", period, checkUpdate.period)
	}
	if version != checkUpdate.version {
		t.Fatalf("Expected version '%d', was '%d'", version, checkUpdate.version)
	}
	if !bytes.Equal(data, checkUpdate.data) {
		t.Fatalf("Expectedn data '%x', was '%x'", data, checkUpdate.data)
	}
}

//
func TestResourceHandler(t *testing.T) {

//
	timeProvider := &fakeTimeProvider{
		currentTime: startTime.Time,
	}

//
	signer := newAliceSigner()

	rh, datadir, teardownTest, err := setupTest(timeProvider, signer)
	if err != nil {
		t.Fatal(err)
	}
	defer teardownTest()

//
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	metadata := &ResourceMetadata{
		Name:      resourceName,
		Frequency: resourceFrequency,
		StartTime: Timestamp{Time: timeProvider.Now().Time},
		Owner:     signer.Address(),
	}

	request, err := NewCreateUpdateRequest(metadata)
	if err != nil {
		t.Fatal(err)
	}
	request.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	err = rh.New(ctx, request)
	if err != nil {
		t.Fatal(err)
	}

	chunk, err := rh.chunkStore.Get(context.TODO(), storage.Address(request.rootAddr))
	if err != nil {
		t.Fatal(err)
	} else if len(chunk.SData) < 16 {
		t.Fatalf("chunk data must be minimum 16 bytes, is %d", len(chunk.SData))
	}

	var recoveredMetadata ResourceMetadata

	recoveredMetadata.binaryGet(chunk.SData)
	if err != nil {
		t.Fatal(err)
	}
	if recoveredMetadata.StartTime.Time != timeProvider.currentTime {
		t.Fatalf("stored startTime %d does not match provided startTime %d", recoveredMetadata.StartTime.Time, timeProvider.currentTime)
	}
	if recoveredMetadata.Frequency != resourceFrequency {
		t.Fatalf("stored frequency %d does not match provided frequency %d", recoveredMetadata.Frequency, resourceFrequency)
	}

//
	updates := []string{
		"blinky",
		"pinky",
		"inky",
		"clyde",
	}

//
	resourcekey := make(map[string]storage.Address)
	fwdClock(int(resourceFrequency/2), timeProvider)
	data := []byte(updates[0])
	request.SetData(data, false)
	if err := request.Sign(signer); err != nil {
		t.Fatal(err)
	}
	resourcekey[updates[0]], err = rh.Update(ctx, &request.SignedResourceUpdate)
	if err != nil {
		t.Fatal(err)
	}

//
	request, err = rh.NewUpdateRequest(ctx, request.rootAddr)
	if err != nil {
		t.Fatal(err)
	}
	if request.version != 2 || request.period != 1 {
		t.Fatal("Suggested period should be 1 and version should be 2")
	}

request.version = 1 //
	data = []byte(updates[1])
	request.SetData(data, false)
	if err := request.Sign(signer); err != nil {
		t.Fatal(err)
	}
	resourcekey[updates[1]], err = rh.Update(ctx, &request.SignedResourceUpdate)
	if err == nil {
		t.Fatal("Expected update to fail since this version already exists")
	}

//
	fwdClock(int(resourceFrequency/2), timeProvider)
	request, err = rh.NewUpdateRequest(ctx, request.rootAddr)
	if err != nil {
		t.Fatal(err)
	}
	request.SetData(data, false)
	if err := request.Sign(signer); err != nil {
		t.Fatal(err)
	}
	resourcekey[updates[1]], err = rh.Update(ctx, &request.SignedResourceUpdate)
	if err != nil {
		t.Fatal(err)
	}

	fwdClock(int(resourceFrequency), timeProvider)
//
	request, err = rh.NewUpdateRequest(ctx, request.rootAddr)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(updates[2])
	request.SetData(data, false)
	if err := request.Sign(signer); err != nil {
		t.Fatal(err)
	}
	resourcekey[updates[2]], err = rh.Update(ctx, &request.SignedResourceUpdate)
	if err != nil {
		t.Fatal(err)
	}

//
	fwdClock(1, timeProvider)
	request, err = rh.NewUpdateRequest(ctx, request.rootAddr)
	if err != nil {
		t.Fatal(err)
	}
	if request.period != 3 || request.version != 2 {
		t.Fatal("Suggested period should be 3 and version should be 2")
	}
	data = []byte(updates[3])
	request.SetData(data, false)

	if err := request.Sign(signer); err != nil {
		t.Fatal(err)
	}
	resourcekey[updates[3]], err = rh.Update(ctx, &request.SignedResourceUpdate)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)
	rh.Close()

//
//
	fwdClock(int(resourceFrequency*2)-1, timeProvider)

	rhparams := &HandlerParams{}

	rh2, err := NewTestHandler(datadir, rhparams)
	if err != nil {
		t.Fatal(err)
	}

	rsrc2, err := rh2.Load(context.TODO(), request.rootAddr)
	if err != nil {
		t.Fatal(err)
	}

	_, err = rh2.Lookup(ctx, LookupLatest(request.rootAddr))
	if err != nil {
		t.Fatal(err)
	}

//
	if !bytes.Equal(rsrc2.data, []byte(updates[len(updates)-1])) {
		t.Fatalf("resource data was %v, expected %v", string(rsrc2.data), updates[len(updates)-1])
	}
	if rsrc2.version != 2 {
		t.Fatalf("resource version was %d, expected 2", rsrc2.version)
	}
	if rsrc2.period != 3 {
		t.Fatalf("resource period was %d, expected 3", rsrc2.period)
	}
	log.Debug("Latest lookup", "period", rsrc2.period, "version", rsrc2.version, "data", rsrc2.data)

//
	rsrc, err := rh2.Lookup(ctx, LookupLatestVersionInPeriod(request.rootAddr, 3))
	if err != nil {
		t.Fatal(err)
	}
//
	if !bytes.Equal(rsrc.data, []byte(updates[len(updates)-1])) {
		t.Fatalf("resource data (historical) was %v, expected %v", string(rsrc2.data), updates[len(updates)-1])
	}
	log.Debug("Historical lookup", "period", rsrc2.period, "version", rsrc2.version, "data", rsrc2.data)

//
	lookupParams := LookupVersion(request.rootAddr, 3, 1)
	rsrc, err = rh2.Lookup(ctx, lookupParams)
	if err != nil {
		t.Fatal(err)
	}
//
	if !bytes.Equal(rsrc.data, []byte(updates[2])) {
		t.Fatalf("resource data (historical) was %v, expected %v", string(rsrc2.data), updates[2])
	}
	log.Debug("Specific version lookup", "period", rsrc2.period, "version", rsrc2.version, "data", rsrc2.data)

//
//
	for i := 1; i >= 0; i-- {
		rsrc, err := rh2.LookupPrevious(ctx, lookupParams)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(rsrc.data, []byte(updates[i])) {
			t.Fatalf("resource data (previous) was %v, expected %v", rsrc.data, updates[i])

		}
	}

//
	rsrc, err = rh2.LookupPrevious(ctx, lookupParams)
	if err == nil {
		t.Fatalf("expected previous to fail, returned period %d version %d data %v", rsrc.period, rsrc.version, rsrc.data)
	}

}

func TestMultihash(t *testing.T) {

//
	timeProvider := &fakeTimeProvider{
		currentTime: startTime.Time,
	}

//
	signer := newAliceSigner()

//
	rh, datadir, teardownTest, err := setupTest(timeProvider, signer)
	if err != nil {
		t.Fatal(err)
	}
	defer teardownTest()

//
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	metadata := &ResourceMetadata{
		Name:      resourceName,
		Frequency: resourceFrequency,
		StartTime: Timestamp{Time: timeProvider.Now().Time},
		Owner:     signer.Address(),
	}

	mr, err := NewCreateRequest(metadata)
	if err != nil {
		t.Fatal(err)
	}
	err = rh.New(ctx, mr)
	if err != nil {
		t.Fatal(err)
	}

//
//
	multihashbytes := ens.EnsNode("foo")
	multihashmulti := multihash.ToMultihash(multihashbytes.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	mr.SetData(multihashmulti, true)
	mr.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	multihashkey, err := rh.Update(ctx, &mr.SignedResourceUpdate)
	if err != nil {
		t.Fatal(err)
	}

	sha1bytes := make([]byte, multihash.MultihashLength)
	sha1multi := multihash.ToMultihash(sha1bytes)
	if err != nil {
		t.Fatal(err)
	}
	mr, err = rh.NewUpdateRequest(ctx, mr.rootAddr)
	if err != nil {
		t.Fatal(err)
	}
	mr.SetData(sha1multi, true)
	mr.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	sha1key, err := rh.Update(ctx, &mr.SignedResourceUpdate)
	if err != nil {
		t.Fatal(err)
	}

//
	mr, err = rh.NewUpdateRequest(ctx, mr.rootAddr)
	if err != nil {
		t.Fatal(err)
	}
	mr.SetData(multihashmulti[1:], true)
	mr.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	_, err = rh.Update(ctx, &mr.SignedResourceUpdate)
	if err == nil {
		t.Fatalf("Expected update to fail with first byte skipped")
	}
	mr, err = rh.NewUpdateRequest(ctx, mr.rootAddr)
	if err != nil {
		t.Fatal(err)
	}
	mr.SetData(multihashmulti[:len(multihashmulti)-2], true)
	mr.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}

	_, err = rh.Update(ctx, &mr.SignedResourceUpdate)
	if err == nil {
		t.Fatalf("Expected update to fail with last byte skipped")
	}

	data, err := getUpdateDirect(rh.Handler, multihashkey)
	if err != nil {
		t.Fatal(err)
	}
	multihashdecode, err := multihash.FromMultihash(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(multihashdecode, multihashbytes.Bytes()) {
		t.Fatalf("Decoded hash '%x' does not match original hash '%x'", multihashdecode, multihashbytes.Bytes())
	}
	data, err = getUpdateDirect(rh.Handler, sha1key)
	if err != nil {
		t.Fatal(err)
	}
	shadecode, err := multihash.FromMultihash(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(shadecode, sha1bytes) {
		t.Fatalf("Decoded hash '%x' does not match original hash '%x'", shadecode, sha1bytes)
	}
	rh.Close()

	rhparams := &HandlerParams{}
//
	rh2, err := NewTestHandler(datadir, rhparams)
	if err != nil {
		t.Fatal(err)
	}
	mr, err = NewCreateRequest(metadata)
	if err != nil {
		t.Fatal(err)
	}
	err = rh2.New(ctx, mr)
	if err != nil {
		t.Fatal(err)
	}

	mr.SetData(multihashmulti, true)
	mr.Sign(signer)

	if err != nil {
		t.Fatal(err)
	}
	multihashsignedkey, err := rh2.Update(ctx, &mr.SignedResourceUpdate)
	if err != nil {
		t.Fatal(err)
	}

	mr, err = rh2.NewUpdateRequest(ctx, mr.rootAddr)
	if err != nil {
		t.Fatal(err)
	}
	mr.SetData(sha1multi, true)
	mr.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}

	sha1signedkey, err := rh2.Update(ctx, &mr.SignedResourceUpdate)
	if err != nil {
		t.Fatal(err)
	}

	data, err = getUpdateDirect(rh2.Handler, multihashsignedkey)
	if err != nil {
		t.Fatal(err)
	}
	multihashdecode, err = multihash.FromMultihash(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(multihashdecode, multihashbytes.Bytes()) {
		t.Fatalf("Decoded hash '%x' does not match original hash '%x'", multihashdecode, multihashbytes.Bytes())
	}
	data, err = getUpdateDirect(rh2.Handler, sha1signedkey)
	if err != nil {
		t.Fatal(err)
	}
	shadecode, err = multihash.FromMultihash(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(shadecode, sha1bytes) {
		t.Fatalf("Decoded hash '%x' does not match original hash '%x'", shadecode, sha1bytes)
	}
}

//
func TestValidator(t *testing.T) {

//
	timeProvider := &fakeTimeProvider{
		currentTime: startTime.Time,
	}

//
	signer := newAliceSigner()

//
	falseSigner := newBobSigner()

//
	rh, _, teardownTest, err := setupTest(timeProvider, signer)
	if err != nil {
		t.Fatal(err)
	}
	defer teardownTest()

//
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	metadata := &ResourceMetadata{
		Name:      resourceName,
		Frequency: resourceFrequency,
		StartTime: Timestamp{Time: timeProvider.Now().Time},
		Owner:     signer.Address(),
	}
	mr, err := NewCreateRequest(metadata)
	if err != nil {
		t.Fatal(err)
	}
	mr.Sign(signer)

	err = rh.New(ctx, mr)
	if err != nil {
		t.Fatalf("Create resource fail: %v", err)
	}

//
	data := []byte("foo")
	mr.SetData(data, false)
	if err := mr.Sign(signer); err != nil {
		t.Fatalf("sign fail: %v", err)
	}
	chunk, err := mr.SignedResourceUpdate.toChunk()
	if err != nil {
		t.Fatal(err)
	}
	if !rh.Validate(chunk.Addr, chunk.SData) {
		t.Fatal("Chunk validator fail on update chunk")
	}

//
	if err := mr.Sign(falseSigner); err == nil {
		t.Fatalf("Expected Sign to fail since we are using a different OwnerAddr: %v", err)
	}

//
mr.metadata.Owner = zeroAddr //
	if err := mr.Sign(falseSigner); err != nil {
		t.Fatalf("sign fail: %v", err)
	}

	chunk, err = mr.SignedResourceUpdate.toChunk()
	if err != nil {
		t.Fatal(err)
	}

	if rh.Validate(chunk.Addr, chunk.SData) {
		t.Fatal("Chunk validator did not fail on update chunk with false address")
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	metadata = &ResourceMetadata{
		Name:      resourceName,
		StartTime: TimestampProvider.Now(),
		Frequency: resourceFrequency,
		Owner:     signer.Address(),
	}
	chunk, _, err = metadata.newChunk()
	if err != nil {
		t.Fatal(err)
	}

	if !rh.Validate(chunk.Addr, chunk.SData) {
		t.Fatal("Chunk validator fail on metadata chunk")
	}
}

//
//
//
//
func TestValidatorInStore(t *testing.T) {

//
	TimestampProvider = &fakeTimeProvider{
		currentTime: startTime.Time,
	}

//
	signer := newAliceSigner()

//
	datadir, err := ioutil.TempDir("", "storage-testresourcevalidator")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(datadir)

	params := storage.NewDefaultLocalStoreParams()
	params.Init(datadir)
	store, err := storage.NewLocalStore(params, nil)
	if err != nil {
		t.Fatal(err)
	}

//
	rhParams := &HandlerParams{}
	rh := NewHandler(rhParams)
	store.Validators = append(store.Validators, rh)

//
	chunks := storage.GenerateRandomChunks(chunk.DefaultSize, 2)
	goodChunk := chunks[0]
	badChunk := chunks[1]
	badChunk.SData = goodChunk.SData

	metadata := &ResourceMetadata{
		StartTime: startTime,
		Name:      "xyzzy",
		Frequency: resourceFrequency,
		Owner:     signer.Address(),
	}

	rootChunk, metaHash, err := metadata.newChunk()
	if err != nil {
		t.Fatal(err)
	}
//
	updateLookup := UpdateLookup{
		period:   42,
		version:  1,
		rootAddr: rootChunk.Addr,
	}

	updateAddr := updateLookup.UpdateAddr()
	data := []byte("bar")

	r := SignedResourceUpdate{
		updateAddr: updateAddr,
		resourceUpdate: resourceUpdate{
			updateHeader: updateHeader{
				UpdateLookup: updateLookup,
				metaHash:     metaHash,
			},
			data: data,
		},
	}

	r.Sign(signer)

	uglyChunk, err := r.toChunk()
	if err != nil {
		t.Fatal(err)
	}

//
	storage.PutChunks(store, goodChunk)
	if goodChunk.GetErrored() == nil {
		t.Fatal("expected error on good content address chunk with resource validator only, but got nil")
	}
	storage.PutChunks(store, badChunk)
	if badChunk.GetErrored() == nil {
		t.Fatal("expected error on bad content address chunk with resource validator only, but got nil")
	}
	storage.PutChunks(store, uglyChunk)
	if err := uglyChunk.GetErrored(); err != nil {
		t.Fatalf("expected no error on resource update chunk with resource validator only, but got: %s", err)
	}
}

//
func fwdClock(count int, timeProvider *fakeTimeProvider) {
	for i := 0; i < count; i++ {
		timeProvider.Tick()
	}
}

//
func setupTest(timeProvider timestampProvider, signer Signer) (rh *TestHandler, datadir string, teardown func(), err error) {

	var fsClean func()
	var rpcClean func()
	cleanF = func() {
		if fsClean != nil {
			fsClean()
		}
		if rpcClean != nil {
			rpcClean()
		}
	}

//
	datadir, err = ioutil.TempDir("", "rh")
	if err != nil {
		return nil, "", nil, err
	}
	fsClean = func() {
		os.RemoveAll(datadir)
	}

	TimestampProvider = timeProvider
	rhparams := &HandlerParams{}
	rh, err = NewTestHandler(datadir, rhparams)
	return rh, datadir, cleanF, err
}

func newAliceSigner() *GenericSigner {
	privKey, _ := crypto.HexToECDSA("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	return NewGenericSigner(privKey)
}

func newBobSigner() *GenericSigner {
	privKey, _ := crypto.HexToECDSA("accedeaccedeaccedeaccedeaccedeaccedeaccedeaccedeaccedeaccedecaca")
	return NewGenericSigner(privKey)
}

func newCharlieSigner() *GenericSigner {
	privKey, _ := crypto.HexToECDSA("facadefacadefacadefacadefacadefacadefacadefacadefacadefacadefaca")
	return NewGenericSigner(privKey)
}

func getUpdateDirect(rh *Handler, addr storage.Address) ([]byte, error) {
	chunk, err := rh.chunkStore.Get(context.TODO(), addr)
	if err != nil {
		return nil, err
	}
	var r SignedResourceUpdate
	if err := r.fromChunk(addr, chunk.SData); err != nil {
		return nil, err
	}
	return r.data, nil
}
