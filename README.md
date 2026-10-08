# Pursuit

**Pursuit · acompanhamento das minhas candidaturas a vagas.**

[![CI](https://github.com/Lakes777/pursuit/actions/workflows/ci.yml/badge.svg)](https://github.com/Lakes777/pursuit/actions/workflows/ci.yml)

Uma API para acompanhar as vagas em que me candidatei: empresa, etapa, datas e anotações de cada
entrevista. O objetivo é mostrar o funil (quantas candidaturas passam de cada etapa), avisar pelo
Telegram quando uma candidatura fica parada e, com o [Beacon](https://github.com/Lakes777/beacon),
dizer se a empresa abriu o link do meu portfólio. **Em construção:** as fases abaixo mostram o que
já está pronto.

É o meu primeiro projeto em **Go**.

![Quadro do Pursuit: uma pasta por etapa, com as fichas das candidaturas; as paradas em âmbar](docs/quadro.png)

## Fases

- [x] **1. Esqueleto:** Go com o roteador da biblioteca padrão, PostgreSQL com migrações (goose), `/saude`, testes com Testcontainers, CI com lint
- [x] **2. Candidaturas:** cadastro com etapas (interesse, enviada, triagem, entrevista, técnica, proposta, contratado, recusada, desisti) e o histórico de cada mudança
- [x] **3. Login:** conta única com senha (argon2id), sessão por cookie guardada no banco, limite de tentativas; toda rota de `/api` pede login
- [x] **4. Números:** funil por etapa, tempo até a resposta, taxa por fonte (LinkedIn, Gupy...), envios por semana
- [x] **5. Lembretes:** uma goroutine em segundo plano avisa pelo Telegram as candidaturas paradas há dias (uma mensagem por dia, no máximo)
- [ ] **6. Beacon:** um link curto por candidatura, com os cliques
- [x] **7. Interface:** página servida pelo próprio binário, com identidade visual própria ("Fichário"): quadro por etapa com arrastar e soltar, ficha com o histórico, formulário, números com gráficos e lembretes
- [ ] **8. Publicação:** imagem Docker só com o binário, numa VM da Oracle. Atenção: atrás do Caddy todo pedido chega com o IP do proxy; o limite de login precisa confiar no `X-Forwarded-For` só quando a conexão vier do proxy

## Interface

Em `/`, servida pelo próprio Go (`internal/web`, com `go:embed`): HTML, CSS e JavaScript puros, sem
etapa de build nem bibliotecas.

- **Identidade "Fichário":** a busca por vaga como um arquivo de pastas. Cada etapa é uma pasta com
  orelha e cada candidatura uma ficha; petróleo e verde-água, âmbar só para o que está parado.
  Fontes Familjen Grotesk e Atkinson Hyperlegible (feita para leitura fácil), servidas localmente.
- **Quadro:** arrastar a ficha para outra pasta muda a etapa; o botão "Mover" faz o mesmo pelo
  teclado e no celular, e já sugere a etapa seguinte. Os dois pedem data e observação opcionais.
- **Ficha da candidatura** com a linha do tempo das etapas; **Números** com o funil, o tempo de
  resposta, os envios por semana (SVG desenhado à mão) e as fontes; **Lembretes** com o que vai
  no Telegram e as regras (lidas da API, sem números repetidos no JavaScript).
- **Segurança:** CSP só `'self'` (nada de CDN, script ou estilo inline), `nosniff`, `DENY`, sem
  `Referer`. Nenhum dado entra como HTML: os elementos são montados com `textContent`.
- **Cache:** cada arquivo tem um ETag (sha256 do conteúdo); com `no-cache`, o navegador pergunta
  sempre, recebe `304` quando nada mudou e vê uma versão nova na hora.
- **Testes:** as funções puras (datas, escalas dos gráficos, agrupamento do quadro, corpo dos
  pedidos) com `node --test`, sem dependências, no CI.
- Feita em duas partes por dois agentes em paralelo, cada um no seu *git worktree* e nos próprios
  arquivos (quadro/ficha/formulário e números/lembretes), sobre uma base comum; depois revisão e merge.

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
docker compose up -d                                         # Postgres na porta 5435
go run ./cmd/pursuit definir-senha andre                     # cria a conta (pergunta a senha, escondida)
PURSUIT_COOKIE_SEGURO=false go run ./cmd/pursuit             # API em http://127.0.0.1:8095 (sem HTTPS no computador)
curl 127.0.0.1:8095/saude                                    # {"banco":"ok","status":"ok"}
go test -race ./...                                          # testes (sobem um Postgres próprio pelo Testcontainers)
```

Configuração por variáveis de ambiente:

| Variável | Padrão | O que é |
|---|---|---|
| `PURSUIT_ENDERECO` | `127.0.0.1:8095` | Onde o servidor escuta |
| `PURSUIT_BANCO_URL` | `postgres://pursuit:pursuit@localhost:5435/pursuit` | Conexão com o Postgres |
| `PURSUIT_COOKIE_SEGURO` | `true` | Cookie da sessão só por HTTPS; `false` só no computador |
| `PURSUIT_LOGIN_POR_MINUTO` | `10` | Tentativas de login por minuto por IP |
| `PURSUIT_TELEGRAM_TOKEN` | (vazio) | Token do bot (o do Sidekick). Sem ele, os lembretes ficam desligados |
| `PURSUIT_TELEGRAM_CHAT` | (vazio) | O meu chat com o bot. Os dois vazios desligam os lembretes; só um, ou um token mal escrito, impede a API de subir |
| `PURSUIT_TELEGRAM_URL` | (vazio) | Endereço da API do Telegram; vazio = `api.telegram.org` (só os testes trocam) |

## Rotas

| Método | Rota | O que faz |
|---|---|---|
| `GET` | `/saude` | `200` com o banco no ar, `503` sem ele (pública) |
| `POST` | `/api/sessao` | Entra: `usuario` e `senha`; grava o cookie da sessão (pública, com limite de tentativas: `429`) |
| `GET` | `/api/sessao` | Quem está logado |
| `DELETE` | `/api/sessao` | Sai: apaga a sessão no banco e o cookie (pública) |
| `GET` | `/api/etapas` | As etapas na ordem do processo, com o nome para a tela e se é final |
| `POST` | `/api/candidaturas` | Cadastra: `empresa` e `vaga` obrigatórios; `link`, `fonte`, `modalidade` (`remoto`, `hibrido`, `presencial`), `salario`, `anotacoes`; opcionais `etapa` inicial (padrão `interesse`), `observacao` e `em` (quando foi, pode ser no passado; formato RFC 3339 com fuso, como `2026-10-06T10:00:00-03:00`) |
| `GET` | `/api/candidaturas?etapa=&busca=` | Lista, da mexida por último à mais antiga; filtra por etapa e busca na empresa ou na vaga. Cada uma traz `etapaDesde`, a última mudança de etapa (`atualizadaEm` muda também ao editar) |
| `GET` | `/api/candidaturas/{id}` | Uma candidatura com o histórico de etapas |
| `PUT` | `/api/candidaturas/{id}` | Troca os dados. Aceita só os campos de dados (`empresa` a `anotacoes`): `etapa`, `id` ou `historico` dão `400`, porque a etapa tem rota própria, que guarda o histórico |
| `DELETE` | `/api/candidaturas/{id}` | Apaga a candidatura e o histórico |
| `POST` | `/api/candidaturas/{id}/etapas` | Muda a etapa: `etapa`, opcionais `observacao` e `em` |
| `GET` | `/api/numeros` | Funil, tempo de resposta, fontes, etapas atuais e envios por semana (abaixo) |
| `GET` | `/api/lembretes` | O que seria lembrado agora (as candidaturas paradas além do prazo) |
| `GET` | `/api/lembretes/prazos` | Os prazos por etapa (`etapa`, `nome`, `dias`), na ordem do processo; a tela lê daqui |

Todas as rotas de `/api`, menos entrar e sair, respondem `401` sem login:

```bash
curl -c cookies -X POST 127.0.0.1:8095/api/sessao -d '{"usuario":"andre","senha":"..."}'
curl -b cookies -X POST 127.0.0.1:8095/api/candidaturas \
  -d '{"empresa":"Nubank","vaga":"Back-end Go","fonte":"LinkedIn","etapa":"enviada","em":"2026-10-06T10:00:00-03:00"}'
curl -b cookies -X POST 127.0.0.1:8095/api/candidaturas/1/etapas -d '{"etapa":"triagem","observacao":"o RH ligou"}'
```

### Números

```json
{"total": 6, "emAndamento": 3,
 "funil": [{"etapa": "enviada", "nome": "Candidatura enviada", "total": 5, "taxa": 100},
           {"etapa": "entrevista", "nome": "Entrevista", "total": 3, "taxa": 60}, ...],
 "respostas": {"respondidas": 2, "aguardando": 1, "mediaDias": 3, "medianaDias": 3},
 "porFonte": [{"fonte": "LinkedIn", "enviadas": 2, "entrevistas": 2, "propostas": 1, "contratados": 1, "taxaDeEntrevista": 100}, ...],
 "porEtapa": [{"etapa": "interesse", "nome": "Interesse", "total": 1}, ...],
 "porSemana": [{"inicio": "2026-07-20", "total": 0}, ..., {"inicio": "2026-10-05", "total": 2}]}
```

- **Funil pelo histórico, não pela etapa atual:** uma candidatura recusada depois da entrevista conta
  como "chegou à entrevista". Cada uma vale pela etapa mais adiantada que já atingiu, e pular uma
  etapa (ir direto para a entrevista) conta como ter passado pela triagem. Taxa = sobre as enviadas.
  Uma candidatura cadastrada já como "recusada" conta como enviada (a empresa só recusa o que
  recebeu); "desisti" não conta (dá para desistir antes de mandar). A ordem das etapas vem de um
  lugar só (`candidaturas.Etapas`) e vai para o SQL como parâmetro: uma etapa nova entra no funil
  sem mexer na consulta.
- **Tempo de resposta:** do primeiro "enviada" até a primeira mudança que veio da empresa. "Desisti"
  é decisão minha e não conta como resposta. Média e **mediana** (uma resposta que levou dois meses
  puxa a média, mas não a mediana). Duas mudanças no mesmo instante ficam na ordem do registro.
- **Fontes iguais sem diferenciar maiúsculas nem espaços nas pontas** ("LinkedIn" e " linkedin " são
  uma só); o nome mostrado é a grafia mais usada.
- **Semanas de segunda a domingo no horário de Brasília**, as 12 últimas, com zero nas vazias (o
  gráfico precisa delas). Conta o primeiro envio de cada candidatura: voltar para "enviada" não
  conta de novo, e uma cadastrada direto na entrevista entra no funil mas não nas semanas.
- Tudo numa transação só de leitura (*repeatable read*): os números saem da mesma "foto" do banco.

### Lembretes

```
Pursuit: 2 candidaturas paradas.

- Nubank (Back-end Go): enviada há 8 dias, sem resposta: vale mandar uma mensagem ao recrutador.
- iFood (Java): proposta há 3 dias: falta responder.
```

| Etapa | Parada há | | Etapa | Parada há |
|---|---|---|---|---|
| Interesse | 10 dias | | Entrevista | 5 dias |
| Enviada | 7 dias | | Técnica | 5 dias |
| Triagem | 5 dias | | Proposta | 2 dias |

- **Uma goroutine** tenta ao subir e depois a cada 15 min, e para junto com o servidor (`context`).
- **Uma mensagem por dia, no máximo,** juntando todas, entre 9h e 21h de Brasília: se a VM estava
  fora às 9h, sai mais tarde no mesmo dia, mas nunca de madrugada. Os dias são do calendário de
  Brasília, não horas corridas (enviada no dia 1º às 18h é lembrada no dia 8 às 9h).
- **Até o limite do Telegram** (4096 caracteres): as mais antigas primeiro; as que não couberem
  ficam para o dia seguinte ("E mais N"). Sem isso, muitas paradas travariam os lembretes.
- **Sem repetir:** cada candidatura é lembrada uma vez por mudança de etapa e, se continuar parada,
  de novo a cada 7 dias. Mudar de etapa recomeça a contagem.
- **Gravado antes de mandar:** os lembretes vão para a tabela `lembrete` numa transação com uma
  trava do Postgres (`pg_advisory_xact_lock`), e a chamada ao Telegram fica fora da transação. Se o
  Telegram falhar, os lembretes são apagados e a próxima tentativa (15 min depois) manda. É
  entrega "pelo menos uma vez": se a mensagem chegar mas a resposta do Telegram não, ela sai de
  novo (uma repetida rara é melhor que um lembrete perdido). O desligamento não corta um envio no meio.
- **O token nunca aparece em erro nem no log:** o erro do `net/http` traz a URL inteira, com o
  token, então só o motivo é repassado. A mensagem vai sem formatação: o que eu digitei numa
  candidatura não vira HTML.
- Os prazos ficam num mapa ao lado das etapas; um teste falha se uma etapa em andamento ficar sem prazo.

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
- **Conta única:** as candidaturas não são separadas por pessoa, então o `definir-senha` recusa um
  segundo nome (uma segunda conta veria tudo). Com o mesmo nome, troca a senha.
- **Senha com argon2id** (parâmetros da OWASP: 19 MiB, 2 passadas), no formato PHC, que guarda os
  parâmetros junto do hash. Comparação em tempo constante. Senha de 12 a 128 caracteres, sem regra
  de "maiúscula e símbolo" (o NIST recomenda tamanho, não complexidade).
- **Nome errado e senha errada respondem igual,** no texto e no tempo: com um nome que não existe,
  a senha é conferida contra um hash de mentira, e o tempo não entrega quais nomes existem.
- **Sessão no banco, não num token assinado:** o cookie leva 32 bytes aleatórios e o banco guarda
  só o sha256 deles. Sair apaga a linha e o cookie deixa de valer na hora; trocar a senha
  (`definir-senha`) apaga todas as sessões. Dura 7 dias e se renova com o uso (no máximo uma
  escrita por hora). Uma goroutine apaga as vencidas de hora em hora e para junto com o servidor.
- **Cookie `HttpOnly`, `SameSite=Strict`, `Secure` e `Path=/api`:** o JavaScript não lê (um XSS não
  rouba a sessão) e outro site não consegue fazer o navegador mandá-lo (proteção contra CSRF).
- **Limite de tentativas por IP** (*token bucket* do `golang.org/x/time/rate`, 10 por minuto). IPv6
  conta pelo `/64`, porque quem tem um costuma ter a faixa inteira. IPs parados são esquecidos.
  Além disso, no máximo 2 conferências de senha ao mesmo tempo: cada uma usa 19 MiB, e muitos
  logins de muitos IPs acabariam com a memória da VM.
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
cmd/pursuit/          main.go (sobe o servidor e desliga com calma) · definir-senha
internal/config/      variáveis de ambiente
internal/banco/       conexão (pgxpool) e migrações (goose) · migracoes/*.sql · consultas/*.sql
internal/banco/bd/    código gerado pelo sqlc (não editar)
internal/candidaturas/ regras: validação, etapas, histórico, transações
internal/contas/      senha (argon2id), sessões, limite de tentativas
internal/numeros/     funil, tempo de resposta, fontes, envios por semana
internal/lembretes/   candidaturas paradas, mensagem do dia, goroutine
internal/telegram/    cliente da API de bots (sem vazar o token)
internal/web/         a interface: static/ (HTML, CSS, JS, fontes) e o servidor com CSP e ETag
internal/web/testes/  testes do JavaScript (node --test)
internal/api/         rotas HTTP, login (cookie), erros (RFC 9457), middlewares
internal/testebanco/  Postgres dos testes (Testcontainers)
```
