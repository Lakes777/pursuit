package demo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Lakes777/pursuit/internal/candidaturas"
	"github.com/Lakes777/pursuit/internal/contas"
	"github.com/Lakes777/pursuit/internal/lembretes"
	"github.com/Lakes777/pursuit/internal/testebanco"
)

var ctx = context.Background()

func TestSemearDadosCoerentes(t *testing.T) {
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	agora := time.Now()

	if err := Semear(ctx, pool, agora); err != nil {
		t.Fatal(err)
	}

	lista, err := candidaturas.NovoServico(pool).Listar(ctx, candidaturas.Filtro{})
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) != len(Exemplos) || len(lista) < 9 {
		t.Fatalf("%d candidaturas", len(lista))
	}
	etapas := map[candidaturas.Etapa]bool{}
	comBeacon := 0
	for _, c := range lista {
		d, err := candidaturas.NovoServico(pool).Buscar(ctx, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		h := d.Historico
		if len(h) == 0 {
			t.Fatalf("%s sem histórico", d.Empresa)
		}
		// A etapa atual é a última do histórico, e cada mudança sai de onde a anterior chegou
		if ultima := h[len(h)-1]; d.Etapa != ultima.Para || !d.EtapaDesde.Equal(ultima.Em) {
			t.Errorf("%s: etapa %s desde %v, histórico termina em %s (%v)", d.Empresa, d.Etapa, d.EtapaDesde, ultima.Para, ultima.Em)
		}
		if h[0].De != nil {
			t.Errorf("%s: a primeira mudança não deveria ter 'de'", d.Empresa)
		}
		for i := 1; i < len(h); i++ {
			if h[i].De == nil || *h[i].De != h[i-1].Para {
				t.Errorf("%s: mudança %d não sai de %s", d.Empresa, i, h[i-1].Para)
			}
			if !h[i].Em.After(h[i-1].Em) {
				t.Errorf("%s: histórico fora de ordem (%v depois de %v)", d.Empresa, h[i].Em, h[i-1].Em)
			}
		}
		// Tudo no passado: o cadastro na primeira mudança e a última mexida na última
		for _, m := range h {
			if !m.Em.Before(agora) {
				t.Errorf("%s: mudança no futuro (%v)", d.Empresa, m.Em)
			}
		}
		if !d.CriadaEm.Equal(h[0].Em) || !d.AtualizadaEm.Equal(h[len(h)-1].Em) {
			t.Errorf("%s: criada %v, atualizada %v", d.Empresa, d.CriadaEm, d.AtualizadaEm)
		}
		if d.Fonte == nil || d.Modalidade == nil {
			t.Errorf("%s sem fonte ou modalidade", d.Empresa)
		}
		if d.Beacon != nil {
			comBeacon++
			if d.Beacon.URL != DominioDoBeacon+d.Beacon.Codigo {
				t.Errorf("%s: link %s", d.Empresa, d.Beacon.URL)
			}
		}
		etapas[d.Etapa] = true
	}
	for _, e := range []candidaturas.Etapa{candidaturas.Interesse, candidaturas.Enviada, candidaturas.Triagem,
		candidaturas.Entrevista, candidaturas.Tecnica, candidaturas.Proposta, candidaturas.Recusada} {
		if !etapas[e] {
			t.Errorf("nenhum exemplo em %s", e)
		}
	}
	if comBeacon < 2 || comBeacon > 3 {
		t.Errorf("%d com link do Beacon", comBeacon)
	}
	// A lista vem da mexida por último à mais antiga (a ordem do quadro)
	for i := 1; i < len(lista); i++ {
		if lista[i].AtualizadaEm.After(lista[i-1].AtualizadaEm) {
			t.Errorf("lista fora de ordem em %d", i)
		}
	}

	// Pelo menos duas paradas além do prazo, para a tela de lembretes ter o que mostrar
	paradas, err := lembretes.NovoServico(pool, nil).Paradas(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(paradas) < 2 {
		t.Errorf("%d paradas: %+v", len(paradas), paradas)
	}
}

func TestSemearDeNovoVoltaAoOriginal(t *testing.T) {
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	agora := time.Now()
	if err := Semear(ctx, pool, agora); err != nil {
		t.Fatal(err)
	}
	servico := candidaturas.NovoServico(pool)
	// O visitante mexe: apaga uma, cria outra e muda a etapa de uma terceira
	if err := servico.Apagar(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := servico.Criar(ctx, candidaturas.Dados{Empresa: "Teste", Vaga: "Qualquer"}, candidaturas.NovaEtapa{}); err != nil {
		t.Fatal(err)
	}
	if _, err := servico.MudarEtapa(ctx, 2, candidaturas.NovaEtapa{Etapa: candidaturas.Desisti}); err != nil {
		t.Fatal(err)
	}

	if err := Semear(ctx, pool, agora); err != nil {
		t.Fatal(err)
	}

	lista, err := servico.Listar(ctx, candidaturas.Filtro{})
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) != len(Exemplos) {
		t.Fatalf("%d candidaturas depois de semear de novo", len(lista))
	}
	// Os ids recomeçam do 1, e cada um volta a ser o mesmo exemplo (o link de uma ficha não muda)
	for _, c := range lista {
		ex := Exemplos[c.ID-1]
		if c.Empresa != ex.Empresa || c.Etapa != ex.Passos[len(ex.Passos)-1].Etapa {
			t.Errorf("id %d: %s em %s", c.ID, c.Empresa, c.Etapa)
		}
	}
	var historico int
	if err := pool.QueryRow(ctx, "select count(*) from etapa_historico").Scan(&historico); err != nil {
		t.Fatal(err)
	}
	esperado := 0
	for _, ex := range Exemplos {
		esperado += len(ex.Passos)
	}
	if historico != esperado {
		t.Errorf("%d linhas de histórico, esperado %d (sobrou algo da semeadura anterior)", historico, esperado)
	}
}

func TestSemearDerrubaSoAsSessoesDoVisitante(t *testing.T) {
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	s := contas.NovoServico(pool).ComVisitante()
	if err := s.DefinirSenha(ctx, "andre", "uma frase longa de teste"); err != nil {
		t.Fatal(err)
	}
	doVisitante, _, err := s.EntrarComoVisitante(ctx)
	if err != nil {
		t.Fatal(err)
	}
	minha, _, err := s.Entrar(ctx, "andre", "uma frase longa de teste")
	if err != nil {
		t.Fatal(err)
	}
	// Um lembrete já registrado também some (não fica apontando para um histórico que não existe)
	if err := Semear(ctx, pool, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "insert into lembrete (candidatura_id, historico_id) values (1, 1)"); err != nil {
		t.Fatal(err)
	}

	if err := Semear(ctx, pool, time.Now()); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.Sessao(ctx, doVisitante); !errors.Is(err, contas.ErrSemSessao) {
		t.Errorf("a sessão do visitante deveria cair: %v", err)
	}
	if _, _, err := s.Sessao(ctx, minha); err != nil {
		t.Errorf("a minha sessão caiu: %v", err)
	}
	var lembretesGravados int
	if err := pool.QueryRow(ctx, "select count(*) from lembrete").Scan(&lembretesGravados); err != nil {
		t.Fatal(err)
	}
	if lembretesGravados != 0 {
		t.Errorf("%d lembretes sobraram", lembretesGravados)
	}
}

func TestSemearSeVazio(t *testing.T) {
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)

	semeou, err := SemearSeVazio(ctx, pool, time.Now())
	if err != nil || !semeou {
		t.Fatalf("banco vazio: semeou = %v, err = %v", semeou, err)
	}
	// Com candidaturas (as do visitante, mexidas), a partida não apaga nada
	if err := candidaturas.NovoServico(pool).Apagar(ctx, 1); err != nil {
		t.Fatal(err)
	}
	semeou, err = SemearSeVazio(ctx, pool, time.Now())
	if err != nil || semeou {
		t.Fatalf("banco com dados: semeou = %v, err = %v", semeou, err)
	}
	lista, err := candidaturas.NovoServico(pool).Listar(ctx, candidaturas.Filtro{})
	if err != nil || len(lista) != len(Exemplos)-1 {
		t.Errorf("%d candidaturas, err = %v", len(lista), err)
	}
}

func TestDatasRelativasNoHorarioDeBrasilia(t *testing.T) {
	// 4h UTC do dia 2 é 1h da manhã do dia 2 em Brasília: "há 1 dia, às 10h" cai no dia 1º às
	// 10h de Brasília, qualquer que seja o fuso do servidor
	agora := time.Date(2026, 10, 2, 4, 0, 0, 0, time.UTC)
	got := quando(Passo{DiasAtras: 1, Hora: 10}, agora)
	if want := time.Date(2026, 10, 1, 10, 0, 0, 0, brasilia); !got.Equal(want) {
		t.Errorf("quando = %v, esperado %v", got, want)
	}
	// Virada de mês
	got = quando(Passo{DiasAtras: 3, Hora: 9}, time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC))
	if want := time.Date(2026, 9, 28, 9, 0, 0, 0, brasilia); !got.Equal(want) {
		t.Errorf("virada de mês: %v", got)
	}
}

func TestExemplosEmOrdem(t *testing.T) {
	for _, ex := range Exemplos {
		for i, p := range ex.Passos {
			if !p.Etapa.Valida() || p.DiasAtras < 1 {
				t.Errorf("%s: passo %d inválido (%+v)", ex.Empresa, i, p)
			}
			if i > 0 && p.DiasAtras >= ex.Passos[i-1].DiasAtras {
				t.Errorf("%s: passo %d não vem depois do anterior", ex.Empresa, i)
			}
		}
	}
}

// O banco de verdade (com candidaturas e sem a conta do visitante) nunca é apagado, mesmo que a
// instância de demonstração aponte para ele por engano
func TestBancoRealNuncaEApagado(t *testing.T) {
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	minha, err := candidaturas.NovoServico(pool).Criar(ctx, candidaturas.Dados{Empresa: "Empresa de verdade", Vaga: "Back-end"}, candidaturas.NovaEtapa{})
	if err != nil {
		t.Fatal(err)
	}

	if err := Semear(ctx, pool, time.Now()); !errors.Is(err, ErrBancoReal) {
		t.Errorf("Semear: err = %v", err)
	}
	if semeou, err := SemearSeVazio(ctx, pool, time.Now()); semeou || !errors.Is(err, ErrBancoReal) {
		t.Errorf("SemearSeVazio: semeou = %v, err = %v", semeou, err)
	}
	if _, err := candidaturas.NovoServico(pool).Buscar(ctx, minha.ID); err != nil {
		t.Errorf("a candidatura de verdade sumiu: %v", err)
	}
}
