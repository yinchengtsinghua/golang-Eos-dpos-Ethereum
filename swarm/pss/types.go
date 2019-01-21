
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

package pss

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/swarm/storage"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv5"
)

const (
	defaultWhisperTTL = 6000
)

const (
	pssControlSym = 1
	pssControlRaw = 1 << 1
)

var (
	topicHashMutex = sync.Mutex{}
	topicHashFunc  = storage.MakeHashFunc("SHA256")()
	rawTopic       = Topic{}
)

//
type Topic whisper.TopicType

func (t *Topic) String() string {
	return hexutil.Encode(t[:])
}

//
func (t Topic) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

//
func (t *Topic) UnmarshalJSON(input []byte) error {
	topicbytes, err := hexutil.Decode(string(input[1 : len(input)-1]))
	if err != nil {
		return err
	}
	copy(t[:], topicbytes)
	return nil
}

//
type PssAddress []byte

//
func (a PssAddress) MarshalJSON() ([]byte, error) {
	return json.Marshal(hexutil.Encode(a[:]))
}

//
func (a *PssAddress) UnmarshalJSON(input []byte) error {
	b, err := hexutil.Decode(string(input[1 : len(input)-1]))
	if err != nil {
		return err
	}
	for _, bb := range b {
		*a = append(*a, bb)
	}
	return nil
}

//
type pssDigest [digestLength]byte

//
type msgParams struct {
	raw bool
	sym bool
}

func newMsgParamsFromBytes(paramBytes []byte) *msgParams {
	if len(paramBytes) != 1 {
		return nil
	}
	return &msgParams{
		raw: paramBytes[0]&pssControlRaw > 0,
		sym: paramBytes[0]&pssControlSym > 0,
	}
}

func (m *msgParams) Bytes() (paramBytes []byte) {
	var b byte
	if m.raw {
		b |= pssControlRaw
	}
	if m.sym {
		b |= pssControlSym
	}
	paramBytes = append(paramBytes, b)
	return paramBytes
}

//
type PssMsg struct {
	To      []byte
	Control []byte
	Expire  uint32
	Payload *whisper.Envelope
}

func newPssMsg(param *msgParams) *PssMsg {
	return &PssMsg{
		Control: param.Bytes(),
	}
}

//
func (msg *PssMsg) isRaw() bool {
	return msg.Control[0]&pssControlRaw > 0
}

//
func (msg *PssMsg) isSym() bool {
	return msg.Control[0]&pssControlSym > 0
}

//
func (msg *PssMsg) serialize() []byte {
	rlpdata, _ := rlp.EncodeToBytes(struct {
		To      []byte
		Payload *whisper.Envelope
	}{
		To:      msg.To,
		Payload: msg.Payload,
	})
	return rlpdata
}

//
func (msg *PssMsg) String() string {
	return fmt.Sprintf("PssMsg: Recipient: %x", common.ToHex(msg.To))
}

//
//
//
type Handler func(msg []byte, p *p2p.Peer, asymmetric bool, keyid string) error

//
//
type stateStore struct {
	values map[string][]byte
}

func newStateStore() *stateStore {
	return &stateStore{values: make(map[string][]byte)}
}

func (store *stateStore) Load(key string) ([]byte, error) {
	return nil, nil
}

func (store *stateStore) Save(key string, v []byte) error {
	return nil
}

//
func BytesToTopic(b []byte) Topic {
	topicHashMutex.Lock()
	defer topicHashMutex.Unlock()
	topicHashFunc.Reset()
	topicHashFunc.Write(b)
	return Topic(whisper.BytesToTopic(topicHashFunc.Sum(nil)))
}
