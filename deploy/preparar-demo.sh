#!/usr/bin/env bash
# Prepara a demonstração uma vez só (rodar de novo não estraga nada): cria o banco pursuit_demo e o
# usuário dele no Postgres do Beacon e o ~/pursuit-demo/.env, com PURSUIT_DEMO=true, sem Telegram e
# apontando para a demonstração do Beacon, com a chave da demo (INTEGRACAO_CHAVE_DEMO), nunca a real.
# Uso: deploy/preparar-demo.sh
set -euo pipefail

VM="${VM:-ubuntu@159.112.189.95}"
CHAVE_SSH="${CHAVE_SSH:-$HOME/.ssh/oracle_bot}"

ssh -i "$CHAVE_SSH" "$VM" bash -s <<'REMOTO'
set -euo pipefail
if [ -f ~/pursuit-demo/.env ]; then
	echo "O ~/pursuit-demo/.env já existe: nada a fazer."
	exit 0
fi
mkdir -p ~/pursuit-demo
senha=$(openssl rand -hex 24)
cd ~/beacon
# Pela entrada padrão: a senha não aparece na lista de processos
docker compose exec -T banco psql -U beacon -d beacon -v ON_ERROR_STOP=1 -q <<SQL
create role pursuit_demo login password '$senha';
create database pursuit_demo owner pursuit_demo;
revoke connect on database pursuit_demo from public;
SQL
rede=$(docker network inspect -f '{{range .IPAM.Config}}{{.Subnet}}{{end}}' beacon_interna)
dominio_beacon=$(grep '^DOMINIO_DEMO=' ~/beacon/.env | cut -d= -f2-)
chave=$(grep '^INTEGRACAO_CHAVE_DEMO=' ~/beacon/.env | cut -d= -f2-)
umask 077
cat > ~/pursuit-demo/.env <<ENV
PURSUIT_DEMO=true
PURSUIT_BANCO_URL=postgres://pursuit_demo:$senha@beacon-banco:5432/pursuit_demo?pool_max_conns=3
PURSUIT_PROXY_CONFIAVEL=$rede
PURSUIT_BEACON_URL=https://$dominio_beacon
PURSUIT_BEACON_CHAVE=$chave
ENV
echo "Banco e .env da demonstração criados."
REMOTO
