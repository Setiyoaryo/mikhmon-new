package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/mikhmon/go-mikhmon/internal/config"
)

func renderLogin(w http.ResponseWriter, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Mikhmon Go - Login</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#1a1a2e;display:flex;justify-content:center;align-items:center;min-height:100vh;color:#e0e0e0}
.login-box{background:#16213e;padding:2rem;border-radius:12px;box-shadow:0 8px 32px rgba(0,0,0,.3);width:360px}
h1{text-align:center;margin-bottom:1.5rem;color:#0f3460;font-size:1.5rem}
h1 span{color:#e94560}
input{width:100%%;padding:.75rem;margin:.5rem 0;border:1px solid #333;border-radius:8px;background:#0f3460;color:#fff;font-size:1rem}
input:focus{outline:none;border-color:#e94560}
button{width:100%%;padding:.75rem;margin-top:1rem;border:none;border-radius:8px;background:#e94560;color:#fff;font-size:1rem;cursor:pointer;font-weight:600}
button:hover{background:#c73e54}
.error{background:#e94560;color:#fff;padding:.5rem;border-radius:6px;text-align:center;margin-bottom:1rem;font-size:.9rem}
.ver{text-align:center;margin-top:1rem;font-size:.8rem;color:#666}
</style>
</head><body>
<div class="login-box">
<h1>🚀 <span>Mikhmon</span> Go</h1>
%s
<form method="POST" action="/login">
<input type="text" name="user" placeholder="Username" required autofocus>
<input type="password" name="pass" placeholder="Password" required>
<button type="submit">Login</button>
</form>
<div class="ver">Mikhmon Go v1.0 — Rewrite for Speed</div>
</div>
</body></html>`, func() string {
		if errMsg != "" {
			return `<div class="error">` + template.HTMLEscapeString(errMsg) + `</div>`
		}
		return ""
	}())
}

func renderSessions(w http.ResponseWriter, cfg *config.Config) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var rows strings.Builder
	for name, s := range cfg.Sessions {
		rows.WriteString(fmt.Sprintf(`<tr>
<td><a href="/dashboard?session=%s" class="btn">%s</a></td>
<td>%s</td><td>%s</td>
<td>
<a href="/settings?session=%s" class="btn btn-sm">⚙️</a>
<a href="/api/connect?session=%s" class="btn btn-sm btn-green">🔌</a>
<a href="/api/config/session/delete?name=%s" class="btn btn-sm btn-red" onclick="return confirm('Delete session %s?')">🗑️</a>
</td>
</tr>`, name, name, s.IP, s.User, name, name, name, name))
	}

	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Mikhmon Go - Sessions</title>
<style>%s</style>
</head><body>
<div class="container">
<nav class="nav">
<a href="/sessions" class="brand">🚀 Mikhmon Go</a>
<a href="/logout" class="btn btn-red">Logout</a>
</nav>
<div class="card">
<h2>Router Sessions</h2>
<table><thead><tr><th>Name</th><th>IP</th><th>User</th><th>Actions</th></tr></thead>
<tbody>%s</tbody></table>
<h3 style="margin-top:1.5rem">Add New Session</h3>
<form method="POST" action="/api/config/session" id="addForm">
<div class="grid">
<div><label>Name</label><input name="name" required></div>
<div><label>IP:Port</label><input name="ip" placeholder="192.168.1.1:8728" required></div>
<div><label>User</label><input name="user" required></div>
<div><label>Password</label><input name="password" type="password" required></div>
<div><label>Currency</label><input name="currency" value="Rp" ></div>
<div><label>Interface #</label><input name="interface" value="1"></div>
</div>
<button type="submit" class="btn btn-green" style="margin-top:1rem">Add Session</button>
</form>
</div>
</div>
<script>
document.getElementById('addForm').addEventListener('submit',function(e){
e.preventDefault();
fetch('/api/config/session',{method:'POST',body:new FormData(this)})
.then(r=>r.json()).then(()=>location.reload());
});
</script>
</body></html>`, baseCSS, rows.String())
}

func renderDashboard(w http.ResponseWriter, session string, cfg *config.Config) {
	rs := cfg.GetSession(session)
	reload := 10
	iface := "ether1"
	currency := "Rp"
	if rs != nil {
		if rs.Reload > 0 { reload = rs.Reload }
		if rs.Interface != "" { iface = rs.Interface }
		if rs.Currency != "" { currency = rs.Currency }
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Mikhmon Go - Dashboard</title>
<style>%s</style>
<script src="https://cdn.jsdelivr.net/npm/highcharts@11/highcharts.js"></script>
<script src="https://cdn.jsdelivr.net/npm/qrious@4/dist/qrious.min.js"></script>
</head><body>
<div class="container">
<nav class="nav">
<a href="/dashboard?session=%s" class="brand">🚀 Mikhmon Go</a>
<div class="nav-links">
<a href="/dashboard?session=%s">Dashboard</a>
<div class="dropdown">
<a href="#">Hotspot ▾</a>
<div class="dropdown-content">
<a href="#" onclick="loadPage('users','all')">Users</a>
<a href="#" onclick="loadPage('profiles')">Profiles</a>
<a href="#" onclick="loadPage('active')">Active</a>
<a href="#" onclick="loadPage('hosts')">Hosts</a>
<a href="#" onclick="loadPage('cookies')">Cookies</a>
<a href="#" onclick="loadPage('ipbinding')">IP Binding</a>
<a href="#" onclick="loadPage('log')">Log</a>
<a href="#" onclick="loadPage('dhcp')">DHCP Leases</a>
</div>
</div>
<div class="dropdown">
<a href="#">Users ▾</a>
<div class="dropdown-content">
<a href="#" onclick="loadPage('generate')">Generate</a>
<a href="#" onclick="loadPage('adduser')">Add User</a>
<a href="#" onclick="loadPage('export')">Export</a>
</div>
</div>
<div class="dropdown">
<a href="#">Report ▾</a>
<div class="dropdown-content">
<a href="#" onclick="loadPage('selling')">Selling</a>
</div>
</div>
<div class="dropdown">
<a href="#">System ▾</a>
<div class="dropdown-content">
<a href="#" onclick="loadPage('scheduler')">Scheduler</a>
<a href="#" onclick="loadPage('traffic')">Traffic Monitor</a>
</div>
</div>
<a href="/settings?session=%s">Settings</a>
<a href="/sessions">Sessions</a>
<a href="/logout" class="btn btn-red btn-sm">Logout</a>
</div>
</nav>
<div id="content">
<div id="dash-info" class="row"></div>
<div class="row">
<div class="col-8"><div id="dash-hotspot" class="card"></div><div class="card"><div id="trafficChart" style="height:300px"></div></div></div>
<div class="col-4"><div id="dash-report" class="card"></div><div id="dash-log" class="card" style="max-height:400px;overflow:auto"></div></div>
</div>
</div>
</div>

<script>
const SESSION='%s', RELOAD=%d, IFACE='%s', CURRENCY='%s';

function api(path,opts){return fetch('/api/'+path+'?session='+SESSION+(opts?.params||''),opts).then(r=>r.json())}

function loadDashboard(){
api('dashboard').then(d=>{
let r=d.resource||{},rb=d.routerboard||{},ck=d.clock||{};
document.getElementById('dash-info').innerHTML=
'<div class="col-4"><div class="box box-blue"><b>'+ck.date+' '+ck.time+'</b><br>Uptime: '+(r.uptime||'-')+'</div></div>'+
'<div class="col-4"><div class="box box-green"><b>'+rb.model+'</b><br>'+(r['board-name']||'')+' | ROS '+(r.version||'')+'</div></div>'+
'<div class="col-4"><div class="box box-purple"><b>CPU: '+(r['cpu-load']||0)+'%%</b><br>Mem: '+fmtB(r['free-memory'])+' | HDD: '+fmtB(r['free-hdd-space'])+'</div></div>';
document.getElementById('dash-hotspot').innerHTML=
'<h3>🌐 Hotspot</h3><div class="row">'+
'<div class="col-3"><div class="box box-blue clickable" onclick="loadPage(\'active\')"><h2>'+(d.active_count||0)+'</h2>Active</div></div>'+
'<div class="col-3"><div class="box box-green clickable" onclick="loadPage(\'users\',\'all\')"><h2>'+(d.user_count||0)+'</h2>Users</div></div>'+
'<div class="col-3"><div class="box box-yellow clickable" onclick="loadPage(\'adduser\')"><h2>➕</h2>Add User</div></div>'+
'<div class="col-3"><div class="box box-red clickable" onclick="loadPage(\'generate\')"><h2>🎫</h2>Generate</div></div></div>';
});
api('report/live').then(d=>{
document.getElementById('dash-report').innerHTML=
'<h3>💰 Income</h3><p>Today: '+d.day_total+' vcr — '+CURRENCY+' '+d.day_income.toLocaleString()+'</p>'+
'<p>This Month: '+d.month_total+' vcr — '+CURRENCY+' '+d.month_income.toLocaleString()+'</p>';
});
api('hotspot/log').then(logs=>{
let h='<h3>📋 Hotspot Log</h3><table><thead><tr><th>Time</th><th>Message</th></tr></thead><tbody>';
(logs||[]).slice(0,50).forEach(l=>{h+='<tr><td>'+esc(l.time||'')+'</td><td>'+esc(l.message||'')+'</td></tr>'});
h+='</tbody></table>';
document.getElementById('dash-log').innerHTML=h;
});
}

let chart;
function initTrafficChart(){
chart=Highcharts.chart('trafficChart',{chart:{type:'areaspline',animation:true},title:{text:'Traffic: '+IFACE},
xAxis:{type:'datetime',tickPixelInterval:150},
yAxis:{title:{text:null},labels:{formatter:function(){return fmtBps(this.value)}}},
tooltip:{shared:true,formatter:function(){let s='<b>Traffic</b><br>';
this.points.forEach(p=>{s+=p.series.name+': '+fmtBps(p.y)+'<br>'});return s}},
series:[{name:'Tx',data:[]},{name:'Rx',data:[]}]});
setInterval(()=>{
api('traffic','&iface='+IFACE).then(d=>{if(!d||!d.length)return;
let x=(new Date).getTime(),sh=chart.series[0].data.length>30;
chart.series[0].addPoint([x,parseInt(d[0].data)||0],true,sh);
chart.series[1].addPoint([x,parseInt(d[1].data)||0],true,sh);
}).catch(()=>{})},3000);
}

function fmtB(b){b=parseInt(b)||0;if(b<1024)return b+' B';if(b<1048576)return(b/1024).toFixed(1)+' KB';
if(b<1073741824)return(b/1048576).toFixed(1)+' MB';return(b/1073741824).toFixed(2)+' GB'}
function fmtBps(b){if(!b)return'0 bps';const u=['bps','Kbps','Mbps','Gbps'];
let i=Math.floor(Math.log(b)/Math.log(1024));return(b/Math.pow(1024,i)).toFixed(1)+' '+u[i]}
function esc(s){let d=document.createElement('div');d.textContent=s;return d.innerHTML}

// === PAGE LOADER ===
function loadPage(page,extra){
const c=document.getElementById('content');
c.innerHTML='<div class="loading">Loading...</div>';
switch(page){
case 'users': loadUsers(extra);break;
case 'profiles': loadProfiles();break;
case 'active': loadActive();break;
case 'hosts': loadHosts();break;
case 'cookies': loadCookies();break;
case 'ipbinding': loadIPBinding();break;
case 'log': loadLog();break;
case 'dhcp': loadDHCP();break;
case 'generate': loadGenerate();break;
case 'adduser': loadAddUser();break;
case 'export': loadExport();break;
case 'selling': loadSelling();break;
case 'scheduler': loadScheduler();break;
case 'traffic': loadTrafficPage();break;
default: loadDashboard();initTrafficChart();
}
}

function loadUsers(profile){
api('hotspot/users&profile='+(profile||'all')).then(users=>{
let h='<div class="card"><h2>Hotspot Users</h2><div class="toolbar">'+
'<input id="filterTable" placeholder="🔍 Filter..." oninput="filterRows()">'+
'<button class="btn btn-green btn-sm" onclick="loadPage(\'generate\')">Generate</button>'+
'<button class="btn btn-sm" onclick="loadPage(\'adduser\')">Add User</button>'+
'<button class="btn btn-red btn-sm" onclick="removeSelected()">Delete Selected</button>'+
'<button class="btn btn-sm" onclick="removeExpired()">Remove Expired</button></div>'+
'<table id="dataTable"><thead><tr><th><input type="checkbox" onchange="toggleAll(this)"></th>'+
'<th>Name</th><th>Password</th><th>Profile</th><th>Uptime</th><th>Bytes In/Out</th><th>Comment</th><th>Actions</th></tr></thead><tbody>';
(users||[]).forEach(u=>{
h+='<tr><td><input type="checkbox" value="'+esc(u['.id']||'')+'"></td>'+
'<td><a href="#" onclick="loadUserDetail(\''+esc(u['.id']||'')+'\')">'+esc(u.name||'')+'</a></td>'+
'<td>'+esc(u.password||'')+'</td><td>'+esc(u.profile||'')+'</td>'+
'<td>'+esc(u.uptime||'0')+'</td><td>'+esc(u['bytes-in']||'0')+'/'+esc(u['bytes-out']||'0')+'</td>'+
'<td>'+esc(u.comment||'')+'</td>'+
'<td><button class="btn btn-red btn-xs" onclick="removeUser(\''+esc(u['.id']||'')+'\')">🗑️</button></td></tr>'});
h+='</tbody></table></div>';
document.getElementById('content').innerHTML=h;
});
}

function loadGenerate(){
api('hotspot/profiles').then(profiles=>{
api('hotspot/servers').then(servers=>{
let profOpts=profiles.map(p=>'<option>'+esc(p.name)+'</option>').join('');
let srvOpts='<option>all</option>'+servers.map(s=>'<option>'+esc(s.name)+'</option>').join('');
document.getElementById('content').innerHTML=
'<div class="card"><h2>🎫 Generate Users</h2>'+
'<form id="genForm"><div class="grid">'+
'<div><label>Quantity</label><input name="qty" type="number" min="1" max="99999" value="10" required></div>'+
'<div><label>Server</label><select name="server">'+srvOpts+'</select></div>'+
'<div><label>Mode</label><select name="mode"><option value="vc">Voucher (user=pass)</option><option value="up">User + Password</option></select></div>'+
'<div><label>Length</label><select name="length"><option>4</option><option>5</option><option>6</option><option>7</option><option>8</option></select></div>'+
'<div><label>Prefix</label><input name="prefix" maxlength="6"></div>'+
'<div><label>Character</label><select name="charset">'+
'<option value="lower">abcd</option><option value="upper">ABCD</option><option value="upplow">aBcD</option>'+
'<option value="mix">5ab2c (num+lower)</option><option value="mix1">5AB2C (num+upper)</option><option value="mix2">5aB2c (num+mixed)</option>'+
'<option value="num">1234 (numbers)</option></select></div>'+
'<div><label>Profile</label><select name="profile" required>'+profOpts+'</select></div>'+
'<div><label>Time Limit</label><input name="time_limit" placeholder="e.g. 1h30m"></div>'+
'<div><label>Data Limit (bytes)</label><input name="data_limit" type="number" min="0" value="0"></div>'+
'<div><label>Comment</label><input name="comment"></div>'+
'</div><button type="submit" class="btn btn-green" style="margin-top:1rem">⚡ Generate</button>'+
'<span id="genStatus" style="margin-left:1rem"></span></form>'+
'<div id="genResult" style="margin-top:1rem"></div></div>';
document.getElementById('genForm').addEventListener('submit',function(e){
e.preventDefault();
document.getElementById('genStatus').textContent='Generating...';
let t=performance.now();
fetch('/api/hotspot/generate?session='+SESSION,{method:'POST',body:new FormData(this)})
.then(r=>r.json()).then(d=>{
let elapsed=((performance.now()-t)/1000).toFixed(2);
if(d.error){document.getElementById('genStatus').textContent='Error: '+d.error;return}
let ok=d.users.filter(u=>!u.error).length,fail=d.users.filter(u=>u.error).length;
document.getElementById('genStatus').innerHTML='<b style="color:#4ecdc4">✅ Done! '+ok+' created, '+fail+' failed — '+elapsed+'s (server: '+d.elapsed+')</b>';
let h='<table><thead><tr><th>#</th><th>Username</th><th>Password</th><th>Status</th></tr></thead><tbody>';
d.users.forEach((u,i)=>{h+='<tr><td>'+(i+1)+'</td><td>'+esc(u.username)+'</td><td>'+esc(u.password)+'</td><td>'+(u.error?'❌ '+esc(u.error):'✅')+'</td></tr>'});
h+='</tbody></table>';
h+='<button class="btn btn-sm" style="margin-top:.5rem" onclick="printVouchers(\''+esc(d.comment)+'\')">🖨️ Print Vouchers</button>';
document.getElementById('genResult').innerHTML=h;
});
});
});
});
}

function loadProfiles(){api('hotspot/profiles').then(p=>{
let h='<div class="card"><h2>User Profiles</h2><table><thead><tr><th>Name</th><th>Shared</th><th>Rate Limit</th><th>Actions</th></tr></thead><tbody>';
(p||[]).forEach(pr=>{h+='<tr><td>'+esc(pr.name||'')+'</td><td>'+esc(pr['shared-users']||'')+'</td><td>'+esc(pr['rate-limit']||'')+'</td>'+
'<td><button class="btn btn-red btn-xs" onclick="removeProfile(\''+esc(pr['.id']||'')+'\')">🗑️</button></td></tr>'});
h+='</tbody></table></div>';document.getElementById('content').innerHTML=h})}

function loadActive(){api('hotspot/active').then(a=>{
let h='<div class="card"><h2>Active Sessions</h2><table><thead><tr><th>User</th><th>Address</th><th>MAC</th><th>Uptime</th><th>Server</th><th>Actions</th></tr></thead><tbody>';
(a||[]).forEach(s=>{h+='<tr><td>'+esc(s.user||'')+'</td><td>'+esc(s.address||'')+'</td><td>'+esc(s['mac-address']||'')+'</td>'+
'<td>'+esc(s.uptime||'')+'</td><td>'+esc(s.server||'')+'</td>'+
'<td><button class="btn btn-red btn-xs" onclick="removeActive(\''+esc(s['.id']||'')+'\',\''+esc(s.user||'')+'\')">🗑️</button></td></tr>'});
h+='</tbody></table></div>';document.getElementById('content').innerHTML=h})}

function loadHosts(){api('hotspot/hosts').then(h2=>{
let h='<div class="card"><h2>Hotspot Hosts</h2><table><thead><tr><th>MAC</th><th>Address</th><th>To Address</th><th>Server</th><th>Comment</th></tr></thead><tbody>';
(h2||[]).forEach(host=>{h+='<tr><td>'+esc(host['mac-address']||'')+'</td><td>'+esc(host.address||'')+'</td><td>'+esc(host['to-address']||'')+'</td>'+
'<td>'+esc(host.server||'')+'</td><td>'+esc(host.comment||'')+'</td></tr>'});
h+='</tbody></table></div>';document.getElementById('content').innerHTML=h})}

function loadCookies(){api('hotspot/cookies').then(c=>{
let h='<div class="card"><h2>Hotspot Cookies</h2><table><thead><tr><th>User</th><th>MAC</th><th>Domain</th><th>Expires</th><th>Actions</th></tr></thead><tbody>';
(c||[]).forEach(ck=>{h+='<tr><td>'+esc(ck.user||'')+'</td><td>'+esc(ck['mac-address']||'')+'</td><td>'+esc(ck.domain||'')+'</td>'+
'<td>'+esc(ck['expires-in']||'')+'</td><td><button class="btn btn-red btn-xs" onclick="removeCookie(\''+esc(ck['.id']||'')+'\')">🗑️</button></td></tr>'});
h+='</tbody></table></div>';document.getElementById('content').innerHTML=h})}

function loadIPBinding(){api('hotspot/ipbinding').then(b=>{
let h='<div class="card"><h2>IP Binding</h2><table><thead><tr><th>MAC</th><th>Address</th><th>To Address</th><th>Server</th><th>Type</th><th>Comment</th></tr></thead><tbody>';
(b||[]).forEach(ib=>{h+='<tr><td>'+esc(ib['mac-address']||'')+'</td><td>'+esc(ib.address||'')+'</td><td>'+esc(ib['to-address']||'')+'</td>'+
'<td>'+esc(ib.server||'')+'</td><td>'+esc(ib.type||'')+'</td><td>'+esc(ib.comment||'')+'</td></tr>'});
h+='</tbody></table></div>';document.getElementById('content').innerHTML=h})}

function loadLog(){api('hotspot/log').then(logs=>{
let h='<div class="card"><h2>Hotspot Log</h2><table><thead><tr><th>Time</th><th>Message</th></tr></thead><tbody>';
(logs||[]).slice(0,200).forEach(l=>{h+='<tr><td>'+esc(l.time||'')+'</td><td>'+esc(l.message||'')+'</td></tr>'});
h+='</tbody></table></div>';document.getElementById('content').innerHTML=h})}

function loadDHCP(){api('dhcp/leases').then(l=>{
let h='<div class="card"><h2>DHCP Leases</h2><table><thead><tr><th>Address</th><th>MAC</th><th>Hostname</th><th>Server</th><th>Status</th></tr></thead><tbody>';
(l||[]).forEach(le=>{h+='<tr><td>'+esc(le.address||'')+'</td><td>'+esc(le['mac-address']||'')+'</td><td>'+esc(le['host-name']||'')+'</td>'+
'<td>'+esc(le.server||'')+'</td><td>'+esc(le.status||'')+'</td></tr>'});
h+='</tbody></table></div>';document.getElementById('content').innerHTML=h})}

function loadScheduler(){api('system/scheduler').then(s=>{
let h='<div class="card"><h2>System Scheduler</h2><table><thead><tr><th>Name</th><th>Start Date</th><th>Start Time</th><th>Interval</th><th>Next Run</th><th>Run Count</th></tr></thead><tbody>';
(s||[]).forEach(sc=>{h+='<tr><td>'+esc(sc.name||'')+'</td><td>'+esc(sc['start-date']||'')+'</td><td>'+esc(sc['start-time']||'')+'</td>'+
'<td>'+esc(sc.interval||'')+'</td><td>'+esc(sc['next-run']||'')+'</td><td>'+esc(sc['run-count']||'')+'</td></tr>'});
h+='</tbody></table></div>';document.getElementById('content').innerHTML=h})}

function loadAddUser(){
api('hotspot/profiles').then(profiles=>{
api('hotspot/servers').then(servers=>{
let profOpts=profiles.map(p=>'<option>'+esc(p.name)+'</option>').join('');
let srvOpts='<option>all</option>'+servers.map(s=>'<option>'+esc(s.name)+'</option>').join('');
document.getElementById('content').innerHTML=
'<div class="card"><h2>Add User</h2>'+
'<form id="addUserForm"><div class="grid">'+
'<div><label>Server</label><select name="server">'+srvOpts+'</select></div>'+
'<div><label>Name</label><input name="name" required></div>'+
'<div><label>Password</label><input name="password" required></div>'+
'<div><label>Profile</label><select name="profile">'+profOpts+'</select></div>'+
'<div><label>Time Limit</label><input name="limit-uptime"></div>'+
'<div><label>Data Limit</label><input name="limit-bytes-total" type="number" min="0"></div>'+
'<div><label>Comment</label><input name="comment"></div>'+
'</div><button type="submit" class="btn btn-green" style="margin-top:1rem">Add</button>'+
'<span id="addStatus" style="margin-left:1rem"></span></form></div>';
document.getElementById('addUserForm').addEventListener('submit',function(e){
e.preventDefault();
fetch('/api/hotspot/user/add?session='+SESSION,{method:'POST',body:new FormData(this)})
.then(r=>r.json()).then(d=>{
document.getElementById('addStatus').innerHTML=d.error?'<span style="color:red">'+d.error+'</span>':'<span style="color:#4ecdc4">✅ Added</span>';
});
});
});
});
}

function loadExport(){
document.getElementById('content').innerHTML=
'<div class="card"><h2>Export Users</h2>'+
'<a href="/api/hotspot/export?session='+SESSION+'&format=csv" class="btn btn-green">📥 Export CSV</a> '+
'<a href="/api/hotspot/export?session='+SESSION+'&format=rsc" class="btn">📥 Export RSC</a></div>';
}

function loadSelling(){api('report/selling').then(scripts=>{
let h='<div class="card"><h2>Selling Report</h2><table><thead><tr><th>Name</th><th>Owner</th><th>Source</th><th>Run Count</th></tr></thead><tbody>';
(scripts||[]).forEach(s=>{h+='<tr><td>'+esc(s.name||'')+'</td><td>'+esc(s.owner||'')+'</td><td>'+esc(s.source||'')+'</td><td>'+esc(s['run-count']||'')+'</td></tr>'});
h+='</tbody></table></div>';document.getElementById('content').innerHTML=h})}

function loadTrafficPage(){
document.getElementById('content').innerHTML='<div class="card"><h2>Traffic Monitor</h2><div id="trafficChartFull" style="height:400px"></div></div>';
let ch=Highcharts.chart('trafficChartFull',{chart:{type:'areaspline'},title:{text:'Traffic: '+IFACE},
xAxis:{type:'datetime'},yAxis:{title:{text:null},labels:{formatter:function(){return fmtBps(this.value)}}},
tooltip:{shared:true,formatter:function(){let s='';this.points.forEach(p=>{s+=p.series.name+': '+fmtBps(p.y)+'<br>'});return s}},
series:[{name:'Tx',data:[]},{name:'Rx',data:[]}]});
setInterval(()=>{api('traffic&iface='+IFACE).then(d=>{if(!d||!d.length)return;let x=(new Date).getTime(),sh=ch.series[0].data.length>60;
ch.series[0].addPoint([x,parseInt(d[0].data)||0],true,sh);ch.series[1].addPoint([x,parseInt(d[1].data)||0],true,sh)}).catch(()=>{})},3000);
}

// Utility functions
function removeUser(id){if(!confirm('Remove user?'))return;
api('hotspot/user/remove&id='+id).then(()=>loadPage('users','all'))}
function removeProfile(id){if(!confirm('Remove profile?'))return;
api('hotspot/profile/remove&id='+id).then(()=>loadProfiles())}
function removeActive(id,user){if(!confirm('Remove active session?'))return;
api('hotspot/active/remove&id='+id+'&user='+user).then(()=>loadActive())}
function removeCookie(id){api('hotspot/cookies/remove&id='+id).then(()=>loadCookies())}
function removeExpired(){if(!confirm('Remove all expired users?'))return;
api('hotspot/user/remove-expired').then(d=>alert('Removed: '+(d.removed||0))).then(()=>loadPage('users','all'))}
function removeSelected(){let ids=[...document.querySelectorAll('#dataTable input[type=checkbox]:checked')].map(c=>c.value).filter(v=>v!='on');
if(!ids.length||!confirm('Remove '+ids.length+' users?'))return;
api('hotspot/user/remove&id='+ids.join(',')).then(()=>loadPage('users','all'))}
function toggleAll(el){document.querySelectorAll('#dataTable input[type=checkbox]').forEach(c=>c.checked=el.checked)}
function filterRows(){let v=document.getElementById('filterTable').value.toLowerCase();
document.querySelectorAll('#dataTable tbody tr').forEach(r=>{r.style.display=r.textContent.toLowerCase().includes(v)?'':'none'})}
function loadUserDetail(id){api('hotspot/user&id='+id).then(u=>{
if(u.error){alert(u.error);return}
document.getElementById('content').innerHTML='<div class="card"><h2>User: '+esc(u.name||'')+'</h2>'+
'<table><tr><td>Name</td><td>'+esc(u.name||'')+'</td></tr>'+
'<tr><td>Password</td><td>'+esc(u.password||'')+'</td></tr>'+
'<tr><td>Profile</td><td>'+esc(u.profile||'')+'</td></tr>'+
'<tr><td>Uptime</td><td>'+esc(u.uptime||'0')+'</td></tr>'+
'<tr><td>Bytes In</td><td>'+esc(u['bytes-in']||'0')+'</td></tr>'+
'<tr><td>Bytes Out</td><td>'+esc(u['bytes-out']||'0')+'</td></tr>'+
'<tr><td>Comment</td><td>'+esc(u.comment||'')+'</td></tr>'+
'<tr><td>Disabled</td><td>'+esc(u.disabled||'')+'</td></tr></table>'+
'<div style="margin-top:1rem">'+
'<button class="btn btn-sm" onclick="api(\'hotspot/user/enable&id='+esc(u['.id']||'')+'\').then(()=>loadUserDetail(\''+esc(u['.id']||'')+'\'))">Enable</button> '+
'<button class="btn btn-sm" onclick="api(\'hotspot/user/disable&id='+esc(u['.id']||'')+'\').then(()=>loadUserDetail(\''+esc(u['.id']||'')+'\'))">Disable</button> '+
'<button class="btn btn-sm" onclick="api(\'hotspot/user/reset&id='+esc(u['.id']||'')+'\').then(()=>loadUserDetail(\''+esc(u['.id']||'')+'\'))">Reset</button> '+
'<button class="btn btn-red btn-sm" onclick="removeUser(\''+esc(u['.id']||'')+'\')">Delete</button> '+
'<button class="btn btn-sm" onclick="loadPage(\'users\',\'all\')">Back</button>'+
'</div></div>'})}
function printVouchers(comment){window.open('/voucher/print?session='+SESSION+'&comment='+encodeURIComponent(comment)+'&qr=yes','_blank')}

// Init
loadDashboard();
setTimeout(initTrafficChart,500);
setInterval(loadDashboard,RELOAD*1000);
</script>
</body></html>`,
		baseCSS, session, session, session, session, session, reload, iface, currency)
}

func renderSettings(w http.ResponseWriter, session string, cfg *config.Config) {
	rs := cfg.GetSession(session)
	if rs == nil {
		rs = &config.RouterSession{Name: session}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Settings - %s</title><style>%s</style></head><body>
<div class="container">
<nav class="nav"><a href="/sessions" class="brand">🚀 Mikhmon Go</a><a href="/dashboard?session=%s" class="btn">Dashboard</a></nav>
<div class="card"><h2>Settings: %s</h2>
<form id="settingsForm">
<input type="hidden" name="name" value="%s">
<div class="grid">
<div><label>IP:Port</label><input name="ip" value="%s" required></div>
<div><label>User</label><input name="user" value="%s" required></div>
<div><label>Password</label><input name="password" value="%s" type="password" required></div>
<div><label>Hotspot Name</label><input name="hotspot" value="%s"></div>
<div><label>DNS Name</label><input name="dns" value="%s"></div>
<div><label>Currency</label><input name="currency" value="%s"></div>
<div><label>Auto Reload (sec)</label><input name="reload" type="number" value="%d"></div>
<div><label>Interface</label><input name="interface" value="%s"></div>
<div><label>Idle Timeout (min)</label><input name="idle_timeout" type="number" value="%d"></div>
<div><label>Live Report</label><select name="live_report"><option value="enable" %s>Enable</option><option value="disable" %s>Disable</option></select></div>
</div>
<button type="submit" class="btn btn-green" style="margin-top:1rem">Save</button>
<span id="saveStatus" style="margin-left:1rem"></span>
</form></div></div>
<script>
document.getElementById('settingsForm').addEventListener('submit',function(e){
e.preventDefault();
fetch('/api/config/session',{method:'POST',body:new FormData(this)})
.then(r=>r.json()).then(d=>{
document.getElementById('saveStatus').innerHTML=d.error?'<span style="color:red">'+d.error+'</span>':'<span style="color:#4ecdc4">✅ Saved</span>';
});
});
</script></body></html>`,
		session, baseCSS, session, session, rs.Name, rs.IP, rs.User, rs.Password,
		rs.Hotspot, rs.DNS, rs.Currency, rs.Reload, rs.Interface, rs.IdleTimeout,
		sel(rs.LiveReport, "enable"), sel(rs.LiveReport, "disable"))
}

func sel(val, opt string) string {
	if val == opt || (val == "" && opt == "enable") {
		return "selected"
	}
	return ""
}

func renderVoucherPrint(w http.ResponseWriter, users []map[string]string, showQR bool, currency, dns, hotspot string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8">
<title>Print Vouchers</title>
<script src="https://cdn.jsdelivr.net/npm/qrious@4/dist/qrious.min.js"></script>
<style>
@media print{body{margin:0}.no-print{display:none}}
body{font-family:monospace;font-size:12px}
.voucher{border:1px dashed #333;padding:10px;margin:5px;width:220px;display:inline-block;page-break-inside:avoid;text-align:center}
.voucher h3{margin:0 0 5px;font-size:14px}
.voucher p{margin:2px 0}
.voucher canvas{margin:5px 0}
.no-print{text-align:center;margin:20px}
</style></head><body>
<div class="no-print"><button onclick="window.print()">🖨️ Print</button></div>`)

	for _, u := range users {
		name := u["name"]
		pass := u["password"]
		profile := u["profile"]
		qrID := "qr_" + strings.ReplaceAll(name, " ", "_")
		fmt.Fprintf(w, `<div class="voucher">
<h3>%s</h3>
<p><b>%s</b></p>
<p>User: <b>%s</b></p>
<p>Pass: <b>%s</b></p>
<p>Profile: %s</p>`,
			template.HTMLEscapeString(hotspot),
			template.HTMLEscapeString(dns),
			template.HTMLEscapeString(name),
			template.HTMLEscapeString(pass),
			template.HTMLEscapeString(profile))

		if showQR {
			fmt.Fprintf(w, `<canvas id="%s" width="80" height="80"></canvas>
<script>new QRious({element:document.getElementById('%s'),value:'%s',size:80});</script>`,
				qrID, qrID, template.JSEscapeString(name))
		}
		fmt.Fprint(w, `</div>`)
	}

	fmt.Fprint(w, `<script>setTimeout(function(){window.print()},500)</script></body></html>`)
}

const baseCSS = `
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0f0f23;color:#e0e0e0;min-height:100vh}
.container{max-width:1400px;margin:0 auto;padding:1rem}
.nav{display:flex;align-items:center;justify-content:space-between;padding:.75rem 1rem;background:#1a1a3e;border-radius:10px;margin-bottom:1rem;flex-wrap:wrap;gap:.5rem}
.nav-links{display:flex;gap:.25rem;align-items:center;flex-wrap:wrap}
.nav-links a{color:#ccc;text-decoration:none;padding:.4rem .8rem;border-radius:6px;font-size:.85rem;transition:background .2s}
.nav-links a:hover{background:#2a2a5e;color:#fff}
.brand{color:#e94560!important;font-weight:700;text-decoration:none;font-size:1.1rem}
.dropdown{position:relative;display:inline-block}
.dropdown-content{display:none;position:absolute;background:#1a1a3e;border:1px solid #333;border-radius:8px;min-width:160px;z-index:10;box-shadow:0 8px 24px rgba(0,0,0,.4)}
.dropdown:hover .dropdown-content{display:block}
.dropdown-content a{display:block;padding:.5rem .8rem;color:#ccc;text-decoration:none}
.dropdown-content a:hover{background:#2a2a5e;color:#fff}
.card{background:#16213e;border-radius:10px;padding:1.25rem;margin-bottom:1rem;box-shadow:0 4px 16px rgba(0,0,0,.2)}
.card h2,.card h3{margin-bottom:1rem;color:#e0e0e0}
.row{display:flex;flex-wrap:wrap;gap:1rem;margin-bottom:1rem}
.col-3{flex:0 0 calc(25%% - .75rem)}.col-4{flex:0 0 calc(33.33%% - .67rem)}.col-8{flex:0 0 calc(66.67%% - .33rem)}
.box{padding:1rem;border-radius:10px;text-align:center;transition:transform .2s}
.box h1,.box h2{margin:0}
.box-blue{background:#1e3a5f}.box-green{background:#1a4731}.box-yellow{background:#4a3f1f}.box-red{background:#4a1f2e}.box-purple{background:#2d1b4e}
.clickable{cursor:pointer}.clickable:hover{transform:scale(1.03)}
table{width:100%%;border-collapse:collapse;font-size:.85rem}
th,td{padding:.5rem .75rem;text-align:left;border-bottom:1px solid #2a2a4a}
th{background:#0f1a3a;color:#8888aa;font-weight:600;position:sticky;top:0}
tr:hover{background:#1a2a5a}
.toolbar{display:flex;gap:.5rem;margin-bottom:1rem;flex-wrap:wrap;align-items:center}
.toolbar input{padding:.4rem .8rem;border:1px solid #333;border-radius:6px;background:#0f1a3a;color:#fff;font-size:.85rem}
input,select{padding:.5rem;border:1px solid #333;border-radius:6px;background:#0f1a3a;color:#fff;font-size:.9rem;width:100%%}
input:focus,select:focus{outline:none;border-color:#e94560}
label{display:block;margin-bottom:.25rem;font-size:.8rem;color:#888}
.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:1rem}
.btn{display:inline-block;padding:.5rem 1rem;border:none;border-radius:8px;background:#2a2a5e;color:#fff;cursor:pointer;text-decoration:none;font-size:.85rem;transition:background .2s}
.btn:hover{background:#3a3a7e}.btn-green{background:#1a6b3f}.btn-green:hover{background:#22885a}
.btn-red{background:#9b2335}.btn-red:hover{background:#c03040}.btn-sm{padding:.3rem .6rem;font-size:.8rem}
.btn-xs{padding:.2rem .4rem;font-size:.75rem}
.loading{text-align:center;padding:3rem;color:#666;font-size:1.2rem}
@media(max-width:768px){.col-3,.col-4,.col-8{flex:0 0 100%%}.nav-links{width:100%%}}
`
