// Package candidaturas tem as regras das candidaturas e do histórico de etapas.
package candidaturas

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Lakes777/pursuit/internal/banco/bd"
)

var (
	// ErrNaoEncontrada: não existe candidatura com esse id.
	ErrNaoEncontrada = errors.New("candidatura não encontrada")
	// ErrMesmaEtapa: pediu para mudar para a etapa em que ela já está.
	ErrMesmaEtapa = errors.New("a candidatura já está nessa etapa")
)

// Candidatura é o que a API devolve.
type Candidatura struct {
	ID int64 `json:"id"`
	Dados
	Etapa        Etapa     `json:"etapa"`
	CriadaEm     time.Time `json:"criadaEm"`
	AtualizadaEm time.Time `json:"atualizadaEm"`
}

// Mudanca é uma linha do histórico de etapas. De é nil na primeira.
type Mudanca struct {
	De         *Etapa    `json:"de"`
	Para       Etapa     `json:"para"`
	Em         time.Time `json:"em"`
	Observacao *string   `json:"observacao"`
}

// Detalhe é a candidatura com o histórico, da mudança mais antiga para a mais nova.
type Detalhe struct {
	Candidatura
	Historico []Mudanca `json:"historico"`
}

// NovaEtapa é o pedido de mudança de etapa (também a etapa inicial, no cadastro).
type NovaEtapa struct {
	Etapa      Etapa      `json:"etapa"`
	Observacao *string    `json:"observacao"`
	Em         *time.Time `json:"em"` // nil = agora; pode ser no passado
}

// Filtro da listagem; campos vazios não filtram.
type Filtro struct {
	Etapa Etapa
	Busca string
}

// Servico guarda e consulta candidaturas.
type Servico struct {
	pool  *pgxpool.Pool
	agora func() time.Time
}

// NovoServico usa o relógio de verdade.
func NovoServico(pool *pgxpool.Pool) *Servico {
	return &Servico{pool: pool, agora: time.Now}
}

// Criar cadastra a candidatura e registra a primeira etapa (Interesse, se não vier outra),
// numa transação só: ou fica tudo, ou nada.
func (s *Servico) Criar(ctx context.Context, dados Dados, inicial NovaEtapa) (Detalhe, error) {
	dados, err := dados.normalizar()
	if err != nil {
		return Detalhe{}, err
	}
	if inicial.Etapa == "" {
		inicial.Etapa = Interesse
	}
	em, observacao, err := s.conferirEtapa(inicial, time.Time{})
	if err != nil {
		return Detalhe{}, err
	}
	var detalhe Detalhe
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := bd.New(tx)
		linha, err := q.CriarCandidatura(ctx, bd.CriarCandidaturaParams{
			Empresa: dados.Empresa, Vaga: dados.Vaga, Link: dados.Link, Fonte: dados.Fonte,
			Modalidade: dados.Modalidade, Salario: dados.Salario, Anotacoes: dados.Anotacoes,
			Etapa: string(inicial.Etapa),
		})
		if err != nil {
			return err
		}
		if _, err := q.RegistrarEtapa(ctx, bd.RegistrarEtapaParams{
			CandidaturaID: linha.ID, Para: string(inicial.Etapa), Em: em, Observacao: observacao,
		}); err != nil {
			return err
		}
		detalhe, err = detalhar(ctx, q, linha)
		return err
	})
	return detalhe, err
}

// Listar devolve as candidaturas, da mexida mais recentemente para a mais antiga.
func (s *Servico) Listar(ctx context.Context, filtro Filtro) ([]Candidatura, error) {
	var erros ErroDeValidacao
	params := bd.ListarCandidaturasParams{}
	if filtro.Etapa != "" {
		if !filtro.Etapa.Valida() {
			erros.adicionar("etapa", "etapa desconhecida")
			return nil, &erros
		}
		etapa := string(filtro.Etapa)
		params.Etapa = &etapa
	}
	// Sem o NUL, que o Postgres recusa (a busca não é um dado guardado, então só some)
	if busca := strings.TrimSpace(strings.ReplaceAll(filtro.Busca, "\x00", "")); busca != "" {
		busca = escaparLike(busca)
		params.Busca = &busca
	}
	linhas, err := bd.New(s.pool).ListarCandidaturas(ctx, params)
	if err != nil {
		return nil, err
	}
	lista := make([]Candidatura, 0, len(linhas))
	for _, linha := range linhas {
		lista = append(lista, converter(linha))
	}
	return lista, nil
}

// Buscar devolve a candidatura com o histórico. As duas leituras ficam numa transação só de
// leitura (repeatable read): uma mudança de etapa no meio não deixa a etapa e o histórico
// desencontrados.
func (s *Servico) Buscar(ctx context.Context, id int64) (Detalhe, error) {
	var detalhe Detalhe
	opcoes := pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}
	err := pgx.BeginTxFunc(ctx, s.pool, opcoes, func(tx pgx.Tx) error {
		q := bd.New(tx)
		linha, err := q.BuscarCandidatura(ctx, id)
		if err != nil {
			return naoEncontrada(err)
		}
		detalhe, err = detalhar(ctx, q, linha)
		return err
	})
	return detalhe, err
}

// Editar troca os dados (não a etapa: essa muda por MudarEtapa, que guarda o histórico).
func (s *Servico) Editar(ctx context.Context, id int64, dados Dados) (Candidatura, error) {
	dados, err := dados.normalizar()
	if err != nil {
		return Candidatura{}, err
	}
	linha, err := bd.New(s.pool).EditarCandidatura(ctx, bd.EditarCandidaturaParams{
		ID: id, Empresa: dados.Empresa, Vaga: dados.Vaga, Link: dados.Link, Fonte: dados.Fonte,
		Modalidade: dados.Modalidade, Salario: dados.Salario, Anotacoes: dados.Anotacoes,
	})
	if err != nil {
		return Candidatura{}, naoEncontrada(err)
	}
	return converter(linha), nil
}

// Apagar remove a candidatura e o histórico dela.
func (s *Servico) Apagar(ctx context.Context, id int64) error {
	apagadas, err := bd.New(s.pool).ApagarCandidatura(ctx, id)
	if err != nil {
		return err
	}
	if apagadas == 0 {
		return ErrNaoEncontrada
	}
	return nil
}

// MudarEtapa registra a nova etapa no histórico. A linha fica travada durante a transação:
// duas mudanças ao mesmo tempo acontecem uma depois da outra, e a segunda vê a primeira.
func (s *Servico) MudarEtapa(ctx context.Context, id int64, nova NovaEtapa) (Detalhe, error) {
	var detalhe Detalhe
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := bd.New(tx)
		atual, err := q.TravarCandidatura(ctx, id)
		if err != nil {
			return naoEncontrada(err)
		}
		ultima, err := q.UltimaMudanca(ctx, id)
		if err != nil {
			return err
		}
		em, observacao, err := s.conferirEtapa(nova, ultima)
		if err != nil {
			return err
		}
		if Etapa(atual.Etapa) == nova.Etapa {
			return ErrMesmaEtapa
		}
		if _, err := q.RegistrarEtapa(ctx, bd.RegistrarEtapaParams{
			CandidaturaID: id, De: &atual.Etapa, Para: string(nova.Etapa), Em: em, Observacao: observacao,
		}); err != nil {
			return err
		}
		linha, err := q.MudarEtapa(ctx, bd.MudarEtapaParams{ID: id, Etapa: string(nova.Etapa)})
		if err != nil {
			return err
		}
		detalhe, err = detalhar(ctx, q, linha)
		return err
	})
	return detalhe, err
}

// conferirEtapa valida a etapa, a data (padrão: agora) e a observação.
func (s *Servico) conferirEtapa(nova NovaEtapa, desde time.Time) (time.Time, *string, error) {
	var erros ErroDeValidacao
	if !nova.Etapa.Valida() {
		erros.adicionar("etapa", "etapa desconhecida")
	}
	observacao := opcional(&erros, "observacao", nova.Observacao, 2000)
	if err := erros.ouNil(); err != nil {
		return time.Time{}, nil, err
	}
	agora := s.agora()
	em := agora
	if nova.Em != nil {
		em = *nova.Em
		if err := conferirData(em, agora, desde); err != nil {
			return time.Time{}, nil, err
		}
	} else if em.Before(desde) {
		// A última mudança foi registrada com uma data "agora" adiantada (dentro da folga)
		em = desde
	}
	return em.UTC(), observacao, nil
}

func detalhar(ctx context.Context, q *bd.Queries, linha bd.Candidatura) (Detalhe, error) {
	historico, err := q.HistoricoDaCandidatura(ctx, linha.ID)
	if err != nil {
		return Detalhe{}, err
	}
	detalhe := Detalhe{Candidatura: converter(linha), Historico: make([]Mudanca, 0, len(historico))}
	for _, h := range historico {
		mudanca := Mudanca{Para: Etapa(h.Para), Em: h.Em, Observacao: h.Observacao}
		if h.De != nil {
			de := Etapa(*h.De)
			mudanca.De = &de
		}
		detalhe.Historico = append(detalhe.Historico, mudanca)
	}
	return detalhe, nil
}

func converter(linha bd.Candidatura) Candidatura {
	return Candidatura{
		ID: linha.ID,
		Dados: Dados{
			Empresa: linha.Empresa, Vaga: linha.Vaga, Link: linha.Link, Fonte: linha.Fonte,
			Modalidade: linha.Modalidade, Salario: linha.Salario, Anotacoes: linha.Anotacoes,
		},
		Etapa:        Etapa(linha.Etapa),
		CriadaEm:     linha.CriadaEm,
		AtualizadaEm: linha.AtualizadaEm,
	}
}

func naoEncontrada(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNaoEncontrada
	}
	return err
}

// escaparLike faz %, _ e \ valerem como texto no ilike ("100%" busca "100%", não "100...").
func escaparLike(texto string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(texto)
}
