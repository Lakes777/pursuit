// O Pursuit: acompanha as minhas candidaturas a vagas. Sobe o servidor HTTP e desliga com calma
// ao receber Ctrl+C ou o sinal de parada do Docker (SIGTERM).
//
// Uso: pursuit                        sobe a API
//
//	pursuit definir-senha <nome>   cria a conta ou troca a senha (pergunta a senha escondida)
//	pursuit saude                  confere se a API responde (o healthcheck do Docker)
//	pursuit telegram-teste         manda uma mensagem de teste pelo Telegram configurado
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	// O banco de fusos vai dentro do binário: a imagem de produção não tem o do sistema, e sem
	// ele Brasília viraria um fuso fixo (os lembretes e as semanas dependem dele)
	_ "time/tzdata"

	"github.com/Lakes777/pursuit/internal/api"
	"github.com/Lakes777/pursuit/internal/banco"
	"github.com/Lakes777/pursuit/internal/candidaturas"
	"github.com/Lakes777/pursuit/internal/config"
	"github.com/Lakes777/pursuit/internal/contas"
	"github.com/Lakes777/pursuit/internal/lembretes"
	"github.com/Lakes777/pursuit/internal/numeros"
	"github.com/Lakes777/pursuit/internal/telegram"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := rodar(log, os.Args[1:]); err != nil {
		var uso erroDeUso
		if errors.As(err, &uso) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		log.Error("o Pursuit parou com erro", "erro", err)
		os.Exit(1)
	}
}

func rodar(log *slog.Logger, args []string) error {
	// ctx é cancelado no Ctrl+C ou no SIGTERM
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	cfg, err := config.DoAmbiente()
	if err != nil {
		return err
	}
	// Argumentos errados: o uso, antes de tentar qualquer conexão
	if !argumentosValidos(args) {
		return erroDeUso("Uso: pursuit [definir-senha <nome> | saude | telegram-teste]")
	}
	// Os comandos que não precisam do banco vêm antes de conectar
	if len(args) > 0 {
		switch args[0] {
		case "saude":
			return conferirSaude(ctx, cfg.Endereco)
		case "telegram-teste":
			cliente, err := telegram.Novo(cfg.TelegramURL, cfg.TelegramToken, cfg.TelegramChat)
			if err != nil {
				return err
			}
			if err := cliente.Enviar(ctx, "Pursuit: teste do Telegram. Os lembretes vão chegar aqui."); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "Mensagem de teste enviada.")
			return nil
		}
	}
	pool, err := banco.Conectar(ctx, cfg.BancoURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := banco.Migrar(ctx, pool); err != nil {
		return err
	}
	servicoDeContas := contas.NovoServico(pool)

	if len(args) > 0 { // definir-senha (os outros já voltaram)
		return definirSenha(ctx, servicoDeContas, args[1], os.Stdin, os.Stderr)
	}

	// Lembretes pelo Telegram: sem token e sem chat, ficam desligados (a prévia continua na API).
	// Com só um dos dois, ou com o token mal escrito, a API nem sobe: senão eu passaria semanas
	// sem lembrete achando que estava tudo certo.
	var enviador lembretes.Enviador
	if cfg.TelegramToken == "" && cfg.TelegramChat == "" {
		log.Warn("lembretes pelo Telegram desligados (sem PURSUIT_TELEGRAM_TOKEN e PURSUIT_TELEGRAM_CHAT)")
	} else {
		cliente, err := telegram.Novo(cfg.TelegramURL, cfg.TelegramToken, cfg.TelegramChat)
		if err != nil {
			return fmt.Errorf("configuração do Telegram: %w", err)
		}
		enviador = cliente
	}
	servicoDeLembretes := lembretes.NovoServico(pool, enviador)

	// Abre a porta antes de dizer "no ar": uma porta ocupada dá erro aqui mesmo
	porta, err := net.Listen("tcp", cfg.Endereco)
	if err != nil {
		return err
	}
	servidor := &http.Server{
		Handler: api.Novo(api.Dependencias{
			Banco: pool, Candidaturas: candidaturas.NovoServico(pool), Contas: servicoDeContas,
			Numeros: numeros.NovoServico(pool), Lembretes: servicoDeLembretes,
			Limite: contas.NovoLimite(cfg.LoginPorMinuto), CookieSeguro: cfg.CookieSeguro, Proxies: cfg.ProxiesConfiaveis, Log: log,
		}),
		// Sem limites, um cliente lento seguraria uma conexão para sempre (Slowloris)
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	erros := make(chan error, 1)
	go func() {
		erros <- servidor.Serve(porta)
	}()
	// Tarefas em segundo plano: param quando o ctx é cancelado, e o desligamento espera por
	// elas antes de fechar o pool (senão uma limpeza no meio usaria uma conexão fechada)
	// (contexto próprio: se o Serve falhar e rodar() voltar antes do Ctrl+C, o defer cancela as
	// tarefas e só então espera; esperar sem cancelar travaria o programa para sempre)
	ctxTarefas, pararTarefas := context.WithCancel(ctx)
	var tarefas sync.WaitGroup
	defer func() {
		pararTarefas()
		tarefas.Wait()
	}()
	tarefas.Go(func() { servicoDeContas.LimparSessoesVencidas(ctxTarefas, time.Hour, log) })
	if enviador != nil {
		tarefas.Go(func() { servicoDeLembretes.Rodar(ctxTarefas, 15*time.Minute, log) })
	}
	log.Info("Pursuit no ar", "endereco", porta.Addr().String())

	select {
	case err := <-erros:
		return err
	case <-ctx.Done():
	}
	// Devolve o Ctrl+C ao padrão: um segundo Ctrl+C encerra na hora, sem esperar
	parar()

	// Desligamento com calma: para de aceitar conexões e espera as que estão no meio (até 10 s)
	log.Info("desligando...")
	ctxFim, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()
	if err := servidor.Shutdown(ctxFim); err != nil {
		// Passou do tempo: fecha à força as conexões que sobraram
		_ = servidor.Close()
		return err
	}
	if err := <-erros; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// argumentosValidos: nenhum (sobe a API), "saude", "telegram-teste" ou "definir-senha <nome>".
func argumentosValidos(args []string) bool {
	if len(args) == 0 {
		return true
	}
	switch args[0] {
	case "saude", "telegram-teste":
		return len(args) == 1
	case "definir-senha":
		return len(args) == 2
	}
	return false
}
