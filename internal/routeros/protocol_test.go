package routeros_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Setiyoaryo/mikhmon-new/internal/mockrouteros"
	"github.com/Setiyoaryo/mikhmon-new/internal/routeros"
)

func startMock(t *testing.T) *mockrouteros.Server {
	t.Helper()
	srv, err := mockrouteros.Start("127.0.0.1:0", mockrouteros.Options{
		Username: "admin",
		Password: "secret",
		Quiet:    true,
	})
	if err != nil {
		t.Fatalf("start mock: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

func TestDialAndLoginModern(t *testing.T) {
	srv := startMock(t)

	c, err := routeros.DialAndLogin(srv.Addr(), "admin", "secret", false, 3*time.Second)
	if err != nil {
		t.Fatalf("dial and login: %v", err)
	}
	defer c.Close()

	replies, err := c.RunCommand("/ip/hotspot/user/add", "=name=alice", "=password=pw")
	if err != nil {
		t.Fatalf("add user: %v", err)
	}
	if len(replies) == 0 || replies[len(replies)-1].Type() != "!done" {
		t.Fatalf("expected a !done reply, got %v", replies)
	}
	if srv.UserCount() != 1 {
		t.Fatalf("expected 1 user on the mock, got %d", srv.UserCount())
	}
}

func TestDialAndLoginRejectsBadPassword(t *testing.T) {
	srv := startMock(t)

	if _, err := routeros.DialAndLogin(srv.Addr(), "admin", "nope", false, 3*time.Second); err == nil {
		t.Fatal("expected an authentication error")
	}
}

func TestLegacyLoginFlow(t *testing.T) {
	srv, err := mockrouteros.Start("127.0.0.1:0", mockrouteros.Options{
		Username: "admin",
		Password: "secret",
		Legacy:   true,
		Quiet:    true,
	})
	if err != nil {
		t.Fatalf("start mock: %v", err)
	}
	defer srv.Close()

	c, err := routeros.DialAndLogin(srv.Addr(), "admin", "secret", false, 3*time.Second)
	if err != nil {
		t.Fatalf("legacy login: %v", err)
	}
	defer c.Close()

	if _, err := c.RunCommand("/system/identity/print"); err != nil {
		t.Fatalf("command after legacy login: %v", err)
	}
}

func TestRunCommandSurfacesTrap(t *testing.T) {
	srv := startMock(t)

	c, err := routeros.DialAndLogin(srv.Addr(), "admin", "secret", false, 3*time.Second)
	if err != nil {
		t.Fatalf("dial and login: %v", err)
	}
	defer c.Close()

	if _, err := c.RunCommand("/ip/hotspot/user/add", "=name=dup", "=password=x"); err != nil {
		t.Fatalf("first add should succeed: %v", err)
	}

	_, err = c.RunCommand("/ip/hotspot/user/add", "=name=dup", "=password=x")
	if err == nil {
		t.Fatal("expected a duplicate-name error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "already have") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

func TestPoolReusesConnections(t *testing.T) {
	srv := startMock(t)

	mgr := routeros.NewManager(routeros.ManagerOptions{MaxConnPerRouter: 4})
	defer mgr.Close()

	target := routeros.Target{
		Addr:     srv.Addr(),
		Username: "admin",
		Password: "secret",
		Timeout:  3 * time.Second,
	}

	sess, err := mgr.Connect(target)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	for i := 0; i < 25; i++ {
		if _, err := mgr.Exec(sess.ID, 3*time.Second, []string{"/system/clock/print"}); err != nil {
			t.Fatalf("exec %d: %v", i, err)
		}
	}

	stats := mgr.Stats()
	routers, ok := stats["routers"].(map[string]any)
	if !ok || len(routers) != 1 {
		t.Fatalf("expected exactly one pooled router, got %v", stats)
	}
	for _, v := range routers {
		info := v.(map[string]int)
		// The mock accepted a single TCP connection; everything else reused it.
		if info["open"] != 1 || info["idle"] != 1 {
			t.Fatalf("expected 1 idle reused connection, got %v", info)
		}
	}
}

func TestExecRetriesDeadPooledConnection(t *testing.T) {
	srv := startMock(t)

	mgr := routeros.NewManager(routeros.ManagerOptions{MaxConnPerRouter: 2})
	defer mgr.Close()

	target := routeros.Target{
		Addr:     srv.Addr(),
		Username: "admin",
		Password: "secret",
		Timeout:  3 * time.Second,
	}
	sess, err := mgr.Connect(target)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	// Kill the pooled socket behind the manager's back: exactly what happens
	// when a router drops an idle API session.
	if err := srv.Close(); err != nil {
		t.Fatalf("close mock: %v", err)
	}
	restarted, err := mockrouteros.Start(srv.Addr(), mockrouteros.Options{
		Username: "admin",
		Password: "secret",
		Quiet:    true,
	})
	if err != nil {
		t.Fatalf("restart mock: %v", err)
	}
	defer restarted.Close()

	if _, err := mgr.Exec(sess.ID, 3*time.Second, []string{"/system/clock/print"}); err != nil {
		t.Fatalf("expected the pool to reconnect and retry, got %v", err)
	}
}

func TestExecBatchRunsInParallel(t *testing.T) {
	srv := startMock(t)

	mgr := routeros.NewManager(routeros.ManagerOptions{MaxConnPerRouter: 16})
	defer mgr.Close()

	sess, err := mgr.Connect(routeros.Target{
		Addr:     srv.Addr(),
		Username: "admin",
		Password: "secret",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	const n = 300
	cmds := make([][]string, n)
	for i := range cmds {
		cmds[i] = []string{"/ip/hotspot/user/add", "=name=u" + itoa(i), "=password=p" + itoa(i)}
	}

	_, errs, fatal := mgr.ExecBatch(sess.ID, 16, 5*time.Second, cmds)
	if fatal != nil {
		t.Fatalf("exec batch: %v", fatal)
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("command %d failed: %v", i, err)
		}
	}
	if srv.UserCount() != n {
		t.Fatalf("expected %d users, got %d", n, srv.UserCount())
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [8]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
