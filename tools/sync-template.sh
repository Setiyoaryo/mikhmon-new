#!/usr/bin/env bash
#
# sync-template.sh — Sinkronisasi template kustom per-customer ke VPS secara instan.
#
# Penggunaan:
#   ./tools/sync-template.sh <customer-subdomain> [staging|prod]
#
# Contoh:
#   ./tools/sync-template.sh warkop-berkah staging
#   ./tools/sync-template.sh taufiq prod
#

set -e

CUSTOMER="${1:-}"
TARGET="${2:-staging}"
VPS_HOST="43.133.142.67"
VPS_USER="ubuntu"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_ed25519}"

if [ -z "$CUSTOMER" ]; then
  echo "Penggunaan: $0 <customer-subdomain> [staging|prod]"
  echo "Contoh:     $0 warkop-berkah staging"
  exit 1
fi

LOCAL_DIR="custom-templates/$CUSTOMER"
if [ ! -d "$LOCAL_DIR" ]; then
  echo "Error: Folder $LOCAL_DIR tidak ditemukan."
  echo "Buat foldernya dulu atau salin dari custom-templates/_contoh/"
  exit 1
fi

if [ "$TARGET" = "prod" ] || [ "$TARGET" = "production" ]; then
  REMOTE_DIR="/opt/mikhmon-new/custom-templates/$CUSTOMER"
  DOMAIN_SUFFIX="nocify.id"
else
  REMOTE_DIR="/opt/mikhmon-staging/custom-templates/$CUSTOMER"
  DOMAIN_SUFFIX="staging.nocify.id"
fi

echo "==> Menyinkronkan template '$CUSTOMER' ke $TARGET ($VPS_HOST)..."

# Pastikan folder remote ada
ssh -i "$SSH_KEY" -o BatchMode=yes "$VPS_USER@$VPS_HOST" "sudo -n mkdir -p $REMOTE_DIR && sudo -n chown -R $VPS_USER:$VPS_USER $REMOTE_DIR"

# Rsync file template
rsync -avz -e "ssh -i $SSH_KEY -o BatchMode=yes" "$LOCAL_DIR/" "$VPS_USER@$VPS_HOST:$REMOTE_DIR/"

echo
echo "✓ Selesai! Template '$CUSTOMER' aktif di $TARGET."
echo "  Panel login : https://$CUSTOMER.$DOMAIN_SUFFIX/admin.php?id=login"
echo "  WiFi hotspot: https://$CUSTOMER.$DOMAIN_SUFFIX/hotspot-login/"
