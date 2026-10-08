#!/usr/bin/env bash
# Publica uma versão nova na VM: monta a imagem aqui, manda pela conexão SSH e reinicia.
# A VM não precisa do código nem do Go, só do Docker. Uso: deploy/publicar.sh
set -euo pipefail

VM="${VM:-ubuntu@147.15.40.173}"
CHAVE_SSH="${CHAVE_SSH:-$HOME/.ssh/oracle_bot}"
DOMINIO="${DOMINIO:-pursuit.147-15-40-173.sslip.io}"
cd "$(dirname "$0")/.."

# Só publica o que está no git: a imagem leva o commit, e dá para saber o que está no ar
if [ -n "$(git status --porcelain)" ]; then
	echo "Há mudanças sem commit. Faça o commit antes de publicar." >&2
	exit 1
fi
commit=$(git rev-parse --short HEAD)

if ! ssh -i "$CHAVE_SSH" "$VM" "test -f pursuit/.env"; then
	echo "Falta o ~/pursuit/.env na VM. Rode deploy/preparar-vm.sh antes." >&2
	exit 1
fi

echo "Montando a imagem ($commit)..."
docker build -q --platform linux/amd64 --build-arg COMMIT="$commit" -t pursuit:latest -t "pursuit:$commit" .

echo "Enviando para a VM (comprimida)..."
docker save pursuit:latest | gzip | ssh -i "$CHAVE_SSH" "$VM" "gunzip | docker load -q"
scp -q -i "$CHAVE_SSH" deploy/compose.yaml deploy/backup.sh "$VM:pursuit/"
sed "s/DOMINIO_PURSUIT/$DOMINIO/" deploy/pursuit.caddy | ssh -i "$CHAVE_SSH" "$VM" "cat > vigil/sites/pursuit.caddy"

echo "Reiniciando..."
ssh -i "$CHAVE_SSH" "$VM" "cd pursuit && docker compose up -d --remove-orphans && docker image prune -f >/dev/null \
	&& cd ../vigil && docker compose exec -T caddy caddy reload --config /etc/caddy/Caddyfile"

echo "Conferindo a saúde..."
for _ in $(seq 1 20); do
	if ssh -i "$CHAVE_SSH" "$VM" "cd pursuit && docker compose exec -T api /pursuit saude" 2>/dev/null; then
		echo "No ar: https://$DOMINIO ($commit)"
		exit 0
	fi
	sleep 3
done
echo "O Pursuit não respondeu. Veja: ssh ... 'cd pursuit && docker compose logs api --tail 50'" >&2
exit 1
