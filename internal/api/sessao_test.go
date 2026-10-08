package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/Lakes777/pursuit/internal/contas"
	"github.com/Lakes777/pursuit/internal/testebanco"
)

func TestSemLoginTudoDa401(t *testing.T) {
	s := novoServidorSemLogin(t, 10)
	rotas := [][2]string{
		{"GET", "/api/candidaturas"}, {"POST", "/api/candidaturas"}, {"GET", "/api/candidaturas/1"},
		{"PUT", "/api/candidaturas/1"}, {"DELETE", "/api/candidaturas/1"},
		{"POST", "/api/candidaturas/1/etapas"}, {"GET", "/api/etapas"}, {"GET", "/api/sessao"}, {"GET", "/api/numeros"}, {"GET", "/api/lembretes"},
		{"GET", "/api/lembretes/prazos"},
	}
	for _, rota := range rotas {
		if r := s.pedir(t, rota[0], s.URL+rota[1], `{}`); r.status != http.StatusUnauthorized {
			t.Errorf("%s %s: %d", rota[0], rota[1], r.status)
		}
	}
	if r := s.pedir(t, "GET", s.URL+"/saude", ""); r.status != http.StatusOK {
		t.Errorf("/saude deveria ser pública: %d", r.status)
	}
}

func TestLoginGravaCookieProtegido(t *testing.T) {
	s := novoServidorSemLogin(t, 10)
	if err := contas.NovoServico(testebanco.Pool(t)).DefinirSenha(context.Background(), "andre", senhaDeTeste); err != nil {
		t.Fatal(err)
	}

	// O nome não diferencia maiúsculas
	r := s.pedir(t, "POST", s.URL+"/api/sessao", `{"usuario":"Andre","senha":"`+senhaDeTeste+`"}`)

	if r.status != http.StatusOK || decodificar[map[string]string](t, r.corpo)["usuario"] != "andre" {
		t.Fatalf("login: %+v", r)
	}
	if len(r.cookies) != 1 {
		t.Fatalf("cookies = %v", r.cookies)
	}
	c := r.cookies[0]
	if c.Name != "pursuit_sessao" || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/api" ||
		len(c.Value) < 40 {
		t.Errorf("cookie = %+v", c)
	}
	if r := s.pedir(t, "GET", s.URL+"/api/sessao", ""); r.status != http.StatusOK {
		t.Errorf("quem sou: %+v", r)
	}
}

func TestLoginErradoNaoDizQualParteErrou(t *testing.T) {
	s := novoServidor(t)
	senhaErrada := s.pedir(t, "POST", s.URL+"/api/sessao", `{"usuario":"andre","senha":"outra senha qualquer"}`)
	nomeErrado := s.pedir(t, "POST", s.URL+"/api/sessao", `{"usuario":"ninguem","senha":"`+senhaDeTeste+`"}`)
	nomeInvalido := s.pedir(t, "POST", s.URL+"/api/sessao", `{"usuario":"nome com espaço","senha":"x"}`)

	for _, r := range []resposta{senhaErrada, nomeErrado, nomeInvalido} {
		if r.status != http.StatusUnauthorized || r.corpo != senhaErrada.corpo {
			t.Errorf("r = %+v", r)
		}
	}
}

func TestSairDerrubaASessao(t *testing.T) {
	s := novoServidor(t)

	r := s.pedir(t, "DELETE", s.URL+"/api/sessao", "")

	if r.status != http.StatusNoContent || len(r.cookies) != 1 {
		t.Fatalf("sair: %+v", r)
	}
	if r.cookies[0].Value != "" || r.cookies[0].MaxAge >= 0 {
		t.Errorf("o cookie deveria ser apagado: %+v", r.cookies[0])
	}
	if r := s.pedir(t, "GET", s.URL+"/api/candidaturas", ""); r.status != http.StatusUnauthorized {
		t.Errorf("depois de sair: %d", r.status)
	}
	// Sair sem estar logado também dá certo
	if r := s.pedir(t, "DELETE", s.URL+"/api/sessao", ""); r.status != http.StatusNoContent {
		t.Errorf("sair de novo: %d", r.status)
	}
}

func TestTokenAntigoNaoValeDepoisDeSair(t *testing.T) {
	s := novoServidor(t)
	antigo := s.cliente.Jar.Cookies(mustURL(t, s.URL+"/api"))[0]
	s.pedir(t, "DELETE", s.URL+"/api/sessao", "")

	pedido, _ := http.NewRequest("GET", s.URL+"/api/candidaturas", nil)
	pedido.AddCookie(&http.Cookie{Name: antigo.Name, Value: antigo.Value}) //nolint:gosec // cookie mandado pelo cliente do teste
	r, err := http.DefaultClient.Do(pedido)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Body.Close()

	if r.StatusCode != http.StatusUnauthorized {
		t.Errorf("token antigo: %d", r.StatusCode)
	}
}

func TestTrocarASenhaDerrubaAsSessoes(t *testing.T) {
	s := novoServidor(t)

	if err := contas.NovoServico(testebanco.Pool(t)).DefinirSenha(context.Background(), "andre", "outra frase bem longa"); err != nil {
		t.Fatal(err)
	}

	if r := s.pedir(t, "GET", s.URL+"/api/candidaturas", ""); r.status != http.StatusUnauthorized {
		t.Errorf("sessão sobreviveu à troca de senha: %d", r.status)
	}
}

func TestSessaoRenovadaRegravaOCookie(t *testing.T) {
	s := novoServidor(t)
	// Como se o login tivesse sido há 6 dias: falta 1 dia para vencer
	if _, err := testebanco.Pool(t).Exec(context.Background(), "update sessao set expira_em = now() + interval '1 day'"); err != nil {
		t.Fatal(err)
	}

	r := s.pedir(t, "GET", s.URL+"/api/candidaturas", "")

	if r.status != http.StatusOK || len(r.cookies) != 1 || r.cookies[0].Name != "pursuit_sessao" {
		t.Fatalf("r = %+v", r)
	}
	if !r.cookies[0].Expires.After(time.Now().Add(6 * 24 * time.Hour)) {
		t.Errorf("validade nova = %v", r.cookies[0].Expires)
	}
	if r := s.pedir(t, "GET", s.URL+"/api/candidaturas", ""); len(r.cookies) != 0 {
		t.Errorf("acabou de renovar: não precisava mandar de novo: %+v", r.cookies)
	}
}

func TestCookieSeguroEmProducao(t *testing.T) {
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)
	if err := contas.NovoServico(pool).DefinirSenha(context.Background(), "andre", senhaDeTeste); err != nil {
		t.Fatal(err)
	}
	servidor := httptest.NewServer(Novo(Dependencias{
		Banco: pool, Contas: contas.NovoServico(pool), Limite: contas.NovoLimite(10), CookieSeguro: true, Log: semLog,
	}))
	defer servidor.Close()

	r, err := http.Post(servidor.URL+"/api/sessao", "application/json",
		strings.NewReader(`{"usuario":"andre","senha":"`+senhaDeTeste+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Body.Close()

	if c := r.Cookies(); len(c) != 1 || !c[0].Secure {
		t.Errorf("cookies = %+v", c)
	}
}

func TestLimiteDeTentativas(t *testing.T) {
	s := novoServidorSemLogin(t, 3)
	for i := range 3 {
		if r := s.pedir(t, "POST", s.URL+"/api/sessao", `{"usuario":"a","senha":"b"}`); r.status != http.StatusUnauthorized {
			t.Fatalf("tentativa %d: %d", i+1, r.status)
		}
	}

	pedido, _ := http.NewRequest("POST", s.URL+"/api/sessao", nil)
	r, err := s.cliente.Do(pedido)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Body.Close()

	if r.StatusCode != http.StatusTooManyRequests || r.Header.Get("Retry-After") != "60" {
		t.Errorf("4ª tentativa: %d %v", r.StatusCode, r.Header)
	}
}

func TestIPDoPedidoAtrasDoProxy(t *testing.T) {
	proxies := []netip.Prefix{netip.MustParsePrefix("172.20.0.0/16")}
	pedido := func(remoto string, xff ...string) *http.Request {
		r := httptest.NewRequest("POST", "/api/sessao", nil)
		r.RemoteAddr = remoto
		for _, v := range xff {
			r.Header.Add("X-Forwarded-For", v)
		}
		return r
	}
	casos := []struct {
		nome string
		r    *http.Request
		ip   string
	}{
		{"sem proxy", pedido("200.1.2.3:5000"), "200.1.2.3"},
		{"de fora, com cabeçalho inventado", pedido("200.1.2.3:5000", "1.1.1.1"), "200.1.2.3"},
		{"pelo Caddy", pedido("172.20.0.5:4000", "200.1.2.3"), "200.1.2.3"},
		{"cliente mandou um falso antes", pedido("172.20.0.5:4000", "1.1.1.1, 200.1.2.3"), "200.1.2.3"},
		{"em dois cabeçalhos", pedido("172.20.0.5:4000", "1.1.1.1", "200.1.2.3"), "200.1.2.3"},
		{"pelo Caddy, sem cabeçalho", pedido("172.20.0.5:4000"), "172.20.0.5"},
		{"cabeçalho estragado", pedido("172.20.0.5:4000", "lixo"), "172.20.0.5"},
		{"IPv6", pedido("172.20.0.5:4000", "2804:14c::1"), "2804:14c::1"},
	}
	for _, c := range casos {
		if got := ipDoPedido(c.r, proxies); got.String() != c.ip {
			t.Errorf("%s: %s, esperado %s", c.nome, got, c.ip)
		}
	}
}
