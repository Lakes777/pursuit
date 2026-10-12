package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Lakes777/pursuit/internal/config"
	"github.com/Lakes777/pursuit/internal/demo"
	"github.com/Lakes777/pursuit/internal/lembretes"
	"github.com/Lakes777/pursuit/internal/telegram"
)

// erroDeRecusa: o comando existe, mas não pode rodar aqui; o programa mostra a mensagem e sai com 1.
type erroDeRecusa string

func (e erroDeRecusa) Error() string { return string(e) }

// semearDemo apaga as candidaturas e põe os exemplos de volta. Só com PURSUIT_DEMO=true: a
// semeadura começa apagando tudo, e rodar no banco de verdade levaria as minhas candidaturas
// junto. O banco nem é tocado na recusa.
func semearDemo(ctx context.Context, demoLigado bool, pool *pgxpool.Pool, saida io.Writer) error {
	if !demoLigado {
		return erroDeRecusa("semear-demo só roda com PURSUIT_DEMO=true: ele apaga todas as candidaturas " +
			"(proteção para não apagar o banco de verdade)")
	}
	if err := demo.Semear(ctx, pool, time.Now()); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(saida, "Demonstração semeada: %d candidaturas de exemplo.\n", len(demo.Exemplos))
	return nil
}

// montarEnviador decide para onde vão os lembretes. Sem token e sem chat, desligados (a prévia
// continua na API). Com só um dos dois, ou com o token mal escrito, a API nem sobe: senão eu
// passaria semanas sem lembrete achando que estava tudo certo. Na demonstração, desligados mesmo
// com token: o visitante não pode fazer o bot mandar mensagens para mim.
func montarEnviador(cfg config.Config, log *slog.Logger) (lembretes.Enviador, error) {
	if cfg.Demo {
		log.Warn("lembretes pelo Telegram desligados no modo demonstração")
		return nil, nil
	}
	if cfg.TelegramToken == "" && cfg.TelegramChat == "" {
		log.Warn("lembretes pelo Telegram desligados (sem PURSUIT_TELEGRAM_TOKEN e PURSUIT_TELEGRAM_CHAT)")
		return nil, nil
	}
	cliente, err := telegram.Novo(cfg.TelegramURL, cfg.TelegramToken, cfg.TelegramChat)
	if err != nil {
		return nil, fmt.Errorf("configuração do Telegram: %w", err)
	}
	return cliente, nil
}
