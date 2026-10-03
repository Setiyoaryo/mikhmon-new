package api

import (
	"net/http"
	"strconv"
	"strings"
)

// handleTraffic returns a single sample. Absent data must remain an error;
// converting it to zero would hide a failed RouterOS command on the chart.
func (s *Server) handleTraffic(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Session   string `json:"session"`
		Interface string `json:"interface"`
		TimeoutMS int64  `json:"timeout_ms"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Interface) == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "missing interface"})
		return
	}
	replies, err := s.mgr.Exec(req.Session, s.execTimeout(req.TimeoutMS), []string{
		"/interface/monitor-traffic", "=interface=" + req.Interface, "=once=",
	})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	for _, reply := range replies {
		if reply.Type() != "!re" {
			continue
		}
		tx, txErr := strconv.ParseInt(reply.Get("tx-bits-per-second"), 10, 64)
		rx, rxErr := strconv.ParseInt(reply.Get("rx-bits-per-second"), 10, 64)
		if txErr != nil || rxErr != nil || tx < 0 || rx < 0 {
			continue
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "tx": tx, "rx": rx})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no valid traffic sample for interface " + req.Interface})
}
