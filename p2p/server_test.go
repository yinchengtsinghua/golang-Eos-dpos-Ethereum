
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2014 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

package p2p

import (
	"crypto/ecdsa"
	"errors"
	"math/rand"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
)

func init() {
//log.root（）.sethandler（log.lvlfilterhandler（log.lvlerror，log.streamhandler（os.stderr，log.terminalformat（false）））
}

type testTransport struct {
	id discover.NodeID
	*rlpx

	closeErr error
}

func newTestTransport(id discover.NodeID, fd net.Conn) transport {
	wrapped := newRLPX(fd).(*rlpx)
	wrapped.rw = newRLPXFrameRW(fd, secrets{
		MAC:        zero16,
		AES:        zero16,
		IngressMAC: sha3.NewKeccak256(),
		EgressMAC:  sha3.NewKeccak256(),
	})
	return &testTransport{id: id, rlpx: wrapped}
}

func (c *testTransport) doEncHandshake(prv *ecdsa.PrivateKey, dialDest *discover.Node) (discover.NodeID, error) {
	return c.id, nil
}

func (c *testTransport) doProtoHandshake(our *protoHandshake) (*protoHandshake, error) {
	return &protoHandshake{ID: c.id, Name: "test"}, nil
}

func (c *testTransport) close(err error) {
	c.rlpx.fd.Close()
	c.closeErr = err
}

func startTestServer(t *testing.T, id discover.NodeID, pf func(*Peer)) *Server {
	config := Config{
		Name:       "test",
		MaxPeers:   10,
		ListenAddr: "127.0.0.1:0",
		PrivateKey: newkey(),
	}
	server := &Server{
		Config:       config,
		newPeerHook:  pf,
		newTransport: func(fd net.Conn) transport { return newTestTransport(id, fd) },
	}
	if err := server.Start(); err != nil {
		t.Fatalf("Could not start server: %v", err)
	}
	return server
}

func TestServerListen(t *testing.T) {
//启动测试服务器
	connected := make(chan *Peer)
	remid := randomID()
	srv := startTestServer(t, remid, func(p *Peer) {
		if p.ID() != remid {
			t.Error("peer func called with wrong node id")
		}
		if p == nil {
			t.Error("peer func called with nil conn")
		}
		connected <- p
	})
	defer close(connected)
	defer srv.Stop()

//拨号测试服务器
	conn, err := net.DialTimeout("tcp", srv.ListenAddr, 5*time.Second)
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	defer conn.Close()

	select {
	case peer := <-connected:
		if peer.LocalAddr().String() != conn.RemoteAddr().String() {
			t.Errorf("peer started with wrong conn: got %v, want %v",
				peer.LocalAddr(), conn.RemoteAddr())
		}
		peers := srv.Peers()
		if !reflect.DeepEqual(peers, []*Peer{peer}) {
			t.Errorf("Peers mismatch: got %v, want %v", peers, []*Peer{peer})
		}
	case <-time.After(1 * time.Second):
		t.Error("server did not accept within one second")
	}
}

func TestServerDial(t *testing.T) {
//运行一次性TCP服务器来处理连接。
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not setup listener: %v", err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Error("accept error:", err)
			return
		}
		accepted <- conn
	}()

//启动服务器
	connected := make(chan *Peer)
	remid := randomID()
	srv := startTestServer(t, remid, func(p *Peer) { connected <- p })
	defer close(connected)
	defer srv.Stop()

//告诉服务器连接
	tcpAddr := listener.Addr().(*net.TCPAddr)
	node := &discover.Node{ID: remid, IP: tcpAddr.IP, TCP: uint16(tcpAddr.Port)}
	srv.AddPeer(node)

	select {
	case conn := <-accepted:
		defer conn.Close()

		select {
		case peer := <-connected:
			if peer.ID() != remid {
				t.Errorf("peer has wrong id")
			}
			if peer.Name() != "test" {
				t.Errorf("peer has wrong name")
			}
			if peer.RemoteAddr().String() != conn.LocalAddr().String() {
				t.Errorf("peer started with wrong conn: got %v, want %v",
					peer.RemoteAddr(), conn.LocalAddr())
			}
			peers := srv.Peers()
			if !reflect.DeepEqual(peers, []*Peer{peer}) {
				t.Errorf("Peers mismatch: got %v, want %v", peers, []*Peer{peer})
			}

//测试addtrustedpeer/removetrustedpeer并更改可信标志
//尤其是在改变旗国的比赛条件下。
			if peer := srv.Peers()[0]; peer.Info().Network.Trusted {
				t.Errorf("peer is trusted prematurely: %v", peer)
			}
			done := make(chan bool)
			go func() {
				srv.AddTrustedPeer(node)
				if peer := srv.Peers()[0]; !peer.Info().Network.Trusted {
					t.Errorf("peer is not trusted after AddTrustedPeer: %v", peer)
				}
				srv.RemoveTrustedPeer(node)
				if peer := srv.Peers()[0]; peer.Info().Network.Trusted {
					t.Errorf("peer is trusted after RemoveTrustedPeer: %v", peer)
				}
				done <- true
			}()
//触发潜在的竞争条件
			peer = srv.Peers()[0]
			_ = peer.Inbound()
			_ = peer.Info()
			<-done
		case <-time.After(1 * time.Second):
			t.Error("server did not launch peer within one second")
		}

	case <-time.After(1 * time.Second):
		t.Error("server did not connect within one second")
	}
}

//此测试检查由拨号状态生成的任务是否
//实际执行并调用taskdone。
func TestServerTaskScheduling(t *testing.T) {
	var (
		done           = make(chan *testTask)
		quit, returned = make(chan struct{}), make(chan struct{})
		tc             = 0
		tg             = taskgen{
			newFunc: func(running int, peers map[discover.NodeID]*Peer) []task {
				tc++
				return []task{&testTask{index: tc - 1}}
			},
			doneFunc: func(t task) {
				select {
				case done <- t.(*testTask):
				case <-quit:
				}
			},
		}
	)

//此测试中的服务器没有实际运行
//因为我们只对Run的功能感兴趣。
	srv := &Server{
		Config:  Config{MaxPeers: 10},
		quit:    make(chan struct{}),
		ntab:    fakeTable{},
		running: true,
		log:     log.New(),
	}
	srv.loopWG.Add(1)
	go func() {
		srv.run(tg)
		close(returned)
	}()

	var gotdone []*testTask
	for i := 0; i < 100; i++ {
		gotdone = append(gotdone, <-done)
	}
	for i, task := range gotdone {
		if task.index != i {
			t.Errorf("task %d has wrong index, got %d", i, task.index)
			break
		}
		if !task.called {
			t.Errorf("task %d was not called", i)
			break
		}
	}

	close(quit)
	srv.Stop()
	select {
	case <-returned:
	case <-time.After(500 * time.Millisecond):
		t.Error("Server.run did not return within 500ms")
	}
}

//此测试检查服务器是否不删除任务，
//即使newtasks返回的任务数超过最大值。
func TestServerManyTasks(t *testing.T) {
	alltasks := make([]task, 300)
	for i := range alltasks {
		alltasks[i] = &testTask{index: i}
	}

	var (
		srv = &Server{
			quit:    make(chan struct{}),
			ntab:    fakeTable{},
			running: true,
			log:     log.New(),
		}
		done       = make(chan *testTask)
		start, end = 0, 0
	)
	defer srv.Stop()
	srv.loopWG.Add(1)
	go srv.run(taskgen{
		newFunc: func(running int, peers map[discover.NodeID]*Peer) []task {
			start, end = end, end+maxActiveDialTasks+10
			if end > len(alltasks) {
				end = len(alltasks)
			}
			return alltasks[start:end]
		},
		doneFunc: func(tt task) {
			done <- tt.(*testTask)
		},
	})

	doneset := make(map[int]bool)
	timeout := time.After(2 * time.Second)
	for len(doneset) < len(alltasks) {
		select {
		case tt := <-done:
			if doneset[tt.index] {
				t.Errorf("task %d got done more than once", tt.index)
			} else {
				doneset[tt.index] = true
			}
		case <-timeout:
			t.Errorf("%d of %d tasks got done within 2s", len(doneset), len(alltasks))
			for i := 0; i < len(alltasks); i++ {
				if !doneset[i] {
					t.Logf("task %d not done", i)
				}
			}
			return
		}
	}
}

type taskgen struct {
	newFunc  func(running int, peers map[discover.NodeID]*Peer) []task
	doneFunc func(task)
}

func (tg taskgen) newTasks(running int, peers map[discover.NodeID]*Peer, now time.Time) []task {
	return tg.newFunc(running, peers)
}
func (tg taskgen) taskDone(t task, now time.Time) {
	tg.doneFunc(t)
}
func (tg taskgen) addStatic(*discover.Node) {
}
func (tg taskgen) removeStatic(*discover.Node) {
}

type testTask struct {
	index  int
	called bool
}

func (t *testTask) Do(srv *Server) {
	t.called = true
}

//此测试检查连接是否断开
//当服务器处于
//按容量计算。仍应接受可信连接。
func TestServerAtCap(t *testing.T) {
	trustedID := randomID()
	srv := &Server{
		Config: Config{
			PrivateKey:   newkey(),
			MaxPeers:     10,
			NoDial:       true,
			TrustedNodes: []*discover.Node{{ID: trustedID}},
		},
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("could not start: %v", err)
	}
	defer srv.Stop()

	newconn := func(id discover.NodeID) *conn {
		fd, _ := net.Pipe()
		tx := newTestTransport(id, fd)
		return &conn{fd: fd, transport: tx, flags: inboundConn, id: id, cont: make(chan error)}
	}

//注入一些连接以填充对等集。
	for i := 0; i < 10; i++ {
		c := newconn(randomID())
		if err := srv.checkpoint(c, srv.addpeer); err != nil {
			t.Fatalf("could not add conn %d: %v", i, err)
		}
	}
//尝试插入不受信任的连接。
	anotherID := randomID()
	c := newconn(anotherID)
	if err := srv.checkpoint(c, srv.posthandshake); err != DiscTooManyPeers {
		t.Error("wrong error for insert:", err)
	}
//尝试插入可信连接。
	c = newconn(trustedID)
	if err := srv.checkpoint(c, srv.posthandshake); err != nil {
		t.Error("unexpected error for trusted conn @posthandshake:", err)
	}
	if !c.is(trustedConn) {
		t.Error("Server did not set trusted flag")
	}

//从受信任集中删除，然后重试
	srv.RemoveTrustedPeer(&discover.Node{ID: trustedID})
	c = newconn(trustedID)
	if err := srv.checkpoint(c, srv.posthandshake); err != DiscTooManyPeers {
		t.Error("wrong error for insert:", err)
	}

//将另一个ID添加到可信集，然后重试
	srv.AddTrustedPeer(&discover.Node{ID: anotherID})
	c = newconn(anotherID)
	if err := srv.checkpoint(c, srv.posthandshake); err != nil {
		t.Error("unexpected error for trusted conn @posthandshake:", err)
	}
	if !c.is(trustedConn) {
		t.Error("Server did not set trusted flag")
	}
}

func TestServerPeerLimits(t *testing.T) {
	srvkey := newkey()

	clientid := randomID()
	clientnode := &discover.Node{ID: clientid}

	var tp *setupTransport = &setupTransport{
		id: clientid,
		phs: &protoHandshake{
			ID: clientid,
//由于未匹配的大写字母而强制使用“discuselesspeer”
//大写：[]Cap Discard.Cap（），
		},
	}
	var flags connFlag = dynDialedConn
	var dialDest *discover.Node = &discover.Node{ID: clientid}

	srv := &Server{
		Config: Config{
			PrivateKey: srvkey,
			MaxPeers:   0,
			NoDial:     true,
			Protocols:  []Protocol{discard},
		},
		newTransport: func(fd net.Conn) transport { return tp },
		log:          log.New(),
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("couldn't start server: %v", err)
	}
	defer srv.Stop()

//检查服务器是否已满（maxpeers=0）
	conn, _ := net.Pipe()
	srv.SetupConn(conn, flags, dialDest)
	if tp.closeErr != DiscTooManyPeers {
		t.Errorf("unexpected close error: %q", tp.closeErr)
	}
	conn.Close()

	srv.AddTrustedPeer(clientnode)

//检查服务器是否允许受信任的对等机（尽管已满）。
	conn, _ = net.Pipe()
	srv.SetupConn(conn, flags, dialDest)
	if tp.closeErr == DiscTooManyPeers {
		t.Errorf("failed to bypass MaxPeers with trusted node: %q", tp.closeErr)
	}

	if tp.closeErr != DiscUselessPeer {
		t.Errorf("unexpected close error: %q", tp.closeErr)
	}
	conn.Close()

	srv.RemoveTrustedPeer(clientnode)

//再次检查服务器是否已满。
	conn, _ = net.Pipe()
	srv.SetupConn(conn, flags, dialDest)
	if tp.closeErr != DiscTooManyPeers {
		t.Errorf("unexpected close error: %q", tp.closeErr)
	}
	conn.Close()
}

func TestServerSetupConn(t *testing.T) {
	id := randomID()
	srvkey := newkey()
	srvid := discover.PubkeyID(&srvkey.PublicKey)
	tests := []struct {
		dontstart bool
		tt        *setupTransport
		flags     connFlag
		dialDest  *discover.Node

		wantCloseErr error
		wantCalls    string
	}{
		{
			dontstart:    true,
			tt:           &setupTransport{id: id},
			wantCalls:    "close,",
			wantCloseErr: errServerStopped,
		},
		{
			tt:           &setupTransport{id: id, encHandshakeErr: errors.New("read error")},
			flags:        inboundConn,
			wantCalls:    "doEncHandshake,close,",
			wantCloseErr: errors.New("read error"),
		},
		{
			tt:           &setupTransport{id: id},
			dialDest:     &discover.Node{ID: randomID()},
			flags:        dynDialedConn,
			wantCalls:    "doEncHandshake,close,",
			wantCloseErr: DiscUnexpectedIdentity,
		},
		{
			tt:           &setupTransport{id: id, phs: &protoHandshake{ID: randomID()}},
			dialDest:     &discover.Node{ID: id},
			flags:        dynDialedConn,
			wantCalls:    "doEncHandshake,doProtoHandshake,close,",
			wantCloseErr: DiscUnexpectedIdentity,
		},
		{
			tt:           &setupTransport{id: id, protoHandshakeErr: errors.New("foo")},
			dialDest:     &discover.Node{ID: id},
			flags:        dynDialedConn,
			wantCalls:    "doEncHandshake,doProtoHandshake,close,",
			wantCloseErr: errors.New("foo"),
		},
		{
			tt:           &setupTransport{id: srvid, phs: &protoHandshake{ID: srvid}},
			flags:        inboundConn,
			wantCalls:    "doEncHandshake,close,",
			wantCloseErr: DiscSelf,
		},
		{
			tt:           &setupTransport{id: id, phs: &protoHandshake{ID: id}},
			flags:        inboundConn,
			wantCalls:    "doEncHandshake,doProtoHandshake,close,",
			wantCloseErr: DiscUselessPeer,
		},
	}

	for i, test := range tests {
		srv := &Server{
			Config: Config{
				PrivateKey: srvkey,
				MaxPeers:   10,
				NoDial:     true,
				Protocols:  []Protocol{discard},
			},
			newTransport: func(fd net.Conn) transport { return test.tt },
			log:          log.New(),
		}
		if !test.dontstart {
			if err := srv.Start(); err != nil {
				t.Fatalf("couldn't start server: %v", err)
			}
		}
		p1, _ := net.Pipe()
		srv.SetupConn(p1, test.flags, test.dialDest)
		if !reflect.DeepEqual(test.tt.closeErr, test.wantCloseErr) {
			t.Errorf("test %d: close error mismatch: got %q, want %q", i, test.tt.closeErr, test.wantCloseErr)
		}
		if test.tt.calls != test.wantCalls {
			t.Errorf("test %d: calls mismatch: got %q, want %q", i, test.tt.calls, test.wantCalls)
		}
	}
}

type setupTransport struct {
	id              discover.NodeID
	encHandshakeErr error

	phs               *protoHandshake
	protoHandshakeErr error

	calls    string
	closeErr error
}

func (c *setupTransport) doEncHandshake(prv *ecdsa.PrivateKey, dialDest *discover.Node) (discover.NodeID, error) {
	c.calls += "doEncHandshake,"
	return c.id, c.encHandshakeErr
}
func (c *setupTransport) doProtoHandshake(our *protoHandshake) (*protoHandshake, error) {
	c.calls += "doProtoHandshake,"
	if c.protoHandshakeErr != nil {
		return nil, c.protoHandshakeErr
	}
	return c.phs, nil
}
func (c *setupTransport) close(err error) {
	c.calls += "close,"
	c.closeErr = err
}

//SetupConn不应写入/读取连接。
func (c *setupTransport) WriteMsg(Msg) error {
	panic("WriteMsg called on setupTransport")
}
func (c *setupTransport) ReadMsg() (Msg, error) {
	panic("ReadMsg called on setupTransport")
}

func newkey() *ecdsa.PrivateKey {
	key, err := crypto.GenerateKey()
	if err != nil {
		panic("couldn't generate key: " + err.Error())
	}
	return key
}

func randomID() (id discover.NodeID) {
	for i := range id {
		id[i] = byte(rand.Intn(255))
	}
	return id
}
