// O Pursuit: acompanha as minhas candidaturas a vagas. Sobe o servidor HTTP e desliga com calma
// ao receber Ctrl+C ou o sinal de parada do Docker (SIGTERM).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Lakes777/pursuit/internal/api"
	"github.com/Lakes777/pursuit/internal/banco"
	"github.com/Lakes777/pursuit/internal/config"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := rodar(log); err != nil {
		log.Error("o Pursuit parou com erro", "erro", err)
		os.Exit(1)
	}
}

func rodar(log *slog.Logger) error {
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

	// Abre a porta antes de dizer "no ar": uma porta ocupada dá erro aqui mesmo
	porta, err := net.Listen("tcp", cfg.Endereco)
	if err != nil {
		return err
	}
	servidor := &http.Server{
		Handler: api.Novo(pool, log),
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
