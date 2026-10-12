package contas

import (
	"errors"
	"testing"

	"github.com/Lakes777/pursuit/internal/banco/bd"
	"github.com/Lakes777/pursuit/internal/testebanco"
)

func TestVisitanteDesligadoPorPadrao(t *testing.T) {
	s, _ := novoServico(t)

	if _, _, err := s.EntrarComoVisitante(ctx); !errors.Is(err, ErrSemDemo) {
		t.Errorf("err = %v", err)
	}
	// E a conta nem é criada
	if _, err := bd.New(s.pool).BuscarUsuarioPorNome(ctx, NomeDoVisitante); err == nil {
		t.Error("a conta do visitante não deveria existir com a demonstração desligada")
	}
}

func TestEntrarComoVisitante(t *testing.T) {
	base, _ := novoServico(t)
	s := base.ComVisitante()

	token, usuario, err := s.EntrarComoVisitante(ctx)
	if err != nil || usuario.Nome != NomeDoVisitante || token == "" {
		t.Fatalf("usuario = %+v, err = %v", usuario, err)
	}
	if u, _, err := s.Sessao(ctx, token); err != nil || u.Nome != NomeDoVisitante {
		t.Errorf("sessão = %+v, %v", u, err)
	}
	// Uma segunda entrada abre outra sessão na mesma conta (cada visitante com o seu cookie)
	outro, usuario2, err := s.EntrarComoVisitante(ctx)
	if err != nil || outro == token || usuario2.ID != usuario.ID {
		t.Errorf("segunda entrada: %+v, %v", usuario2, err)
	}
	// ComVisitante devolve uma cópia: o serviço original continua sem a demonstração
	if _, _, err := base.EntrarComoVisitante(ctx); !errors.Is(err, ErrSemDemo) {
		t.Errorf("o original mudou: %v", err)
	}
}

func TestVisitanteNaoEntraComSenha(t *testing.T) {
	base, _ := novoServico(t)
	s := base.ComVisitante()
	if _, _, err := s.EntrarComoVisitante(ctx); err != nil {
		t.Fatal(err)
	}

	// Nenhuma senha confere com a conta sem senha, nem a vazia nem o próprio marcador; e a
	// resposta é a de nome errado, não um erro interno pelo hash que não é argon2id
	for _, senha := range []string{"", semSenha, "uma frase longa de teste"} {
		if _, _, err := s.Entrar(ctx, NomeDoVisitante, senha); !errors.Is(err, ErrCredenciais) {
			t.Errorf("senha %q: err = %v", senha, err)
		}
	}
}

func TestContaUnicaNaDemonstracao(t *testing.T) {
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	s := NovoServico(pool).ComVisitante()
	if _, _, err := s.EntrarComoVisitante(ctx); err != nil {
		t.Fatal(err)
	}

	// O visitante não conta como "outra conta": a minha conta convive com ele
	if err := s.DefinirSenha(ctx, "andre", "uma frase longa de teste"); err != nil {
		t.Errorf("definir-senha com o visitante já criado: %v", err)
	}
	// Mas uma terceira continua recusada
	if err := s.DefinirSenha(ctx, "outra", "uma frase longa de teste"); !errors.Is(err, ErrOutraConta) {
		t.Errorf("terceira conta: %v", err)
	}
	// E o visitante não ganha senha
	if err := s.DefinirSenha(ctx, NomeDoVisitante, "uma frase longa de teste"); !errors.Is(err, ErrNomeReservado) {
		t.Errorf("senha para o visitante: %v", err)
	}
	// Fora da demonstração, uma conta "visitante" que existisse contaria como outra (nada muda)
	if err := NovoServico(pool).DefinirSenha(ctx, "andre", "outra frase longa de teste"); !errors.Is(err, ErrOutraConta) {
		t.Errorf("sem a demonstração: %v", err)
	}
}

func TestPrepararVisitanteDerrubaAsSessoesDele(t *testing.T) {
	base, _ := novoServico(t)
	s := base.ComVisitante()
	tokenDoVisitante, _, err := s.EntrarComoVisitante(ctx)
	if err != nil {
		t.Fatal(err)
	}
	meuToken, _, err := s.Entrar(ctx, "andre", "uma frase longa de teste")
	if err != nil {
		t.Fatal(err)
	}

	if err := PrepararVisitante(ctx, bd.New(s.pool)); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.Sessao(ctx, tokenDoVisitante); !errors.Is(err, ErrSemSessao) {
		t.Errorf("a sessão do visitante deveria cair: %v", err)
	}
	if _, _, err := s.Sessao(ctx, meuToken); err != nil {
		t.Errorf("a minha sessão não deveria cair: %v", err)
	}
}
