package candidaturas

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Lakes777/pursuit/internal/testebanco"
)

var ctx = context.Background()

// novoServico usa o banco dos testes, limpo, e um relógio parado em 08/10/2026 15h (UTC).
func novoServico(t *testing.T) (*Servico, time.Time) {
	t.Helper()
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	agora := time.Date(2026, 10, 8, 15, 0, 0, 0, time.UTC)
	return &Servico{pool: pool, agora: func() time.Time { return agora }}, agora
}

func criar(t *testing.T, s *Servico, empresa string) Detalhe {
	t.Helper()
	d, err := s.Criar(ctx, Dados{Empresa: empresa, Vaga: "Back-end Júnior"}, NovaEtapa{})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestCriarComecaEmInteresseComHistorico(t *testing.T) {
	s, agora := novoServico(t)

	d := criar(t, s, "Nubank")

	if d.ID == 0 || d.Etapa != Interesse {
		t.Fatalf("d = %+v", d)
	}
	if len(d.Historico) != 1 || d.Historico[0].De != nil || d.Historico[0].Para != Interesse ||
		!d.Historico[0].Em.Equal(agora) {
		t.Errorf("historico = %+v", d.Historico)
	}
}

func TestCriarJaEnviadaComDataPassada(t *testing.T) {
	s, agora := novoServico(t)
	segunda := agora.Add(-72 * time.Hour)

	d, err := s.Criar(ctx, Dados{Empresa: "iFood", Vaga: "Go"},
		NovaEtapa{Etapa: Enviada, Em: &segunda, Observacao: texto("pela Gupy")})

	if err != nil {
		t.Fatal(err)
	}
	if d.Etapa != Enviada || !d.Historico[0].Em.Equal(segunda) || *d.Historico[0].Observacao != "pela Gupy" {
		t.Errorf("d = %+v", d)
	}
}

func TestCriarInvalidoNaoGravaNada(t *testing.T) {
	s, _ := novoServico(t)

	_, err := s.Criar(ctx, Dados{Empresa: "", Vaga: "Go"}, NovaEtapa{Etapa: "sonho"})

	c := campos(t, err)
	if c["empresa"] == "" {
		t.Errorf("campos = %v", c)
	}
	if lista, _ := s.Listar(ctx, Filtro{}); len(lista) != 0 {
		t.Errorf("gravou mesmo inválido: %v", lista)
	}
}

func TestMudarEtapaGuardaOHistoricoEmOrdem(t *testing.T) {
	s, agora := novoServico(t)
	d := criar(t, s, "Nubank")

	if _, err := s.MudarEtapa(ctx, d.ID, NovaEtapa{Etapa: Enviada}); err != nil {
		t.Fatal(err)
	}
	d, err := s.MudarEtapa(ctx, d.ID, NovaEtapa{Etapa: Entrevista, Observacao: texto("com o tech lead")})
	if err != nil {
		t.Fatal(err)
	}

	if d.Etapa != Entrevista {
		t.Errorf("etapa = %s", d.Etapa)
	}
	var passos []string
	for _, m := range d.Historico {
		de := "-"
		if m.De != nil {
			de = string(*m.De)
		}
		passos = append(passos, de+">"+string(m.Para))
	}
	if got := passos; len(got) != 3 || got[0] != "->interesse" || got[1] != "interesse>enviada" ||
		got[2] != "enviada>entrevista" {
		t.Errorf("passos = %v", got)
	}
	if !d.Historico[2].Em.Equal(agora) || *d.Historico[2].Observacao != "com o tech lead" {
		t.Errorf("última = %+v", d.Historico[2])
	}
}

func TestMudarParaAMesmaEtapa(t *testing.T) {
	s, _ := novoServico(t)
	d := criar(t, s, "Nubank")

	_, err := s.MudarEtapa(ctx, d.ID, NovaEtapa{Etapa: Interesse})

	if !errors.Is(err, ErrMesmaEtapa) {
		t.Errorf("err = %v", err)
	}
}

func TestMudarEtapaComDataAntesDaUltima(t *testing.T) {
	s, agora := novoServico(t)
	d := criar(t, s, "Nubank")
	ontem := agora.Add(-24 * time.Hour)

	_, err := s.MudarEtapa(ctx, d.ID, NovaEtapa{Etapa: Enviada, Em: &ontem})

	if c := campos(t, err); c["em"] == "" {
		t.Errorf("campos = %v", c)
	}
}

func TestMudarEtapaDeQuemNaoExiste(t *testing.T) {
	s, _ := novoServico(t)

	if _, err := s.MudarEtapa(ctx, 999, NovaEtapa{Etapa: Enviada}); !errors.Is(err, ErrNaoEncontrada) {
		t.Errorf("err = %v", err)
	}
}

func TestMudancasAoMesmoTempoNaoSeAtropelam(t *testing.T) {
	// 10 pedidos juntos para "enviada": a trava da linha faz um passar e os outros verem
	// que ela já está lá. Sem a trava, vários passariam e o histórico teria "interesse>enviada" repetido.
	s, _ := novoServico(t)
	d := criar(t, s, "Nubank")

	var grupo sync.WaitGroup
	var trava sync.Mutex
	certos, repetidos := 0, 0
	for range 10 {
		grupo.Go(func() {
			_, err := s.MudarEtapa(ctx, d.ID, NovaEtapa{Etapa: Enviada})
			trava.Lock()
			defer trava.Unlock()
			switch {
			case err == nil:
				certos++
			case errors.Is(err, ErrMesmaEtapa):
				repetidos++
			default:
				t.Errorf("err = %v", err)
			}
		})
	}
	grupo.Wait()

	if certos != 1 || repetidos != 9 {
		t.Errorf("certos = %d, repetidos = %d", certos, repetidos)
	}
	d, _ = s.Buscar(ctx, d.ID)
	if len(d.Historico) != 2 {
		t.Errorf("historico = %+v", d.Historico)
	}
}

func TestMudarEtapaDeCandidaturaSemHistorico(t *testing.T) {
	// Uma linha inserida sem passar pelo serviço (ou de antes do histórico existir)
	s, _ := novoServico(t)
	var id int64
	if err := s.pool.QueryRow(ctx, "insert into candidatura (empresa, vaga) values ('a', 'b') returning id").Scan(&id); err != nil {
		t.Fatal(err)
	}

	d, err := s.MudarEtapa(ctx, id, NovaEtapa{Etapa: Enviada})

	if err != nil || d.Etapa != Enviada {
		t.Fatalf("d = %+v, err = %v", d, err)
	}
}

func TestBuscaComNULNaoQuebra(t *testing.T) {
	s, _ := novoServico(t)
	criar(t, s, "Nubank")

	if lista, err := s.Listar(ctx, Filtro{Busca: "nu\x00bank"}); err != nil || len(lista) != 1 {
		t.Errorf("lista = %v, err = %v", lista, err)
	}
}

func TestListarFiltraEBusca(t *testing.T) {
	s, _ := novoServico(t)
	nubank := criar(t, s, "Nubank")
	criar(t, s, "iFood")
	criar(t, s, "100% Remoto Ltda")
	if _, err := s.MudarEtapa(ctx, nubank.ID, NovaEtapa{Etapa: Enviada}); err != nil {
		t.Fatal(err)
	}

	nomes := func(f Filtro) []string {
		lista, err := s.Listar(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		var n []string
		for _, c := range lista {
			n = append(n, c.Empresa)
		}
		return n
	}

	if got := nomes(Filtro{}); len(got) != 3 || got[0] != "Nubank" {
		t.Errorf("todas (Nubank mexida por último vem primeiro) = %v", got)
	}
	if got := nomes(Filtro{Etapa: Enviada}); len(got) != 1 || got[0] != "Nubank" {
		t.Errorf("enviada = %v", got)
	}
	if got := nomes(Filtro{Busca: "IFOOD"}); len(got) != 1 || got[0] != "iFood" {
		t.Errorf("busca sem diferenciar maiúsculas = %v", got)
	}
	if got := nomes(Filtro{Busca: "júnior"}); len(got) != 3 {
		t.Errorf("busca na vaga = %v", got)
	}
	if got := nomes(Filtro{Busca: "%"}); len(got) != 1 || got[0] != "100% Remoto Ltda" {
		t.Errorf("%% vale como texto = %v", got)
	}
	if _, err := s.Listar(ctx, Filtro{Etapa: "sonho"}); err == nil {
		t.Error("aceitou etapa desconhecida")
	}
}

func TestEditarNaoMexeNaEtapa(t *testing.T) {
	s, _ := novoServico(t)
	d := criar(t, s, "Nubank")
	if _, err := s.MudarEtapa(ctx, d.ID, NovaEtapa{Etapa: Triagem}); err != nil {
		t.Fatal(err)
	}

	c, err := s.Editar(ctx, d.ID, Dados{Empresa: "Nu", Vaga: "Go Pleno", Salario: texto("R$ 8 mil")})

	if err != nil {
		t.Fatal(err)
	}
	if c.Empresa != "Nu" || c.Vaga != "Go Pleno" || *c.Salario != "R$ 8 mil" || c.Etapa != Triagem {
		t.Errorf("c = %+v", c)
	}
	if _, err := s.Editar(ctx, 999, Dados{Empresa: "a", Vaga: "b"}); !errors.Is(err, ErrNaoEncontrada) {
		t.Errorf("inexistente: %v", err)
	}
}

func TestApagarLevaOHistorico(t *testing.T) {
	s, _ := novoServico(t)
	d := criar(t, s, "Nubank")

	if err := s.Apagar(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Buscar(ctx, d.ID); !errors.Is(err, ErrNaoEncontrada) {
		t.Errorf("buscar depois de apagar: %v", err)
	}
	var sobrou int
	if err := s.pool.QueryRow(ctx, "select count(*) from etapa_historico").Scan(&sobrou); err != nil || sobrou != 0 {
		t.Errorf("histórico sobrou: %d (%v)", sobrou, err)
	}
	if err := s.Apagar(ctx, d.ID); !errors.Is(err, ErrNaoEncontrada) {
		t.Errorf("apagar de novo: %v", err)
	}
}
