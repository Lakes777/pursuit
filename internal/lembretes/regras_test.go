package lembretes

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Lakes777/pursuit/internal/candidaturas"
)

func TestTodaEtapaEmAndamentoTemPrazo(t *testing.T) {
	for _, info := range candidaturas.Etapas {
		_, tem := Prazos[info.Etapa]
		if tem == info.Final {
			t.Errorf("%s: final = %v, prazo = %v", info.Etapa, info.Final, tem)
		}
		if tem && !strings.Contains(Prazos[info.Etapa].Frase, "%s") {
			t.Errorf("%s: a frase precisa do %%s", info.Etapa)
		}
	}
}

func TestTabelaDePrazosNaOrdemDasEtapas(t *testing.T) {
	tabela := TabelaDePrazos()
	if len(tabela) != len(Prazos) {
		t.Fatalf("%d linhas para %d prazos", len(tabela), len(Prazos))
	}
	esperadas := []candidaturas.Etapa{candidaturas.Interesse, candidaturas.Enviada, candidaturas.Triagem,
		candidaturas.Entrevista, candidaturas.Tecnica, candidaturas.Proposta}
	for i, linha := range tabela {
		if linha.Etapa != esperadas[i] || linha.Dias != Prazos[linha.Etapa].Dias || linha.Nome == "" {
			t.Errorf("linha %d = %+v", i, linha)
		}
	}
	if tabela[1].Nome != "Candidatura enviada" || tabela[1].Dias != 7 {
		t.Errorf("enviada = %+v", tabela[1])
	}
}

func TestPrecisaLembrar(t *testing.T) {
	agora := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	dias := func(n float64) time.Time { return agora.Add(-time.Duration(n * float64(24*time.Hour))) }
	nunca := time.Time{}

	casos := []struct {
		nome    string
		etapa   candidaturas.Etapa
		desde   time.Time
		ultimo  time.Time
		lembrar bool
	}{
		{"enviada há 6 dias", candidaturas.Enviada, dias(6), nunca, false},
		{"enviada há 7 dias", candidaturas.Enviada, dias(7), nunca, true},
		// 6,8 dias em horas, mas 7 viradas de dia no calendário de Brasília
		{"enviada há 6,8 dias corridos", candidaturas.Enviada, dias(6.8), nunca, true},
		{"proposta há 2 dias", candidaturas.Proposta, dias(2), nunca, true},
		{"lembrada há 3 dias", candidaturas.Enviada, dias(20), dias(3), false},
		{"lembrada há 7 dias", candidaturas.Enviada, dias(20), dias(7), true},
		{"lembrada há 6,9 dias corridos", candidaturas.Enviada, dias(20), dias(6.9), true},
		{"etapa final", candidaturas.Recusada, dias(90), nunca, false},
	}
	for _, c := range casos {
		if _, lembrar := precisaLembrar(c.etapa, c.desde, c.ultimo, agora); lembrar != c.lembrar {
			t.Errorf("%s: %v", c.nome, lembrar)
		}
	}
}

func TestDiasEntreUsaOCalendarioDeBrasilia(t *testing.T) {
	sp := func(dia, hora int) time.Time { return time.Date(2026, 10, dia, hora, 0, 0, 0, brasilia) }
	casos := []struct {
		de, ate time.Time
		dias    int
	}{
		{sp(1, 23), sp(2, 0), 1},
		{sp(1, 0), sp(1, 23), 0},
		{sp(1, 18), sp(8, 9), 7},
		// 02h UTC do dia 2 ainda é 23h do dia 1 em Brasília
		{time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC), sp(2, 10), 1},
	}
	for _, c := range casos {
		if got := diasEntre(c.de, c.ate); got != c.dias {
			t.Errorf("%v -> %v = %d, esperado %d", c.de, c.ate, got, c.dias)
		}
	}
}

func TestMensagemRespeitaOLimiteDoTelegram(t *testing.T) {
	var muitas []Parada
	for i := range 60 {
		muitas = append(muitas, Parada{Texto: fmt.Sprintf("%s (%s): enviada há 9 dias, sem resposta: vale mandar uma mensagem ao recrutador. %d",
			strings.Repeat("Empresa", 10), strings.Repeat("Vaga", 10), i)})
	}

	m, couberam := mensagem(muitas)

	if utf8.RuneCountInString(m) > 4096 || couberam == 0 || couberam >= 60 {
		t.Fatalf("tamanho = %d, couberam = %d", utf8.RuneCountInString(m), couberam)
	}
	if !strings.HasPrefix(m, "Pursuit: 60 candidaturas paradas.") ||
		!strings.HasSuffix(m, fmt.Sprintf("E mais %d, que ficam para amanhã.", 60-couberam)) {
		t.Errorf("m = %q", m)
	}
}

func TestTextos(t *testing.T) {
	um := textoDaParada("Nubank", "Go", candidaturas.Proposta, 1)
	if um != "Nubank (Go): proposta há 1 dia: falta responder." {
		t.Errorf("um = %q", um)
	}
	p := []Parada{{Texto: "A"}, {Texto: "B"}}
	if m, n := mensagem(p); m != "Pursuit: 2 candidaturas paradas.\n\n- A\n- B" || n != 2 {
		t.Errorf("m = %q", m)
	}
	if m, _ := mensagem(p[:1]); !strings.HasPrefix(m, "Pursuit: 1 candidatura parada.") {
		t.Errorf("m = %q", m)
	}
}
