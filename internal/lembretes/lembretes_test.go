package lembretes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Lakes777/pursuit/internal/candidaturas"
	"github.com/Lakes777/pursuit/internal/testebanco"
)

var ctx = context.Background()

// enviadorFalso guarda as mensagens; com falhar = true, recusa como um Telegram fora do ar.
// Com limite > 0, recusa como o Telegram um texto maior que isso ("message is too long").
type enviadorFalso struct {
	mu         sync.Mutex
	mensagens  []string
	falhar     bool
	tentativas int
	limite     int
	recusadas  int
}

func (e *enviadorFalso) Enviar(_ context.Context, texto string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tentativas++
	if e.falhar {
		return errors.New("telegram fora do ar")
	}
	if e.limite > 0 && utf8.RuneCountInString(texto) > e.limite {
		e.recusadas++
		return errors.New("Bad Request: message is too long")
	}
	e.mensagens = append(e.mensagens, texto)
	return nil
}

type teste struct {
	t        *testing.T
	cand     *candidaturas.Servico
	lemb     *Servico
	enviador *enviadorFalso
	agora    time.Time
}

// Quarta 07/10/2026 às 10h em Brasília (dentro da janela das 9h às 21h).
func novoTeste(t *testing.T) *teste {
	t.Helper()
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	tt := &teste{t: t, cand: candidaturas.NovoServico(pool), enviador: &enviadorFalso{},
		agora: time.Date(2026, 10, 7, 10, 0, 0, 0, brasilia)}
	tt.lemb = &Servico{pool: pool, enviador: tt.enviador, agora: func() time.Time { return tt.agora }}
	return tt
}

func (tt *teste) candidatura(empresa string, etapa candidaturas.Etapa, diasAtras int) int64 {
	tt.t.Helper()
	em := tt.agora.AddDate(0, 0, -diasAtras)
	d, err := tt.cand.Criar(ctx, candidaturas.Dados{Empresa: empresa, Vaga: "Go"},
		candidaturas.NovaEtapa{Etapa: etapa, Em: &em})
	if err != nil {
		tt.t.Fatal(err)
	}
	return d.ID
}

func (tt *teste) enviar() int {
	tt.t.Helper()
	n, err := tt.lemb.EnviarSeHora(ctx)
	if err != nil {
		tt.t.Fatal(err)
	}
	return n
}

func TestUmaMensagemComAsParadas(t *testing.T) {
	tt := novoTeste(t)
	tt.candidatura("Nubank", candidaturas.Enviada, 8)
	tt.candidatura("iFood", candidaturas.Proposta, 3)
	tt.candidatura("Recente", candidaturas.Enviada, 2) // ainda no prazo
	tt.candidatura("Fim", candidaturas.Recusada, 30)   // final: nunca lembra

	if n := tt.enviar(); n != 2 {
		t.Fatalf("lembradas = %d", n)
	}
	if len(tt.enviador.mensagens) != 1 {
		t.Fatalf("mensagens = %v", tt.enviador.mensagens)
	}
	m := tt.enviador.mensagens[0]
	if !strings.HasPrefix(m, "Pursuit: 2 candidaturas paradas.") || !strings.Contains(m, "Nubank (Go): enviada há 8 dias") ||
		!strings.Contains(m, "iFood (Go): proposta há 3 dias") || strings.Contains(m, "Recente") {
		t.Errorf("mensagem = %q", m)
	}
}

func TestNoMaximoUmaPorDia(t *testing.T) {
	tt := novoTeste(t)
	tt.candidatura("Nubank", candidaturas.Enviada, 8)
	tt.enviar()
	// À tarde, outra passa do prazo: fica para amanhã
	tt.candidatura("iFood", candidaturas.Enviada, 7)
	tt.agora = tt.agora.Add(5 * time.Hour)

	if n := tt.enviar(); n != 0 || len(tt.enviador.mensagens) != 1 {
		t.Errorf("n = %d, mensagens = %d", n, len(tt.enviador.mensagens))
	}

	tt.agora = tt.agora.Add(19 * time.Hour) // dia seguinte, 10h
	if n := tt.enviar(); n != 1 || !strings.Contains(tt.enviador.mensagens[1], "iFood") ||
		strings.Contains(tt.enviador.mensagens[1], "Nubank") {
		t.Errorf("n = %d, mensagens = %v", n, tt.enviador.mensagens)
	}
}

func TestSoNaJanelaDoDia(t *testing.T) {
	tt := novoTeste(t)
	tt.candidatura("Nubank", candidaturas.Enviada, 8)

	for _, hora := range []int{3, 8, 21, 23} {
		tt.agora = time.Date(2026, 10, 7, hora, 30, 0, 0, brasilia)
		if n := tt.enviar(); n != 0 {
			t.Errorf("%dh30: mandou %d", hora, n)
		}
	}
	tt.agora = time.Date(2026, 10, 7, 20, 59, 0, 0, brasilia)
	if n := tt.enviar(); n != 1 {
		t.Errorf("20h59: %d", n)
	}
}

func TestContinuaParadaVoltaDepoisDe7Dias(t *testing.T) {
	tt := novoTeste(t)
	tt.candidatura("Nubank", candidaturas.Enviada, 8)
	tt.enviar()

	tt.agora = tt.agora.AddDate(0, 0, 6)
	if n := tt.enviar(); n != 0 {
		t.Errorf("6 dias depois: %d", n)
	}
	tt.agora = tt.agora.AddDate(0, 0, 1)
	if n := tt.enviar(); n != 1 || !strings.Contains(tt.enviador.mensagens[1], "enviada há 15 dias") {
		t.Errorf("7 dias depois: %d %v", n, tt.enviador.mensagens)
	}
}

func TestMudarDeEtapaRecomecaAContagem(t *testing.T) {
	tt := novoTeste(t)
	id := tt.candidatura("Nubank", candidaturas.Enviada, 8)
	tt.enviar()
	ontem := tt.agora.AddDate(0, 0, -1)
	if _, err := tt.cand.MudarEtapa(ctx, id, candidaturas.NovaEtapa{Etapa: candidaturas.Entrevista, Em: &ontem}); err != nil {
		t.Fatal(err)
	}

	tt.agora = tt.agora.AddDate(0, 0, 3) // entrevista há 4 dias: prazo é 5
	if n := tt.enviar(); n != 0 {
		t.Errorf("antes do prazo da entrevista: %d", n)
	}
	tt.agora = tt.agora.AddDate(0, 0, 1) // há 5 dias
	if n := tt.enviar(); n != 1 || !strings.Contains(tt.enviador.mensagens[1], "entrevista há 5 dias") {
		t.Errorf("depois do prazo: %d %v", n, tt.enviador.mensagens)
	}
}

func TestTelegramForaTentaDeNovo(t *testing.T) {
	tt := novoTeste(t)
	tt.candidatura("Nubank", candidaturas.Enviada, 8)
	tt.enviador.falhar = true

	if _, err := tt.lemb.EnviarSeHora(ctx); err == nil {
		t.Fatal("deveria devolver o erro do Telegram")
	}

	tt.enviador.falhar = false
	tt.agora = tt.agora.Add(15 * time.Minute)
	if n := tt.enviar(); n != 1 || len(tt.enviador.mensagens) != 1 {
		t.Errorf("segunda tentativa: %d %v", n, tt.enviador.mensagens)
	}
}

func TestSemNadaParadoNaoMandaENaoGastaODia(t *testing.T) {
	tt := novoTeste(t)
	tt.candidatura("Recente", candidaturas.Enviada, 6)

	if n := tt.enviar(); n != 0 || tt.enviador.tentativas != 0 {
		t.Fatalf("n = %d, tentativas = %d", n, tt.enviador.tentativas)
	}
	// Mais tarde no mesmo dia cadastro uma que mandei há 8 dias: o dia não foi "gasto" com
	// uma mensagem vazia, e ela sai ainda hoje
	tt.agora = tt.agora.Add(5 * time.Hour)
	tt.candidatura("Antiga", candidaturas.Enviada, 8)
	if n := tt.enviar(); n != 1 || !strings.Contains(tt.enviador.mensagens[0], "Antiga") {
		t.Errorf("depois: %d %v", n, tt.enviador.mensagens)
	}
}

func TestMuitasParadasSaemEmDiasSeguidos(t *testing.T) {
	tt := novoTeste(t)
	tt.enviador.limite = 4096
	empresa := strings.Repeat("Empresa grande ", 12)
	for i := range 40 {
		tt.candidatura(fmt.Sprintf("%s%d", empresa, i), candidaturas.Enviada, 8+i%3)
	}

	primeiro := tt.enviar()
	tt.agora = tt.agora.AddDate(0, 0, 1)
	segundo := tt.enviar()

	if primeiro == 0 || primeiro >= 40 || segundo == 0 {
		t.Errorf("primeiro = %d, segundo = %d", primeiro, segundo)
	}
	if tt.enviador.recusadas != 0 {
		t.Errorf("o Telegram recusou %d mensagem(ns) grande(s) demais", tt.enviador.recusadas)
	}
}

func TestSemTelegramNaoFazNada(t *testing.T) {
	tt := novoTeste(t)
	tt.candidatura("Nubank", candidaturas.Enviada, 8)
	tt.lemb.enviador = nil

	if n := tt.enviar(); n != 0 {
		t.Errorf("n = %d", n)
	}
	if lista, err := tt.lemb.Paradas(ctx); err != nil || len(lista) != 1 {
		t.Errorf("a prévia funciona sem Telegram: %v %v", lista, err)
	}
}

func TestRodarTentaAoSubirEParaNoCancelamento(t *testing.T) {
	tt := novoTeste(t)
	tt.candidatura("Nubank", candidaturas.Enviada, 8)
	ctxRodar, parar := context.WithCancel(ctx)
	fim := make(chan struct{})

	go func() {
		tt.lemb.Rodar(ctxRodar, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(fim)
	}()
	prazo := time.Now().Add(5 * time.Second)
	for time.Now().Before(prazo) {
		tt.enviador.mu.Lock()
		n := len(tt.enviador.mensagens)
		tt.enviador.mu.Unlock()
		if n == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	parar()

	select {
	case <-fim:
	case <-time.After(5 * time.Second):
		t.Fatal("Rodar não parou com o cancelamento")
	}
	if len(tt.enviador.mensagens) != 1 {
		t.Errorf("mensagens = %v", tt.enviador.mensagens)
	}
}
