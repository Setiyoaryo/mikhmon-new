package routeros

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/Setiyoaryo/mikhmon-new/internal/mockrouteros"
)

func TestBatchDoesNotReplayAddAfterLostReply(t *testing.T) {
	srv, err := mockrouteros.Start("127.0.0.1:0", mockrouteros.Options{Username: "admin", Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	m := NewManager(ManagerOptions{MaxConnPerRouter: 1})
	defer m.Close()
	sess, err := m.Connect(Target{Addr: srv.Addr(), Username: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	clientConn, routerConn := net.Pipe()
	sess.pool.idle[0].Close()
	sess.pool.idle[0] = &Client{
		conn: clientConn, reader: bufio.NewReader(clientConn), writer: bufio.NewWriter(clientConn), timeout: time.Second,
	}
	received := make(chan Sentence, 1)
	go func() {
		peer := &Client{conn: routerConn, reader: bufio.NewReader(routerConn)}
		command, _ := peer.readSentence()
		received <- command
		// The command was received; its reply is lost. Whether it was applied
		// is unknown to the client, so replaying a mutation is unsafe.
		routerConn.Close()
	}()
	_, errs, fatal := m.ExecBatch(sess.ID, 1, time.Second, [][]string{{"/ip/hotspot/user/add", "=name=uncertain"}})
	if command := <-received; command.Type() != "/ip/hotspot/user/add" {
		t.Fatalf("wrong command: %v", command)
	}
	if fatal != nil || errs[0] == nil {
		t.Fatalf("lost acknowledgement must be reported: %v %v", fatal, errs)
	}
	if srv.UserCount() != 0 {
		t.Fatal("mutation was replayed after losing its acknowledgement")
	}
}

func TestBatchRecoversDeadIdleConnection(t *testing.T) {
	srv, err := mockrouteros.Start("127.0.0.1:0", mockrouteros.Options{Username: "admin", Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	m := NewManager(ManagerOptions{MaxConnPerRouter: 1})
	defer m.Close()
	sess, err := m.Connect(Target{Addr: srv.Addr(), Username: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	p := sess.pool
	p.idle[0].Close()
	_, errs, fatal := m.ExecBatch(sess.ID, 1, time.Second, [][]string{{"/system/clock/print"}})
	if fatal != nil || errs[0] != nil {
		t.Fatalf("batch failed on stale pooled socket: %v %v", fatal, errs)
	}
}

func TestBatchKeepsConnectionAfterRouterRejection(t *testing.T) {
	srv, err := mockrouteros.Start("127.0.0.1:0", mockrouteros.Options{Username: "admin", Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	m := NewManager(ManagerOptions{MaxConnPerRouter: 1})
	defer m.Close()
	sess, err := m.Connect(Target{Addr: srv.Addr(), Username: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	_, errs, fatal := m.ExecBatch(sess.ID, 1, time.Second, [][]string{
		{"/ip/hotspot/user/add", "=name=dup"}, {"/ip/hotspot/user/add", "=name=dup"},
	})
	if fatal != nil || errs[0] != nil || errs[1] == nil {
		t.Fatalf("incorrect errors: %v %v", fatal, errs)
	}
	open, idle := sess.pool.stats()
	if open != 1 || idle != 1 {
		t.Fatalf("trap discarded healthy connection: open=%d idle=%d", open, idle)
	}
}
