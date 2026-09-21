# Mikhmon — Docker ops

Two containers:

| Service       | Build file      | Image contents                        | Exposure                          |
| ------------- | --------------- | ------------------------------------- | --------------------------------- |
| `mikhmon-php` | `Dockerfile.php`| nginx + php-fpm (one container), original PHP UI | Public, via Traefik on `proxy` network |
| `mikhmon-api` | `Dockerfile`    | Go microservice `mikhmon-api` on :8088 | **No public port** — only reachable from php-fpm over the `mikhmon-internal` network |

## Build

```bash
docker compose -f docker-compose.vps.yml build
```

## Run

```bash
docker compose -f docker-compose.vps.yml up -d --build
```

Requires an existing external Docker network named `proxy` (Traefik) and a
Traefik instance with the `web`/`websecure` entrypoints and a `letsencrypt`
certresolver. The host `taufiq.nocify.id` is routed to `mikhmon-php`.

The app directory is bind-mounted (`./:/var/www`), so `include/config.php`,
generated vouchers, themes/lang files and uploaded logos live on the host and
survive container rebuilds.

## Logs

```bash
docker logs -f mikhmon-php   # nginx + php-fpm (supervisord merges both)
docker logs -f mikhmon-api   # Go microservice
```

## Override MIKHMON_API_URL

`mikhmon-php` gets `MIKHMON_API_URL=http://mikhmon-api:8088` by default. To
point it elsewhere, edit `environment:` in `docker-compose.vps.yml` (or use a
compose override file) and recreate the container:

```bash
docker compose -f docker-compose.vps.yml up -d --force-recreate mikhmon-php
```

`docker-compose.yml` is the older local-dev stack (Traefik + separate nginx +
fake RouterOS) and is unrelated to production.
