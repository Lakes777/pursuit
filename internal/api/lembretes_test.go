package api

import (
	"net/http"
	"testing"

	"github.com/Lakes777/pursuit/internal/candidaturas"
	"github.com/Lakes777/pursuit/internal/lembretes"
)

func TestPrazosDosLembretes(t *testing.T) {
	s := novoServidor(t)

	r := s.pedir(t, "GET", s.URL+"/api/lembretes/prazos", "")

	prazos := decodificar[[]lembretes.PrazoDaEtapa](t, r.corpo)
	if r.status != http.StatusOK || len(prazos) != len(lembretes.Prazos) {
		t.Fatalf("r = %+v", r)
	}
	// Na ordem do processo, com o nome da tela e o mesmo número das regras
	if p := prazos[0]; p.Etapa != candidaturas.Interesse || p.Nome != "Interesse" ||
		p.Dias != lembretes.Prazos[candidaturas.Interesse].Dias {
		t.Errorf("primeira = %+v", p)
	}
	for _, p := range prazos {
		if p.Etapa == candidaturas.Contratado || p.Etapa == candidaturas.Recusada || p.Etapa == candidaturas.Desisti {
			t.Errorf("etapa final com prazo: %+v", p)
		}
	}
}
