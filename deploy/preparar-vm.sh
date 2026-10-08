#!/usr/bin/env bash
# Prepara a VM uma vez só (rodar de novo não estraga nada): cria o banco e o usuário do Pursuit
# no Postgres do Vigil e o .env com os segredos, que são gerados e ficam só na VM.
# Uso: deploy/preparar-vm.sh
set -euo pipefail

VM="${VM:-ubuntu@147.15.40.173}"
CHAVE_SSH="${CHAVE_SSH:-$HOME/.ssh/oracle_bot}"

ssh -i "$CHAVE_SSH" "$VM" bash -s <<'REMOTO'
set -euo pipefail
mkdir -p ~/pursuit/backups ~/vigil/sites
if [ -f ~/pursuit/.env ]; then
	echo "O ~/pursuit/.env já existe: nada a fazer."
	exit 0
fi
senha=$(openssl rand -hex 24)
# Pela entrada padrão (e não como argumento): a senha não aparece na lista de processos
cd ~/vigil
docker compose exec -T banco psql -U vigil -d vigil -v ON_ERROR_STOP=1 -q <<SQL
create role pursuit login password '$senha';
create database pursuit owner pursuit;
-- Por padrão todo usuário pode se conectar a qualquer banco; o do Vigil passa a ser só dele
revoke connect on database vigil from public;
SQL
rede=$(docker network inspect -f '{{range .IPAM.Config}}{{.Subnet}}{{end}}' vigil_default)
token=$(grep '^TELEGRAM_TOKEN=' ~/vigil/.env | cut -d= -f2-)
chat=$(grep '^TELEGRAM_CHAT_ID=' ~/vigil/.env | cut -d= -f2-)
umask 077
cat > ~/pursuit/.env <<ENV
PURSUIT_BANCO_URL=postgres://pursuit:$senha@banco:5432/pursuit?pool_max_conns=3
PURSUIT_TELEGRAM_TOKEN=$token
PURSUIT_TELEGRAM_CHAT=$chat
PURSUIT_PROXY_CONFIAVEL=$rede
ENV
echo "Banco e .env criados (rede do Caddy: $rede)."
REMOTO
