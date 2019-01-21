
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package mru

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func areEqualJSON(s1, s2 string) (bool, error) {
//
	var o1 interface{}
	var o2 interface{}

	err := json.Unmarshal([]byte(s1), &o1)
	if err != nil {
		return false, fmt.Errorf("Error mashalling string 1 :: %s", err.Error())
	}
	err = json.Unmarshal([]byte(s2), &o2)
	if err != nil {
		return false, fmt.Errorf("Error mashalling string 2 :: %s", err.Error())
	}

	return reflect.DeepEqual(o1, o2), nil
}

//
//
func TestEncodingDecodingUpdateRequests(t *testing.T) {

signer := newCharlieSigner()  //
falseSigner := newBobSigner() //

//
	createRequest, err := NewCreateRequest(&ResourceMetadata{
		Name:      "a good resource name",
		Frequency: 300,
		StartTime: Timestamp{Time: 1528900000},
		Owner:     signer.Address()})

	if err != nil {
		t.Fatalf("Error creating resource name: %s", err)
	}

//
	messageRawData, err := createRequest.MarshalJSON()
	if err != nil {
		t.Fatalf("Error encoding create resource request: %s", err)
	}

//
	var recoveredCreateRequest Request
	if err := recoveredCreateRequest.UnmarshalJSON(messageRawData); err != nil {
		t.Fatalf("Error decoding create resource request: %s", err)
	}

//
	if err := recoveredCreateRequest.Verify(); err == nil {
		t.Fatal("Expected Verify to fail since the message is not signed")
	}

//
//
//

	metaHash := createRequest.metaHash
	rootAddr := createRequest.rootAddr
	const expectedSignature = "0x1c2bab66dc4ed63783d62934e3a628e517888d6949aef0349f3bd677121db9aa09bbfb865904e6c50360e209e0fe6fe757f8a2474cf1b34169c99b95e3fd5a5101"
	const expectedJSON = `{"rootAddr":"0x6e744a730f7ea0881528576f0354b6268b98e35a6981ef703153ff1b8d32bbef","metaHash":"0x0c0d5c18b89da503af92302a1a64fab6acb60f78e288eb9c3d541655cd359b60","version":1,"period":7,"data":"0x5468697320686f75722773207570646174653a20537761726d2039392e3020686173206265656e2072656c656173656421","multiHash":false}`

//
	data := []byte("This hour's update: Swarm 99.0 has been released!")
	request := &Request{
		SignedResourceUpdate: SignedResourceUpdate{
			resourceUpdate: resourceUpdate{
				updateHeader: updateHeader{
					UpdateLookup: UpdateLookup{
						period:   7,
						version:  1,
						rootAddr: rootAddr,
					},
					multihash: false,
					metaHash:  metaHash,
				},
				data: data,
			},
		},
	}

	messageRawData, err = request.MarshalJSON()
	if err != nil {
		t.Fatalf("Error encoding update request: %s", err)
	}

	equalJSON, err := areEqualJSON(string(messageRawData), expectedJSON)
	if err != nil {
		t.Fatalf("Error decoding update request JSON: %s", err)
	}
	if !equalJSON {
		t.Fatalf("Received a different JSON message. Expected %s, got %s", expectedJSON, string(messageRawData))
	}

//

//
	var recoveredRequest Request
	if err := recoveredRequest.UnmarshalJSON(messageRawData); err != nil {
		t.Fatalf("Error decoding update request: %s", err)
	}

//
	if err := recoveredRequest.Sign(signer); err != nil {
		t.Fatalf("Error signing request: %s", err)
	}

	compareByteSliceToExpectedHex(t, "signature", recoveredRequest.signature[:], expectedSignature)

//
//
	var j updateRequestJSON
	if err := json.Unmarshal([]byte(expectedJSON), &j); err != nil {
		t.Fatal("Error unmarshalling test json, check expectedJSON constant")
	}
	j.Signature = "Certainly not a signature"
corruptMessage, _ := json.Marshal(j) //
	var corruptRequest Request
	if err = corruptRequest.UnmarshalJSON(corruptMessage); err == nil {
		t.Fatal("Expected DecodeUpdateRequest to fail when trying to interpret a corrupt message with an invalid signature")
	}

//
//
	if err := request.Sign(falseSigner); err != nil {
		t.Fatalf("Error signing: %s", err)
	}

//
	messageRawData, err = request.MarshalJSON()
	if err != nil {
		t.Fatalf("Error encoding message:%s", err)
	}

//
	recoveredRequest = Request{}
	if err := recoveredRequest.UnmarshalJSON(messageRawData); err != nil {
		t.Fatalf("Error decoding message:%s", err)
	}

//
//
savedSignature := *recoveredRequest.signature                               //
binary.LittleEndian.PutUint64(recoveredRequest.signature[5:], 556845463424) //
	if err = recoveredRequest.Verify(); err == nil {
		t.Fatal("Expected Verify to fail on corrupt signature")
	}

//
	*recoveredRequest.signature = savedSignature

//
	if err = recoveredRequest.Verify(); err == nil {
		t.Fatalf("Expected Verify to fail because this resource belongs to Charlie, not Bob the attacker:%s", err)
	}

//
	if err := recoveredRequest.Sign(signer); err != nil {
		t.Fatalf("Error signing with the correct private key: %s", err)
	}

//
	if err = recoveredRequest.Verify(); err != nil {
		t.Fatalf("Error verifying that Charlie, the good guy, can sign his resource:%s", err)
	}
}
