
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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/swarm/log"
)

//
//
func TestHandshake(t *testing.T) {
	t.Run("32", testHandshake)
	t.Run("8", testHandshake)
	t.Run("0", testHandshake)
}

func testHandshake(t *testing.T) {

//
	useHandshake = true
	var addrsize int64
	var err error
	addrsizestring := strings.Split(t.Name(), "/")
	addrsize, _ = strconv.ParseInt(addrsizestring[1], 10, 0)

//
//
	clients, err := setupNetwork(2)
	if err != nil {
		t.Fatal(err)
	}

	var topic string
	err = clients[0].Call(&topic, "pss_stringToTopic", "foo:42")
	if err != nil {
		t.Fatal(err)
	}

	var loaddr string
	err = clients[0].Call(&loaddr, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 1 baseaddr fail: %v", err)
	}
//
	loaddr = loaddr[:2+(addrsize*2)]
	var roaddr string
	err = clients[1].Call(&roaddr, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 2 baseaddr fail: %v", err)
	}
	roaddr = roaddr[:2+(addrsize*2)]
	log.Debug("addresses", "left", loaddr, "right", roaddr)

//
//
	var lpubkey string
	err = clients[0].Call(&lpubkey, "pss_getPublicKey")
	if err != nil {
		t.Fatalf("rpc get node 1 pubkey fail: %v", err)
	}
	var rpubkey string
	err = clients[1].Call(&rpubkey, "pss_getPublicKey")
	if err != nil {
		t.Fatalf("rpc get node 2 pubkey fail: %v", err)
	}

time.Sleep(time.Millisecond * 1000) //

//
	err = clients[0].Call(nil, "pss_setPeerPublicKey", rpubkey, topic, roaddr)
	if err != nil {
		t.Fatal(err)
	}
	err = clients[1].Call(nil, "pss_setPeerPublicKey", lpubkey, topic, loaddr)
	if err != nil {
		t.Fatal(err)
	}

//
//
//
//
//
//
	err = clients[0].Call(nil, "pss_addHandshake", topic)
	if err != nil {
		t.Fatal(err)
	}
	err = clients[1].Call(nil, "pss_addHandshake", topic)
	if err != nil {
		t.Fatal(err)
	}

	var lhsendsymkeyids []string
	err = clients[0].Call(&lhsendsymkeyids, "pss_handshake", rpubkey, topic, true, true)
	if err != nil {
		t.Fatal(err)
	}

//
	time.Sleep(time.Second)

//
	var lsendsymkeyids []string
	err = clients[0].Call(&lsendsymkeyids, "pss_getHandshakeKeys", rpubkey, topic, false, true)
	if err != nil {
		t.Fatal(err)
	}
	m := 0
	for _, hid := range lhsendsymkeyids {
		for _, lid := range lsendsymkeyids {
			if lid == hid {
				m++
			}
		}
	}
	if m != defaultSymKeyCapacity {
		t.Fatalf("buffer size mismatch, expected %d, have %d: %v", defaultSymKeyCapacity, m, lsendsymkeyids)
	}

//
	var rsendsymkeyids []string
	err = clients[1].Call(&rsendsymkeyids, "pss_getHandshakeKeys", lpubkey, topic, false, true)
	if err != nil {
		t.Fatal(err)
	}
	var lrecvsymkeyids []string
	err = clients[0].Call(&lrecvsymkeyids, "pss_getHandshakeKeys", rpubkey, topic, true, false)
	if err != nil {
		t.Fatal(err)
	}
	var rrecvsymkeyids []string
	err = clients[1].Call(&rrecvsymkeyids, "pss_getHandshakeKeys", lpubkey, topic, true, false)
	if err != nil {
		t.Fatal(err)
	}

//
	var lsendsymkeys []string
	for _, id := range lsendsymkeyids {
		var key string
		err = clients[0].Call(&key, "pss_getSymmetricKey", id)
		if err != nil {
			t.Fatal(err)
		}
		lsendsymkeys = append(lsendsymkeys, key)
	}
	var rsendsymkeys []string
	for _, id := range rsendsymkeyids {
		var key string
		err = clients[1].Call(&key, "pss_getSymmetricKey", id)
		if err != nil {
			t.Fatal(err)
		}
		rsendsymkeys = append(rsendsymkeys, key)
	}

//
	var lrecvsymkeys []string
	for _, id := range lrecvsymkeyids {
		var key string
		err = clients[0].Call(&key, "pss_getSymmetricKey", id)
		if err != nil {
			t.Fatal(err)
		}
		match := false
		for _, otherkey := range rsendsymkeys {
			if otherkey == key {
				match = true
			}
		}
		if !match {
			t.Fatalf("no match right send for left recv key %s", id)
		}
		lrecvsymkeys = append(lrecvsymkeys, key)
	}
	var rrecvsymkeys []string
	for _, id := range rrecvsymkeyids {
		var key string
		err = clients[1].Call(&key, "pss_getSymmetricKey", id)
		if err != nil {
			t.Fatal(err)
		}
		match := false
		for _, otherkey := range lsendsymkeys {
			if otherkey == key {
				match = true
			}
		}
		if !match {
			t.Fatalf("no match left send for right recv key %s", id)
		}
		rrecvsymkeys = append(rrecvsymkeys, key)
	}

//
	err = clients[0].Call(nil, "pss_handshake", rpubkey, topic, false)
	if err == nil {
		t.Fatal("expected full symkey buffer error")
	}

//
	err = clients[0].Call(nil, "pss_releaseHandshakeKey", rpubkey, topic, lsendsymkeyids[0], true)
	if err != nil {
		t.Fatalf("release left send key %s fail: %v", lsendsymkeyids[0], err)
	}

	var newlhsendkeyids []string

//
//
	err = clients[0].Call(&newlhsendkeyids, "pss_handshake", rpubkey, topic, true, false)
	if err != nil {
		t.Fatalf("handshake send fail: %v", err)
	} else if len(newlhsendkeyids) != defaultSymKeyCapacity {
		t.Fatalf("wrong receive count, expected 1, got %d", len(newlhsendkeyids))
	}

	var newlrecvsymkey string
	err = clients[0].Call(&newlrecvsymkey, "pss_getSymmetricKey", newlhsendkeyids[0])
	if err != nil {
		t.Fatal(err)
	}
	var rmatchsymkeyid *string
	for i, id := range rrecvsymkeyids {
		var key string
		err = clients[1].Call(&key, "pss_getSymmetricKey", id)
		if err != nil {
			t.Fatal(err)
		}
		if newlrecvsymkey == key {
			rmatchsymkeyid = &rrecvsymkeyids[i]
		}
	}
	if rmatchsymkeyid != nil {
		t.Fatalf("right sent old key id %s in second handshake", *rmatchsymkeyid)
	}

//
	var cleancount int
	clients[0].Call(&cleancount, "psstest_clean")
	if cleancount > 1 {
		t.Fatalf("pss clean count mismatch; expected 1, got %d", cleancount)
	}
}
