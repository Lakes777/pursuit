// Package numeros calcula o painel de números das candidaturas: funil, tempo de resposta,
// fontes e ritmo de envio. As contas são feitas no banco, numa transação só de leitura.
package numeros

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Lakes777/pursuit/internal/banco/bd"
	"github.com/Lakes777/pursuit/internal/candidaturas"
)

// Semanas mostradas no ritmo de envio, contando a atual.
const Semanas = 12

// Numeros é o que GET /api/numeros devolve.
type Numeros struct {
	Total       int64            `json:"total"`
	EmAndamento int64            `json:"emAndamento"`
	PorEtapa    []EtapaComTotal  `json:"porEtapa"`
	Funil       []PassoDoFunil   `json:"funil"`
	Respostas   Respostas        `json:"respostas"`
	PorFonte    []Fonte          `json:"porFonte"`
	PorSemana   []SemanaDeEnvios `json:"porSemana"`
}

// EtapaComTotal: quantas candidaturas estão nessa etapa agora.
type EtapaComTotal struct {
	Etapa candidaturas.Etapa `json:"etapa"`
	Nome  string             `json:"nome"`
	Total int64              `json:"total"`
}

// PassoDoFunil: quantas chegaram pelo menos até essa etapa, e a porcentagem sobre as enviadas.
type PassoDoFunil struct {
	Etapa candidaturas.Etapa `json:"etapa"`
	Nome  string             `json:"nome"`
	Total int64              `json:"total"`
	// Taxa em %, com uma casa; null se nenhuma foi enviada
	Taxa *float64 `json:"taxa"`
}

// Respostas: quanto as empresas demoram para responder depois do envio.
type Respostas struct {
	Respondidas int64 `json:"respondidas"`
	// Aguardando: ainda em "enviada", sem nenhuma resposta
	Aguardando int64 `json:"aguardando"`
	// Em dias, com uma casa; null sem nenhuma resposta
	MediaDias   *float64 `json:"mediaDias"`
	MedianaDias *float64 `json:"medianaDias"`
}

// Fonte: de onde vieram as vagas e quanto cada uma rendeu.
type Fonte struct {
	// Fonte: null para as candidaturas sem fonte
	Fonte       *string `json:"fonte"`
	Enviadas    int64   `json:"enviadas"`
	Entrevistas int64   `json:"entrevistas"`
	Propostas   int64   `json:"propostas"`
	Contratados int64   `json:"contratados"`
	// TaxaDeEntrevista: entrevistas / enviadas, em %
	TaxaDeEntrevista float64 `json:"taxaDeEntrevista"`
}

// SemanaDeEnvios: candidaturas enviadas na semana que começa na segunda Inicio.
type SemanaDeEnvios struct {
	Inicio string `json:"inicio"` // AAAA-MM-DD
	Total  int64  `json:"total"`
}

// Servico calcula os números.
type Servico struct {
	pool  *pgxpool.Pool
	agora func() time.Time
}

// NovoServico usa o relógio de verdade.
func NovoServico(pool *pgxpool.Pool) *Servico {
	return &Servico{pool: pool, agora: time.Now}
}

// Calcular faz todas as consultas numa transação só de leitura (repeatable read): os números
// saem de uma mesma "foto" do banco, mesmo com uma mudança de etapa no meio.
func (s *Servico) Calcular(ctx context.Context) (Numeros, error) {
	var n Numeros
	opcoes := pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}
	err := pgx.BeginTxFunc(ctx, s.pool, opcoes, func(tx pgx.Tx) error {
		q := bd.New(tx)
		ordem := niveis()
		funil, err := q.Funil(ctx, bd.FunilParams{Etapas: ordem.etapas, Niveis: ordem.niveis})
		if err != nil {
			return err
		}
		porEtapa, err := q.PorEtapaAtual(ctx)
		if err != nil {
			return err
		}
		fontes, err := q.PorFonte(ctx, bd.PorFonteParams{
			Etapas: ordem.etapas, Niveis: ordem.niveis,
			NivelEnviada: ordem.de(candidaturas.Enviada), NivelEntrevista: ordem.de(candidaturas.Entrevista),
			NivelProposta: ordem.de(candidaturas.Proposta), NivelContratado: ordem.de(candidaturas.Contratado),
		})
		if err != nil {
			return err
		}
		tempos, err := q.TempoDeResposta(ctx)
		if err != nil {
			return err
		}
		primeira := s.primeiraSegunda()
		semanas, err := q.EnviadasPorSemana(ctx, primeira)
		if err != nil {
			return err
		}
		n = montar(ordem, funil, porEtapa, fontes, tempos, semanas, primeira)
		return nil
	})
	return n, err
}

// primeiraSegunda: a segunda-feira 0h (Brasília) de Semanas-1 semanas atrás.
func (s *Servico) primeiraSegunda() time.Time {
	hoje := s.agora().In(brasilia)
	diasDesdeSegunda := (int(hoje.Weekday()) + 6) % 7
	segunda := time.Date(hoje.Year(), hoje.Month(), hoje.Day()-diasDesdeSegunda, 0, 0, 0, 0, brasilia)
	return segunda.AddDate(0, 0, -7*(Semanas-1))
}

// ordemDoProcesso: o nível de cada etapa no funil (quanto maior, mais adiantada).
type ordemDoProcesso struct {
	etapas []string
	niveis []int32
}

func (o ordemDoProcesso) de(etapa candidaturas.Etapa) int32 {
	for i, e := range o.etapas {
		if e == string(etapa) {
			return o.niveis[i]
		}
	}
	return -1
}

// niveis monta a ordem a partir de candidaturas.Etapas, o único lugar onde ela está escrita:
// as etapas não finais na ordem da lista (interesse = 0, enviada = 1...), depois contratado, o
// fim do caminho. Recusada vale o mesmo que enviada: a empresa só recusa o que recebeu (quem
// passou de etapa antes de ser recusado continua com o nível maior). Desisti fica de fora: dá
// para desistir antes de mandar.
func niveis() ordemDoProcesso {
	var o ordemDoProcesso
	var nivel int32
	for _, info := range candidaturas.Etapas {
		if !info.Final {
			o.etapas = append(o.etapas, string(info.Etapa))
			o.niveis = append(o.niveis, nivel)
			nivel++
		}
	}
	o.etapas = append(o.etapas, string(candidaturas.Contratado), string(candidaturas.Recusada))
	o.niveis = append(o.niveis, nivel, o.de(candidaturas.Enviada))
	return o
}

func montar(ordem ordemDoProcesso, funil []bd.FunilRow, porEtapa []bd.PorEtapaAtualRow, fontes []bd.PorFonteRow,
	tempos bd.TempoDeRespostaRow, semanas []bd.EnviadasPorSemanaRow, primeira time.Time) Numeros {
	var n Numeros

	atuais := map[candidaturas.Etapa]int64{}
	for _, linha := range porEtapa {
		atuais[candidaturas.Etapa(linha.Etapa)] = linha.Total
		n.Total += linha.Total
	}
	for _, info := range candidaturas.Etapas {
		n.PorEtapa = append(n.PorEtapa, EtapaComTotal{info.Etapa, info.Nome, atuais[info.Etapa]})
		if !info.Final {
			n.EmAndamento += atuais[info.Etapa]
		}
	}

	// Os passos do funil: de "enviada" até "contratado" (as etapas com nível próprio, sem a recusada)
	porNivel := map[int32]int64{}
	for _, linha := range funil {
		porNivel[linha.Nivel] = linha.Total
	}
	enviadas := porNivel[ordem.de(candidaturas.Enviada)]
	for i, etapa := range ordem.etapas {
		nivel := ordem.niveis[i]
		if nivel < ordem.de(candidaturas.Enviada) || etapa == string(candidaturas.Recusada) {
			continue
		}
		passo := PassoDoFunil{Etapa: candidaturas.Etapa(etapa), Nome: nomeDa(candidaturas.Etapa(etapa)), Total: porNivel[nivel]}
		if enviadas > 0 {
			taxa := porcentagem(passo.Total, enviadas)
			passo.Taxa = &taxa
		}
		n.Funil = append(n.Funil, passo)
	}

	n.Respostas = Respostas{Respondidas: tempos.Respondidas, Aguardando: tempos.Aguardando}
	if tempos.Respondidas > 0 {
		media, mediana := umaCasa(tempos.MediaDias), umaCasa(tempos.MedianaDias)
		n.Respostas.MediaDias, n.Respostas.MedianaDias = &media, &mediana
	}

	n.PorFonte = make([]Fonte, 0, len(fontes))
	for _, f := range fontes {
		fonte := Fonte{Enviadas: f.Enviadas, Entrevistas: f.Entrevistas, Propostas: f.Propostas,
			Contratados: f.Contratados, TaxaDeEntrevista: porcentagem(f.Entrevistas, f.Enviadas)}
		if f.Fonte != "" {
			nome := f.Fonte
			fonte.Fonte = &nome
		}
		n.PorFonte = append(n.PorFonte, fonte)
	}

	// Todas as semanas aparecem, inclusive as sem nenhum envio (o gráfico precisa do zero)
	porInicio := map[string]int64{}
	for _, s := range semanas {
		porInicio[s.Semana.Format(time.DateOnly)] = s.Total
	}
	for i := range Semanas {
		inicio := primeira.AddDate(0, 0, 7*i).Format(time.DateOnly)
		n.PorSemana = append(n.PorSemana, SemanaDeEnvios{Inicio: inicio, Total: porInicio[inicio]})
	}
	return n
}

func nomeDa(etapa candidaturas.Etapa) string {
	for _, info := range candidaturas.Etapas {
		if info.Etapa == etapa {
			return info.Nome
		}
	}
	return string(etapa)
}

// porcentagem com uma casa: 1 de 3 = 33.3.
func porcentagem(parte, total int64) float64 {
	if total == 0 {
		return 0
	}
	return umaCasa(float64(parte) * 100 / float64(total))
}

func umaCasa(v float64) float64 {
	return math.Round(v*10) / 10
}

var brasilia = func() *time.Location {
	local, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.FixedZone("BRT", -3*60*60)
	}
	return local
}()
