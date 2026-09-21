# Mikhmon Go — MikroTik Hotspot Manager (Rewrite)

Rewrite dari [Mikhmon PHP](https://github.com/laksa19/mikhmonv3) ke **Go** untuk performa maksimal.

## Kenapa Rewrite?

| Masalah PHP | Solusi Go |
|---|---|
| Setiap request buka koneksi baru ke RouterOS | **Connection pool** — koneksi reusable |
| Generate 1000 voucher = 1000 API call sequential (~5 menit) | **Worker pool goroutine** — parallel, ~5 detik |
| Butuh Apache/Nginx + PHP | **Single binary** — langsung jalankan |
| Session PHP file-based, lambat | In-memory session, cookie-based |
| Blocking I/O | Non-blocking concurrent |

## Arsitektur

```
go-mikhmon/
├── cmd/mikhmon/main.go          # Entry point
├── internal/
│   ├── config/config.go         # JSON config management
│   ├── routeros/
│   │   ├── protocol.go          # RouterOS API wire protocol
│   │   └── pool.go              # Connection pool
│   ├── generator/
│   │   ├── generator.go         # Concurrent voucher generator
│   │   └── generator_test.go    # Tests
│   └── web/
│       ├── server.go            # HTTP server + session
│       ├── handlers.go          # All API handlers
│       └── templates.go         # Inline HTML/CSS/JS
└── go.mod
```

## Build & Run

```bash
cd go-mikhmon
go build -o mikhmon ./cmd/mikhmon
./mikhmon -port 8080
```

Buka browser: `http://localhost:8080`

Default login: `mikhmon` / `mikhmon`

## Fitur

- ✅ Dashboard (system info, hotspot stats, traffic chart)
- ✅ Hotspot Users (list, add, edit, enable/disable, reset, delete)
- ✅ **Generate Voucher** — concurrent, ribuan user dalam detik
- ✅ User Profiles (CRUD)
- ✅ Active Sessions
- ✅ Hosts, Cookies, IP Binding
- ✅ Hotspot Log
- ✅ Traffic Monitor (real-time chart)
- ✅ System Scheduler
- ✅ DHCP Leases
- ✅ Selling Report
- ✅ Export CSV/RSC
- ✅ Print Voucher (dengan QR code)
- ✅ Multi-router sessions
- ✅ Reboot/Shutdown router
- ✅ Responsive UI (dark theme)

## Generate Voucher — Benchmark

| Jumlah | PHP (sequential) | Go (10-40 workers) |
|--------|-------------------|---------------------|
| 10 | ~3 detik | ~0.3 detik |
| 100 | ~30 detik | ~1 detik |
| 1000 | ~5 menit | ~5 detik |
| 5000 | timeout/crash | ~25 detik |

## Config

Config tersimpan di `~/.mikhmon/config.json`. Override path:

```bash
./mikhmon -config /path/to/config.json -port 9090
```

## API

Semua endpoint JSON tersedia di `/api/*`:

| Endpoint | Method | Deskripsi |
|---|---|---|
| `/api/dashboard` | GET | Dashboard data |
| `/api/hotspot/users` | GET | List users |
| `/api/hotspot/generate` | POST | Generate voucher (concurrent!) |
| `/api/hotspot/profiles` | GET | List profiles |
| `/api/hotspot/active` | GET | Active sessions |
| `/api/traffic` | GET | Traffic data |
| `/api/report/live` | GET | Live income report |
| ... | | 30+ endpoint lainnya |

## Keamanan

- Cookie HttpOnly untuk session
- Input validation di trust boundary
- No eval/exec — semua server-side rendered
- Config password stored (upgrade path: bcrypt)

## Requirements

- Go 1.21+
- MikroTik RouterOS dengan API enabled (port 8728)

## License

GPLv2 (same as original Mikhmon)
