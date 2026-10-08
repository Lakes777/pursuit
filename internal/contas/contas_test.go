package contas

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Lakes777/pursuit/internal/testebanco"
)

var ctx = context.Background()

type relogio struct{ agora time.Time }

func (r *relogio) andar(d time.Duration) { r.agora = r.agora.Add(d) }

func novoServico(t *testing.T) (*Servico, *relogio) {
	t.Helper()
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	r := &relogio{agora: time.Date(2026, 10, 8, 15, 0, 0, 0, time.UTC)}
	s := &Servico{pool: pool, agora: func() time.Time { return r.agora }}
	if err := s.DefinirSenha(ctx, "andre", "uma frase longa de teste"); err != nil {
		t.Fatal(err)
	}
	return s, r
}

func TestContaUnica(t *testing.T) {
	s, _ := novoServico(t)

	if err := s.DefinirSenha(ctx, "outra", "uma frase longa de teste"); !errors.Is(err, ErrOutraConta) {
		t.Errorf("err = %v", err)
	}
	if err := s.DefinirSenha(ctx, "ANDRE", "trocando a senha agora"); err != nil {
		t.Errorf("trocar a senha da mesma conta: %v", err)
	}
}

func TestLoginCanceladoEsperandoAVez(t *testing.T) {
	s, _ := novoServico(t)
	// Ocupa as duas vagas do argon2
	vagasDoArgon2 <- struct{}{}
	vagasDoArgon2 <- struct{}{}
	defer func() { <-vagasDoArgon2; <-vagasDoArgon2 }()
	ctxCurto, cancelar := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancelar()

	if _, _, err := s.Entrar(ctxCurto, "andre", "uma frase longa de teste"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v", err)
	}
}

func TestEntrarEConferirASessao(t *testing.T) {
	s, _ := novoServico(t)

	token, u, err := s.Entrar(ctx, "andre", "uma frase longa de teste")
	if err != nil || u.Nome != "andre" {
		t.Fatalf("u = %+v, err = %v", u, err)
	}
	dono, _, err := s.Sessao(ctx, token)
	if err != nil || dono.Nome != "andre" {
		t.Errorf("dono = %+v, err = %v", dono, err)
	}
}

func TestBancoGuardaSoOHashDoToken(t *testing.T) {
	s, _ := novoServico(t)
	token, _, _ := s.Entrar(ctx, "andre", "uma frase longa de teste")

	var guardado []byte
	if err := s.pool.QueryRow(ctx, "select token_hash from sessao").Scan(&guardado); err != nil {
		t.Fatal(err)
	}
	if string(guardado) == token || len(guardado) != 32 {
		t.Errorf("guardado = %x", guardado)
	}
}

func TestSenhaGiganteNaoEntra(t *testing.T) {
	s, _ := novoServico(t)
	gigante := make([]byte, 10_000)
	for i := range gigante {
		gigante[i] = 'a'
	}
	if _, _, err := s.Entrar(ctx, "andre", string(gigante)); !errors.Is(err, ErrCredenciais) {
		t.Errorf("err = %v", err)
	}
}

func TestSessaoVenceSemUso(t *testing.T) {
	s, r := novoServico(t)
	token, _, _ := s.Entrar(ctx, "andre", "uma frase longa de teste")

	r.andar(Duracao + time.Second)

	if _, _, err := s.Sessao(ctx, token); !errors.Is(err, ErrSemSessao) {
		t.Errorf("err = %v", err)
	}
}

func TestUsoRenovaASessao(t *testing.T) {
	s, r := novoServico(t)
	token, _, _ := s.Entrar(ctx, "andre", "uma frase longa de teste")

	r.andar(30 * time.Minute)
	if _, renovada, _ := s.Sessao(ctx, token); renovada {
		t.Error("renovou antes de 1 h")
	}
	r.andar(6 * 24 * time.Hour) // 6 dias e meio depois do login
	if _, renovada, err := s.Sessao(ctx, token); !renovada || err != nil {
		t.Errorf("renovada = %v, err = %v", renovada, err)
	}
	r.andar(3 * 24 * time.Hour) // 9 dias e meio depois do login, 3 depois do último uso
	if _, _, err := s.Sessao(ctx, token); err != nil {
		t.Errorf("a sessão renovada deveria valer: %v", err)
	}
}

func TestSairInvalidaOToken(t *testing.T) {
	s, _ := novoServico(t)
	token, _, _ := s.Entrar(ctx, "andre", "uma frase longa de teste")

	if err := s.Sair(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Sessao(ctx, token); !errors.Is(err, ErrSemSessao) {
		t.Errorf("err = %v", err)
	}
}

func TestLimpezaApagaSoAsVencidas(t *testing.T) {
	s, r := novoServico(t)
	entrar := func() {
		if _, _, err := s.Entrar(ctx, "andre", "uma frase longa de teste"); err != nil {
			t.Fatal(err)
		}
	}
	entrar()
	r.andar(Duracao + time.Minute) // a 1ª vence; a 2ª é aberta já depois disso
	entrar()

	ctxLimpeza, parar := context.WithCancel(ctx)
	fim := make(chan struct{})
	go func() {
		s.LimparSessoesVencidas(ctxLimpeza, 10*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(fim)
	}()
	prazo := time.Now().Add(5 * time.Second)
	for contar(t, s) != 1 && time.Now().Before(prazo) {
		time.Sleep(20 * time.Millisecond)
	}
	parar()
	<-fim // a goroutine termina quando o ctx é cancelado

	if n := contar(t, s); n != 1 {
		t.Errorf("sessões = %d", n)
	}
}

func contar(t *testing.T, s *Servico) int {
	t.Helper()
	var n int
	if err := s.pool.QueryRow(ctx, "select count(*) from sessao").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
