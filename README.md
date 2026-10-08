# Pursuit

**Pursuit · acompanhamento das minhas candidaturas a vagas.**

[![CI](https://github.com/Lakes777/pursuit/actions/workflows/ci.yml/badge.svg)](https://github.com/Lakes777/pursuit/actions/workflows/ci.yml)

Uma API para acompanhar as vagas em que me candidatei: empresa, etapa, datas e anotações de cada
entrevista. O objetivo é mostrar o funil (quantas candidaturas passam de cada etapa), avisar pelo
Telegram quando uma candidatura fica parada e, com o [Beacon](https://github.com/Lakes777/beacon),
dizer se a empresa abriu o link do meu portfólio. **Em construção:** as fases abaixo mostram o que
já está pronto.

É o meu primeiro projeto em **Go**.

## Fases

- [x] **1. Esqueleto:** Go com o roteador da biblioteca padrão, PostgreSQL com migrações (goose), `/saude`, testes com Testcontainers, CI com lint
- [x] **2. Candidaturas:** cadastro com etapas (interesse, enviada, triagem, entrevista, técnica, proposta, contratado, recusada, desisti) e o histórico de cada mudança
- [ ] **3. Login:** conta única com senha e sessão por cookie
- [ ] **4. Números:** funil por etapa, tempo até a resposta, taxa por fonte (LinkedIn, Gupy...)
- [ ] **5. Lembretes:** uma goroutine em segundo plano avisa pelo Telegram as candidaturas paradas há dias
- [ ] **6. Beacon:** um link curto por candidatura, com os cliques
- [ ] **7. Interface:** página própria, com identidade visual
- [ ] **8. Publicação:** imagem Docker só com o binário, numa VM da Oracle

## Tecnologias

- Go 1.27, `net/http` (roteador da biblioteca padrão, sem framework), `log/slog`
- PostgreSQL 17 com [pgx](https://github.com/jackc/pgx) e migrações pelo [goose](https://github.com/pressly/goose), embutidas no binário
- [sqlc](https://sqlc.dev): eu escrevo o SQL e ele gera as funções Go com os tipos certos
- Testes com o pacote `testing`, `httptest` e [Testcontainers](https://golang.testcontainers.org/) (Postgres de verdade num contêiner), sempre com `-race`
- GitHub Actions: `gofmt`, `sqlc diff`, `go vet`, [golangci-lint](https://golangci-lint.run/) (com `gosec`) e os testes

## Como rodar

Precisa do Go 1.27 e do Docker. Para mudar o SQL, também do sqlc
(`go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`, depois `sqlc generate`).

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
| `GET` | `/api/etapas` | As etapas na ordem do processo, com o nome para a tela e se é final |
| `POST` | `/api/candidaturas` | Cadastra: `empresa` e `vaga` obrigatórios; `link`, `fonte`, `modalidade` (`remoto`, `hibrido`, `presencial`), `salario`, `anotacoes`; opcionais `etapa` inicial (padrão `interesse`), `observacao` e `em` (quando foi, pode ser no passado; formato RFC 3339 com fuso, como `2026-10-06T10:00:00-03:00`) |
| `GET` | `/api/candidaturas?etapa=&busca=` | Lista, da mexida por último à mais antiga; filtra por etapa e busca na empresa ou na vaga |
| `GET` | `/api/candidaturas/{id}` | Uma candidatura com o histórico de etapas |
| `PUT` | `/api/candidaturas/{id}` | Troca os dados. Aceita só os campos de dados (`empresa` a `anotacoes`): `etapa`, `id` ou `historico` dão `400`, porque a etapa tem rota própria, que guarda o histórico |
| `DELETE` | `/api/candidaturas/{id}` | Apaga a candidatura e o histórico |
| `POST` | `/api/candidaturas/{id}/etapas` | Muda a etapa: `etapa`, opcionais `observacao` e `em` |

```bash
curl -X POST 127.0.0.1:8095/api/candidaturas \
  -d '{"empresa":"Nubank","vaga":"Back-end Go","fonte":"LinkedIn","etapa":"enviada","em":"2026-10-06T10:00:00-03:00"}'
curl -X POST 127.0.0.1:8095/api/candidaturas/1/etapas -d '{"etapa":"triagem","observacao":"o RH ligou"}'
```

Erros no formato da RFC 9457 (`application/problem+json`), com o problema de cada campo:

```json
{"title": "Dados inválidos", "status": 422, "campos": {"empresa": "obrigatório", "link": "use um endereço completo, começando com http:// ou https://"}}
```

`400` para JSON inválido (inclusive campo desconhecido, como um `emrpesa` digitado errado), `404` para
candidatura que não existe, `409` para mudar para a etapa em que ela já está, `413` para corpo acima de 64 KB.

## Decisões

- **Etapa só muda pela rota de etapas, numa transação:** a linha da candidatura é travada
  (`select ... for update`), a mudança vai para o histórico e a etapa é atualizada. Dez pedidos
  juntos para a mesma etapa: um passa e os outros recebem `409` (há um teste que faz exatamente isso).
- **Data da mudança opcional e no passado:** dá para registrar hoje que me candidatei na segunda.
  Não pode ser no futuro (com 1 min de folga para o relógio do celular) nem antes da última mudança,
  para o histórico ficar em ordem.
- **sqlc em vez de ORM:** o SQL fica à vista em `internal/banco/consultas`, e o código Go é gerado com
  os tipos das colunas. O CI roda `sqlc diff`: se alguém mudar o SQL e esquecer de gerar, falha.
- **Validação com o nome do campo** no serviço, com os mesmos limites das `check` do banco: o banco
  é a última barreira, mas a mensagem útil para a tela vem antes. Os limites contam letras, não bytes.
- **Busca com `%` e `_` escapados:** buscar "100%" acha "100%", e não tudo que começa com "100".
- **Middlewares** feitos à mão: um registra cada pedido no log (método, caminho, status, tempo) e
  outro transforma um `panic` num `500`, sem derrubar a conexão.
- **Leitura coerente:** a candidatura e o histórico são lidos numa transação só de leitura
  (*repeatable read*), então uma mudança de etapa no meio não deixa os dois desencontrados.
- **Caractere NUL recusado com `422`:** o Postgres não guarda o byte 0 em `text`, e sem a checagem
  ele viraria um `500`.
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
internal/banco/       conexão (pgxpool) e migrações (goose) · migracoes/*.sql · consultas/*.sql
internal/banco/bd/    código gerado pelo sqlc (não editar)
internal/candidaturas/ regras: validação, etapas, histórico, transações
internal/api/         rotas HTTP, erros (RFC 9457), middlewares
internal/testebanco/  Postgres dos testes (Testcontainers)
```
