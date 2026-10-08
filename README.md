# Pursuit

**Pursuit · acompanhamento das minhas candidaturas a vagas.**

[![CI](https://github.com/Lakes777/pursuit/actions/workflows/ci.yml/badge.svg)](https://github.com/Lakes777/pursuit/actions/workflows/ci.yml)

Uma API para acompanhar as vagas em que me candidatei: empresa, etapa, datas e anotações de cada
entrevista. O objetivo é mostrar o funil (quantas candidaturas passam de cada etapa), avisar pelo
Telegram quando uma candidatura fica parada e, com o [Beacon](https://github.com/Lakes777/beacon),
dizer se a empresa abriu o link do meu portfólio. **Em construção:** hoje existe o esqueleto (fase 1);
as fases abaixo mostram o que já está pronto.

É o meu primeiro projeto em **Go**.

## Fases

- [x] **1. Esqueleto:** Go com o roteador da biblioteca padrão, PostgreSQL com migrações (goose), `/saude`, testes com Testcontainers, CI com lint
- [ ] **2. Candidaturas:** cadastro com etapas (interesse, enviada, triagem, entrevista, técnica, proposta, recusada, desisti) e o histórico de cada mudança
- [ ] **3. Login:** conta única com senha e sessão por cookie
- [ ] **4. Números:** funil por etapa, tempo até a resposta, taxa por fonte (LinkedIn, Gupy...)
- [ ] **5. Lembretes:** uma goroutine em segundo plano avisa pelo Telegram as candidaturas paradas há dias
- [ ] **6. Beacon:** um link curto por candidatura, com os cliques
- [ ] **7. Interface:** página própria, com identidade visual
- [ ] **8. Publicação:** imagem Docker só com o binário, numa VM da Oracle

## Tecnologias

- Go 1.27, `net/http` (roteador da biblioteca padrão, sem framework), `log/slog`
- PostgreSQL 17 com [pgx](https://github.com/jackc/pgx) e migrações pelo [goose](https://github.com/pressly/goose), embutidas no binário
- Testes com o pacote `testing`, `httptest` e [Testcontainers](https://golang.testcontainers.org/) (Postgres de verdade num contêiner), sempre com `-race`
- GitHub Actions: `gofmt`, `go vet`, [golangci-lint](https://golangci-lint.run/) (com `gosec`) e os testes

## Como rodar

Precisa do Go 1.27 e do Docker.

```bash
docker compose up -d          # Postgres na porta 5435
go run ./cmd/pursuit          # API em http://127.0.0.1:8095 (as migrações rodam ao subir)
curl 127.0.0.1:8095/saude     # {"banco":"ok","status":"ok"}
go test -race ./...           # testes (sobem um Postgres próprio pelo Testcontainers)
```

Configuração por variáveis de ambiente:

| Variável | Padrão | O que é |
|---|---|---|
| `PURSUIT_ENDERECO` | `127.0.0.1:8095` | Onde o servidor escuta |
| `PURSUIT_BANCO_URL` | `postgres://pursuit:pursuit@localhost:5435/pursuit` | Conexão com o Postgres |

## Rotas

| Método | Rota | O que faz |
|---|---|---|
| `GET` | `/saude` | `200` com o banco no ar, `503` sem ele |

## Decisões

- **Sem framework web.** Desde o Go 1.22 o `http.ServeMux` da biblioteca padrão entende método e
  parâmetros no caminho (`GET /api/candidaturas/{id}`), então dá para ficar só com ela.
- **Migrações dentro do binário** (`go:embed`): a imagem de produção leva só o executável. Duas
  instâncias subindo juntas não migram ao mesmo tempo: o goose segura uma trava do Postgres
  (*advisory lock*) enquanto aplica.
- **Desligamento com calma:** no Ctrl+C ou no `SIGTERM` do Docker, o servidor para de aceitar
  conexões e espera até 10 s as que estão no meio; depois disso, fecha à força. Um segundo Ctrl+C
  encerra na hora.
- **A porta abre antes do "no ar":** o `net.Listen` vem primeiro, então uma porta ocupada dá erro
  logo, sem uma mensagem de "no ar" antes.
- **Limites de tempo no servidor** (`ReadHeaderTimeout` e outros): sem eles, um cliente que manda o
  pedido bem devagar seguraria uma conexão para sempre (ataque Slowloris). O `gosec` cobra isso.
- **Um Postgres por pacote de teste,** subido no primeiro uso (`sync.Once`), com as tabelas limpas por
  teste e encerrado no `TestMain`. Por isso os testes que usam o banco não rodam em paralelo.

## Organização

```
cmd/pursuit/          main.go (sobe o servidor e desliga com calma)
internal/config/      variáveis de ambiente
internal/banco/       conexão (pgxpool) e migrações (goose) · migracoes/*.sql
internal/api/         rotas HTTP
internal/testebanco/  Postgres dos testes (Testcontainers)
```
