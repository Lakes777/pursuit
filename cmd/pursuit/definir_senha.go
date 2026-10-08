package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/Lakes777/pursuit/internal/contas"
)

// erroDeUso: argumentos errados; o programa mostra o uso e sai com código 2.
type erroDeUso string

func (e erroDeUso) Error() string { return string(e) }

// definirSenha pergunta a senha. No terminal, escondida e duas vezes (para não errar sem ver);
// fora dele (ex.: echo ... | pursuit definir-senha andre), lê a primeira linha da entrada.
func definirSenha(ctx context.Context, s *contas.Servico, nome string, entrada *os.File, saida io.Writer) error {
	nome, err := contas.NormalizarNome(nome)
	if err != nil {
		return erroDeUso(err.Error())
	}
	var senha string
	fd := int(entrada.Fd()) //nolint:gosec // descritor de arquivo cabe num int
	if term.IsTerminal(fd) {
		primeira, err := lerEscondido(fd, saida, "Senha nova: ")
		if err != nil {
			return err
		}
		// Confere já, para não pedir a repetição de uma senha que vai ser recusada
		if err := contas.ConferirSenhaNova(primeira); err != nil {
			return erroDeUso(err.Error())
		}
		segunda, err := lerEscondido(fd, saida, "Repita a senha: ")
		if err != nil {
			return err
		}
		if primeira != segunda {
			return erroDeUso("as senhas não são iguais")
		}
		senha = primeira
	} else {
		linha, err := bufio.NewReader(entrada).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		senha = strings.TrimRight(linha, "\r\n")
	}
	if err := contas.ConferirSenhaNova(senha); err != nil {
		return erroDeUso(err.Error())
	}
	if err := s.DefinirSenha(ctx, nome, senha); errors.Is(err, contas.ErrOutraConta) {
		return erroDeUso(err.Error())
	} else if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(saida, "Senha de %q definida. Sessões abertas foram encerradas.\n", nome)
	return nil
}

func lerEscondido(fd int, saida io.Writer, pergunta string) (string, error) {
	_, _ = fmt.Fprint(saida, pergunta)
	b, err := term.ReadPassword(fd)
	_, _ = fmt.Fprintln(saida)
	return string(b), err
}
