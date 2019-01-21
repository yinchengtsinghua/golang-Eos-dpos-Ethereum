
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
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

//
type updateRequestJSON struct {
	Name      string `json:"name,omitempty"`
	Frequency uint64 `json:"frequency,omitempty"`
	StartTime uint64 `json:"startTime,omitempty"`
	Owner     string `json:"ownerAddr,omitempty"`
	RootAddr  string `json:"rootAddr,omitempty"`
	MetaHash  string `json:"metaHash,omitempty"`
	Version   uint32 `json:"version,omitempty"`
	Period    uint32 `json:"period,omitempty"`
	Data      string `json:"data,omitempty"`
	Multihash bool   `json:"multiHash"`
	Signature string `json:"signature,omitempty"`
}

//
type Request struct {
	SignedResourceUpdate
	metadata ResourceMetadata
	isNew    bool
}

var zeroAddr = common.Address{}

//
func NewCreateUpdateRequest(metadata *ResourceMetadata) (*Request, error) {

	request, err := NewCreateRequest(metadata)
	if err != nil {
		return nil, err
	}

//
	now := TimestampProvider.Now().Time

	request.version = 1
	request.period, err = getNextPeriod(metadata.StartTime.Time, now, metadata.Frequency)
	if err != nil {
		return nil, err
	}
	return request, nil
}

//
func NewCreateRequest(metadata *ResourceMetadata) (request *Request, err error) {
if metadata.StartTime.Time == 0 { //
		metadata.StartTime = TimestampProvider.Now()
	}

	if metadata.Owner == zeroAddr {
		return nil, NewError(ErrInvalidValue, "OwnerAddr is not set")
	}

	request = &Request{
		metadata: *metadata,
	}
	request.rootAddr, request.metaHash, _, err = request.metadata.serializeAndHash()
	request.isNew = true
	return request, nil
}

//
func (r *Request) Frequency() uint64 {
	return r.metadata.Frequency
}

//
func (r *Request) Name() string {
	return r.metadata.Name
}

//
func (r *Request) Multihash() bool {
	return r.multihash
}

//
func (r *Request) Period() uint32 {
	return r.period
}

//
func (r *Request) Version() uint32 {
	return r.version
}

//
func (r *Request) RootAddr() storage.Address {
	return r.rootAddr
}

//
func (r *Request) StartTime() Timestamp {
	return r.metadata.StartTime
}

//
func (r *Request) Owner() common.Address {
	return r.metadata.Owner
}

//
func (r *Request) Sign(signer Signer) error {
	if r.metadata.Owner != zeroAddr && r.metadata.Owner != signer.Address() {
		return NewError(ErrInvalidSignature, "Signer does not match current owner of the resource")
	}

	if err := r.SignedResourceUpdate.Sign(signer); err != nil {
		return err
	}
	r.metadata.Owner = signer.Address()
	return nil
}

//
func (r *Request) SetData(data []byte, multihash bool) {
	r.data = data
	r.multihash = multihash
	r.signature = nil
	if !r.isNew {
r.metadata.Frequency = 0 //
	}
}

func (r *Request) IsNew() bool {
	return r.metadata.Frequency > 0 && (r.period <= 1 || r.version <= 1)
}

func (r *Request) IsUpdate() bool {
	return r.signature != nil
}

//
func (r *Request) fromJSON(j *updateRequestJSON) error {

	r.version = j.Version
	r.period = j.Period
	r.multihash = j.Multihash
	r.metadata.Name = j.Name
	r.metadata.Frequency = j.Frequency
	r.metadata.StartTime.Time = j.StartTime

	if err := decodeHexArray(r.metadata.Owner[:], j.Owner, "ownerAddr"); err != nil {
		return err
	}

	var err error
	if j.Data != "" {
		r.data, err = hexutil.Decode(j.Data)
		if err != nil {
			return NewError(ErrInvalidValue, "Cannot decode data")
		}
	}

	var declaredRootAddr storage.Address
	var declaredMetaHash []byte

	declaredRootAddr, err = decodeHexSlice(j.RootAddr, storage.KeyLength, "rootAddr")
	if err != nil {
		return err
	}
	declaredMetaHash, err = decodeHexSlice(j.MetaHash, 32, "metaHash")
	if err != nil {
		return err
	}

	if r.IsNew() {
//
//
//

		r.rootAddr, r.metaHash, _, err = r.metadata.serializeAndHash()
		if err != nil {
			return err
		}
		if j.RootAddr != "" && !bytes.Equal(declaredRootAddr, r.rootAddr) {
			return NewError(ErrInvalidValue, "rootAddr does not match resource metadata")
		}
		if j.MetaHash != "" && !bytes.Equal(declaredMetaHash, r.metaHash) {
			return NewError(ErrInvalidValue, "metaHash does not match resource metadata")
		}

	} else {
//
		r.rootAddr = declaredRootAddr
		r.metaHash = declaredMetaHash
	}

	if j.Signature != "" {
		sigBytes, err := hexutil.Decode(j.Signature)
		if err != nil || len(sigBytes) != signatureLength {
			return NewError(ErrInvalidSignature, "Cannot decode signature")
		}
		r.signature = new(Signature)
		r.updateAddr = r.UpdateAddr()
		copy(r.signature[:], sigBytes)
	}
	return nil
}

func decodeHexArray(dst []byte, src, name string) error {
	bytes, err := decodeHexSlice(src, len(dst), name)
	if err != nil {
		return err
	}
	if bytes != nil {
		copy(dst, bytes)
	}
	return nil
}

func decodeHexSlice(src string, expectedLength int, name string) (bytes []byte, err error) {
	if src != "" {
		bytes, err = hexutil.Decode(src)
		if err != nil || len(bytes) != expectedLength {
			return nil, NewErrorf(ErrInvalidValue, "Cannot decode %s", name)
		}
	}
	return bytes, nil
}

//
//
func (r *Request) UnmarshalJSON(rawData []byte) error {
	var requestJSON updateRequestJSON
	if err := json.Unmarshal(rawData, &requestJSON); err != nil {
		return err
	}
	return r.fromJSON(&requestJSON)
}

//
//
func (r *Request) MarshalJSON() (rawData []byte, err error) {
	var signatureString, dataHashString, rootAddrString, metaHashString string
	if r.signature != nil {
		signatureString = hexutil.Encode(r.signature[:])
	}
	if r.data != nil {
		dataHashString = hexutil.Encode(r.data)
	}
	if r.rootAddr != nil {
		rootAddrString = hexutil.Encode(r.rootAddr)
	}
	if r.metaHash != nil {
		metaHashString = hexutil.Encode(r.metaHash)
	}
	var ownerAddrString string
	if r.metadata.Frequency == 0 {
		ownerAddrString = ""
	} else {
		ownerAddrString = hexutil.Encode(r.metadata.Owner[:])
	}

	requestJSON := &updateRequestJSON{
		Name:      r.metadata.Name,
		Frequency: r.metadata.Frequency,
		StartTime: r.metadata.StartTime.Time,
		Version:   r.version,
		Period:    r.period,
		Owner:     ownerAddrString,
		Data:      dataHashString,
		Multihash: r.multihash,
		Signature: signatureString,
		RootAddr:  rootAddrString,
		MetaHash:  metaHashString,
	}

	return json.Marshal(requestJSON)
}
