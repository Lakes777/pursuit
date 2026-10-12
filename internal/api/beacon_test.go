package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Lakes777/pursuit/internal/beacon"
	"github.com/Lakes777/pursuit/internal/candidaturas"
)

const chaveDoBeacon = "chave-de-teste-com-mais-de-32-caracteres-000"

// beaconFalso responde como o Beacon; status, se diferente de 0, vale para todos os pedidos.
type beaconFalso struct {
	*httptest.Server
	status     atomic.Int32
	criados    atomic.Int32
	autorizado atomic.Bool
}

func novoBeaconFalso(t *testing.T) *beaconFalso {
	t.Helper()
	f := &beaconFalso{}
	f.autorizado.Store(true)
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+chaveDoBeacon {
			f.autorizado.Store(false)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if s := f.status.Load(); s != 0 {
			w.WriteHeader(int(s))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/links":
			f.criados.Add(1)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"codigo":"ab3k9xq","urlCurta":"https://b.exemplo/r/ab3k9xq"}`))
		case r.URL.Path == "/api/links/ab3k9xq/estatisticas":
			_, _ = w.Write([]byte(`{"codigo":"ab3k9xq","total":2,"robos":5,"porDia":[{"dia":"2026-10-12","cliques":2}]}`))
		case r.URL.Path == "/api/estatisticas":
			_, _ = w.Write([]byte(`{"total":2,"porLink":[{"codigo":"ab3k9xq","cliques":2}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func servidorComBeacon(t *testing.T) (*servidorDeTeste, *beaconFalso) {
	t.Helper()
	falso := novoBeaconFalso(t)
	cliente, err := beacon.Novo(falso.URL, chaveDoBeacon, "")
	if err != nil {
		t.Fatal(err)
	}
	return novoServidorComBeacon(t, cliente), falso
}

func (s *servidorDeTeste) criarCandidatura(t *testing.T) string {
	t.Helper()
	r := s.pedir(t, "POST", s.URL+"/api/candidaturas", `{"empresa":"Nubank","vaga":"Back-end Go"}`)
	if r.status != http.StatusCreated {
		t.Fatalf("criar: %+v", r)
	}
	return s.URL + r.local
}

func TestEstadoDoBeacon(t *testing.T) {
	desligado := novoServidor(t)
	if r := desligado.pedir(t, "GET", desligado.URL+"/api/beacon", ""); r.status != http.StatusOK || !strings.Contains(r.corpo, `"ligado":false`) {
		t.Errorf("desligado: %+v", r)
	}
	ligado, _ := servidorComBeacon(t)
	if r := ligado.pedir(t, "GET", ligado.URL+"/api/beacon", ""); !strings.Contains(r.corpo, `"ligado":true`) || !strings.Contains(r.corpo, `"dias":365`) {
		t.Errorf("ligado: %+v", r)
	}
}

func TestCriaLinkELeOsCliques(t *testing.T) {
	s, falso := servidorComBeacon(t)
	candidatura := s.criarCandidatura(t)

	r := s.pedir(t, "POST", candidatura+"/beacon", "")
	if r.status != http.StatusOK {
		t.Fatalf("criar link: %+v", r)
	}
	c := decodificar[candidaturas.Candidatura](t, r.corpo)
	if c.Beacon == nil || c.Beacon.URL != "https://b.exemplo/r/ab3k9xq" {
		t.Fatalf("beacon = %+v", c.Beacon)
	}
	// Repetir não cria outro no Beacon
	if r := s.pedir(t, "POST", candidatura+"/beacon", ""); r.status != http.StatusOK || falso.criados.Load() != 1 {
		t.Errorf("repetir: %+v, criados = %d", r, falso.criados.Load())
	}

	cliques := s.pedir(t, "GET", candidatura+"/beacon/cliques", "")
	if cliques.status != http.StatusOK {
		t.Fatalf("cliques: %+v", cliques)
	}
	n := decodificar[beacon.Cliques](t, cliques.corpo)
	if n.Total != 2 || len(n.PorDia) != 1 {
		t.Errorf("cliques = %+v", n)
	}

	todos := s.pedir(t, "GET", s.URL+"/api/beacon/cliques", "")
	if todos.status != http.StatusOK || !strings.Contains(todos.corpo, `"ab3k9xq":2`) {
		t.Errorf("todos: %+v", todos)
	}
	if !falso.autorizado.Load() {
		t.Error("algum pedido foi sem a chave")
	}
}

func TestChaveNuncaChegaAoNavegador(t *testing.T) {
	s, _ := servidorComBeacon(t)
	candidatura := s.criarCandidatura(t)
	for _, r := range []resposta{
		s.pedir(t, "GET", s.URL+"/api/beacon", ""),
		s.pedir(t, "POST", candidatura+"/beacon", ""),
		s.pedir(t, "GET", candidatura, ""),
		s.pedir(t, "GET", candidatura+"/beacon/cliques", ""),
	} {
		if strings.Contains(r.corpo, chaveDoBeacon) {
			t.Errorf("a chave apareceu: %s", r.corpo)
		}
	}
}

func TestSemLinkOuSemCandidatura(t *testing.T) {
	s, _ := servidorComBeacon(t)
	candidatura := s.criarCandidatura(t)

	if r := s.pedir(t, "GET", candidatura+"/beacon/cliques", ""); r.status != http.StatusNotFound || !strings.Contains(r.corpo, "ainda não tem link") {
		t.Errorf("sem link: %+v", r)
	}
	if r := s.pedir(t, "POST", s.URL+"/api/candidaturas/999999/beacon", ""); r.status != http.StatusNotFound {
		t.Errorf("sem candidatura: %+v", r)
	}
}

func TestBeaconDesligadoDa503(t *testing.T) {
	s := novoServidor(t)
	candidatura := s.criarCandidatura(t)

	for _, r := range []resposta{
		s.pedir(t, "POST", candidatura+"/beacon", ""),
		s.pedir(t, "GET", candidatura+"/beacon/cliques", ""),
		s.pedir(t, "GET", s.URL+"/api/beacon/cliques", ""),
	} {
		if r.status != http.StatusServiceUnavailable {
			t.Errorf("r = %+v", r)
		}
	}
}

func TestBeaconComProblemaDa502(t *testing.T) {
	s, falso := servidorComBeacon(t)
	candidatura := s.criarCandidatura(t)
	falso.status.Store(http.StatusInternalServerError)

	if r := s.pedir(t, "POST", candidatura+"/beacon", ""); r.status != http.StatusBadGateway {
		t.Errorf("criar: %+v", r)
	}
	if r := s.pedir(t, "GET", s.URL+"/api/beacon/cliques", ""); r.status != http.StatusBadGateway {
		t.Errorf("todos: %+v", r)
	}
}

func TestLinkApagadoNoBeaconDa404(t *testing.T) {
	s, falso := servidorComBeacon(t)
	candidatura := s.criarCandidatura(t)
	if r := s.pedir(t, "POST", candidatura+"/beacon", ""); r.status != http.StatusOK {
		t.Fatalf("criar: %+v", r)
	}
	falso.status.Store(http.StatusNotFound)

	r := s.pedir(t, "GET", candidatura+"/beacon/cliques", "")

	if r.status != http.StatusNotFound || !strings.Contains(r.corpo, "não existe mais no Beacon") {
		t.Errorf("r = %+v", r)
	}
}

func TestRotasDoBeaconPedemLogin(t *testing.T) {
	s := novoServidorSemLogin(t, 10)
	for _, rota := range []string{"/api/beacon", "/api/beacon/cliques", "/api/candidaturas/1/beacon/cliques"} {
		if r := s.pedir(t, "GET", s.URL+rota, ""); r.status != http.StatusUnauthorized {
			t.Errorf("%s: %+v", rota, r)
		}
	}
	if r := s.pedir(t, "POST", s.URL+"/api/candidaturas/1/beacon", ""); r.status != http.StatusUnauthorized {
		t.Errorf("POST: %+v", r)
	}
}

func TestErrosDoBeaconNuncaViram401Nem404(t *testing.T) {
	// Um 401 do Beacon (chave trocada) não pode chegar como 401: a página entenderia que a sessão do
	// Pursuit caiu e deslogaria. Um 404 ao criar é endereço errado, não "candidatura não existe".
	for _, status := range []int32{http.StatusUnauthorized, http.StatusNotFound} {
		s, falso := servidorComBeacon(t)
		candidatura := s.criarCandidatura(t)
		falso.status.Store(status)

		if r := s.pedir(t, "POST", candidatura+"/beacon", ""); r.status != http.StatusBadGateway {
			t.Errorf("%d ao criar: %+v", status, r)
		}
		if r := s.pedir(t, "GET", s.URL+"/api/beacon/cliques", ""); r.status != http.StatusBadGateway {
			t.Errorf("%d em todos os cliques: %+v", status, r)
		}
	}
}
