package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Problema é o formato de erro da RFC 9457 (application/problem+json), o mesmo do Vigil.
type Problema struct {
	Titulo  string `json:"title"`
	Status  int    `json:"status"`
	Detalhe string `json:"detail,omitempty"`
	// Campos: o problema de cada campo, nos erros de validação
	Campos map[string]string `json:"campos,omitempty"`
}

func responderProblema(w http.ResponseWriter, p Problema) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// tamanhoMaximo do corpo de um pedido: as anotações têm até 10 mil caracteres, com folga.
const tamanhoMaximo = 64 << 10

// lerJSON decodifica o corpo num valor só, recusando campos desconhecidos (um "emrpesa"
// digitado errado não pode ser ignorado em silêncio) e corpos grandes demais.
func lerJSON(w http.ResponseWriter, r *http.Request, destino any) bool {
	decodificador := json.NewDecoder(http.MaxBytesReader(w, r.Body, tamanhoMaximo))
	decodificador.DisallowUnknownFields()
	err := decodificador.Decode(destino)
	if err == nil && decodificador.Decode(&struct{}{}) != io.EOF {
		err = errors.New("depois do objeto JSON veio mais coisa")
	}
	if err == nil {
		return true
	}
	var grande *http.MaxBytesError
	if errors.As(err, &grande) {
		responderProblema(w, Problema{Titulo: "Pedido grande demais", Status: http.StatusRequestEntityTooLarge,
			Detalhe: fmt.Sprintf("o corpo pode ter no máximo %d KB", tamanhoMaximo>>10)})
		return false
	}
	responderProblema(w, Problema{Titulo: "JSON inválido", Status: http.StatusBadRequest, Detalhe: err.Error()})
	return false
}
