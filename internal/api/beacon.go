package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Lakes777/pursuit/internal/beacon"
	"github.com/Lakes777/pursuit/internal/candidaturas"
)

// rotasDoBeacon: o link curto de cada candidatura e os cliques dele. O navegador só fala com o
// Pursuit; é o servidor que fala com o Beacon, com a chave, que nunca sai daqui.
type rotasDoBeacon struct {
	cliente      *beacon.Cliente // nil = integração desligada (sem PURSUIT_BEACON_URL e _CHAVE)
	candidaturas *candidaturas.Servico
	log          *slog.Logger
}

// estado diz à tela se mostra o botão "Criar link".
func (b rotasDoBeacon) estado(w http.ResponseWriter, _ *http.Request) {
	responderJSON(w, http.StatusOK, map[string]any{"ligado": b.cliente != nil, "dias": beacon.DiasDeCliques})
}

// criar dá à candidatura um link curto (ou devolve o que ela já tem).
func (b rotasDoBeacon) criar(w http.ResponseWriter, r *http.Request) {
	id, ok := lerID(w, r)
	if !ok || !b.ligado(w) {
		return
	}
	candidatura, err := b.candidaturas.CriarLinkDoBeacon(r.Context(), id, b.cliente)
	if err != nil {
		b.erro(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, candidatura)
}

// cliques de uma candidatura: quantas vezes o link dela foi aberto, por dia.
func (b rotasDoBeacon) cliques(w http.ResponseWriter, r *http.Request) {
	id, ok := lerID(w, r)
	if !ok || !b.ligado(w) {
		return
	}
	detalhe, err := b.candidaturas.Buscar(r.Context(), id)
	if err != nil {
		b.erro(w, r, err)
		return
	}
	if detalhe.Beacon == nil {
		responderProblema(w, Problema{Titulo: "Esta candidatura ainda não tem link", Status: http.StatusNotFound})
		return
	}
	cliques, err := b.cliente.CliquesDoLink(r.Context(), detalhe.Beacon.Codigo)
	if errors.Is(err, beacon.ErrNaoEncontrado) {
		// Só aqui um 404 do Beacon quer dizer "este link sumiu" (apagado pelo painel de lá): o Pursuit
		// continua guardando o endereço, só não há números. Nas outras rotas, um 404 é erro de configuração.
		responderProblema(w, Problema{Titulo: "O link não existe mais no Beacon", Status: http.StatusNotFound})
		return
	}
	if err != nil {
		b.erro(w, r, fmt.Errorf("%w: %w", candidaturas.ErrBeacon, err))
		return
	}
	responderJSON(w, http.StatusOK, cliques)
}

// cliquesPorLink: os cliques de todos os links de uma vez (o quadro marca as candidaturas abertas).
func (b rotasDoBeacon) cliquesPorLink(w http.ResponseWriter, r *http.Request) {
	if !b.ligado(w) {
		return
	}
	porLink, err := b.cliente.CliquesPorLink(r.Context())
	if err != nil {
		b.erro(w, r, fmt.Errorf("%w: %w", candidaturas.ErrBeacon, err))
		return
	}
	responderJSON(w, http.StatusOK, map[string]any{"porLink": porLink})
}

func (b rotasDoBeacon) ligado(w http.ResponseWriter) bool {
	if b.cliente == nil {
		responderProblema(w, Problema{Titulo: "Integração com o Beacon desligada", Status: http.StatusServiceUnavailable})
		return false
	}
	return true
}

func (b rotasDoBeacon) erro(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, candidaturas.ErrNaoEncontrada):
		responderProblema(w, Problema{Titulo: "Candidatura não encontrada", Status: http.StatusNotFound})
	case errors.Is(err, candidaturas.ErrBeacon):
		// Beacon fora do ar, chave recusada (nunca 401 aqui: a página entenderia que a sessão do
		// Pursuit caiu), endereço errado, resposta estranha: o problema está do outro lado (502).
		// O motivo vai para o log; a chave nunca aparece nele (ver beacon.Cliente).
		b.log.Error("erro ao falar com o Beacon", "metodo", r.Method, "caminho", r.URL.Path, "erro", err)
		responderProblema(w, Problema{Titulo: "O Beacon não respondeu", Status: http.StatusBadGateway})
	default:
		b.log.Error("erro inesperado", "metodo", r.Method, "caminho", r.URL.Path, "erro", err)
		responderProblema(w, Problema{Titulo: "Erro interno", Status: http.StatusInternalServerError})
	}
}
