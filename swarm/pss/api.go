
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
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/log"
)

//
//
type APIMsg struct {
	Msg        hexutil.Bytes
	Asymmetric bool
	Key        string
}

//
type API struct {
	*Pss
}

func NewAPI(ps *Pss) *API {
	return &API{Pss: ps}
}

//
//
//
//
//
//
func (pssapi *API) Receive(ctx context.Context, topic Topic) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return nil, fmt.Errorf("Subscribe not supported")
	}

	psssub := notifier.CreateSubscription()

	handler := func(msg []byte, p *p2p.Peer, asymmetric bool, keyid string) error {
		apimsg := &APIMsg{
			Msg:        hexutil.Bytes(msg),
			Asymmetric: asymmetric,
			Key:        keyid,
		}
		if err := notifier.Notify(psssub.ID, apimsg); err != nil {
			log.Warn(fmt.Sprintf("notification on pss sub topic rpc (sub %v) msg %v failed!", psssub.ID, msg))
		}
		return nil
	}

	deregf := pssapi.Register(&topic, handler)
	go func() {
		defer deregf()
		select {
		case err := <-psssub.Err():
			log.Warn(fmt.Sprintf("caught subscription error in pss sub topic %x: %v", topic, err))
		case <-notifier.Closed():
			log.Warn(fmt.Sprintf("rpc sub notifier closed"))
		}
	}()

	return psssub, nil
}

func (pssapi *API) GetAddress(topic Topic, asymmetric bool, key string) (PssAddress, error) {
	var addr *PssAddress
	if asymmetric {
		peer, ok := pssapi.Pss.pubKeyPool[key][topic]
		if !ok {
			return nil, fmt.Errorf("pubkey/topic pair %x/%x doesn't exist", key, topic)
		}
		addr = peer.address
	} else {
		peer, ok := pssapi.Pss.symKeyPool[key][topic]
		if !ok {
			return nil, fmt.Errorf("symkey/topic pair %x/%x doesn't exist", key, topic)
		}
		addr = peer.address

	}
	return *addr, nil
}

//
func (pssapi *API) BaseAddr() (PssAddress, error) {
	return PssAddress(pssapi.Pss.BaseAddr()), nil
}

//
func (pssapi *API) GetPublicKey() (keybytes hexutil.Bytes) {
	key := pssapi.Pss.PublicKey()
	keybytes = crypto.FromECDSAPub(key)
	return keybytes
}

//
func (pssapi *API) SetPeerPublicKey(pubkey hexutil.Bytes, topic Topic, addr PssAddress) error {
	pk, err := crypto.UnmarshalPubkey(pubkey)
	if err != nil {
		return fmt.Errorf("Cannot unmarshal pubkey: %x", pubkey)
	}
	err = pssapi.Pss.SetPeerPublicKey(pk, topic, &addr)
	if err != nil {
		return fmt.Errorf("Invalid key: %x", pk)
	}
	return nil
}

func (pssapi *API) GetSymmetricKey(symkeyid string) (hexutil.Bytes, error) {
	symkey, err := pssapi.Pss.GetSymmetricKey(symkeyid)
	return hexutil.Bytes(symkey), err
}

func (pssapi *API) GetSymmetricAddressHint(topic Topic, symkeyid string) (PssAddress, error) {
	return *pssapi.Pss.symKeyPool[symkeyid][topic].address, nil
}

func (pssapi *API) GetAsymmetricAddressHint(topic Topic, pubkeyid string) (PssAddress, error) {
	return *pssapi.Pss.pubKeyPool[pubkeyid][topic].address, nil
}

func (pssapi *API) StringToTopic(topicstring string) (Topic, error) {
	topicbytes := BytesToTopic([]byte(topicstring))
	if topicbytes == rawTopic {
		return rawTopic, errors.New("Topic string hashes to 0x00000000 and cannot be used")
	}
	return topicbytes, nil
}

func (pssapi *API) SendAsym(pubkeyhex string, topic Topic, msg hexutil.Bytes) error {
	return pssapi.Pss.SendAsym(pubkeyhex, topic, msg[:])
}

func (pssapi *API) SendSym(symkeyhex string, topic Topic, msg hexutil.Bytes) error {
	return pssapi.Pss.SendSym(symkeyhex, topic, msg[:])
}

func (pssapi *API) GetPeerTopics(pubkeyhex string) ([]Topic, error) {
	topics, _, err := pssapi.Pss.GetPublickeyPeers(pubkeyhex)
	return topics, err

}

func (pssapi *API) GetPeerAddress(pubkeyhex string, topic Topic) (PssAddress, error) {
	return pssapi.Pss.getPeerAddress(pubkeyhex, topic)
}
