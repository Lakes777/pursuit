// O Pursuit: acompanha as minhas candidaturas a vagas. Sobe o servidor HTTP e desliga com calma
// ao receber Ctrl+C ou o sinal de parada do Docker (SIGTERM).
//
// Uso: pursuit                        sobe a API
//
//	pursuit definir-senha <nome>   cria a conta ou troca a senha (pergunta a senha escondida)
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

	"github.com/Lakes777/pursuit/internal/api"
	"github.com/Lakes777/pursuit/internal/banco"
	"github.com/Lakes777/pursuit/internal/candidaturas"
	"github.com/Lakes777/pursuit/internal/config"
	"github.com/Lakes777/pursuit/internal/contas"
	"github.com/Lakes777/pursuit/internal/numeros"
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
	pool, err := banco.Conectar(ctx, cfg.BancoURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := banco.Migrar(ctx, pool); err != nil {
		return err
	}
	servicoDeContas := contas.NovoServico(pool)

	if len(args) > 0 {
		if args[0] != "definir-senha" || len(args) != 2 {
			return erroDeUso("Uso: pursuit definir-senha <nome>")
		}
		return definirSenha(ctx, servicoDeContas, args[1], os.Stdin, os.Stderr)
	}

	// Abre a porta antes de dizer "no ar": uma porta ocupada dá erro aqui mesmo
	porta, err := net.Listen("tcp", cfg.Endereco)
	if err != nil {
		return err
	}
	servidor := &http.Server{
		Handler: api.Novo(api.Dependencias{
			Banco: pool, Candidaturas: candidaturas.NovoServico(pool), Contas: servicoDeContas,
			Numeros: numeros.NovoServico(pool),
			Limite:  contas.NovoLimite(cfg.LoginPorMinuto), CookieSeguro: cfg.CookieSeguro, Log: log,
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
