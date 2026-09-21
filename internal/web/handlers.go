package web

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mikhmon/go-mikhmon/internal/config"
	"github.com/mikhmon/go-mikhmon/internal/generator"
)

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Auth
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/logout", s.handleLogout)

	// Dashboard
	mux.HandleFunc("/", s.requireAuth(s.handleDashboard))
	mux.HandleFunc("/dashboard", s.requireAuth(s.handleDashboard))

	// Sessions/Settings
	mux.HandleFunc("/sessions", s.requireAuth(s.handleSessions))
	mux.HandleFunc("/settings", s.requireAuth(s.handleSettings))

	// API endpoints (JSON)
	mux.HandleFunc("/api/connect", s.requireAuth(s.handleConnect))
	mux.HandleFunc("/api/dashboard", s.requireAuth(s.handleDashboardData))
	mux.HandleFunc("/api/hotspot/users", s.requireAuth(s.handleHotspotUsers))
	mux.HandleFunc("/api/hotspot/user", s.requireAuth(s.handleHotspotUser))
	mux.HandleFunc("/api/hotspot/user/add", s.requireAuth(s.handleAddUser))
	mux.HandleFunc("/api/hotspot/user/remove", s.requireAuth(s.handleRemoveUser))
	mux.HandleFunc("/api/hotspot/user/enable", s.requireAuth(s.handleEnableUser))
	mux.HandleFunc("/api/hotspot/user/disable", s.requireAuth(s.handleDisableUser))
	mux.HandleFunc("/api/hotspot/user/reset", s.requireAuth(s.handleResetUser))
	mux.HandleFunc("/api/hotspot/generate", s.requireAuth(s.handleGenerate))
	mux.HandleFunc("/api/hotspot/profiles", s.requireAuth(s.handleProfiles))
	mux.HandleFunc("/api/hotspot/profile", s.requireAuth(s.handleProfile))
	mux.HandleFunc("/api/hotspot/profile/add", s.requireAuth(s.handleAddProfile))
	mux.HandleFunc("/api/hotspot/profile/remove", s.requireAuth(s.handleRemoveProfile))
	mux.HandleFunc("/api/hotspot/active", s.requireAuth(s.handleActive))
	mux.HandleFunc("/api/hotspot/active/remove", s.requireAuth(s.handleRemoveActive))
	mux.HandleFunc("/api/hotspot/hosts", s.requireAuth(s.handleHosts))
	mux.HandleFunc("/api/hotspot/cookies", s.requireAuth(s.handleCookies))
	mux.HandleFunc("/api/hotspot/cookies/remove", s.requireAuth(s.handleRemoveCookie))
	mux.HandleFunc("/api/hotspot/ipbinding", s.requireAuth(s.handleIPBinding))
	mux.HandleFunc("/api/hotspot/log", s.requireAuth(s.handleHotspotLog))
	mux.HandleFunc("/api/hotspot/export", s.requireAuth(s.handleExport))
	mux.HandleFunc("/api/hotspot/servers", s.requireAuth(s.handleServers))
	mux.HandleFunc("/api/traffic", s.requireAuth(s.handleTraffic))
	mux.HandleFunc("/api/system/scheduler", s.requireAuth(s.handleScheduler))
	mux.HandleFunc("/api/system/reboot", s.requireAuth(s.handleReboot))
	mux.HandleFunc("/api/system/shutdown", s.requireAuth(s.handleShutdown))
	mux.HandleFunc("/api/report/selling", s.requireAuth(s.handleSellingReport))
	mux.HandleFunc("/api/report/live", s.requireAuth(s.handleLiveReport))
	mux.HandleFunc("/api/dhcp/leases", s.requireAuth(s.handleDHCPLeases))
	mux.HandleFunc("/api/config/session", s.requireAuth(s.handleConfigSession))
	mux.HandleFunc("/api/config/session/delete", s.requireAuth(s.handleDeleteSession))
	mux.HandleFunc("/api/ping", s.requireAuth(s.handlePing))
	mux.HandleFunc("/api/hotspot/user/remove-expired", s.requireAuth(s.handleRemoveExpired))
	mux.HandleFunc("/api/hotspot/user/remove-by-comment", s.requireAuth(s.handleRemoveByComment))

	// Voucher print (HTML)
	mux.HandleFunc("/voucher/print", s.requireAuth(s.handleVoucherPrint))
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		user := r.FormValue("user")
		pass := r.FormValue("pass")
		if user == s.cfg.Admin.Username && pass == s.cfg.Admin.Password {
			s.createSession(w, user)
			http.Redirect(w, r, "/sessions", http.StatusFound)
			return
		}
		renderLogin(w, "Invalid username or password")
		return
	}
	renderLogin(w, "")
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.destroySession(w, r)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	if session == "" {
		http.Redirect(w, r, "/sessions", http.StatusFound)
		return
	}
	renderDashboard(w, session, s.cfg)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	renderSessions(w, s.cfg)
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	renderSettings(w, session, s.cfg)
}

// === API HANDLERS ===

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, "Connection failed: "+err.Error())
		return
	}
	reply, err := c.Run("/system/identity/print")
	pool.Put(c)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	identity := ""
	if len(reply.Re) > 0 {
		identity = reply.Re[0]["name"]
	}
	s.jsonResponse(w, 200, map[string]string{"status": "connected", "identity": identity})
}

func (s *Server) handleDashboardData(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	type DashData struct {
		Identity    string            `json:"identity"`
		Resource    map[string]string `json:"resource"`
		Routerboard map[string]string `json:"routerboard"`
		Clock       map[string]string `json:"clock"`
		UserCount   string            `json:"user_count"`
		ActiveCount string            `json:"active_count"`
	}

	var data DashData

	if r1, err := c.Run("/system/identity/print"); err == nil && len(r1.Re) > 0 {
		data.Identity = r1.Re[0]["name"]
	}
	if r2, err := c.Run("/system/resource/print"); err == nil && len(r2.Re) > 0 {
		data.Resource = r2.Re[0]
	}
	if r3, err := c.Run("/system/routerboard/print"); err == nil && len(r3.Re) > 0 {
		data.Routerboard = r3.Re[0]
	}
	if r4, err := c.Run("/system/clock/print"); err == nil && len(r4.Re) > 0 {
		data.Clock = r4.Re[0]
	}
	if r5, err := c.RunArgs("/ip/hotspot/user/print", map[string]string{"count-only": ""}); err == nil && r5.Done != nil {
		if v, ok := r5.Done["ret"]; ok {
			data.UserCount = v
		}
	}
	if r6, err := c.RunArgs("/ip/hotspot/active/print", map[string]string{"count-only": ""}); err == nil && r6.Done != nil {
		if v, ok := r6.Done["ret"]; ok {
			data.ActiveCount = v
		}
	}

	s.jsonResponse(w, 200, data)
}

func (s *Server) handleHotspotUsers(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	profile := r.URL.Query().Get("profile")
	comment := r.URL.Query().Get("comment")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	args := map[string]string{}
	if profile != "" && profile != "all" {
		args["?profile"] = profile
	}
	if comment != "" {
		args["?comment"] = comment
	}

	reply, err := c.RunArgs("/ip/hotspot/user/print", args)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	s.jsonResponse(w, 200, reply.Re)
}

func (s *Server) handleHotspotUser(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	name := r.URL.Query().Get("name")
	id := r.URL.Query().Get("id")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	args := map[string]string{}
	if id != "" {
		args["?.id"] = id
	} else if name != "" {
		args["?name"] = name
	}

	reply, err := c.RunArgs("/ip/hotspot/user/print", args)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	if len(reply.Re) == 0 {
		s.jsonError(w, 404, "user not found")
		return
	}
	s.jsonResponse(w, 200, reply.Re[0])
}

func (s *Server) handleAddUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.jsonError(w, 405, "POST required")
		return
	}
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	args := map[string]string{
		"server":   r.FormValue("server"),
		"name":     r.FormValue("name"),
		"password": r.FormValue("password"),
		"profile":  r.FormValue("profile"),
	}
	if tl := r.FormValue("limit-uptime"); tl != "" && tl != "0" {
		args["limit-uptime"] = tl
	}
	if dl := r.FormValue("limit-bytes-total"); dl != "" && dl != "0" {
		args["limit-bytes-total"] = dl
	}
	if cm := r.FormValue("comment"); cm != "" {
		args["comment"] = cm
	}

	reply, err := c.RunArgs("/ip/hotspot/user/add", args)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	if len(reply.Trap) > 0 {
		s.jsonError(w, 400, reply.Trap[0]["message"])
		return
	}
	s.jsonResponse(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.jsonError(w, 405, "POST required")
		return
	}
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}

	qty, _ := strconv.Atoi(r.FormValue("qty"))
	if qty < 1 {
		qty = 1
	}
	length, _ := strconv.Atoi(r.FormValue("length"))
	if length < 3 {
		length = 4
	}
	dataLimit, _ := strconv.ParseInt(r.FormValue("data_limit"), 10, 64)

	// Determine worker count based on qty
	workers := 10
	if qty > 100 {
		workers = 20
	}
	if qty > 500 {
		workers = 30
	}
	if qty > 1000 {
		workers = 40
	}

	req := generator.GenerateRequest{
		Qty:       qty,
		Server:    r.FormValue("server"),
		Mode:      generator.UserMode(r.FormValue("mode")),
		Length:    length,
		Prefix:    r.FormValue("prefix"),
		Charset:   generator.CharSet(r.FormValue("charset")),
		Profile:   r.FormValue("profile"),
		TimeLimit: r.FormValue("time_limit"),
		DataLimit: dataLimit,
		Comment:   r.FormValue("comment"),
	}

	result, err := generator.Generate(pool, req, workers)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}

	s.jsonResponse(w, 200, result)
}

func (s *Server) handleRemoveUser(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	id := r.URL.Query().Get("id")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	ids := strings.Split(id, ",")
	for _, uid := range ids {
		uid = strings.TrimSpace(uid)
		if uid == "" {
			continue
		}
		c.RunArgs("/ip/hotspot/user/remove", map[string]string{".id": uid})
	}
	s.jsonResponse(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleEnableUser(w http.ResponseWriter, r *http.Request) {
	s.setUserDisabled(w, r, "false")
}

func (s *Server) handleDisableUser(w http.ResponseWriter, r *http.Request) {
	s.setUserDisabled(w, r, "true")
}

func (s *Server) setUserDisabled(w http.ResponseWriter, r *http.Request, disabled string) {
	session := r.URL.Query().Get("session")
	id := r.URL.Query().Get("id")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	_, err = c.RunArgs("/ip/hotspot/user/set", map[string]string{".id": id, "disabled": disabled})
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	s.jsonResponse(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleResetUser(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	id := r.URL.Query().Get("id")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	c.RunArgs("/ip/hotspot/user/set", map[string]string{
		".id":               id,
		"limit-uptime":      "0",
		"limit-bytes-total": "0",
		"comment":           "",
	})
	s.jsonResponse(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	reply, err := c.Run("/ip/hotspot/user/profile/print")
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	s.jsonResponse(w, 200, reply.Re)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	name := r.URL.Query().Get("name")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	reply, err := c.RunArgs("/ip/hotspot/user/profile/print", map[string]string{"?name": name})
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	if len(reply.Re) == 0 {
		s.jsonError(w, 404, "profile not found")
		return
	}
	s.jsonResponse(w, 200, reply.Re[0])
}

func (s *Server) handleAddProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.jsonError(w, 405, "POST required")
		return
	}
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	args := map[string]string{
		"name": r.FormValue("name"),
	}
	if v := r.FormValue("shared-users"); v != "" {
		args["shared-users"] = v
	}
	if v := r.FormValue("rate-limit"); v != "" {
		args["rate-limit"] = v
	}
	if v := r.FormValue("on-login"); v != "" {
		args["on-login"] = v
	}
	if v := r.FormValue("parent-queue"); v != "" {
		args["parent-queue"] = v
	}

	reply, err := c.RunArgs("/ip/hotspot/user/profile/add", args)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	if len(reply.Trap) > 0 {
		s.jsonError(w, 400, reply.Trap[0]["message"])
		return
	}
	s.jsonResponse(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleRemoveProfile(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	id := r.URL.Query().Get("id")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)
	c.RunArgs("/ip/hotspot/user/profile/remove", map[string]string{".id": id})
	s.jsonResponse(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleActive(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	server := r.URL.Query().Get("server")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	args := map[string]string{}
	if server != "" {
		args["?server"] = server
	}
	reply, err := c.RunArgs("/ip/hotspot/active/print", args)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	s.jsonResponse(w, 200, reply.Re)
}

func (s *Server) handleRemoveActive(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	id := r.URL.Query().Get("id")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)
	c.RunArgs("/ip/hotspot/active/remove", map[string]string{".id": id})
	// also remove cookie
	if reply, err := c.RunArgs("/ip/hotspot/cookie/print", map[string]string{}); err == nil {
		for _, cookie := range reply.Re {
			if cookie["user"] == r.URL.Query().Get("user") {
				c.RunArgs("/ip/hotspot/cookie/remove", map[string]string{".id": cookie[".id"]})
			}
		}
	}
	s.jsonResponse(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleHosts(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)
	reply, err := c.Run("/ip/hotspot/host/print")
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	s.jsonResponse(w, 200, reply.Re)
}

func (s *Server) handleCookies(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)
	reply, err := c.Run("/ip/hotspot/cookie/print")
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	s.jsonResponse(w, 200, reply.Re)
}

func (s *Server) handleRemoveCookie(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	id := r.URL.Query().Get("id")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)
	c.RunArgs("/ip/hotspot/cookie/remove", map[string]string{".id": id})
	s.jsonResponse(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleIPBinding(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)
	reply, err := c.Run("/ip/hotspot/ip-binding/print")
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	s.jsonResponse(w, 200, reply.Re)
}

func (s *Server) handleHotspotLog(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)
	reply, err := c.RunArgs("/log/print", map[string]string{"?topics": "hotspot,info,debug"})
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	// reverse order (newest first)
	for i, j := 0, len(reply.Re)-1; i < j; i, j = i+1, j-1 {
		reply.Re[i], reply.Re[j] = reply.Re[j], reply.Re[i]
	}
	s.jsonResponse(w, 200, reply.Re)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	profile := r.URL.Query().Get("profile")
	format := r.URL.Query().Get("format") // csv or rsc
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	args := map[string]string{}
	if profile != "" && profile != "all" {
		args["?profile"] = profile
	}
	reply, err := c.RunArgs("/ip/hotspot/user/print", args)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=hotspot-users.csv")
		fmt.Fprintln(w, "name,password,profile,uptime,bytes-in,bytes-out,comment")
		for _, u := range reply.Re {
			fmt.Fprintf(w, "%s,%s,%s,%s,%s,%s,%s\n",
				u["name"], u["password"], u["profile"],
				u["uptime"], u["bytes-in"], u["bytes-out"], u["comment"])
		}
	} else {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", "attachment; filename=hotspot-users.rsc")
		for _, u := range reply.Re {
			fmt.Fprintf(w, "/ip hotspot user add name=%s password=%s profile=%s",
				u["name"], u["password"], u["profile"])
			if u["comment"] != "" {
				fmt.Fprintf(w, " comment=%s", u["comment"])
			}
			fmt.Fprintln(w)
		}
	}
}

func (s *Server) handleServers(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)
	reply, err := c.Run("/ip/hotspot/print")
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	s.jsonResponse(w, 200, reply.Re)
}

func (s *Server) handleTraffic(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	iface := r.URL.Query().Get("iface")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	reply, err := c.RunArgs("/interface/monitor-traffic", map[string]string{
		"interface": iface,
		"once":      "",
	})
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	if len(reply.Re) > 0 {
		s.jsonResponse(w, 200, []map[string]string{
			{"data": reply.Re[0]["tx-bits-per-second"]},
			{"data": reply.Re[0]["rx-bits-per-second"]},
		})
	} else {
		s.jsonResponse(w, 200, []map[string]string{{"data": "0"}, {"data": "0"}})
	}
}

func (s *Server) handleScheduler(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)
	reply, err := c.Run("/system/scheduler/print")
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	s.jsonResponse(w, 200, reply.Re)
}

func (s *Server) handleReboot(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.jsonError(w, 405, "POST required")
		return
	}
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c.Run("/system/reboot")
	// don't return to pool — connection will die
	c.Close()
	s.jsonResponse(w, 200, map[string]string{"status": "rebooting"})
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.jsonError(w, 405, "POST required")
		return
	}
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c.Run("/system/shutdown")
	c.Close()
	s.jsonResponse(w, 200, map[string]string{"status": "shutting_down"})
}

func (s *Server) handleSellingReport(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	reply, err := c.Run("/system/script/print")
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	s.jsonResponse(w, 200, reply.Re)
}

func (s *Server) handleLiveReport(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	now := time.Now()
	idhr := strings.ToLower(now.Format("Jan")) + "/" + now.Format("02") + "/" + now.Format("2006")
	idbl := strings.ToLower(now.Format("Jan")) + now.Format("2006")

	dayScripts, _ := c.RunArgs("/system/script/print", map[string]string{"?source": idhr})
	monthScripts, _ := c.RunArgs("/system/script/print", map[string]string{"?owner": idbl})

	dayTotal := 0
	dayIncome := 0
	if dayScripts != nil {
		dayTotal = len(dayScripts.Re)
		for _, s := range dayScripts.Re {
			parts := strings.Split(s["name"], "-|-")
			if len(parts) > 3 {
				if v, err := strconv.Atoi(parts[3]); err == nil {
					dayIncome += v
				}
			}
		}
	}

	monthTotal := 0
	monthIncome := 0
	if monthScripts != nil {
		monthTotal = len(monthScripts.Re)
		for _, s := range monthScripts.Re {
			parts := strings.Split(s["name"], "-|-")
			if len(parts) > 3 {
				if v, err := strconv.Atoi(parts[3]); err == nil {
					monthIncome += v
				}
			}
		}
	}

	s.jsonResponse(w, 200, map[string]interface{}{
		"day_total":    dayTotal,
		"day_income":   dayIncome,
		"month_total":  monthTotal,
		"month_income": monthIncome,
	})
}

func (s *Server) handleDHCPLeases(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)
	reply, err := c.Run("/ip/dhcp-server/lease/print")
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	s.jsonResponse(w, 200, reply.Re)
}

func (s *Server) handleConfigSession(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		name := r.FormValue("name")
		if name == "" {
			s.jsonError(w, 400, "name required")
			return
		}
		sess := &config.RouterSession{
			Name:       name,
			IP:         r.FormValue("ip"),
			User:       r.FormValue("user"),
			Password:   r.FormValue("password"),
			Hotspot:    r.FormValue("hotspot"),
			DNS:        r.FormValue("dns"),
			Currency:   r.FormValue("currency"),
			Interface:  r.FormValue("interface"),
			LiveReport: r.FormValue("live_report"),
		}
		if v, err := strconv.Atoi(r.FormValue("reload")); err == nil {
			sess.Reload = v
		}
		if v, err := strconv.Atoi(r.FormValue("idle_timeout")); err == nil {
			sess.IdleTimeout = v
		}
		s.cfg.SetSession(name, sess)
		if err := s.cfg.Save(); err != nil {
			s.jsonError(w, 500, err.Error())
			return
		}
		// Reset pool for this session
		s.ClosePool(name)
		s.jsonResponse(w, 200, map[string]string{"status": "ok"})
		return
	}
	// GET — return all sessions
	s.jsonResponse(w, 200, s.cfg.Sessions)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	s.cfg.DeleteSession(name)
	s.cfg.Save()
	s.ClosePool(name)
	s.jsonResponse(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	start := time.Now()
	c, err := pool.Get()
	if err != nil {
		s.jsonResponse(w, 200, map[string]interface{}{
			"status":  "timeout",
			"elapsed": time.Since(start).String(),
		})
		return
	}
	pool.Put(c)
	s.jsonResponse(w, 200, map[string]interface{}{
		"status":  "ok",
		"elapsed": time.Since(start).String(),
	})
}

func (s *Server) handleRemoveExpired(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	reply, err := c.RunArgs("/ip/hotspot/user/print", map[string]string{"?limit-uptime": "1s"})
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	count := 0
	for _, u := range reply.Re {
		c.RunArgs("/ip/hotspot/user/remove", map[string]string{".id": u[".id"]})
		count++
	}
	s.jsonResponse(w, 200, map[string]int{"removed": count})
}

func (s *Server) handleRemoveByComment(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	comment := r.URL.Query().Get("comment")
	pool, err := s.GetPool(session)
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	c, err := pool.Get()
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	defer pool.Put(c)

	reply, err := c.RunArgs("/ip/hotspot/user/print", map[string]string{"?comment": comment})
	if err != nil {
		s.jsonError(w, 500, err.Error())
		return
	}
	count := 0
	for _, u := range reply.Re {
		if u["uptime"] == "0s" || u["uptime"] == "" || u["uptime"] == "00:00:00" {
			c.RunArgs("/ip/hotspot/user/remove", map[string]string{".id": u[".id"]})
			count++
		}
	}
	s.jsonResponse(w, 200, map[string]int{"removed": count})
}

func (s *Server) handleVoucherPrint(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	comment := r.URL.Query().Get("comment")
	qr := r.URL.Query().Get("qr")
	pool, err := s.GetPool(session)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	c, err := pool.Get()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer pool.Put(c)

	args := map[string]string{}
	if comment != "" {
		args["?comment"] = comment
	}
	reply, err := c.RunArgs("/ip/hotspot/user/print", args)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	rs := s.cfg.GetSession(session)
	currency := ""
	dns := ""
	hotspot := ""
	if rs != nil {
		currency = rs.Currency
		dns = rs.DNS
		hotspot = rs.Hotspot
	}

	renderVoucherPrint(w, reply.Re, qr == "yes", currency, dns, hotspot)
}

// Dummy log suppression
func init() {
	_ = log.Println
	_ = fmt.Sprintf
}
