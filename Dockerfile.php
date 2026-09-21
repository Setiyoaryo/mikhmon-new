# syntax=docker/dockerfile:1
#
# Mikhmon PHP frontend: nginx + php-fpm in one container so both share the
# same filesystem (the app writes include/config.php, voucher/temp.php,
# theme/lang files and uploads logos under img/).
# Behind Traefik; the Go API is a separate container (see Dockerfile).

FROM php:7.4-fpm-alpine

# nginx + supervisord (supervisord runs both processes in this one container).
RUN apk add --no-cache nginx supervisor

# No extra PHP extensions are required: the app has no mb_* calls, getimagesize()
# is core, and the RouterOS socket work now happens in the Go service.

# Run nginx and php-fpm as root so both can read/write the bind-mounted app dir
# without uid/gid gymnastics (container is only reachable through Traefik).
RUN sed -i 's/^user[[:space:]]*nginx;/user root;/' /etc/nginx/nginx.conf \
    && sed -i 's/^user[[:space:]]*=[[:space:]]*www-data/user = root/; s/^group[[:space:]]*=[[:space:]]*www-data/group = root/' /usr/local/etc/php-fpm.d/www.conf

# PHP runtime tuning.
COPY docker/php-custom.ini /usr/local/etc/php/conf.d/zz-mikhmon.ini

# Site config (alpine nginx includes http.d/*.conf) and supervisord config.
COPY docker/nginx.default.conf /etc/nginx/http.d/default.conf
COPY docker/supervisord.conf /etc/supervisord.conf

RUN mkdir -p /var/www /run/nginx /var/log/supervisor

WORKDIR /var/www

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget --spider -q http://127.0.0.1/ || exit 1

CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf", "-n"]
