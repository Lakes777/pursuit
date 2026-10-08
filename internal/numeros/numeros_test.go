package numeros

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Lakes777/pursuit/internal/candidaturas"
	"github.com/Lakes777/pursuit/internal/testebanco"
)

var ctx = context.Background()

type cenario struct {
	t     *testing.T
	cand  *candidaturas.Servico
	num   *Servico
	agora time.Time
}

func novoCenario(t *testing.T) *cenario {
	t.Helper()
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	agora := time.Now()
	return &cenario{t: t, cand: candidaturas.NovoServico(pool),
		num: &Servico{pool: pool, agora: func() time.Time { return agora }}, agora: agora}
}

// passo: etapa e há quantos dias aconteceu.
type passo struct {
	etapa candidaturas.Etapa
	dias  float64
}

func (c *cenario) candidatura(empresa string, fonte *string, passos ...passo) {
	c.t.Helper()
	quando := func(dias float64) *time.Time {
		t := c.agora.Add(-time.Duration(dias * float64(24*time.Hour)))
		return &t
	}
	d, err := c.cand.Criar(ctx, candidaturas.Dados{Empresa: empresa, Vaga: "Back-end", Fonte: fonte},
		candidaturas.NovaEtapa{Etapa: passos[0].etapa, Em: quando(passos[0].dias)})
	if err != nil {
		c.t.Fatal(err)
	}
	for _, p := range passos[1:] {
		if _, err := c.cand.MudarEtapa(ctx, d.ID, candidaturas.NovaEtapa{Etapa: p.etapa, Em: quando(p.dias)}); err != nil {
			c.t.Fatal(err)
		}
	}
}

func texto(s string) *string { return &s }

// Seis candidaturas com caminhos diferentes; os números esperados foram contados à mão.
func (c *cenario) montar() {
	c.candidatura("A", texto("LinkedIn"), passo{candidaturas.Interesse, 20})
	c.candidatura("B", texto("LinkedIn"), passo{candidaturas.Enviada, 10}, passo{candidaturas.Triagem, 8},
		passo{candidaturas.Entrevista, 7}, passo{candidaturas.Recusada, 6})
	c.candidatura("C", texto("Gupy"), passo{candidaturas.Enviada, 6})
	// Pulou a triagem (conta como passou) e foi contratado
	c.candidatura("D", texto(" linkedin "), passo{candidaturas.Enviada, 5}, passo{candidaturas.Entrevista, 1},
		passo{candidaturas.Tecnica, 0.5}, passo{candidaturas.Proposta, 0.2}, passo{candidaturas.Contratado, 0.1})
	// Desisti antes de responderem: não é resposta nem "aguardando"
	c.candidatura("E", nil, passo{candidaturas.Enviada, 3}, passo{candidaturas.Desisti, 2})
	// Cadastrada já na entrevista (sem "enviada" no histórico): conta no funil, não no tempo de resposta
	c.candidatura("F", nil, passo{candidaturas.Entrevista, 1})
}

func TestFunil(t *testing.T) {
	c := novoCenario(t)
	c.montar()

	n, err := c.num.Calcular(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if n.Total != 6 {
		t.Errorf("total = %d", n.Total)
	}
	esperado := []struct {
		etapa candidaturas.Etapa
		total int64
		taxa  float64
	}{
		{candidaturas.Enviada, 5, 100}, {candidaturas.Triagem, 3, 60}, {candidaturas.Entrevista, 3, 60},
		{candidaturas.Tecnica, 1, 20}, {candidaturas.Proposta, 1, 20}, {candidaturas.Contratado, 1, 20},
	}
	if len(n.Funil) != len(esperado) {
		t.Fatalf("funil = %+v", n.Funil)
	}
	for i, e := range esperado {
		p := n.Funil[i]
		if p.Etapa != e.etapa || p.Total != e.total || p.Taxa == nil || *p.Taxa != e.taxa {
			t.Errorf("passo %d = %+v (taxa %v), esperado %+v", i, p, p.Taxa, e)
		}
	}
}

func TestEtapasAtuais(t *testing.T) {
	c := novoCenario(t)
	c.montar()

	n, _ := c.num.Calcular(ctx)

	if len(n.PorEtapa) != len(candidaturas.Etapas) {
		t.Fatalf("porEtapa = %+v", n.PorEtapa)
	}
	totais := map[candidaturas.Etapa]int64{}
	for _, e := range n.PorEtapa {
		totais[e.Etapa] = e.Total
	}
	for etapa, total := range map[candidaturas.Etapa]int64{
		candidaturas.Interesse: 1, candidaturas.Enviada: 1, candidaturas.Entrevista: 1, candidaturas.Triagem: 0,
		candidaturas.Recusada: 1, candidaturas.Contratado: 1, candidaturas.Desisti: 1,
	} {
		if totais[etapa] != total {
			t.Errorf("%s = %d, esperado %d", etapa, totais[etapa], total)
		}
	}
	if n.EmAndamento != 3 {
		t.Errorf("em andamento = %d", n.EmAndamento)
	}
}

func TestTempoDeResposta(t *testing.T) {
	c := novoCenario(t)
	c.montar()

	n, _ := c.num.Calcular(ctx)

	// B: 2 dias (enviada -> triagem); D: 4 dias (enviada -> entrevista). C aguarda; E desistiu.
	r := n.Respostas
	if r.Respondidas != 2 || r.Aguardando != 1 || r.MediaDias == nil || *r.MediaDias != 3 || *r.MedianaDias != 3 {
		t.Errorf("respostas = %+v (media %v, mediana %v)", r, r.MediaDias, r.MedianaDias)
	}
}

func TestPorFonte(t *testing.T) {
	c := novoCenario(t)
	c.montar()

	n, _ := c.num.Calcular(ctx)

	if len(n.PorFonte) != 3 {
		t.Fatalf("porFonte = %+v", n.PorFonte)
	}
	linkedin, semFonte, gupy := n.PorFonte[0], n.PorFonte[1], n.PorFonte[2]
	if linkedin.Fonte == nil || *linkedin.Fonte != "LinkedIn" || linkedin.Enviadas != 2 ||
		linkedin.Entrevistas != 2 || linkedin.Propostas != 1 || linkedin.Contratados != 1 || linkedin.TaxaDeEntrevista != 100 {
		t.Errorf("linkedin (juntando maiúsculas e espaços; nome = a grafia mais usada) = %+v", linkedin)
	}
	if semFonte.Fonte != nil || semFonte.Enviadas != 2 || semFonte.Entrevistas != 1 || semFonte.TaxaDeEntrevista != 50 {
		t.Errorf("sem fonte = %+v", semFonte)
	}
	if gupy.Fonte == nil || *gupy.Fonte != "Gupy" || gupy.Enviadas != 1 || gupy.TaxaDeEntrevista != 0 {
		t.Errorf("gupy = %+v", gupy)
	}
}

func TestPorSemana(t *testing.T) {
	c := novoCenario(t)
	c.montar()
	c.candidatura("Antiga", nil, passo{candidaturas.Enviada, 200}) // fora das 12 semanas

	n, _ := c.num.Calcular(ctx)

	if len(n.PorSemana) != Semanas {
		t.Fatalf("semanas = %d", len(n.PorSemana))
	}
	var soma int64
	for _, s := range n.PorSemana {
		soma += s.Total
		inicio, err := time.Parse(time.DateOnly, s.Inicio)
		if err != nil || inicio.Weekday() != time.Monday {
			t.Errorf("semana %q não começa na segunda", s.Inicio)
		}
	}
	if soma != 4 { // B, C, D e E (a antiga fica de fora)
		t.Errorf("soma = %d: %+v", soma, n.PorSemana)
	}
	hoje := c.agora.In(brasilia)
	ultima, _ := time.ParseInLocation(time.DateOnly, n.PorSemana[Semanas-1].Inicio, brasilia)
	if hoje.Before(ultima) || hoje.Sub(ultima) >= 7*24*time.Hour {
		t.Errorf("a última semana (%s) deveria ser a atual", n.PorSemana[Semanas-1].Inicio)
	}
}

func TestSemNadaCadastrado(t *testing.T) {
	c := novoCenario(t)

	n, err := c.num.Calcular(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if n.Total != 0 || n.Funil[0].Taxa != nil || n.Respostas.MediaDias != nil || len(n.PorSemana) != Semanas {
		t.Errorf("n = %+v", n)
	}
	// Listas vazias saem como [] no JSON, não null (a tela não precisa tratar os dois)
	j, _ := json.Marshal(n)
	if !strings.Contains(string(j), `"porFonte":[]`) {
		t.Errorf("json = %s", j)
	}
}

func TestPrimeiraSegunda(t *testing.T) {
	// Quinta 08/10/2026 às 01h em UTC ainda é quarta 07/10 em Brasília: a semana começa na segunda 05/10
	s := &Servico{agora: func() time.Time { return time.Date(2026, 10, 8, 1, 0, 0, 0, time.UTC) }}

	primeira := s.primeiraSegunda()

	if got := primeira.Format(time.DateOnly); got != "2026-07-20" { // 11 semanas antes de 05/10
		t.Errorf("primeira = %s", got)
	}
}

func TestNiveisCobremTodasAsEtapas(t *testing.T) {
	// Se alguém criar uma etapa nova em candidaturas.Etapas, ela entra no funil sozinha (se não
	// for final) ou este teste falha (se for final e ninguém disser o nível dela)
	o := niveis()
	for _, info := range candidaturas.Etapas {
		if info.Etapa == candidaturas.Desisti {
			if o.de(info.Etapa) != -1 {
				t.Error("desisti não deveria ter nível")
			}
			continue
		}
		if o.de(info.Etapa) < 0 {
			t.Errorf("a etapa %s ficou sem nível no funil", info.Etapa)
		}
	}
	if o.de(candidaturas.Recusada) != o.de(candidaturas.Enviada) {
		t.Error("recusada deveria valer como enviada")
	}
	if o.de(candidaturas.Contratado) <= o.de(candidaturas.Proposta) {
		t.Error("contratado deveria ser o fim do caminho")
	}
}

func TestRecusadaDiretoContaComoEnviada(t *testing.T) {
	c := novoCenario(t)
	c.candidatura("Antiga", texto("Gupy"), passo{candidaturas.Recusada, 30})

	n, _ := c.num.Calcular(ctx)

	if n.Funil[0].Total != 1 || n.Funil[1].Total != 0 || len(n.PorFonte) != 1 || n.PorFonte[0].Enviadas != 1 {
		t.Errorf("funil = %+v, fontes = %+v", n.Funil, n.PorFonte)
	}
}

func TestVoltarParaEnviadaNaoContaDeNovo(t *testing.T) {
	c := novoCenario(t)
	c.candidatura("A", nil, passo{candidaturas.Enviada, 20}, passo{candidaturas.Triagem, 19},
		passo{candidaturas.Enviada, 10})

	n, _ := c.num.Calcular(ctx)

	var soma int64
	for _, s := range n.PorSemana {
		soma += s.Total
	}
	if soma != 1 {
		t.Errorf("soma das semanas = %d", soma)
	}
}

func TestMudancasNoMesmoInstanteSeguemAOrdemDoRegistro(t *testing.T) {
	// Cadastrada como "triagem" e corrigida para "enviada" com o mesmo horário: a triagem veio
	// antes no registro, então não é resposta ao envio
	c := novoCenario(t)
	c.candidatura("A", nil, passo{candidaturas.Triagem, 2}, passo{candidaturas.Enviada, 2})

	n, _ := c.num.Calcular(ctx)

	if n.Respostas.Respondidas != 0 || n.Respostas.Aguardando != 1 {
		t.Errorf("respostas = %+v", n.Respostas)
	}
}

func TestViradaDaSemanaNoHorarioDeBrasilia(t *testing.T) {
	c := novoCenario(t)
	c.num.agora = func() time.Time { return time.Date(2026, 10, 7, 12, 0, 0, 0, brasilia) }
	envio := func(empresa string, quando time.Time) {
		if _, err := c.cand.Criar(ctx, candidaturas.Dados{Empresa: empresa, Vaga: "Go"},
			candidaturas.NovaEtapa{Etapa: candidaturas.Enviada, Em: &quando}); err != nil {
			t.Fatal(err)
		}
	}
	// Domingo 23h30 em Brasília já é segunda 02h30 em UTC
	envio("Domingo", time.Date(2026, 10, 4, 23, 30, 0, 0, brasilia))
	envio("Segunda", time.Date(2026, 10, 5, 0, 30, 0, 0, brasilia))

	n, _ := c.num.Calcular(ctx)

	porInicio := map[string]int64{}
	for _, s := range n.PorSemana {
		porInicio[s.Inicio] = s.Total
	}
	if porInicio["2026-09-28"] != 1 || porInicio["2026-10-05"] != 1 {
		t.Errorf("semanas = %+v", n.PorSemana)
	}
}
