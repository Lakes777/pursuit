package lembretes

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Lakes777/pursuit/internal/banco/bd"
	"github.com/Lakes777/pursuit/internal/candidaturas"
)

// Janela do dia em que os lembretes podem sair (horário de Brasília): se a VM estava fora às 9h,
// sai mais tarde no mesmo dia, mas nunca de madrugada.
const (
	HoraInicio = 9
	HoraFim    = 21
)

// Enviador é o que manda a mensagem (o cliente do Telegram; nos testes, um falso).
type Enviador interface {
	Enviar(ctx context.Context, texto string) error
}

// Servico de lembretes.
type Servico struct {
	pool     *pgxpool.Pool
	enviador Enviador
	agora    func() time.Time
}

// NovoServico usa o relógio de verdade. enviador nil: só a prévia funciona (sem Telegram).
func NovoServico(pool *pgxpool.Pool, enviador Enviador) *Servico {
	return &Servico{pool: pool, enviador: enviador, agora: time.Now}
}

// Paradas devolve o que seria lembrado agora (a prévia da tela).
func (s *Servico) Paradas(ctx context.Context) ([]Parada, error) {
	return paradas(ctx, bd.New(s.pool), s.agora())
}

func paradas(ctx context.Context, q *bd.Queries, agora time.Time) ([]Parada, error) {
	linhas, err := q.UltimaMudancaDeCada(ctx)
	if err != nil {
		return nil, err
	}
	lista := []Parada{}
	for _, l := range linhas {
		etapa := candidaturas.Etapa(l.Etapa)
		dias, lembrar := precisaLembrar(etapa, l.Em, l.UltimoLembrete, agora)
		if !lembrar {
			continue
		}
		lista = append(lista, Parada{CandidaturaID: l.ID, HistoricoID: l.HistoricoID, Empresa: l.Empresa,
			Vaga: l.Vaga, Etapa: etapa, Desde: l.Em, Dias: dias, Texto: textoDaParada(l.Empresa, l.Vaga, etapa, dias)})
	}
	return lista, nil
}

// EnviarSeHora manda a mensagem do dia, se for hora (9h às 21h em Brasília), se ainda não saiu
// nenhuma hoje e se há o que lembrar. Devolve quantas candidaturas foram lembradas.
//
// Os lembretes são gravados antes de mandar, numa transação com uma trava do Postgres (duas
// instâncias não montam a mesma mensagem). Se o Telegram falhar, são apagados, e a próxima
// tentativa manda de novo.
func (s *Servico) EnviarSeHora(ctx context.Context) (int, error) {
	if s.enviador == nil {
		return 0, nil
	}
	agora := s.agora()
	local := agora.In(brasilia)
	if local.Hour() < HoraInicio || local.Hour() >= HoraFim {
		return 0, nil
	}
	inicioDoDia := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, brasilia)

	var lembradas []Parada
	var ids []int64
	var texto string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := bd.New(tx)
		if err := q.TravarLembretes(ctx); err != nil {
			return err
		}
		jaSaiu, err := q.LembreteEnviadoDesde(ctx, inicioDoDia)
		if err != nil || jaSaiu {
			return err
		}
		todas, err := paradas(ctx, q, agora)
		if err != nil {
			return err
		}
		var couberam int
		texto, couberam = mensagem(todas)
		lembradas = todas[:couberam]
		for _, p := range lembradas {
			id, err := q.RegistrarLembrete(ctx, bd.RegistrarLembreteParams{
				CandidaturaID: p.CandidaturaID, HistoricoID: p.HistoricoID, EnviadoEm: agora,
			})
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return nil
	})
	if err != nil || len(lembradas) == 0 {
		return 0, err
	}

	// A chamada ao Telegram fica fora da transação: esperar a rede não segura uma conexão do banco.
	// WithoutCancel: o desligamento não corta um envio no meio (o cliente tem limite de 10 s).
	// Se a mensagem chegar mas a resposta não, ela é desfeita e sai de novo depois: entrega
	// "pelo menos uma vez" (uma repetida rara é melhor que um lembrete perdido).
	if err := s.enviador.Enviar(context.WithoutCancel(ctx), texto); err != nil {
		// Desfaz a marca para não perder o lembrete, com prazo: um banco inacessível não pode
		// travar o desligamento
		ctxDesfazer, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancelar()
		if errDesfazer := bd.New(s.pool).DesfazerLembretes(ctxDesfazer, ids); errDesfazer != nil {
			return 0, errors.Join(err, fmt.Errorf(
				"não deu para desfazer a marca: %d candidatura(s) só voltam a ser lembradas em %d dias: %w",
				len(ids), RepetirDias, errDesfazer))
		}
		return 0, err
	}
	return len(lembradas), nil
}

// Rodar tenta mandar logo ao subir e depois a cada intervalo, até o ctx ser cancelado.
func (s *Servico) Rodar(ctx context.Context, cada time.Duration, log *slog.Logger) {
	relogio := time.NewTicker(cada)
	defer relogio.Stop()
	for {
		if n, err := s.EnviarSeHora(ctx); err != nil && ctx.Err() == nil {
			log.Warn("lembretes: não foi possível mandar", "erro", err)
		} else if n > 0 {
			log.Info("lembretes mandados", "candidaturas", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-relogio.C:
		}
	}
}

var brasilia = func() *time.Location {
	local, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.FixedZone("BRT", -3*60*60)
	}
	return local
}()
