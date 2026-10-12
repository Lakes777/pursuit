package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/Lakes777/pursuit/internal/candidaturas"
	"github.com/Lakes777/pursuit/internal/config"
	"github.com/Lakes777/pursuit/internal/demo"
	"github.com/Lakes777/pursuit/internal/testebanco"
)

var semLog = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestSemearDemoRecusaSemOModo(t *testing.T) {
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	servico := candidaturas.NovoServico(pool)
	if _, err := servico.Criar(context.Background(), candidaturas.Dados{Empresa: "Minha de verdade", Vaga: "Back-end"}, candidaturas.NovaEtapa{}); err != nil {
		t.Fatal(err)
	}

	err := semearDemo(context.Background(), false, pool, &bytes.Buffer{})

	var recusa erroDeRecusa
	if !errors.As(err, &recusa) {
		t.Fatalf("err = %v", err)
	}
	// A candidatura de verdade continua lá, sozinha
	lista, err := servico.Listar(context.Background(), candidaturas.Filtro{})
	if err != nil || len(lista) != 1 || lista[0].Empresa != "Minha de verdade" {
		t.Errorf("lista = %+v, err = %v", lista, err)
	}
	// E sem pool (como no main, que recusa antes de conectar), também recusa sem quebrar
	if err := semearDemo(context.Background(), false, nil, &bytes.Buffer{}); !errors.As(err, &recusa) {
		t.Errorf("sem pool: %v", err)
	}
}

func TestSemearDemoComOModo(t *testing.T) {
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	var saida bytes.Buffer

	if err := semearDemo(context.Background(), true, pool, &saida); err != nil {
		t.Fatal(err)
	}

	lista, err := candidaturas.NovoServico(pool).Listar(context.Background(), candidaturas.Filtro{})
	if err != nil || len(lista) != len(demo.Exemplos) {
		t.Errorf("%d candidaturas, err = %v", len(lista), err)
	}
	if saida.Len() == 0 {
		t.Error("deveria dizer o que fez")
	}
}

func TestLembretesDesligadosNaDemo(t *testing.T) {
	// Um token com o formato certo (o Novo do telegram confere): na demo, mesmo assim, desligado
	comTelegram := config.Config{TelegramToken: "123456:ABCdefGhIJKlmNoPQRstuVWXyz0123456789", TelegramChat: "42"}

	enviador, err := montarEnviador(comTelegram, semLog)
	if err != nil || enviador == nil {
		t.Fatalf("fora da demo, com token: enviador = %v, err = %v", enviador, err)
	}

	comTelegram.Demo = true
	enviador, err = montarEnviador(comTelegram, semLog)
	if err != nil || enviador != nil {
		t.Errorf("na demo: enviador = %v, err = %v", enviador, err)
	}
	// Na demo, nem um token mal escrito impede de subir (o Telegram nem é montado)
	enviador, err = montarEnviador(config.Config{Demo: true, TelegramToken: "x"}, semLog)
	if err != nil || enviador != nil {
		t.Errorf("na demo, token ruim: %v, %v", enviador, err)
	}
	// Fora da demo, as regras de antes continuam: só um dos dois impede de subir
	if _, err := montarEnviador(config.Config{TelegramToken: comTelegram.TelegramToken}, semLog); err == nil {
		t.Error("token sem chat deveria dar erro")
	}
}

func TestArgumentosDaDemo(t *testing.T) {
	if !argumentosValidos([]string{"semear-demo"}) || argumentosValidos([]string{"semear-demo", "x"}) {
		t.Error("semear-demo não leva argumentos")
	}
}

// O caminho de verdade do comando: sem o modo, recusa ANTES de conectar ao banco (o endereço aqui
// nem existe; se a ordem mudasse, o erro seria de conexão, e não a recusa)
func TestSemearDemoPeloComandoRecusaAntesDeConectar(t *testing.T) {
	t.Setenv("PURSUIT_DEMO", "")
	t.Setenv("PURSUIT_BANCO_URL", "postgres://ninguem:nada@127.0.0.1:1/nenhum")

	err := rodar(semLog, []string{"semear-demo"})

	var recusa erroDeRecusa
	if !errors.As(err, &recusa) {
		t.Fatalf("err = %v", err)
	}
}
