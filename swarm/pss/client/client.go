
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

package client

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/pss"
)

const (
	handshakeRetryTimeout = 1000
	handshakeRetryCount   = 3
)

//
//
type Client struct {
	BaseAddrHex string

//
	peerPool map[pss.Topic]map[string]*pssRPCRW
	protos   map[pss.Topic]*p2p.Protocol

//
	rpc  *rpc.Client
	subs []*rpc.ClientSubscription

//
	topicsC chan []byte
	quitC   chan struct{}

	poolMu sync.Mutex
}

//
type pssRPCRW struct {
	*Client
	topic    string
	msgC     chan []byte
	addr     pss.PssAddress
	pubKeyId string
	lastSeen time.Time
	closed   bool
}

func (c *Client) newpssRPCRW(pubkeyid string, addr pss.PssAddress, topicobj pss.Topic) (*pssRPCRW, error) {
	topic := topicobj.String()
	err := c.rpc.Call(nil, "pss_setPeerPublicKey", pubkeyid, topic, hexutil.Encode(addr[:]))
	if err != nil {
		return nil, fmt.Errorf("setpeer %s %s: %v", topic, pubkeyid, err)
	}
	return &pssRPCRW{
		Client:   c,
		topic:    topic,
		msgC:     make(chan []byte),
		addr:     addr,
		pubKeyId: pubkeyid,
	}, nil
}

func (rw *pssRPCRW) ReadMsg() (p2p.Msg, error) {
	msg := <-rw.msgC
	log.Trace("pssrpcrw read", "msg", msg)
	pmsg, err := pss.ToP2pMsg(msg)
	if err != nil {
		return p2p.Msg{}, err
	}

	return pmsg, nil
}

//
//
//
//
//
//
//
//
func (rw *pssRPCRW) WriteMsg(msg p2p.Msg) error {
	log.Trace("got writemsg pssclient", "msg", msg)
	if rw.closed {
		return fmt.Errorf("connection closed")
	}
	rlpdata := make([]byte, msg.Size)
	msg.Payload.Read(rlpdata)
	pmsg, err := rlp.EncodeToBytes(pss.ProtocolMsg{
		Code:    msg.Code,
		Size:    msg.Size,
		Payload: rlpdata,
	})
	if err != nil {
		return err
	}

//
	var symkeyids []string
	err = rw.Client.rpc.Call(&symkeyids, "pss_getHandshakeKeys", rw.pubKeyId, rw.topic, false, true)
	if err != nil {
		return err
	}

//
	var symkeycap uint16
	if len(symkeyids) > 0 {
		err = rw.Client.rpc.Call(&symkeycap, "pss_getHandshakeKeyCapacity", symkeyids[0])
		if err != nil {
			return err
		}
	}

	err = rw.Client.rpc.Call(nil, "pss_sendSym", symkeyids[0], rw.topic, hexutil.Encode(pmsg))
	if err != nil {
		return err
	}

//
	if symkeycap == 1 {
		var retries int
		var sync bool
//
		if len(symkeyids) == 1 {
			sync = true
		}
//
		_, err := rw.handshake(retries, sync, false)
		if err != nil {
			log.Warn("failing", "err", err)
			return err
		}
	}
	return nil
}

//
//
func (rw *pssRPCRW) handshake(retries int, sync bool, flush bool) (string, error) {

	var symkeyids []string
	var i int
//
//
	for i = 0; i < 1+retries; i++ {
		log.Debug("handshake attempt pssrpcrw", "pubkeyid", rw.pubKeyId, "topic", rw.topic, "sync", sync)
		err := rw.Client.rpc.Call(&symkeyids, "pss_handshake", rw.pubKeyId, rw.topic, sync, flush)
		if err == nil {
			var keyid string
			if sync {
				keyid = symkeyids[0]
			}
			return keyid, nil
		}
		if i-1+retries > 1 {
			time.Sleep(time.Millisecond * handshakeRetryTimeout)
		}
	}

	return "", fmt.Errorf("handshake failed after %d attempts", i)
}

//
//
//
func NewClient(rpcurl string) (*Client, error) {
	rpcclient, err := rpc.Dial(rpcurl)
	if err != nil {
		return nil, err
	}

	client, err := NewClientWithRPC(rpcclient)
	if err != nil {
		return nil, err
	}
	return client, nil
}

//
//
//
func NewClientWithRPC(rpcclient *rpc.Client) (*Client, error) {
	client := newClient()
	client.rpc = rpcclient
	err := client.rpc.Call(&client.BaseAddrHex, "pss_baseAddr")
	if err != nil {
		return nil, fmt.Errorf("cannot get pss node baseaddress: %v", err)
	}
	return client, nil
}

func newClient() (client *Client) {
	client = &Client{
		quitC:    make(chan struct{}),
		peerPool: make(map[pss.Topic]map[string]*pssRPCRW),
		protos:   make(map[pss.Topic]*p2p.Protocol),
	}
	return
}

//
//
//
//
//
//
//
func (c *Client) RunProtocol(ctx context.Context, proto *p2p.Protocol) error {
	topicobj := pss.BytesToTopic([]byte(fmt.Sprintf("%s:%d", proto.Name, proto.Version)))
	topichex := topicobj.String()
	msgC := make(chan pss.APIMsg)
	c.peerPool[topicobj] = make(map[string]*pssRPCRW)
	sub, err := c.rpc.Subscribe(ctx, "pss", msgC, "receive", topichex)
	if err != nil {
		return fmt.Errorf("pss event subscription failed: %v", err)
	}
	c.subs = append(c.subs, sub)
	err = c.rpc.Call(nil, "pss_addHandshake", topichex)
	if err != nil {
		return fmt.Errorf("pss handshake activation failed: %v", err)
	}

//
	go func() {
		for {
			select {
			case msg := <-msgC:
//
				if msg.Asymmetric {
					continue
				}
//
//
				var pubkeyid string
				err = c.rpc.Call(&pubkeyid, "pss_getHandshakePublicKey", msg.Key)
				if err != nil || pubkeyid == "" {
					log.Trace("proto err or no pubkey", "err", err, "symkeyid", msg.Key)
					continue
				}
//
//
				if c.peerPool[topicobj][pubkeyid] == nil {
					var addrhex string
					err := c.rpc.Call(&addrhex, "pss_getAddress", topichex, false, msg.Key)
					if err != nil {
						log.Trace(err.Error())
						continue
					}
					addrbytes, err := hexutil.Decode(addrhex)
					if err != nil {
						log.Trace(err.Error())
						break
					}
					addr := pss.PssAddress(addrbytes)
					rw, err := c.newpssRPCRW(pubkeyid, addr, topicobj)
					if err != nil {
						break
					}
					c.peerPool[topicobj][pubkeyid] = rw
					nid, _ := discover.HexID("0x00")
					p := p2p.NewPeer(nid, fmt.Sprintf("%v", addr), []p2p.Cap{})
					go proto.Run(p, c.peerPool[topicobj][pubkeyid])
				}
				go func() {
					c.peerPool[topicobj][pubkeyid].msgC <- msg.Msg
				}()
			case <-c.quitC:
				return
			}
		}
	}()

	c.protos[topicobj] = proto
	return nil
}

//
func (c *Client) Close() error {
	for _, s := range c.subs {
		s.Unsubscribe()
	}
	return nil
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
func (c *Client) AddPssPeer(pubkeyid string, addr []byte, spec *protocols.Spec) error {
	topic := pss.ProtocolTopic(spec)
	if c.peerPool[topic] == nil {
		return errors.New("addpeer on unset topic")
	}
	if c.peerPool[topic][pubkeyid] == nil {
		rw, err := c.newpssRPCRW(pubkeyid, addr, topic)
		if err != nil {
			return err
		}
		_, err = rw.handshake(handshakeRetryCount, true, true)
		if err != nil {
			return err
		}
		c.poolMu.Lock()
		c.peerPool[topic][pubkeyid] = rw
		c.poolMu.Unlock()
		nid, _ := discover.HexID("0x00")
		p := p2p.NewPeer(nid, fmt.Sprintf("%v", addr), []p2p.Cap{})
		go c.protos[topic].Run(p, c.peerPool[topic][pubkeyid])
	}
	return nil
}

//
//
//
func (c *Client) RemovePssPeer(pubkeyid string, spec *protocols.Spec) {
	log.Debug("closing pss client peer", "pubkey", pubkeyid, "protoname", spec.Name, "protoversion", spec.Version)
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	topic := pss.ProtocolTopic(spec)
	c.peerPool[topic][pubkeyid].closed = true
	delete(c.peerPool[topic], pubkeyid)
}
