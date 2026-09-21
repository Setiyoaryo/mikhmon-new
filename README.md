# Mikhmon Go — MikroTik Hotspot Manager

Rewrite dari Mikhmon PHP ke **Go** untuk performa maksimal. Single binary, no PHP/Apache.

## Quick Start (Docker)

```bash
git clone https://github.com/Setiyoaryo/mikhmon-new.git
cd mikhmon-new
docker compose up -d --build
```

Buka `http://localhost:8080` → Login: `mikhmon` / `mikhmon`

## Quick Start (VPS + Traefik)

```bash
git clone https://github.com/Setiyoaryo/mikhmon-new.git
cd mikhmon-new
docker compose -f docker-compose.vps.yml up -d --build
```

Akses via `https://taufiq.nocify.id`

## Quick Start (Binary)

```bash
cd mikhmon-new
go build -o mikhmon ./cmd/mikhmon
./mikhmon -port 8080
```

## Kenapa Rewrite ke Go?

| Masalah PHP | Solusi Go |
|---|---|
| Setiap request buka koneksi baru ke RouterOS | **Connection pool** — koneksi reusable |
| Generate 1000 voucher = sequential (~5 menit) | **Goroutine worker pool** — parallel (~5 detik) |
| Butuh Apache/Nginx + PHP | **Single binary** — langsung jalankan |
| Blocking I/O | Non-blocking concurrent |

## Benchmark Generate Voucher

| Jumlah | PHP | Go |
|--------|-----|-----|
| 10 | ~3s | ~0.3s |
| 100 | ~30s | ~1s |
| 1000 | ~5min | ~5s |
| 5000 | timeout | ~25s |

## Struktur

```
├── cmd/mikhmon/main.go           # Entry point
├── internal/
│   ├── config/config.go          # JSON config
│   ├── routeros/
│   │   ├── protocol.go           # RouterOS API protocol
│   │   └── pool.go               # Connection pool
│   ├── generator/
│   │   ├── generator.go          # Concurrent voucher generator
│   │   └── generator_test.go     # Tests
│   └── web/
│       ├── server.go             # HTTP server + session
│       ├── handlers.go           # 40+ API handlers
│       └── templates.go          # SPA UI (dark theme)
├── Dockerfile                    # Multi-stage build (~15MB)
├── docker-compose.yml            # Dev (with RouterOS emulator)
└── docker-compose.vps.yml        # Production (with Traefik)
```

## Fitur

- Dashboard (system info, hotspot stats, traffic chart)
- Hotspot Users (list, add, edit, enable/disable, reset, delete)
- **Generate Voucher** — concurrent, ribuan dalam detik
- User Profiles, Active Sessions, Hosts, Cookies, IP Binding
- Traffic Monitor (real-time Highcharts)
- System Scheduler, DHCP Leases
- Selling Report, Export CSV/RSC
- Print Voucher (QR code)
- Multi-router sessions
- Reboot/Shutdown router
- Responsive dark theme UI

## Requirements

- Docker (recommended), atau Go 1.21+
- MikroTik RouterOS dengan API enabled (port 8728)

## License

GPLv2
