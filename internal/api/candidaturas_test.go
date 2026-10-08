package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Lakes777/pursuit/internal/candidaturas"
	"github.com/Lakes777/pursuit/internal/testebanco"
)

// novoServidor sobe a API de verdade (rotas + middlewares) com o banco dos testes limpo.
func novoServidor(t *testing.T) *httptest.Server {
	t.Helper()
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	servidor := httptest.NewServer(Novo(pool, candidaturas.NovoServico(pool), semLog))
	t.Cleanup(servidor.Close)
	return servidor
}

type resposta struct {
	status int
	corpo  string
	tipo   string
	local  string
}

func pedir(t *testing.T, metodo, url, corpo string) resposta {
	t.Helper()
	var leitor io.Reader
	if corpo != "" {
		leitor = strings.NewReader(corpo)
	}
	pedido, err := http.NewRequest(metodo, url, leitor)
	if err != nil {
		t.Fatal(err)
	}
	pedido.Header.Set("Content-Type", "application/json")
	r, err := http.DefaultClient.Do(pedido)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Body.Close() }()
	b, _ := io.ReadAll(r.Body)
	return resposta{r.StatusCode, string(b), r.Header.Get("Content-Type"), r.Header.Get("Location")}
}

func decodificar[T any](t *testing.T, corpo string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(corpo), &v); err != nil {
		t.Fatalf("%v: %s", err, corpo)
	}
	return v
}

func TestCriarBuscarEApagar(t *testing.T) {
	s := novoServidor(t)

	criada := pedir(t, "POST", s.URL+"/api/candidaturas",
		`{"empresa":"Nubank","vaga":"Back-end Go","link":"https://nubank.com/vaga","modalidade":"remoto"}`)
	if criada.status != http.StatusCreated || criada.local == "" {
		t.Fatalf("criar: %+v", criada)
	}
	d := decodificar[candidaturas.Detalhe](t, criada.corpo)
	if d.Etapa != candidaturas.Interesse || len(d.Historico) != 1 || *d.Modalidade != "remoto" {
		t.Errorf("criada = %+v", d)
	}

	buscada := pedir(t, "GET", s.URL+criada.local, "")
	if buscada.status != http.StatusOK || decodificar[candidaturas.Detalhe](t, buscada.corpo).Empresa != "Nubank" {
		t.Errorf("buscar: %+v", buscada)
	}

	if r := pedir(t, "DELETE", s.URL+criada.local, ""); r.status != http.StatusNoContent {
		t.Errorf("apagar: %+v", r)
	}
	if r := pedir(t, "GET", s.URL+criada.local, ""); r.status != http.StatusNotFound {
		t.Errorf("buscar depois de apagar: %+v", r)
	}
}

func TestErroDeValidacaoTrazOsCampos(t *testing.T) {
	s := novoServidor(t)

	r := pedir(t, "POST", s.URL+"/api/candidaturas", `{"empresa":"","vaga":"Go","link":"nubank.com"}`)

	if r.status != http.StatusUnprocessableEntity || !strings.HasPrefix(r.tipo, "application/problem+json") {
		t.Fatalf("r = %+v", r)
	}
	p := decodificar[Problema](t, r.corpo)
	if p.Campos["empresa"] != "obrigatório" || p.Campos["link"] == "" {
		t.Errorf("campos = %v", p.Campos)
	}
}

func TestJSONRuim(t *testing.T) {
	s := novoServidor(t)
	casos := map[string]struct {
		corpo  string
		status int
	}{
		"campo desconhecido": {`{"emrpesa":"Nubank","vaga":"Go"}`, http.StatusBadRequest},
		"dois objetos":       {`{"empresa":"a","vaga":"b"}{}`, http.StatusBadRequest},
		"não é JSON":         {`empresa=Nubank`, http.StatusBadRequest},
		"grande demais":      {`{"empresa":"a","vaga":"b","anotacoes":"` + strings.Repeat("x", 70_000) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			if r := pedir(t, "POST", s.URL+"/api/candidaturas", caso.corpo); r.status != caso.status {
				t.Errorf("status %d, esperado %d: %s", r.status, caso.status, r.corpo)
			}
		})
	}
}

func TestIdQueNaoENumeroDa404(t *testing.T) {
	s := novoServidor(t)
	for _, id := range []string{"abc", "0", "-1", "99999999999999999999"} {
		if r := pedir(t, "GET", s.URL+"/api/candidaturas/"+id, ""); r.status != http.StatusNotFound {
			t.Errorf("%s: %d", id, r.status)
		}
	}
}

func TestEditar(t *testing.T) {
	s := novoServidor(t)
	criada := pedir(t, "POST", s.URL+"/api/candidaturas", `{"empresa":"Nubank","vaga":"Go"}`)

	r := pedir(t, "PUT", s.URL+criada.local, `{"empresa":"Nu","vaga":"Go Pleno","fonte":"LinkedIn"}`)

	c := decodificar[candidaturas.Candidatura](t, r.corpo)
	if r.status != http.StatusOK || c.Empresa != "Nu" || *c.Fonte != "LinkedIn" {
		t.Errorf("r = %+v", r)
	}
	if r := pedir(t, "PUT", s.URL+criada.local, `{"empresa":"Nu","vaga":"Go","etapa":"proposta"}`); r.status != http.StatusBadRequest {
		t.Errorf("etapa no PUT deveria ser recusada (ela muda pela rota de etapas): %+v", r)
	}
	if r := pedir(t, "POST", s.URL+"/api/candidaturas", `{"empresa":"A\u0000B","vaga":"Go"}`); r.status != http.StatusUnprocessableEntity {
		t.Errorf("NUL: %+v", r)
	}
	if r := pedir(t, "PUT", s.URL+"/api/candidaturas/999", `{"empresa":"a","vaga":"b"}`); r.status != http.StatusNotFound {
		t.Errorf("inexistente: %d", r.status)
	}
}

func TestMudarEtapa(t *testing.T) {
	s := novoServidor(t)
	criada := pedir(t, "POST", s.URL+"/api/candidaturas", `{"empresa":"Nubank","vaga":"Go"}`)
	etapas := s.URL + criada.local + "/etapas"

	r := pedir(t, "POST", etapas, `{"etapa":"enviada","observacao":"pela Gupy"}`)
	d := decodificar[candidaturas.Detalhe](t, r.corpo)
	if r.status != http.StatusOK || d.Etapa != candidaturas.Enviada || len(d.Historico) != 2 {
		t.Errorf("mudar: %+v", r)
	}

	if r := pedir(t, "POST", etapas, `{"etapa":"enviada"}`); r.status != http.StatusConflict {
		t.Errorf("mesma etapa: %+v", r)
	}
	if r := pedir(t, "POST", etapas, `{"etapa":"sonho"}`); r.status != http.StatusUnprocessableEntity {
		t.Errorf("etapa desconhecida: %+v", r)
	}
	if r := pedir(t, "POST", etapas, `{"etapa":"triagem","em":"2999-01-01T00:00:00Z"}`); r.status != http.StatusUnprocessableEntity {
		t.Errorf("data no futuro: %+v", r)
	}
	if r := pedir(t, "POST", etapas, `{"etapa":"triagem","em":"ontem"}`); r.status != http.StatusBadRequest {
		t.Errorf("data que não é data: %+v", r)
	}
	if r := pedir(t, "POST", s.URL+"/api/candidaturas/999/etapas", `{"etapa":"triagem"}`); r.status != http.StatusNotFound {
		t.Errorf("inexistente: %+v", r)
	}
}

func TestListarComFiltros(t *testing.T) {
	s := novoServidor(t)
	pedir(t, "POST", s.URL+"/api/candidaturas", `{"empresa":"Nubank","vaga":"Go"}`)
	pedir(t, "POST", s.URL+"/api/candidaturas", `{"empresa":"iFood","vaga":"Java","etapa":"enviada"}`)

	todas := decodificar[[]candidaturas.Candidatura](t, pedir(t, "GET", s.URL+"/api/candidaturas", "").corpo)
	enviadas := decodificar[[]candidaturas.Candidatura](t, pedir(t, "GET", s.URL+"/api/candidaturas?etapa=enviada", "").corpo)
	busca := decodificar[[]candidaturas.Candidatura](t, pedir(t, "GET", s.URL+"/api/candidaturas?busca=nub", "").corpo)

	if len(todas) != 2 || len(enviadas) != 1 || enviadas[0].Empresa != "iFood" || len(busca) != 1 {
		t.Errorf("todas=%v enviadas=%v busca=%v", todas, enviadas, busca)
	}
	if r := pedir(t, "GET", s.URL+"/api/candidaturas?etapa=sonho", ""); r.status != http.StatusUnprocessableEntity {
		t.Errorf("etapa desconhecida: %d", r.status)
	}
}

func TestListaVaziaEUmArrayENaoNull(t *testing.T) {
	s := novoServidor(t)
	if r := pedir(t, "GET", s.URL+"/api/candidaturas", ""); strings.TrimSpace(r.corpo) != "[]" {
		t.Errorf("corpo = %q", r.corpo)
	}
}

func TestEtapas(t *testing.T) {
	s := novoServidor(t)

	lista := decodificar[[]candidaturas.InfoEtapa](t, pedir(t, "GET", s.URL+"/api/etapas", "").corpo)

	if len(lista) != 9 || lista[0].Etapa != candidaturas.Interesse || !lista[8].Final {
		t.Errorf("etapas = %+v", lista)
	}
}

func TestPanicViraErro500(t *testing.T) {
	quebrado := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("algo quebrou") })
	gravador := httptest.NewRecorder()

	registrar(semLog, recuperar(semLog, quebrado)).ServeHTTP(gravador, httptest.NewRequest("GET", "/", nil))

	if gravador.Code != http.StatusInternalServerError || !strings.Contains(gravador.Body.String(), "Erro interno") {
		t.Errorf("r = %d %s", gravador.Code, gravador.Body)
	}
}
