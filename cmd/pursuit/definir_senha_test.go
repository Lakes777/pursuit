package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Lakes777/pursuit/internal/contas"
	"github.com/Lakes777/pursuit/internal/testebanco"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	testebanco.Encerrar()
	os.Exit(codigo)
}

// entrada devolve um arquivo (não um terminal) com o texto, como num "echo ... | pursuit".
func entrada(t *testing.T, texto string) *os.File {
	t.Helper()
	leitura, escrita, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := escrita.WriteString(texto); err != nil {
		t.Fatal(err)
	}
	_ = escrita.Close()
	t.Cleanup(func() { _ = leitura.Close() })
	return leitura
}

func TestDefinirSenhaPelaEntrada(t *testing.T) {
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	s := contas.NovoServico(pool)
	var saida bytes.Buffer

	if err := definirSenha(context.Background(), s, " Andre ", entrada(t, "uma frase longa de teste\r\n"), &saida); err != nil {
		t.Fatal(err)
	}

	if saida.String() != "Senha de \"andre\" definida. Sessões abertas foram encerradas.\n" {
		t.Errorf("saída = %q", saida.String())
	}
	if _, _, err := s.Entrar(context.Background(), "andre", "uma frase longa de teste"); err != nil {
		t.Errorf("a senha lida (sem o \\r\\n) deveria entrar: %v", err)
	}
}

func TestDefinirSenhaRecusa(t *testing.T) {
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	s := contas.NovoServico(pool)
	if err := s.DefinirSenha(context.Background(), "andre", "uma frase longa de teste"); err != nil {
		t.Fatal(err)
	}
	casos := map[string]struct{ nome, senha string }{
		"senha curta":   {"andre", "curta\n"},
		"nome inválido": {"com espaço", "uma frase longa de teste\n"},
		"outra conta":   {"outra", "uma frase longa de teste\n"},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			err := definirSenha(context.Background(), s, caso.nome, entrada(t, caso.senha), &bytes.Buffer{})
			var uso erroDeUso
			if !errors.As(err, &uso) {
				t.Errorf("err = %v", err)
			}
		})
	}
}
