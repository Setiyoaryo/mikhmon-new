package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Setiyoaryo/mikhmon-new/internal/api"
	"github.com/Setiyoaryo/mikhmon-new/internal/mockrouteros"
)

type harness struct {
	srv  *httptest.Server
	mock *mockrouteros.Server
}

func newHarness(t *testing.T, opts api.Options) *harness {
	t.Helper()

	mock, err := mockrouteros.Start("127.0.0.1:0", mockrouteros.Options{
		Username: "admin",
		Password: "secret",
		Quiet:    true,
	})
	if err != nil {
		t.Fatalf("start mock router: %v", err)
	}

	opts.MaxConnPerRouter = 16
	opts.ReadTimeout = 10 * time.Second
	svc := api.New(opts)

	ts := httptest.NewServer(svc.Handler())
	t.Cleanup(func() {
		ts.Close()
		svc.Close()
		_ = mock.Close()
	})

	return &harness{srv: ts, mock: mock}
}

// rawPost returns the response untouched, for tests that assert on the status.
func (h *harness) rawPost(t *testing.T, path string, body any) *http.Response {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, h.srv.URL+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	return resp
}

func (h *harness) post(t *testing.T, path string, body any, headers map[string]string) map[string]any {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, h.srv.URL+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func (h *harness) connect(t *testing.T) string {
	t.Helper()
	out := h.post(t, "/v1/connect", map[string]any{
		"host": "127.0.0.1",
		"port": h.mock.Port(),
		"user": "admin",
		"pass": "secret",
	}, nil)

	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("connect failed: %v", out["error"])
	}
	sess, _ := out["session"].(string)
	if sess == "" {
		t.Fatal("connect returned an empty session id")
	}
	return sess
}

func TestHealthz(t *testing.T) {
	h := newHarness(t, api.Options{})

	resp, err := http.Get(h.srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("get healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestConnectRejectsBadCredentials(t *testing.T) {
	h := newHarness(t, api.Options{})

	out := h.post(t, "/v1/connect", map[string]any{
		"host": "127.0.0.1",
		"port": h.mock.Port(),
		"user": "admin",
		"pass": "wrong",
	}, nil)

	if ok, _ := out["ok"].(bool); ok {
		t.Fatal("expected ok=false for bad credentials")
	}
	if out["error"] == nil || out["error"] == "" {
		t.Fatal("expected an error message")
	}
}

func TestConnectRequiresHost(t *testing.T) {
	h := newHarness(t, api.Options{})

	out := h.post(t, "/v1/connect", map[string]any{"host": ""}, nil)
	if ok, _ := out["ok"].(bool); ok {
		t.Fatal("expected ok=false when host is missing")
	}
}

func TestExecReturnsRawSentences(t *testing.T) {
	h := newHarness(t, api.Options{})
	sess := h.connect(t)

	out := h.post(t, "/v1/exec", map[string]any{
		"session": sess,
		"sentences": [][]string{
			{"/ip/hotspot/user/add", "=name=alice", "=password=pw"},
		},
	}, nil)

	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("exec failed: %v", out["error"])
	}

	results, _ := out["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	first, _ := results[0].(map[string]any)
	if first["error"] != nil && first["error"] != "" {
		t.Fatalf("unexpected per-command error: %v", first["error"])
	}

	sentences, _ := first["sentences"].([]any)
	if len(sentences) == 0 {
		t.Fatal("expected at least one reply sentence")
	}
	// The PHP side flattens these words and feeds them to parseResponse().
	last, _ := sentences[len(sentences)-1].([]any)
	if len(last) == 0 || last[0] != "!done" {
		t.Fatalf("expected the last sentence to be !done, got %v", sentences)
	}

	if h.mock.UserCount() != 1 {
		t.Fatalf("expected 1 user, got %d", h.mock.UserCount())
	}
}

func TestExecReportsTrapAsPerCommandError(t *testing.T) {
	h := newHarness(t, api.Options{})
	sess := h.connect(t)

	h.post(t, "/v1/exec", map[string]any{
		"session":   sess,
		"sentences": [][]string{{"/ip/hotspot/user/add", "=name=dup", "=password=pw"}},
	}, nil)

	out := h.post(t, "/v1/exec", map[string]any{
		"session":   sess,
		"sentences": [][]string{{"/ip/hotspot/user/add", "=name=dup", "=password=pw"}},
	}, nil)

	results, _ := out["results"].([]any)
	first, _ := results[0].(map[string]any)
	if first["error"] == nil || first["error"] == "" {
		t.Fatal("expected the duplicate add to report an error")
	}
}

func TestExecBatchSendsSeveralSentencesInOneCall(t *testing.T) {
	h := newHarness(t, api.Options{})
	sess := h.connect(t)

	out := h.post(t, "/v1/exec", map[string]any{
		"session": sess,
		"sentences": [][]string{
			{"/ip/hotspot/user/add", "=name=a1", "=password=p1"},
			{"/ip/hotspot/user/add", "=name=a2", "=password=p2"},
			{"/ip/hotspot/user/add", "=name=a3", "=password=p3"},
		},
	}, nil)

	results, _ := out["results"].([]any)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if h.mock.UserCount() != 3 {
		t.Fatalf("expected 3 users, got %d", h.mock.UserCount())
	}
}

func TestBulkUserAddCreatesEverything(t *testing.T) {
	h := newHarness(t, api.Options{})
	sess := h.connect(t)

	users := make([]map[string]any, 400)
	for i := range users {
		users[i] = map[string]any{
			"name":              "bulk" + itoa(i),
			"password":          "bulk" + itoa(i),
			"server":            "all",
			"profile":           "default",
			"limit-uptime":      "1h",
			"limit-bytes-total": 1048576,
			"comment":           "batch",
		}
	}

	out := h.post(t, "/v1/bulk/user-add", map[string]any{
		"session":     sess,
		"users":       users,
		"concurrency": 16,
	}, nil)

	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("bulk add failed: %v", out["error"])
	}
	if added, _ := out["added"].(float64); int(added) != len(users) {
		t.Fatalf("expected %d added, got %v (errors: %v)", len(users), out["added"], out["errors"])
	}
	if h.mock.UserCount() != len(users) {
		t.Fatalf("expected %d users on the router, got %d", len(users), h.mock.UserCount())
	}
}

func TestGenerateDryRunProducesVouchers(t *testing.T) {
	h := newHarness(t, api.Options{})

	out := h.post(t, "/v1/generate", map[string]any{
		"qty":       250,
		"mode":      "vc",
		"char":      "num",
		"userl":     4,
		"profile":   "default",
		"timelimit": "1h",
		"dry_run":   true,
	}, nil)

	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("generate failed: %v", out["error"])
	}
	vouchers, _ := out["vouchers"].([]any)
	if len(vouchers) != 250 {
		t.Fatalf("expected 250 vouchers, got %d", len(vouchers))
	}
	if h.mock.UserCount() != 0 {
		t.Fatalf("dry run must not touch the router, found %d users", h.mock.UserCount())
	}
}

func TestGeneratePushesToRouter(t *testing.T) {
	h := newHarness(t, api.Options{})
	sess := h.connect(t)

	out := h.post(t, "/v1/generate", map[string]any{
		"session":     sess,
		"qty":         150,
		"mode":        "vc",
		"char":        "mix",
		"userl":       5,
		"server":      "all",
		"profile":     "default",
		"timelimit":   "1h",
		"concurrency": 16,
	}, nil)

	if added, _ := out["added"].(float64); int(added) != 150 {
		t.Fatalf("expected 150 added, got %v (errors: %v)", out["added"], out["errors"])
	}
	if h.mock.UserCount() != 150 {
		t.Fatalf("expected 150 users on the router, got %d", h.mock.UserCount())
	}
}

func TestTokenAuth(t *testing.T) {
	h := newHarness(t, api.Options{Token: "s3cret"})

	out := h.post(t, "/v1/connect", map[string]any{"host": "127.0.0.1"}, nil)
	if out["error"] != "unauthorized" {
		t.Fatalf("expected unauthorized, got %v", out)
	}

	out = h.post(t, "/v1/connect", map[string]any{"host": ""}, map[string]string{"X-Mikhmon-Token": "s3cret"})
	if out["error"] == "unauthorized" {
		t.Fatal("valid token was rejected")
	}
}

func TestUnknownSessionIsReported(t *testing.T) {
	h := newHarness(t, api.Options{})

	out := h.post(t, "/v1/exec", map[string]any{
		"session":   "does-not-exist",
		"sentences": [][]string{{"/system/clock/print"}},
	}, nil)

	if ok, _ := out["ok"].(bool); ok {
		t.Fatal("expected ok=false for an unknown session")
	}
}

func TestBulkRemoveDeletesUsers(t *testing.T) {
	h := newHarness(t, api.Options{})
	sess := h.connect(t)

	const n = 300
	users := make([]map[string]any, n)
	for i := range users {
		users[i] = map[string]any{
			"name": "rm" + itoa(i), "password": "p", "server": "all", "profile": "default",
		}
	}
	h.post(t, "/v1/bulk/user-add", map[string]any{"session": sess, "users": users, "concurrency": 16}, nil)
	if h.mock.UserCount() != n {
		t.Fatalf("setup: expected %d users, got %d", n, h.mock.UserCount())
	}

	ids := make([]string, 0, n)
	for _, u := range h.mock.Users() {
		ids = append(ids, u.ID)
	}

	out := h.post(t, "/v1/bulk/remove", map[string]any{
		"session": sess, "ids": ids, "concurrency": 16,
	}, nil)

	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("bulk remove failed: %v", out["error"])
	}
	if removed, _ := out["removed"].(float64); int(removed) != n {
		t.Fatalf("expected %d removed, got %v (errors: %v)", n, out["removed"], out["errors"])
	}
	if h.mock.UserCount() != 0 {
		t.Fatalf("expected an empty router, got %d users", h.mock.UserCount())
	}
}

func TestBulkRemoveDefaultsToHotspotUserRemove(t *testing.T) {
	h := newHarness(t, api.Options{})
	sess := h.connect(t)

	h.post(t, "/v1/bulk/user-add", map[string]any{
		"session": sess, "users": []map[string]any{{"name": "solo", "password": "p"}},
	}, nil)
	ids := []string{h.mock.Users()[0].ID}

	// No "command" field: the endpoint has to fall back to the hotspot user path.
	out := h.post(t, "/v1/bulk/remove", map[string]any{"session": sess, "ids": ids}, nil)
	if removed, _ := out["removed"].(float64); int(removed) != 1 {
		t.Fatalf("expected 1 removed, got %v", out["removed"])
	}
	if h.mock.UserCount() != 0 {
		t.Fatal("user was not removed")
	}
}

func TestBulkRemoveRejectsUnsafeCommand(t *testing.T) {
	h := newHarness(t, api.Options{})
	sess := h.connect(t)

	for _, bad := range []string{"relative/path", "/ip/hotspot/user/remove =extra", "/reboot\nother"} {
		resp := h.rawPost(t, "/v1/bulk/remove", map[string]any{
			"session": sess, "command": bad, "ids": []string{"*1"},
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("command %q: expected 400, got %d", bad, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestBulkRemoveWithNoIDsIsANoOp(t *testing.T) {
	h := newHarness(t, api.Options{})
	sess := h.connect(t)

	out := h.post(t, "/v1/bulk/remove", map[string]any{"session": sess, "ids": []string{}}, nil)
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("expected ok=true for an empty id list, got %v", out)
	}
	if total, _ := out["total"].(float64); total != 0 {
		t.Fatalf("expected total 0, got %v", out["total"])
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [12]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
