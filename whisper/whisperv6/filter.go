
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

package whisperv6

import (
	"crypto/ecdsa"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

//
type Filter struct {
Src        *ecdsa.PublicKey  //
KeyAsym    *ecdsa.PrivateKey //
KeySym     []byte            //
Topics     [][]byte          //
PoW        float64           //
AllowP2P   bool              //
SymKeyHash common.Hash       //
id         string            //

	Messages map[common.Hash]*ReceivedMessage
	mutex    sync.RWMutex
}

//
type Filters struct {
	watchers map[string]*Filter

topicMatcher     map[TopicType]map[*Filter]struct{} //
allTopicsMatcher map[*Filter]struct{}               //

	whisper *Whisper
	mutex   sync.RWMutex
}

//
func NewFilters(w *Whisper) *Filters {
	return &Filters{
		watchers:         make(map[string]*Filter),
		topicMatcher:     make(map[TopicType]map[*Filter]struct{}),
		allTopicsMatcher: make(map[*Filter]struct{}),
		whisper:          w,
	}
}

//
func (fs *Filters) Install(watcher *Filter) (string, error) {
	if watcher.KeySym != nil && watcher.KeyAsym != nil {
		return "", fmt.Errorf("filters must choose between symmetric and asymmetric keys")
	}

	if watcher.Messages == nil {
		watcher.Messages = make(map[common.Hash]*ReceivedMessage)
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", err
	}

	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	if fs.watchers[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}

	if watcher.expectsSymmetricEncryption() {
		watcher.SymKeyHash = crypto.Keccak256Hash(watcher.KeySym)
	}

	watcher.id = id
	fs.watchers[id] = watcher
	fs.addTopicMatcher(watcher)
	return id, err
}

//
//
func (fs *Filters) Uninstall(id string) bool {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	if fs.watchers[id] != nil {
		fs.removeFromTopicMatchers(fs.watchers[id])
		delete(fs.watchers, id)
		return true
	}
	return false
}

//
//
//
func (fs *Filters) addTopicMatcher(watcher *Filter) {
	if len(watcher.Topics) == 0 {
		fs.allTopicsMatcher[watcher] = struct{}{}
	} else {
		for _, t := range watcher.Topics {
			topic := BytesToTopic(t)
			if fs.topicMatcher[topic] == nil {
				fs.topicMatcher[topic] = make(map[*Filter]struct{})
			}
			fs.topicMatcher[topic][watcher] = struct{}{}
		}
	}
}

//
func (fs *Filters) removeFromTopicMatchers(watcher *Filter) {
	delete(fs.allTopicsMatcher, watcher)
	for _, topic := range watcher.Topics {
		delete(fs.topicMatcher[BytesToTopic(topic)], watcher)
	}
}

//
//
func (fs *Filters) getWatchersByTopic(topic TopicType) []*Filter {
	res := make([]*Filter, 0, len(fs.allTopicsMatcher))
	for watcher := range fs.allTopicsMatcher {
		res = append(res, watcher)
	}
	for watcher := range fs.topicMatcher[topic] {
		res = append(res, watcher)
	}
	return res
}

//
func (fs *Filters) Get(id string) *Filter {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()
	return fs.watchers[id]
}

//
//
func (fs *Filters) NotifyWatchers(env *Envelope, p2pMessage bool) {
	var msg *ReceivedMessage

	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	candidates := fs.getWatchersByTopic(env.Topic)
	for _, watcher := range candidates {
		if p2pMessage && !watcher.AllowP2P {
			log.Trace(fmt.Sprintf("msg [%x], filter [%s]: p2p messages are not allowed", env.Hash(), watcher.id))
			continue
		}

		var match bool
		if msg != nil {
			match = watcher.MatchMessage(msg)
		} else {
			match = watcher.MatchEnvelope(env)
			if match {
				msg = env.Open(watcher)
				if msg == nil {
					log.Trace("processing message: failed to open", "message", env.Hash().Hex(), "filter", watcher.id)
				}
			} else {
				log.Trace("processing message: does not match", "message", env.Hash().Hex(), "filter", watcher.id)
			}
		}

		if match && msg != nil {
			log.Trace("processing message: decrypted", "hash", env.Hash().Hex())
			if watcher.Src == nil || IsPubKeyEqual(msg.Src, watcher.Src) {
				watcher.Trigger(msg)
			}
		}
	}
}

func (f *Filter) expectsAsymmetricEncryption() bool {
	return f.KeyAsym != nil
}

func (f *Filter) expectsSymmetricEncryption() bool {
	return f.KeySym != nil
}

//
//
func (f *Filter) Trigger(msg *ReceivedMessage) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if _, exist := f.Messages[msg.EnvelopeHash]; !exist {
		f.Messages[msg.EnvelopeHash] = msg
	}
}

//
//
func (f *Filter) Retrieve() (all []*ReceivedMessage) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	all = make([]*ReceivedMessage, 0, len(f.Messages))
	for _, msg := range f.Messages {
		all = append(all, msg)
	}

f.Messages = make(map[common.Hash]*ReceivedMessage) //
	return all
}

//
//
//
//
func (f *Filter) MatchMessage(msg *ReceivedMessage) bool {
	if f.PoW > 0 && msg.PoW < f.PoW {
		return false
	}

	if f.expectsAsymmetricEncryption() && msg.isAsymmetricEncryption() {
		return IsPubKeyEqual(&f.KeyAsym.PublicKey, msg.Dst)
	} else if f.expectsSymmetricEncryption() && msg.isSymmetricEncryption() {
		return f.SymKeyHash == msg.SymKeyHash
	}
	return false
}

//
//
//
//
func (f *Filter) MatchEnvelope(envelope *Envelope) bool {
	return f.PoW <= 0 || envelope.pow >= f.PoW
}

//
func IsPubKeyEqual(a, b *ecdsa.PublicKey) bool {
	if !ValidatePublicKey(a) {
		return false
	} else if !ValidatePublicKey(b) {
		return false
	}
//
	return a.X.Cmp(b.X) == 0 && a.Y.Cmp(b.Y) == 0
}
