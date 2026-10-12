package api

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Lakes777/pursuit/internal/candidaturas"
	"github.com/Lakes777/pursuit/internal/contas"
	"github.com/Lakes777/pursuit/internal/lembretes"
	"github.com/Lakes777/pursuit/internal/numeros"
	"github.com/Lakes777/pursuit/internal/testebanco"
)

// novoServidorDemo: como no main com PURSUIT_DEMO=true (o serviço de contas com o visitante).
func novoServidorDemo(t *testing.T, loginPorMinuto int) *servidorDeTeste {
	t.Helper()
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	servidor := httptest.NewServer(Novo(Dependencias{
		Banco: pool, Candidaturas: candidaturas.NovoServico(pool), Contas: contas.NovoServico(pool).ComVisitante(),
		Numeros: numeros.NovoServico(pool), Lembretes: lembretes.NovoServico(pool, nil),
		Limite: contas.NovoLimite(loginPorMinuto), Demo: true, Log: semLog,
	}))
	t.Cleanup(servidor.Close)
	jarra, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &servidorDeTeste{Server: servidor, cliente: &http.Client{Jar: jarra}}
}

func TestEstadoDaDemo(t *testing.T) {
	desligada := novoServidorSemLogin(t, 10)
	r := desligada.pedir(t, "GET", desligada.URL+"/api/demo", "")
	if v, ok := decodificar[map[string]bool](t, r.corpo)["demo"]; r.status != http.StatusOK || !ok || v {
		t.Errorf("desligada deveria dizer demo=false: %+v", r)
	}

	ligada := novoServidorDemo(t, 10)
	r = ligada.pedir(t, "GET", ligada.URL+"/api/demo", "")
	if r.status != http.StatusOK || !decodificar[map[string]bool](t, r.corpo)["demo"] {
		t.Errorf("ligada: %+v", r)
	}
}

func TestVisitanteComDemoDesligada(t *testing.T) {
	s := novoServidorSemLogin(t, 1)

	r := s.pedir(t, "POST", s.URL+"/api/sessao/visitante", "")

	if r.status != http.StatusNotFound || !strings.HasPrefix(r.tipo, "application/problem+json") || len(r.cookies) != 0 {
		t.Errorf("r = %+v", r)
	}
	// O 404 não gasta a tentativa do limite (1 por minuto aqui): o login continua possível
	if r := s.pedir(t, "POST", s.URL+"/api/sessao", `{"usuario":"a","senha":"b"}`); r.status != http.StatusUnauthorized {
		t.Errorf("login depois do 404: %d", r.status)
	}
	var contasCriadas int
	if err := testebanco.Pool(t).QueryRow(context.Background(), "select count(*) from usuario").Scan(&contasCriadas); err != nil {
		t.Fatal(err)
	}
	if contasCriadas != 0 {
		t.Errorf("%d contas criadas com a demo desligada", contasCriadas)
	}
}

func TestEntrarComoVisitante(t *testing.T) {
	s := novoServidorDemo(t, 10)

	r := s.pedir(t, "POST", s.URL+"/api/sessao/visitante", "")

	if r.status != http.StatusOK || decodificar[map[string]string](t, r.corpo)["usuario"] != contas.NomeDoVisitante {
		t.Fatalf("r = %+v", r)
	}
	if len(r.cookies) != 1 || !r.cookies[0].HttpOnly || r.cookies[0].SameSite != http.SameSiteStrictMode || r.cookies[0].Path != "/api" {
		t.Errorf("cookie = %+v", r.cookies)
	}
	// Com o cookie, o resto da API funciona como para mim
	if r := s.pedir(t, "GET", s.URL+"/api/candidaturas", ""); r.status != http.StatusOK {
		t.Errorf("listar como visitante: %d", r.status)
	}
	if r := s.pedir(t, "GET", s.URL+"/api/sessao", ""); decodificar[map[string]string](t, r.corpo)["usuario"] != contas.NomeDoVisitante {
		t.Errorf("quem sou: %s", r.corpo)
	}
	// Pelo formulário de senha, o visitante não entra (é tratado como um nome que não existe)
	if r := s.pedir(t, "POST", s.URL+"/api/sessao", `{"usuario":"visitante","senha":""}`); r.status != http.StatusUnauthorized {
		t.Errorf("visitante com senha: %d", r.status)
	}
}

func TestVisitanteUsaOLimiteDoLogin(t *testing.T) {
	s := novoServidorDemo(t, 3)
	// Duas tentativas de senha e uma de visitante esgotam as 3 do minuto: o limite é o mesmo
	for range 2 {
		s.pedir(t, "POST", s.URL+"/api/sessao", `{"usuario":"a","senha":"b"}`)
	}
	if r := s.pedir(t, "POST", s.URL+"/api/sessao/visitante", ""); r.status != http.StatusOK {
		t.Fatalf("3ª tentativa: %d", r.status)
	}

	r := s.pedir(t, "POST", s.URL+"/api/sessao/visitante", "")

	if r.status != http.StatusTooManyRequests || len(r.cookies) != 0 {
		t.Errorf("4ª tentativa: %+v", r)
	}
	if r := s.pedir(t, "POST", s.URL+"/api/sessao", `{"usuario":"a","senha":"b"}`); r.status != http.StatusTooManyRequests {
		t.Errorf("login depois do limite: %d", r.status)
	}
}
