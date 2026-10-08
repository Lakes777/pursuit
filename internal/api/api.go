// Package api tem as rotas HTTP do Pursuit.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Lakes777/pursuit/internal/candidaturas"
	"github.com/Lakes777/pursuit/internal/contas"
	"github.com/Lakes777/pursuit/internal/lembretes"
	"github.com/Lakes777/pursuit/internal/numeros"
)

// Pinger é o pedaço do banco que a rota de saúde usa (o *pgxpool.Pool serve).
type Pinger interface {
	Ping(ctx context.Context) error
}

// Dependencias do servidor HTTP.
type Dependencias struct {
	Banco        Pinger
	Candidaturas *candidaturas.Servico
	Contas       *contas.Servico
	Numeros      *numeros.Servico
	Lembretes    *lembretes.Servico
	Limite       *contas.Limite
	// CookieSeguro: o cookie da sessão só vai por HTTPS (desligar só no computador, sem HTTPS)
	CookieSeguro bool
	Log          *slog.Logger
}

// Novo monta as rotas. Usa o roteador da biblioteca padrão (Go 1.22+), que já entende
// método e parâmetros no caminho ("GET /api/candidaturas/{id}"). Tudo em /api pede login,
// menos entrar e sair; /saude é pública (para o Docker e o Vigil).
func Novo(d Dependencias) http.Handler {
	c := rotasDeCandidaturas{servico: d.Candidaturas, log: d.Log}
	s := rotasDeSessao{contas: d.Contas, limite: d.Limite, cookieSeguro: d.CookieSeguro, log: d.Log}
	logado := s.exigirLogin
	rotas := http.NewServeMux()
	rotas.HandleFunc("GET /saude", saude(d.Banco, d.Log))
	rotas.HandleFunc("POST /api/sessao", s.entrar)
	rotas.HandleFunc("DELETE /api/sessao", s.sair)
	rotas.HandleFunc("GET /api/sessao", logado(s.quemSou))
	rotas.HandleFunc("GET /api/etapas", logado(etapas))
	rotas.HandleFunc("POST /api/candidaturas", logado(c.criar))
	rotas.HandleFunc("GET /api/candidaturas", logado(c.listar))
	rotas.HandleFunc("GET /api/candidaturas/{id}", logado(c.buscar))
	rotas.HandleFunc("PUT /api/candidaturas/{id}", logado(c.editar))
	rotas.HandleFunc("DELETE /api/candidaturas/{id}", logado(c.apagar))
	rotas.HandleFunc("POST /api/candidaturas/{id}/etapas", logado(c.mudarEtapa))
	rotas.HandleFunc("GET /api/numeros", logado(verNumeros(d.Numeros, d.Log)))
	rotas.HandleFunc("GET /api/lembretes", logado(verLembretes(d.Lembretes, d.Log)))
	return registrar(d.Log, recuperar(d.Log, rotas))
}

// saude responde 200 com o banco no ar e 503 sem ele (para o Docker e o Vigil).
func saude(banco Pinger, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancelar := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancelar()
		if err := banco.Ping(ctx); err != nil {
			log.Warn("saúde: o banco não respondeu", "erro", err)
			responderJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "fora", "banco": "fora"})
			return
		}
		responderJSON(w, http.StatusOK, map[string]string{"status": "ok", "banco": "ok"})
	}
}

func verNumeros(servico *numeros.Servico, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := servico.Calcular(r.Context())
		if err != nil {
			log.Error("erro ao calcular os números", "erro", err)
			responderProblema(w, Problema{Titulo: "Erro interno", Status: http.StatusInternalServerError})
			return
		}
		responderJSON(w, http.StatusOK, n)
	}
}

// verLembretes: o que seria lembrado agora (a mensagem do Telegram junta estas).
func verLembretes(servico *lembretes.Servico, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lista, err := servico.Paradas(r.Context())
		if err != nil {
			log.Error("erro ao montar os lembretes", "erro", err)
			responderProblema(w, Problema{Titulo: "Erro interno", Status: http.StatusInternalServerError})
			return
		}
		responderJSON(w, http.StatusOK, lista)
	}
}

func responderJSON(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo)
}
