// Package demo semeia os dados de exemplo do modo demonstração (PURSUIT_DEMO=true): uma
// instância separada, com banco próprio, em que qualquer pessoa entra como "visitante" e mexe à
// vontade. De hora em hora, um cron na VM chama "pursuit semear-demo" e tudo volta ao original.
package demo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Lakes777/pursuit/internal/banco/bd"
	"github.com/Lakes777/pursuit/internal/candidaturas"
	"github.com/Lakes777/pursuit/internal/contas"
)

// brasilia: as datas dos exemplos são "há N dias, às 14h" no horário de Brasília, como as que eu
// registraria de verdade (os lembretes e as semanas dos números também contam por esse fuso).
var brasilia = func() *time.Location {
	local, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic(err) // o binário leva time/tzdata; sem o fuso, é erro de montagem
	}
	return local
}()

// Semear apaga as candidaturas (com o histórico e os lembretes) e as sessões do visitante e
// insere os exemplos, com datas relativas a agora. Tudo numa transação: quem estiver olhando
// vê os dados antigos ou os novos, nunca o banco vazio no meio.
func Semear(ctx context.Context, pool *pgxpool.Pool, agora time.Time) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := bd.New(tx)
		if err := q.TravarDemo(ctx); err != nil {
			return err
		}
		return semear(ctx, q, agora)
	})
}

// SemearSeVazio semeia só se o banco não tem nenhuma candidatura (a primeira partida da
// instância de demonstração). Devolve se semeou.
func SemearSeVazio(ctx context.Context, pool *pgxpool.Pool, agora time.Time) (bool, error) {
	semeou := false
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := bd.New(tx)
		// A trava vem antes da contagem: duas partidas juntas não semeiam duas vezes
		if err := q.TravarDemo(ctx); err != nil {
			return err
		}
		total, err := q.ContarCandidaturas(ctx)
		if err != nil || total > 0 {
			return err
		}
		semeou = true
		return semear(ctx, q, agora)
	})
	return semeou && err == nil, err
}

func semear(ctx context.Context, q *bd.Queries, agora time.Time) error {
	if err := q.ApagarCandidaturas(ctx); err != nil {
		return err
	}
	if err := contas.PrepararVisitante(ctx, q); err != nil {
		return err
	}
	for _, ex := range Exemplos {
		passos := ex.Passos
		primeira := quando(passos[0], agora)
		ultima := quando(passos[len(passos)-1], agora)
		id, err := q.InserirCandidaturaDemo(ctx, bd.InserirCandidaturaDemoParams{
			Empresa: ex.Empresa, Vaga: ex.Vaga, Link: opcional(ex.Link), Fonte: opcional(ex.Fonte),
			Modalidade: opcional(ex.Modalidade), Salario: opcional(ex.Salario), Anotacoes: opcional(ex.Anotacoes),
			Etapa: string(passos[len(passos)-1].Etapa), CriadaEm: primeira, AtualizadaEm: ultima,
			BeaconCodigo: opcional(ex.Beacon), BeaconUrl: urlDoBeacon(ex.Beacon),
		})
		if err != nil {
			return err
		}
		var de *string
		for _, p := range passos {
			if _, err := q.RegistrarEtapa(ctx, bd.RegistrarEtapaParams{
				CandidaturaID: id, De: de, Para: string(p.Etapa), Em: quando(p, agora), Observacao: opcional(p.Observacao),
			}); err != nil {
				return err
			}
			para := string(p.Etapa)
			de = &para
		}
	}
	return nil
}

// quando: "há DiasAtras dias, às Hora:Minuto" em Brasília. Pelo menos um dia atrás, então nunca
// cai no futuro (o histórico real também não aceita datas futuras).
func quando(p Passo, agora time.Time) time.Time {
	hoje := agora.In(brasilia)
	return time.Date(hoje.Year(), hoje.Month(), hoje.Day()-p.DiasAtras, p.Hora, p.Minuto, 0, 0, brasilia)
}

// DominioDoBeacon: os links de exemplo apontam para a demonstração do Beacon, que semeia estes mesmos
// códigos com alguns cliques (a ficha mostra "Aberto N vezes"). Nenhum link real aparece aqui.
// Os códigos seguem as regras do Beacon: 7 caracteres, minúsculas, sem l, o, 0 e 1.
const DominioDoBeacon = "https://demo-beacon.159-112-189-95.sslip.io/r/"

func urlDoBeacon(codigo string) *string {
	if codigo == "" {
		return nil
	}
	url := DominioDoBeacon + codigo
	return &url
}

func opcional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Exemplo é uma candidatura de exemplo, com as etapas por que passou (da mais antiga à mais nova).
type Exemplo struct {
	Empresa, Vaga, Link, Fonte, Modalidade, Salario, Anotacoes string
	Beacon                                                     string // código do link curto ("" = sem link)
	Passos                                                     []Passo
}

// Passo é uma mudança de etapa: há quantos dias, a que horas (Brasília) e a observação.
type Passo struct {
	Etapa      candidaturas.Etapa
	DiasAtras  int
	Hora       int
	Minuto     int
	Observacao string
}

// Exemplos: estágios espalhados por todas as etapas, com algumas paradas além do prazo dos
// lembretes (Stone na triagem há 6 dias, PicPay enviada há 11, iFood com a proposta há 3), para
// a tela de lembretes e as etiquetas "parada" do quadro terem o que mostrar.
var Exemplos = []Exemplo{
	{
		Empresa: "Nubank", Vaga: "Estágio em Desenvolvimento Back-end (Go)", Fonte: "LinkedIn",
		Modalidade: "remoto", Salario: "R$ 2.400", Beacon: "k7qm2xz",
		Anotacoes: "Stack: Go, Kafka, PostgreSQL. Pediram um projeto pessoal em Go: mandei o Pursuit.",
		Passos: []Passo{
			{candidaturas.Enviada, 21, 10, 15, "Candidatura pelo LinkedIn, com o currículo e o link do portfólio."},
			{candidaturas.Triagem, 16, 14, 40, "A recrutadora pediu a disponibilidade de horário."},
			{candidaturas.Entrevista, 10, 16, 0, "Conversa de 40 min com o RH: cultura e por que Go."},
			{candidaturas.Tecnica, 3, 11, 30, "Desafio: uma API de transferências em Go, prazo de 5 dias."},
		},
	},
	{
		Empresa: "Stone", Vaga: "Estágio em Engenharia de Software", Fonte: "Gupy",
		Modalidade: "hibrido", Salario: "R$ 2.100",
		Passos: []Passo{
			{candidaturas.Enviada, 14, 9, 20, ""},
			{candidaturas.Triagem, 6, 15, 5, "Teste de lógica na Gupy feito; aguardando o resultado."},
		},
	},
	{
		Empresa: "iFood", Vaga: "Estágio Back-end Java", Link: "https://carreiras.ifood.com.br", Fonte: "Site da empresa",
		Modalidade: "remoto", Salario: "R$ 2.500 + VR", Beacon: "rt4pw9a",
		Anotacoes: "Início previsto em novembro. Perguntar sobre a efetivação.",
		Passos: []Passo{
			{candidaturas.Interesse, 32, 20, 10, ""},
			{candidaturas.Enviada, 27, 9, 0, ""},
			{candidaturas.Triagem, 22, 11, 45, ""},
			{candidaturas.Entrevista, 17, 14, 0, "Entrevista com a gestora do time de pagamentos."},
			{candidaturas.Tecnica, 11, 10, 0, "Pair programming de 1 h: refatorar um serviço em Java."},
			{candidaturas.Proposta, 3, 17, 20, "Proposta por e-mail: R$ 2.500 + VR, 6 h por dia."},
		},
	},
	{
		Empresa: "PicPay", Vaga: "Estágio em Desenvolvimento", Fonte: "LinkedIn", Modalidade: "presencial",
		Anotacoes: "Escritório em São Paulo. Vale mandar mensagem ao recrutador que publicou a vaga.",
		Passos: []Passo{
			{candidaturas.Enviada, 11, 13, 30, ""},
		},
	},
	{
		Empresa: "Mercado Livre", Vaga: "Estágio em TI", Fonte: "Gupy", Modalidade: "hibrido",
		Passos: []Passo{
			{candidaturas.Enviada, 30, 8, 50, ""},
			{candidaturas.Triagem, 25, 10, 10, ""},
			{candidaturas.Recusada, 18, 18, 0, "E-mail padrão: seguiram com outros candidatos."},
		},
	},
	{
		Empresa: "Itaú", Vaga: "Estágio em Tecnologia 2027", Fonte: "Site da empresa", Modalidade: "presencial",
		Anotacoes: "Inscrições abertas até o fim do mês.",
		Passos: []Passo{
			{candidaturas.Interesse, 4, 21, 15, ""},
		},
	},
	{
		Empresa: "Totvs", Vaga: "Estágio Desenvolvedor Back-end", Fonte: "Indicação", Modalidade: "hibrido",
		Beacon: "b3nvx8e", Anotacoes: "Indicação de um colega da faculdade, que está no time de RH tech.",
		Passos: []Passo{
			{candidaturas.Enviada, 9, 11, 0, "Currículo mandado pelo colega que indicou."},
			{candidaturas.Triagem, 7, 16, 30, ""},
			{candidaturas.Entrevista, 2, 10, 0, "Entrevista com o gestor marcada; levar o portfólio aberto."},
		},
	},
	{
		Empresa: "CI&T", Vaga: "Estágio em Desenvolvimento Web", Fonte: "Site da empresa", Modalidade: "remoto",
		Passos: []Passo{
			{candidaturas.Enviada, 5, 19, 40, ""},
		},
	},
	{
		Empresa: "Banco Inter", Vaga: "Estágio Back-end", Fonte: "LinkedIn", Modalidade: "hibrido",
		Passos: []Passo{
			{candidaturas.Enviada, 26, 9, 30, ""},
			{candidaturas.Triagem, 21, 15, 0, ""},
			{candidaturas.Entrevista, 16, 14, 30, ""},
			{candidaturas.Recusada, 10, 12, 0, "Retorno: procuram alguém com mais experiência em Kotlin."},
		},
	},
}
