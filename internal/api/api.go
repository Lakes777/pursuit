// Package api tem as rotas HTTP do Pursuit.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Pinger é o pedaço do banco que a rota de saúde usa (o *pgxpool.Pool serve).
type Pinger interface {
	Ping(ctx context.Context) error
}

// Novo monta as rotas. Usa o roteador da biblioteca padrão (Go 1.22+), que já entende
// método e parâmetros no caminho ("GET /api/candidaturas/{id}").
func Novo(banco Pinger, log *slog.Logger) http.Handler {
	rotas := http.NewServeMux()
	rotas.HandleFunc("GET /saude", saude(banco, log))
	return rotas
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

func responderJSON(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo)
}
