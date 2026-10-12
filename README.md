# Pursuit

**Pursuit · acompanhamento das minhas candidaturas a vagas.**

[![CI](https://github.com/Lakes777/pursuit/actions/workflows/ci.yml/badge.svg)](https://github.com/Lakes777/pursuit/actions/workflows/ci.yml)

Uma API para acompanhar as vagas em que me candidatei: empresa, etapa, datas e anotações de cada
entrevista. O objetivo é mostrar o funil (quantas candidaturas passam de cada etapa), avisar pelo
Telegram quando uma candidatura fica parada e, com o [Beacon](https://github.com/Lakes777/beacon),
dizer se a empresa abriu o link do meu portfólio.

É o meu primeiro projeto em **Go**.

**No ar:** https://pursuit.159-112-189-95.sslip.io (uso pessoal, com login)

![Pursuit em uso: a ficha da Stone arrastada de Triagem para Entrevista, com a observação; a ficha da Nubank, cujo link do portfólio já foi aberto 4 vezes; e os números do funil](docs/demo.gif)

*Demonstração com dados de exemplo. O sistema no ar é de uso pessoal e pede login.*

## Fases

- [x] **1. Esqueleto:** Go com o roteador da biblioteca padrão, PostgreSQL com migrações (goose), `/saude`, testes com Testcontainers, CI com lint
- [x] **2. Candidaturas:** cadastro com etapas (interesse, enviada, triagem, entrevista, técnica, proposta, contratado, recusada, desisti) e o histórico de cada mudança
- [x] **3. Login:** conta única com senha (argon2id), sessão por cookie guardada no banco, limite de tentativas; toda rota de `/api` pede login
- [x] **4. Números:** funil por etapa, tempo até a resposta, taxa por fonte (LinkedIn, Gupy...), envios por semana
- [x] **5. Lembretes:** uma goroutine em segundo plano avisa pelo Telegram as candidaturas paradas há dias (uma mensagem por dia, no máximo)
- [x] **6. Beacon:** um link curto por candidatura, que leva ao portfólio, com os cliques na ficha e "Aberto" no quadro
- [x] **7. Interface:** página servida pelo próprio binário, com identidade visual própria ("Fichário"): quadro por etapa com arrastar e soltar, ficha com o histórico, formulário, números com gráficos e lembretes
- [x] **8. Publicação:** imagem Docker só com o binário (~25 MB), numa VM da Oracle, com HTTPS, backup diário e vigiado pelo [Vigil](https://github.com/Lakes777/vigil)

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
| `PURSUIT_BEACON_URL` | (vazio) | Endereço do [Beacon](https://github.com/Lakes777/beacon). Sem ele e sem a chave, os links ficam desligados |
| `PURSUIT_BEACON_CHAVE` | (vazio) | A chave de integração do Beacon (`Integracao__Chave` lá). Os dois vazios desligam; só um, ou uma chave curta, impede a API de subir |
| `PURSUIT_BEACON_DESTINO` | `https://lakes777.github.io` | Para onde os links levam |
| `PURSUIT_PROXY_CONFIAVEL` | (vazio) | De onde o `X-Forwarded-For` vale (a rede do Caddy), faixas separadas por vírgula |
| `PURSUIT_DEMO` | `false` | Modo [demonstração](#demonstração): entrada como visitante, dados de exemplo, Telegram desligado. Só na instância separada |

## Rotas

| Método | Rota | O que faz |
|---|---|---|
| `GET` | `/saude` | `200` com o banco no ar, `503` sem ele (pública) |
| `POST` | `/api/sessao` | Entra: `usuario` e `senha`; grava o cookie da sessão (pública, com limite de tentativas: `429`) |
| `GET` | `/api/sessao` | Quem está logado |
| `DELETE` | `/api/sessao` | Sai: apaga a sessão no banco e o cookie (pública) |
| `POST` | `/api/sessao/visitante` | Só na demonstração: entra na conta de exemplo, sem senha (pública, com o mesmo limite do login); `404` com o modo desligado |
| `GET` | `/api/demo` | `{"demo": true}` ou `false`: se a tela de entrada mostra o botão de visitante (pública) |
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
| `GET` | `/api/beacon` | Se os links do Beacon estão ligados (`ligado`) e o período das contas (`dias`) |
| `POST` | `/api/candidaturas/{id}/beacon` | Cria o link curto da candidatura (ou devolve o que ela já tem); `503` com o Beacon desligado, `502` se ele não responder |
| `GET` | `/api/candidaturas/{id}/beacon/cliques` | Quantas vezes o link foi aberto, por dia (`total`, `porDia`), sem robôs |
| `GET` | `/api/beacon/cliques` | Os cliques de todos os links de uma vez (`porLink`): o quadro marca as abertas |

Todas as rotas de `/api`, menos entrar, sair e as da demonstração, respondem `401` sem login:

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

### Link do portfólio (Beacon)

Na ficha de cada candidatura, **Criar link** pede ao [Beacon](https://github.com/Lakes777/beacon) (o
meu encurtador) um endereço curto que leva ao portfólio. Esse endereço vai no currículo mandado para
aquela vaga: quando alguém abre, a ficha mostra "Aberto 2 vezes · o último em 12/10/2026", e a ficha
no quadro ganha a etiqueta **Aberto**.

- **A chave fica no servidor:** o navegador só fala com o Pursuit, que fala com o Beacon com a chave de
  integração (`Authorization: Bearer`). No Beacon, essa chave só lê e cria links: se vazar, não apaga nada.
- **Código aleatório:** o Beacon sorteia 7 caracteres. Um código com o nome da empresa diria a qualquer
  um que visse o link para onde eu me candidatei.
- **Um link por candidatura, que não muda:** ele pode já estar num currículo enviado. Pedir de novo
  devolve o mesmo, sem chamar o Beacon.
- **O Beacon fora da transação,** como o Telegram: o pedido a ele não segura a linha travada. Em troca,
  dois cliques ao mesmo tempo podem criar dois links lá; o banco guarda só o primeiro
  (`update ... where beacon_codigo is null`), e todos os pedidos devolvem esse. O outro sobra no Beacon,
  sem uso (a chave não apaga, de propósito). Um teste dispara 10 pedidos juntos.
- **Robôs fora:** a prévia do LinkedIn ou do WhatsApp "abre" o link sozinha; o Beacon conta à parte, e
  aqui só entram pessoas.
- **Sem o Beacon, nada quebra:** desligado, a ficha não mostra a parte do link; fora do ar, a ficha mostra
  o link e avisa que não deu para contar, e o quadro só não marca as abertas.
- Apagar a candidatura não apaga o link no Beacon: ele pode estar num currículo, e quem clicar continua
  chegando ao portfólio.

Erros no formato da RFC 9457 (`application/problem+json`), com o problema de cada campo:

```json
{"title": "Dados inválidos", "status": 422, "campos": {"empresa": "obrigatório", "link": "use um endereço completo, começando com http:// ou https://"}}
```

`400` para JSON inválido (inclusive campo desconhecido, como um `emrpesa` digitado errado), `404` para
candidatura que não existe, `409` para mudar para a etapa em que ela já está, `413` para corpo acima de 64 KB.

## Demonstração

O sistema no ar é de uso pessoal e pede senha. Para quem quer ver o Pursuit funcionando (um
recrutador, pelo portfólio), há uma **segunda instância** com `PURSUIT_DEMO=true`, em que dá para
entrar sem senha e mexer à vontade em dados de exemplo.

- **Instância separada, banco separado:** outro contêiner, outro banco. A demonstração não é uma
  "conta a mais" no sistema de verdade: as candidaturas não são separadas por pessoa, e o visitante
  veria (e poderia apagar) as minhas. Assim, os dados reais nunca se misturam com os de exemplo, e
  uma falha na demonstração não chega ao banco real. O mesmo binário serve os dois; só a variável muda.
- **Conta "visitante":** na tela de entrada, o botão **Entrar como visitante** (`POST
  /api/sessao/visitante`) abre uma sessão normal, com o mesmo cookie e o mesmo limite de tentativas por
  IP do login. A conta não tem senha utilizável: o hash guardado não é um argon2id válido, então pelo
  formulário ela responde como um nome que não existe. Com o modo desligado, a rota responde `404` e a
  conta nem é criada. Dentro do app, uma faixa no topo avisa que os dados voltam ao original a cada hora.
- **Semeadura:** `pursuit semear-demo` apaga as candidaturas (com o histórico e os lembretes) e as
  sessões do visitante, e põe de volta 9 estágios de exemplo, espalhados por todas as etapas, com
  histórico, fontes, anotações, três paradas além do prazo dos lembretes e três com link do Beacon. As
  datas são **relativas a hoje** no horário de Brasília ("enviada há 11 dias, às 13h30"): os exemplos
  nunca envelhecem nem caem no futuro. Tudo numa transação, com uma trava do Postgres: quem está
  olhando vê os dados antigos ou os novos, nunca o banco vazio. Os ids recomeçam do 1, então o link
  de uma ficha continua o mesmo.
- **Proteção do banco real:** `semear-demo` sem `PURSUIT_DEMO=true` recusa (saída 1) antes mesmo de
  conectar ao banco, porque ele começa apagando tudo.
- **Primeira partida:** com o modo ligado e o banco sem candidaturas, a API semeia sozinha ao subir.
- **De hora em hora:** um cron na VM repõe os dados (o `-T` porque o cron não tem terminal):

  ```
  0 * * * * cd ~/pursuit-demo && docker compose exec -T api /pursuit semear-demo >> ~/pursuit-demo/semear.log 2>&1
  ```

- **Telegram desligado na demonstração,** mesmo com token: o visitante não pode fazer o bot mandar
  mensagens para mim. A tela de lembretes continua mostrando o que seria lembrado.
- **Beacon opcional,** como no sistema de verdade: a demonstração do Pursuit aponta para a demonstração
  do Beacon. Os links de exemplo são códigos inventados.
- `definir-senha` continua funcionando (para eu conferir a instância com a minha conta); o visitante não
  conta como "outra conta" ali, mas não pode receber senha.

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

## Publicação

Roda numa VM ARM grátis da Oracle Cloud (1 núcleo Ampere, 6 GB), a mesma do
[Beacon](https://github.com/Lakes777/beacon). O Pursuit **não sobe outro Postgres nem outro Caddy**: usa
os do Beacon, com banco e usuário próprios. Até 12/10/2026 rodava na VM x86 de 1 GB do
[Sidekick](https://github.com/Lakes777/sidekick) e do [Vigil](https://github.com/Lakes777/vigil), usando os dele;
o endereço antigo (`pursuit.147-15-40-173.sslip.io`) redireciona para o novo.

| Peça | Como |
|---|---|
| Imagem | Duas etapas (`Dockerfile`): compila com `golang:1.27-alpine` e leva só o binário estático para uma `distroless/static` sem root, sem shell. **Montada no PC x86 para ARM sem emulação**: a etapa do Go roda na arquitetura do PC (`--platform=$BUILDPLATFORM`) e gera o binário para a da VM (`GOARCH=$TARGETARCH`) |
| Memória | Contêiner limitado a 64 MB e `GOMEMLIMIT=48MiB`; medido: ~23 MB. Se passar, só ele reinicia |
| Banco | No Postgres do Beacon, com banco e usuário `pursuit` próprios; o banco do Beacon passou a aceitar só o usuário dele (`revoke connect ... from public`). Até 3 conexões |
| HTTPS | O Caddy do Beacon importa `~/beacon/sites/*.caddy`; o `publicar.sh` põe lá o `pursuit.caddy` e recarrega o Caddy sem derrubar o Beacon |
| IP real | `PURSUIT_PROXY_CONFIAVEL` = a rede interna do Beacon (`172.30.0.0/24`): só dela o `X-Forwarded-For` vale |
| Saúde | `pursuit saude` (o healthcheck do Docker: a imagem não tem curl) e o Vigil confere `/saude` a cada 5 min |
| Fusos | `time/tzdata` embutido: a imagem não tem o banco de fusos do sistema |
| Backup | `backup.sh` no cron da VM às 3h20 (7 dias) e `trazer-backup.sh` no Agendador de Tarefas do Windows ("Pursuit - trazer backup", 30 dias no PC) |

```bash
deploy/preparar-vm.sh    # uma vez: cria banco, usuário e o .env (senha gerada na VM; Telegram vem do .env do Vigil)
deploy/publicar.sh       # monta a imagem ARM (exige git limpo; leva o commit), manda pela SSH e reinicia
ssh -t ... 'cd pursuit && docker compose exec api /pursuit definir-senha andre'   # a conta
ssh ... 'cd pursuit && docker compose exec api /pursuit telegram-teste'           # testa o Telegram
```

**Como foi a mudança de VM:** o Pursuit parado na VM antiga (ninguém grava no meio), `pg_dump --no-owner`
do banco, cópia no PC, restauração na VM nova como o usuário `pursuit`, contagem de linhas de cada tabela
conferida nos dois lados, publicação e, só então, o endereço antigo virou um redirecionamento.

## Organização

```
cmd/pursuit/          main.go (sobe o servidor e desliga com calma) · definir-senha · saude · semear-demo
deploy/               compose.yaml · pursuit.caddy · preparar-vm.sh · publicar.sh · backup.sh · trazer-backup.sh
internal/config/      variáveis de ambiente
internal/banco/       conexão (pgxpool) e migrações (goose) · migracoes/*.sql · consultas/*.sql
internal/banco/bd/    código gerado pelo sqlc (não editar)
internal/candidaturas/ regras: validação, etapas, histórico, transações
internal/contas/      senha (argon2id), sessões, limite de tentativas, conta do visitante
internal/demo/        os dados de exemplo da demonstração (semeadura)
internal/numeros/     funil, tempo de resposta, fontes, envios por semana
internal/lembretes/   candidaturas paradas, mensagem do dia, goroutine
internal/telegram/    cliente da API de bots (sem vazar o token)
internal/beacon/      cliente do Beacon: cria o link e lê os cliques (a chave só no cabeçalho)
internal/web/         a interface: static/ (HTML, CSS, JS, fontes) e o servidor com CSP e ETag
internal/web/testes/  testes do JavaScript (node --test)
internal/api/         rotas HTTP, login (cookie), erros (RFC 9457), middlewares
internal/testebanco/  Postgres dos testes (Testcontainers)
```
