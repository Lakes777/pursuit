#!/usr/bin/env bash
# Backup diário do banco do Pursuit, rodado pelo cron da VM (veja "Publicação" no README).
# O banco mora no Postgres do Vigil. Guarda os últimos 7 dias em ~/pursuit/backups.
set -euo pipefail

cd "$(dirname "$0")"
mkdir -p backups
arquivo="backups/pursuit-$(date +%F).sql.gz"
# Escreve num temporário e só renomeia no fim: um backup pela metade nunca passa por bom
(cd ../vigil && docker compose exec -T banco pg_dump -U vigil pursuit) | gzip > "$arquivo.tmp"
mv "$arquivo.tmp" "$arquivo"
ls -1t backups/pursuit-*.sql.gz | tail -n +8 | xargs -r rm --
