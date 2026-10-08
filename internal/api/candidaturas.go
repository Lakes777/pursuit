package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Lakes777/pursuit/internal/candidaturas"
)

// rotasDeCandidaturas traduz HTTP <-> serviço; as regras ficam no pacote candidaturas.
type rotasDeCandidaturas struct {
	servico *candidaturas.Servico
	log     *slog.Logger
}

// entradaNova é o corpo do POST: os dados e, opcionalmente, a etapa inicial e quando foi.
type entradaNova struct {
	candidaturas.Dados
	Etapa      candidaturas.Etapa `json:"etapa"`
	Observacao *string            `json:"observacao"`
	Em         *time.Time         `json:"em"`
}

func (c rotasDeCandidaturas) criar(w http.ResponseWriter, r *http.Request) {
	var entrada entradaNova
	if !lerJSON(w, r, &entrada) {
		return
	}
	detalhe, err := c.servico.Criar(r.Context(), entrada.Dados, candidaturas.NovaEtapa{
		Etapa: entrada.Etapa, Observacao: entrada.Observacao, Em: entrada.Em,
	})
	if err != nil {
		c.erro(w, r, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/candidaturas/%d", detalhe.ID))
	responderJSON(w, http.StatusCreated, detalhe)
}

func (c rotasDeCandidaturas) listar(w http.ResponseWriter, r *http.Request) {
	lista, err := c.servico.Listar(r.Context(), candidaturas.Filtro{
		Etapa: candidaturas.Etapa(r.URL.Query().Get("etapa")),
		Busca: r.URL.Query().Get("busca"),
	})
	if err != nil {
		c.erro(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, lista)
}

func (c rotasDeCandidaturas) buscar(w http.ResponseWriter, r *http.Request) {
	id, ok := lerID(w, r)
	if !ok {
		return
	}
	detalhe, err := c.servico.Buscar(r.Context(), id)
	if err != nil {
		c.erro(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, detalhe)
}

func (c rotasDeCandidaturas) editar(w http.ResponseWriter, r *http.Request) {
	id, ok := lerID(w, r)
	if !ok {
		return
	}
	var dados candidaturas.Dados
	if !lerJSON(w, r, &dados) {
		return
	}
	candidatura, err := c.servico.Editar(r.Context(), id, dados)
	if err != nil {
		c.erro(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, candidatura)
}

func (c rotasDeCandidaturas) apagar(w http.ResponseWriter, r *http.Request) {
	id, ok := lerID(w, r)
	if !ok {
		return
	}
	if err := c.servico.Apagar(r.Context(), id); err != nil {
		c.erro(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c rotasDeCandidaturas) mudarEtapa(w http.ResponseWriter, r *http.Request) {
	id, ok := lerID(w, r)
	if !ok {
		return
	}
	var nova candidaturas.NovaEtapa
	if !lerJSON(w, r, &nova) {
		return
	}
	detalhe, err := c.servico.MudarEtapa(r.Context(), id, nova)
	if err != nil {
		c.erro(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, detalhe)
}

func etapas(w http.ResponseWriter, _ *http.Request) {
	responderJSON(w, http.StatusOK, candidaturas.Etapas)
}

// lerID: um id que não é número (ou é <= 0) não existe, então é 404.
func lerID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		responderProblema(w, Problema{Titulo: "Candidatura não encontrada", Status: http.StatusNotFound})
		return 0, false
	}
	return id, true
}

// erro traduz os erros do serviço em respostas; o que não for conhecido vira 500 e vai
// para o log (sem detalhes na resposta).
func (c rotasDeCandidaturas) erro(w http.ResponseWriter, r *http.Request, err error) {
	var validacao *candidaturas.ErroDeValidacao
	switch {
	case errors.As(err, &validacao):
		responderProblema(w, Problema{Titulo: "Dados inválidos", Status: http.StatusUnprocessableEntity,
			Campos: validacao.Campos})
	case errors.Is(err, candidaturas.ErrNaoEncontrada):
		responderProblema(w, Problema{Titulo: "Candidatura não encontrada", Status: http.StatusNotFound})
	case errors.Is(err, candidaturas.ErrMesmaEtapa):
		responderProblema(w, Problema{Titulo: "A candidatura já está nessa etapa", Status: http.StatusConflict})
	default:
		c.log.Error("erro inesperado", "metodo", r.Method, "caminho", r.URL.Path, "erro", err)
		responderProblema(w, Problema{Titulo: "Erro interno", Status: http.StatusInternalServerError})
	}
}
