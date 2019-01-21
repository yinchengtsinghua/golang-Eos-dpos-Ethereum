
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
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/log"
)

const (
	IsActiveHandshake = true
)

var (
	ctrlSingleton *HandshakeController
)

const (
defaultSymKeyRequestTimeout = 1000 * 8  //
defaultSymKeyExpiryTimeout  = 1000 * 10 //
defaultSymKeySendLimit      = 256       //
defaultSymKeyCapacity       = 4         //
)

//
type handshakeMsg struct {
	From    []byte
	Limit   uint16
	Keys    [][]byte
	Request uint8
	Topic   Topic
}

//
type handshakeKey struct {
	symKeyID  *string
	pubKeyID  *string
	limit     uint16
	count     uint16
	expiredAt time.Time
}

//
//
type handshake struct {
	outKeys []handshakeKey
	inKeys  []handshakeKey
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
type HandshakeParams struct {
	SymKeyRequestTimeout time.Duration
	SymKeyExpiryTimeout  time.Duration
	SymKeySendLimit      uint16
	SymKeyCapacity       uint8
}

//
func NewHandshakeParams() *HandshakeParams {
	return &HandshakeParams{
		SymKeyRequestTimeout: defaultSymKeyRequestTimeout * time.Millisecond,
		SymKeyExpiryTimeout:  defaultSymKeyExpiryTimeout * time.Millisecond,
		SymKeySendLimit:      defaultSymKeySendLimit,
		SymKeyCapacity:       defaultSymKeyCapacity,
	}
}

//
//
type HandshakeController struct {
	pss                  *Pss
keyC                 map[string]chan []string //
	lock                 sync.Mutex
	symKeyRequestTimeout time.Duration
	symKeyExpiryTimeout  time.Duration
	symKeySendLimit      uint16
	symKeyCapacity       uint8
	symKeyIndex          map[string]*handshakeKey
	handshakes           map[string]map[Topic]*handshake
	deregisterFuncs      map[Topic]func()
}

//
//
//
func SetHandshakeController(pss *Pss, params *HandshakeParams) error {
	ctrl := &HandshakeController{
		pss:                  pss,
		keyC:                 make(map[string]chan []string),
		symKeyRequestTimeout: params.SymKeyRequestTimeout,
		symKeyExpiryTimeout:  params.SymKeyExpiryTimeout,
		symKeySendLimit:      params.SymKeySendLimit,
		symKeyCapacity:       params.SymKeyCapacity,
		symKeyIndex:          make(map[string]*handshakeKey),
		handshakes:           make(map[string]map[Topic]*handshake),
		deregisterFuncs:      make(map[Topic]func()),
	}
	api := &HandshakeAPI{
		namespace: "pss",
		ctrl:      ctrl,
	}
	pss.addAPI(rpc.API{
		Namespace: api.namespace,
		Version:   "0.2",
		Service:   api,
		Public:    true,
	})
	ctrlSingleton = ctrl
	return nil
}

//
//
func (ctl *HandshakeController) validKeys(pubkeyid string, topic *Topic, in bool) (validkeys []*string) {
	ctl.lock.Lock()
	defer ctl.lock.Unlock()
	now := time.Now()
	if _, ok := ctl.handshakes[pubkeyid]; !ok {
		return []*string{}
	} else if _, ok := ctl.handshakes[pubkeyid][*topic]; !ok {
		return []*string{}
	}
	var keystore *[]handshakeKey
	if in {
		keystore = &(ctl.handshakes[pubkeyid][*topic].inKeys)
	} else {
		keystore = &(ctl.handshakes[pubkeyid][*topic].outKeys)
	}

	for _, key := range *keystore {
		if key.limit <= key.count {
			ctl.releaseKey(*key.symKeyID, topic)
		} else if !key.expiredAt.IsZero() && key.expiredAt.Before(now) {
			ctl.releaseKey(*key.symKeyID, topic)
		} else {
			validkeys = append(validkeys, key.symKeyID)
		}
	}
	return
}

//
//
func (ctl *HandshakeController) updateKeys(pubkeyid string, topic *Topic, in bool, symkeyids []string, limit uint16) {
	ctl.lock.Lock()
	defer ctl.lock.Unlock()
	if _, ok := ctl.handshakes[pubkeyid]; !ok {
		ctl.handshakes[pubkeyid] = make(map[Topic]*handshake)

	}
	if ctl.handshakes[pubkeyid][*topic] == nil {
		ctl.handshakes[pubkeyid][*topic] = &handshake{}
	}
	var keystore *[]handshakeKey
	expire := time.Now()
	if in {
		keystore = &(ctl.handshakes[pubkeyid][*topic].inKeys)
	} else {
		keystore = &(ctl.handshakes[pubkeyid][*topic].outKeys)
		expire = expire.Add(time.Millisecond * ctl.symKeyExpiryTimeout)
	}
	for _, storekey := range *keystore {
		storekey.expiredAt = expire
	}
	for i := 0; i < len(symkeyids); i++ {
		storekey := handshakeKey{
			symKeyID: &symkeyids[i],
			pubKeyID: &pubkeyid,
			limit:    limit,
		}
		*keystore = append(*keystore, storekey)
		ctl.pss.symKeyPool[*storekey.symKeyID][*topic].protected = true
	}
	for i := 0; i < len(*keystore); i++ {
		ctl.symKeyIndex[*(*keystore)[i].symKeyID] = &((*keystore)[i])
	}
}

//
func (ctl *HandshakeController) releaseKey(symkeyid string, topic *Topic) bool {
	if ctl.symKeyIndex[symkeyid] == nil {
		log.Debug("no symkey", "symkeyid", symkeyid)
		return false
	}
	ctl.symKeyIndex[symkeyid].expiredAt = time.Now()
	log.Debug("handshake release", "symkeyid", symkeyid)
	return true
}

//
//
//
//
//
func (ctl *HandshakeController) cleanHandshake(pubkeyid string, topic *Topic, in bool, out bool) int {
	ctl.lock.Lock()
	defer ctl.lock.Unlock()
	var deletecount int
	var deletes []string
	now := time.Now()
	handshake := ctl.handshakes[pubkeyid][*topic]
	log.Debug("handshake clean", "pubkey", pubkeyid, "topic", topic)
	if in {
		for i, key := range handshake.inKeys {
			if key.expiredAt.Before(now) || (key.expiredAt.IsZero() && key.limit <= key.count) {
				log.Trace("handshake in clean remove", "symkeyid", *key.symKeyID)
				deletes = append(deletes, *key.symKeyID)
				handshake.inKeys[deletecount] = handshake.inKeys[i]
				deletecount++
			}
		}
		handshake.inKeys = handshake.inKeys[:len(handshake.inKeys)-deletecount]
	}
	if out {
		deletecount = 0
		for i, key := range handshake.outKeys {
			if key.expiredAt.Before(now) && (key.expiredAt.IsZero() && key.limit <= key.count) {
				log.Trace("handshake out clean remove", "symkeyid", *key.symKeyID)
				deletes = append(deletes, *key.symKeyID)
				handshake.outKeys[deletecount] = handshake.outKeys[i]
				deletecount++
			}
		}
		handshake.outKeys = handshake.outKeys[:len(handshake.outKeys)-deletecount]
	}
	for _, keyid := range deletes {
		delete(ctl.symKeyIndex, keyid)
		ctl.pss.symKeyPool[keyid][*topic].protected = false
	}
	return len(deletes)
}

//
func (ctl *HandshakeController) clean() {
	peerpubkeys := ctl.handshakes
	for pubkeyid, peertopics := range peerpubkeys {
		for topic := range peertopics {
			ctl.cleanHandshake(pubkeyid, &topic, true, true)
		}
	}
}

//
//
//
//
func (ctl *HandshakeController) handler(msg []byte, p *p2p.Peer, asymmetric bool, symkeyid string) error {
	if !asymmetric {
		if ctl.symKeyIndex[symkeyid] != nil {
			if ctl.symKeyIndex[symkeyid].count >= ctl.symKeyIndex[symkeyid].limit {
				return fmt.Errorf("discarding message using expired key: %s", symkeyid)
			}
			ctl.symKeyIndex[symkeyid].count++
			log.Trace("increment symkey recv use", "symsymkeyid", symkeyid, "count", ctl.symKeyIndex[symkeyid].count, "limit", ctl.symKeyIndex[symkeyid].limit, "receiver", common.ToHex(crypto.FromECDSAPub(ctl.pss.PublicKey())))
		}
		return nil
	}
	keymsg := &handshakeMsg{}
	err := rlp.DecodeBytes(msg, keymsg)
	if err == nil {
		err := ctl.handleKeys(symkeyid, keymsg)
		if err != nil {
			log.Error("handlekeys fail", "error", err)
		}
		return err
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
//
func (ctl *HandshakeController) handleKeys(pubkeyid string, keymsg *handshakeMsg) error {
//
	if len(keymsg.Keys) > 0 {
		log.Debug("received handshake keys", "pubkeyid", pubkeyid, "from", keymsg.From, "count", len(keymsg.Keys))
		var sendsymkeyids []string
		for _, key := range keymsg.Keys {
			sendsymkey := make([]byte, len(key))
			copy(sendsymkey, key)
			var address PssAddress
			copy(address[:], keymsg.From)
			sendsymkeyid, err := ctl.pss.setSymmetricKey(sendsymkey, keymsg.Topic, &address, false, false)
			if err != nil {
				return err
			}
			sendsymkeyids = append(sendsymkeyids, sendsymkeyid)
		}
		if len(sendsymkeyids) > 0 {
			ctl.updateKeys(pubkeyid, &keymsg.Topic, false, sendsymkeyids, keymsg.Limit)

			ctl.alertHandshake(pubkeyid, sendsymkeyids)
		}
	}

//
	if keymsg.Request > 0 {
		_, err := ctl.sendKey(pubkeyid, &keymsg.Topic, keymsg.Request)
		if err != nil {
			return err
		}
	}

	return nil
}

//
//
//
//
//
//
func (ctl *HandshakeController) sendKey(pubkeyid string, topic *Topic, keycount uint8) ([]string, error) {

	var requestcount uint8
	to := &PssAddress{}
	if _, ok := ctl.pss.pubKeyPool[pubkeyid]; !ok {
		return []string{}, errors.New("Invalid public key")
	} else if psp, ok := ctl.pss.pubKeyPool[pubkeyid][*topic]; ok {
		to = psp.address
	}

	recvkeys := make([][]byte, keycount)
	recvkeyids := make([]string, keycount)
	ctl.lock.Lock()
	if _, ok := ctl.handshakes[pubkeyid]; !ok {
		ctl.handshakes[pubkeyid] = make(map[Topic]*handshake)
	}
	ctl.lock.Unlock()

//
	outkeys := ctl.validKeys(pubkeyid, topic, false)
	if len(outkeys) < int(ctl.symKeyCapacity) {
//
		requestcount = ctl.symKeyCapacity
	}
//
	if requestcount == 0 && keycount == 0 {
		return []string{}, nil
	}

//
	for i := 0; i < len(recvkeyids); i++ {
		var err error
		recvkeyids[i], err = ctl.pss.GenerateSymmetricKey(*topic, to, true)
		if err != nil {
			return []string{}, fmt.Errorf("set receive symkey fail (pubkey %x topic %x): %v", pubkeyid, topic, err)
		}
		recvkeys[i], err = ctl.pss.GetSymmetricKey(recvkeyids[i])
		if err != nil {
			return []string{}, fmt.Errorf("GET Generated outgoing symkey fail (pubkey %x topic %x): %v", pubkeyid, topic, err)
		}
	}
	ctl.updateKeys(pubkeyid, topic, true, recvkeyids, ctl.symKeySendLimit)

//
	recvkeymsg := &handshakeMsg{
		From:    ctl.pss.BaseAddr(),
		Keys:    recvkeys,
		Request: requestcount,
		Limit:   ctl.symKeySendLimit,
		Topic:   *topic,
	}
	log.Debug("sending our symkeys", "pubkey", pubkeyid, "symkeys", recvkeyids, "limit", ctl.symKeySendLimit, "requestcount", requestcount, "keycount", len(recvkeys))
	recvkeybytes, err := rlp.EncodeToBytes(recvkeymsg)
	if err != nil {
		return []string{}, fmt.Errorf("rlp keymsg encode fail: %v", err)
	}
//
	err = ctl.pss.SendAsym(pubkeyid, *topic, recvkeybytes)
	if err != nil {
		return []string{}, fmt.Errorf("Send symkey failed: %v", err)
	}
	return recvkeyids, nil
}

//
func (ctl *HandshakeController) alertHandshake(pubkeyid string, symkeys []string) chan []string {
	if len(symkeys) > 0 {
		if _, ok := ctl.keyC[pubkeyid]; ok {
			ctl.keyC[pubkeyid] <- symkeys
			close(ctl.keyC[pubkeyid])
			delete(ctl.keyC, pubkeyid)
		}
		return nil
	}
	if _, ok := ctl.keyC[pubkeyid]; !ok {
		ctl.keyC[pubkeyid] = make(chan []string)
	}
	return ctl.keyC[pubkeyid]
}

type HandshakeAPI struct {
	namespace string
	ctrl      *HandshakeController
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
//
//
//
func (api *HandshakeAPI) Handshake(pubkeyid string, topic Topic, sync bool, flush bool) (keys []string, err error) {
	var hsc chan []string
	var keycount uint8
	if flush {
		keycount = api.ctrl.symKeyCapacity
	} else {
		validkeys := api.ctrl.validKeys(pubkeyid, &topic, false)
		keycount = api.ctrl.symKeyCapacity - uint8(len(validkeys))
	}
	if keycount == 0 {
		return keys, errors.New("Incoming symmetric key store is already full")
	}
	if sync {
		hsc = api.ctrl.alertHandshake(pubkeyid, []string{})
	}
	_, err = api.ctrl.sendKey(pubkeyid, &topic, keycount)
	if err != nil {
		return keys, err
	}
	if sync {
		ctx, cancel := context.WithTimeout(context.Background(), api.ctrl.symKeyRequestTimeout)
		defer cancel()
		select {
		case keys = <-hsc:
			log.Trace("sync handshake response receive", "key", keys)
		case <-ctx.Done():
			return []string{}, errors.New("timeout")
		}
	}
	return keys, nil
}

//
func (api *HandshakeAPI) AddHandshake(topic Topic) error {
	api.ctrl.deregisterFuncs[topic] = api.ctrl.pss.Register(&topic, api.ctrl.handler)
	return nil
}

//
func (api *HandshakeAPI) RemoveHandshake(topic *Topic) error {
	if _, ok := api.ctrl.deregisterFuncs[*topic]; ok {
		api.ctrl.deregisterFuncs[*topic]()
	}
	return nil
}

//
//
//
//
//
//
func (api *HandshakeAPI) GetHandshakeKeys(pubkeyid string, topic Topic, in bool, out bool) (keys []string, err error) {
	if in {
		for _, inkey := range api.ctrl.validKeys(pubkeyid, &topic, true) {
			keys = append(keys, *inkey)
		}
	}
	if out {
		for _, outkey := range api.ctrl.validKeys(pubkeyid, &topic, false) {
			keys = append(keys, *outkey)
		}
	}
	return keys, nil
}

//
//
func (api *HandshakeAPI) GetHandshakeKeyCapacity(symkeyid string) (uint16, error) {
	storekey := api.ctrl.symKeyIndex[symkeyid]
	if storekey == nil {
		return 0, fmt.Errorf("invalid symkey id %s", symkeyid)
	}
	return storekey.limit - storekey.count, nil
}

//
//
func (api *HandshakeAPI) GetHandshakePublicKey(symkeyid string) (string, error) {
	storekey := api.ctrl.symKeyIndex[symkeyid]
	if storekey == nil {
		return "", fmt.Errorf("invalid symkey id %s", symkeyid)
	}
	return *storekey.pubKeyID, nil
}

//
//
//
//
//
func (api *HandshakeAPI) ReleaseHandshakeKey(pubkeyid string, topic Topic, symkeyid string, flush bool) (removed bool, err error) {
	removed = api.ctrl.releaseKey(symkeyid, &topic)
	if removed && flush {
		api.ctrl.cleanHandshake(pubkeyid, &topic, true, true)
	}
	return
}

//
//
//
//
func (api *HandshakeAPI) SendSym(symkeyid string, topic Topic, msg hexutil.Bytes) (err error) {
	err = api.ctrl.pss.SendSym(symkeyid, topic, msg[:])
	if api.ctrl.symKeyIndex[symkeyid] != nil {
		if api.ctrl.symKeyIndex[symkeyid].count >= api.ctrl.symKeyIndex[symkeyid].limit {
			return errors.New("attempted send with expired key")
		}
		api.ctrl.symKeyIndex[symkeyid].count++
		log.Trace("increment symkey send use", "symkeyid", symkeyid, "count", api.ctrl.symKeyIndex[symkeyid].count, "limit", api.ctrl.symKeyIndex[symkeyid].limit, "receiver", common.ToHex(crypto.FromECDSAPub(api.ctrl.pss.PublicKey())))
	}
	return
}
