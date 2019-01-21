
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
	"hash"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

//
type SignedResourceUpdate struct {
resourceUpdate //
	signature      *Signature
updateAddr     storage.Address //
binaryData     []byte          //
}

//
func (r *SignedResourceUpdate) Verify() (err error) {
	if len(r.data) == 0 {
		return NewError(ErrInvalidValue, "Update does not contain data")
	}
	if r.signature == nil {
		return NewError(ErrInvalidSignature, "Missing signature field")
	}

	digest, err := r.GetDigest()
	if err != nil {
		return err
	}

//
	ownerAddr, err := getOwner(digest, *r.signature)
	if err != nil {
		return err
	}

	if !bytes.Equal(r.updateAddr, r.UpdateAddr()) {
		return NewError(ErrInvalidSignature, "Signature address does not match with ownerAddr")
	}

//
	if !verifyOwner(ownerAddr, r.metaHash, r.rootAddr) {
		return NewErrorf(ErrUnauthorized, "signature is valid but signer does not own the resource: %v", err)
	}

	return nil
}

//
func (r *SignedResourceUpdate) Sign(signer Signer) error {

r.binaryData = nil           //
digest, err := r.GetDigest() //
	if err != nil {
		return err
	}

	signature, err := signer.Sign(digest)
	if err != nil {
		return err
	}

//
//
	ownerAddress, err := getOwner(digest, signature)
	if err != nil {
		return NewError(ErrInvalidSignature, "Error verifying signature")
	}

if ownerAddress != signer.Address() { //
		return NewError(ErrInvalidSignature, "Signer address does not match ownerAddr")
	}

	r.signature = &signature
	r.updateAddr = r.UpdateAddr()
	return nil
}

//
func (r *SignedResourceUpdate) toChunk() (*storage.Chunk, error) {

//
//
//
	if r.signature == nil || r.binaryData == nil {
		return nil, NewError(ErrInvalidSignature, "newUpdateChunk called without a valid signature or payload data. Call .Sign() first.")
	}

	chunk := storage.NewChunk(r.updateAddr, nil)
	resourceUpdateLength := r.resourceUpdate.binaryLength()
	chunk.SData = r.binaryData

//
	copy(chunk.SData[resourceUpdateLength:], r.signature[:])

	chunk.Size = int64(len(chunk.SData))
	return chunk, nil
}

//
func (r *SignedResourceUpdate) fromChunk(updateAddr storage.Address, chunkdata []byte) error {
//

//
	if err := r.resourceUpdate.binaryGet(chunkdata); err != nil {
		return err
	}

//
	var signature *Signature
	cursor := r.resourceUpdate.binaryLength()
	sigdata := chunkdata[cursor : cursor+signatureLength]
	if len(sigdata) > 0 {
		signature = &Signature{}
		copy(signature[:], sigdata)
	}

	r.signature = signature
	r.updateAddr = updateAddr
	r.binaryData = chunkdata

	return nil

}

//
//
func (r *SignedResourceUpdate) GetDigest() (result common.Hash, err error) {
	hasher := hashPool.Get().(hash.Hash)
	defer hashPool.Put(hasher)
	hasher.Reset()
	dataLength := r.resourceUpdate.binaryLength()
	if r.binaryData == nil {
		r.binaryData = make([]byte, dataLength+signatureLength)
		if err := r.resourceUpdate.binaryPut(r.binaryData[:dataLength]); err != nil {
			return result, err
		}
	}
hasher.Write(r.binaryData[:dataLength]) //

	return common.BytesToHash(hasher.Sum(nil)), nil
}

//
func getOwner(digest common.Hash, signature Signature) (common.Address, error) {
	pub, err := crypto.SigToPub(digest.Bytes(), signature[:])
	if err != nil {
		return common.Address{}, err
	}
	return crypto.PubkeyToAddress(*pub), nil
}

//
//
//
//
func verifyOwner(ownerAddr common.Address, metaHash []byte, rootAddr storage.Address) bool {
	hasher := hashPool.Get().(hash.Hash)
	defer hashPool.Put(hasher)
	hasher.Reset()
	hasher.Write(metaHash)
	hasher.Write(ownerAddr.Bytes())
	rootAddr2 := hasher.Sum(nil)
	return bytes.Equal(rootAddr2, rootAddr)
}
