#!/usr/bin/env bash
# Prepara a VM uma vez só (rodar de novo não estraga nada): cria o banco e o usuário do Pursuit no
# Postgres do Beacon e o .env com os segredos, que são gerados na VM e ficam só lá.
# Os do Telegram (o token do Sidekick e o meu chat) vêm do .env do Vigil, na VM do bot, e passam
# pela entrada padrão do ssh (nunca como argumento, que apareceria na lista de processos).
# Uso: deploy/preparar-vm.sh
set -euo pipefail

VM="${VM:-ubuntu@159.112.189.95}"
VM_DO_VIGIL="${VM_DO_VIGIL:-ubuntu@147.15.40.173}"
CHAVE_SSH="${CHAVE_SSH:-$HOME/.ssh/oracle_bot}"
DOMINIO_BEACON="${DOMINIO_BEACON:-beacon.andrelagos.dev}"

if ssh -i "$CHAVE_SSH" "$VM" "test -f pursuit/.env"; then
	echo "O ~/pursuit/.env já existe na VM: nada a fazer."
	exit 0
fi

remoto=$(mktemp)
trap 'rm -f "$remoto"' EXIT
cat > "$remoto" <<'REMOTO'
set -euo pipefail
read -r token
read -r chat
dominio_beacon="$1"
mkdir -p ~/pursuit/backups ~/beacon/sites
senha=$(openssl rand -hex 24)
cd ~/beacon
# Pela entrada padrão (e não como argumento): a senha não aparece na lista de processos
docker compose exec -T banco psql -U beacon -d beacon -v ON_ERROR_STOP=1 -q <<SQL
create role pursuit login password '$senha';
create database pursuit owner pursuit;
-- Por padrão todo usuário pode se conectar a qualquer banco; o do Beacon passa a ser só dele
revoke connect on database beacon from public;
SQL
rede=$(docker network inspect -f '{{range .IPAM.Config}}{{.Subnet}}{{end}}' beacon_interna)
chave=$(grep '^INTEGRACAO_CHAVE=' ~/beacon/.env | cut -d= -f2-)
umask 077
cat > ~/pursuit/.env <<ENV
PURSUIT_BANCO_URL=postgres://pursuit:$senha@banco:5432/pursuit?pool_max_conns=3
PURSUIT_TELEGRAM_TOKEN=$token
PURSUIT_TELEGRAM_CHAT=$chat
PURSUIT_PROXY_CONFIAVEL=$rede
PURSUIT_BEACON_URL=https://$dominio_beacon
PURSUIT_BEACON_CHAVE=$chave
ENV
echo "Banco e .env criados (rede do Caddy: $rede)."
REMOTO
scp -q -i "$CHAVE_SSH" "$remoto" "$VM:/tmp/preparar-pursuit.sh"

ssh -i "$CHAVE_SSH" "$VM_DO_VIGIL" "grep '^TELEGRAM_TOKEN=' vigil/.env | cut -d= -f2-; grep '^TELEGRAM_CHAT_ID=' vigil/.env | cut -d= -f2-" \
	| ssh -i "$CHAVE_SSH" "$VM" "bash /tmp/preparar-pursuit.sh '$DOMINIO_BEACON'; rm -f /tmp/preparar-pursuit.sh"
