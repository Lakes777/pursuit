package api

import (
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Um middleware embrulha um http.Handler: faz algo antes e/ou depois do pedido.

// registrar escreve uma linha no log por pedido: método, caminho, status e quanto tempo levou.
func registrar(log *slog.Logger, proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		gravador := &comStatus{ResponseWriter: w, status: http.StatusOK}
		proximo.ServeHTTP(gravador, r)
		// A conferência de saúde do Docker (a cada 30 s) só aparece no log quando falha
		if r.URL.Path == "/saude" && gravador.status == http.StatusOK {
			return
		}
		log.Info("pedido", "metodo", r.Method, "caminho", r.URL.Path, "status", gravador.status,
			"ms", time.Since(inicio).Milliseconds())
	})
}

// recuperar transforma um panic num 500, em vez de derrubar a conexão sem resposta.
func recuperar(log *slog.Logger, proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if erro := recover(); erro != nil {
				if e, ok := erro.(error); ok && errors.Is(e, http.ErrAbortHandler) {
					panic(erro) // o próprio servidor usa esse panic para abortar a resposta
				}
				log.Error("panic", "metodo", r.Method, "caminho", r.URL.Path, "erro", erro)
				responderProblema(w, Problema{Titulo: "Erro interno", Status: http.StatusInternalServerError})
			}
		}()
		proximo.ServeHTTP(w, r)
	})
}

// comStatus guarda o status que o handler escreveu, para o log.
type comStatus struct {
	http.ResponseWriter
	status int
}

func (c *comStatus) WriteHeader(status int) {
	c.status = status
	c.ResponseWriter.WriteHeader(status)
}

// Unwrap deixa o http.ResponseController achar o ResponseWriter original (Flush etc.).
func (c *comStatus) Unwrap() http.ResponseWriter {
	return c.ResponseWriter
}
