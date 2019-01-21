
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

package pss

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/swarm/log"
)

const (
	IsActiveProtocol = true
)

//
type ProtocolMsg struct {
	Code       uint64
	Size       uint32
	Payload    []byte
	ReceivedAt time.Time
}

//
func NewProtocolMsg(code uint64, msg interface{}) ([]byte, error) {

	rlpdata, err := rlp.EncodeToBytes(msg)
	if err != nil {
		return nil, err
	}

//
	smsg := &ProtocolMsg{
		Code:    code,
		Size:    uint32(len(rlpdata)),
		Payload: rlpdata,
	}

	return rlp.EncodeToBytes(smsg)
}

//
//
//
type ProtocolParams struct {
	Asymmetric bool
	Symmetric  bool
}

//
//
//
type PssReadWriter struct {
	*Pss
	LastActive time.Time
	rw         chan p2p.Msg
	spec       *protocols.Spec
	topic      *Topic
	sendFunc   func(string, Topic, []byte) error
	key        string
	closed     bool
}

//
func (prw *PssReadWriter) ReadMsg() (p2p.Msg, error) {
	msg := <-prw.rw
	log.Trace(fmt.Sprintf("pssrw readmsg: %v", msg))
	return msg, nil
}

//
func (prw *PssReadWriter) WriteMsg(msg p2p.Msg) error {
	log.Trace("pssrw writemsg", "msg", msg)
	if prw.closed {
		return fmt.Errorf("connection closed")
	}
	rlpdata := make([]byte, msg.Size)
	msg.Payload.Read(rlpdata)
	pmsg, err := rlp.EncodeToBytes(ProtocolMsg{
		Code:    msg.Code,
		Size:    msg.Size,
		Payload: rlpdata,
	})
	if err != nil {
		return err
	}
	return prw.sendFunc(prw.key, *prw.topic, pmsg)
}

//
func (prw *PssReadWriter) injectMsg(msg p2p.Msg) error {
	log.Trace(fmt.Sprintf("pssrw injectmsg: %v", msg))
	prw.rw <- msg
	return nil
}

//
type Protocol struct {
	*Pss
	proto        *p2p.Protocol
	topic        *Topic
	spec         *protocols.Spec
	pubKeyRWPool map[string]p2p.MsgReadWriter
	symKeyRWPool map[string]p2p.MsgReadWriter
	Asymmetric   bool
	Symmetric    bool
	RWPoolMu     sync.Mutex
}

//
//
//
//
//
//
func RegisterProtocol(ps *Pss, topic *Topic, spec *protocols.Spec, targetprotocol *p2p.Protocol, options *ProtocolParams) (*Protocol, error) {
	if !options.Asymmetric && !options.Symmetric {
		return nil, fmt.Errorf("specify at least one of asymmetric or symmetric messaging mode")
	}
	pp := &Protocol{
		Pss:          ps,
		proto:        targetprotocol,
		topic:        topic,
		spec:         spec,
		pubKeyRWPool: make(map[string]p2p.MsgReadWriter),
		symKeyRWPool: make(map[string]p2p.MsgReadWriter),
		Asymmetric:   options.Asymmetric,
		Symmetric:    options.Symmetric,
	}
	return pp, nil
}

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
func (p *Protocol) Handle(msg []byte, peer *p2p.Peer, asymmetric bool, keyid string) error {
	var vrw *PssReadWriter
	if p.Asymmetric != asymmetric && p.Symmetric == !asymmetric {
		return fmt.Errorf("invalid protocol encryption")
	} else if (!p.isActiveSymKey(keyid, *p.topic) && !asymmetric) ||
		(!p.isActiveAsymKey(keyid, *p.topic) && asymmetric) {

		rw, err := p.AddPeer(peer, *p.topic, asymmetric, keyid)
		if err != nil {
			return err
		} else if rw == nil {
			return fmt.Errorf("handle called on nil MsgReadWriter for new key " + keyid)
		}
		vrw = rw.(*PssReadWriter)
	}

	pmsg, err := ToP2pMsg(msg)
	if err != nil {
		return fmt.Errorf("could not decode pssmsg")
	}
	if asymmetric {
		if p.pubKeyRWPool[keyid] == nil {
			return fmt.Errorf("handle called on nil MsgReadWriter for key " + keyid)
		}
		vrw = p.pubKeyRWPool[keyid].(*PssReadWriter)
	} else {
		if p.symKeyRWPool[keyid] == nil {
			return fmt.Errorf("handle called on nil MsgReadWriter for key " + keyid)
		}
		vrw = p.symKeyRWPool[keyid].(*PssReadWriter)
	}
	vrw.injectMsg(pmsg)
	return nil
}

//
func (p *Protocol) isActiveSymKey(key string, topic Topic) bool {
	return p.symKeyRWPool[key] != nil
}

//
func (p *Protocol) isActiveAsymKey(key string, topic Topic) bool {
	return p.pubKeyRWPool[key] != nil
}

//
func ToP2pMsg(msg []byte) (p2p.Msg, error) {
	payload := &ProtocolMsg{}
	if err := rlp.DecodeBytes(msg, payload); err != nil {
		return p2p.Msg{}, fmt.Errorf("pss protocol handler unable to decode payload as p2p message: %v", err)
	}

	return p2p.Msg{
		Code:       payload.Code,
		Size:       uint32(len(payload.Payload)),
		ReceivedAt: time.Now(),
		Payload:    bytes.NewBuffer(payload.Payload),
	}, nil
}

//
//
//
//
//
func (p *Protocol) AddPeer(peer *p2p.Peer, topic Topic, asymmetric bool, key string) (p2p.MsgReadWriter, error) {
	rw := &PssReadWriter{
		Pss:   p.Pss,
		rw:    make(chan p2p.Msg),
		spec:  p.spec,
		topic: p.topic,
		key:   key,
	}
	if asymmetric {
		rw.sendFunc = p.Pss.SendAsym
	} else {
		rw.sendFunc = p.Pss.SendSym
	}
	if asymmetric {
		p.Pss.pubKeyPoolMu.Lock()
		if _, ok := p.Pss.pubKeyPool[key]; !ok {
			return nil, fmt.Errorf("asym key does not exist: %s", key)
		}
		p.Pss.pubKeyPoolMu.Unlock()
		p.RWPoolMu.Lock()
		p.pubKeyRWPool[key] = rw
		p.RWPoolMu.Unlock()
	} else {
		p.Pss.symKeyPoolMu.Lock()
		if _, ok := p.Pss.symKeyPool[key]; !ok {
			return nil, fmt.Errorf("symkey does not exist: %s", key)
		}
		p.Pss.symKeyPoolMu.Unlock()
		p.RWPoolMu.Lock()
		p.symKeyRWPool[key] = rw
		p.RWPoolMu.Unlock()
	}
	go func() {
		err := p.proto.Run(peer, rw)
		log.Warn(fmt.Sprintf("pss vprotocol quit on %v topic %v: %v", peer, topic, err))
	}()
	return rw, nil
}

func (p *Protocol) RemovePeer(asymmetric bool, key string) {
	log.Debug("closing pss peer", "asym", asymmetric, "key", key)
	p.RWPoolMu.Lock()
	defer p.RWPoolMu.Unlock()
	if asymmetric {
		rw := p.pubKeyRWPool[key].(*PssReadWriter)
		rw.closed = true
		delete(p.pubKeyRWPool, key)
	} else {
		rw := p.symKeyRWPool[key].(*PssReadWriter)
		rw.closed = true
		delete(p.symKeyRWPool, key)
	}
}

//
func ProtocolTopic(spec *protocols.Spec) Topic {
	return BytesToTopic([]byte(fmt.Sprintf("%s:%d", spec.Name, spec.Version)))
}
