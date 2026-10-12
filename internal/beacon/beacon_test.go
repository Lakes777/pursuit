package beacon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const chave = "chave-de-teste-com-mais-de-32-caracteres-000"

var ctx = context.Background()

func novo(t *testing.T, endereco string) *Cliente {
	t.Helper()
	c, err := Novo(endereco, chave, "")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNovoConfereAConfiguracao(t *testing.T) {
	casos := []struct {
		nome, endereco, chave, destino string
	}{
		{"sem endereço", "", chave, ""},
		{"sem chave", "https://beacon.exemplo", "", ""},
		{"endereço sem esquema", "beacon.exemplo", chave, ""},
		{"endereço ftp", "ftp://beacon.exemplo", chave, ""},
		{"http fora do computador", "http://beacon.exemplo", chave, ""},
		{"chave curta", "https://beacon.exemplo", "curta", ""},
		{"chave com aspas", "https://beacon.exemplo", `"` + chave + `"`, ""},
		{"destino sem esquema", "https://beacon.exemplo", chave, "lakes777.github.io"},
	}
	for _, caso := range casos {
		if _, err := Novo(caso.endereco, caso.chave, caso.destino); err == nil {
			t.Errorf("%s: aceitou", caso.nome)
		} else if strings.Contains(err.Error(), chave) {
			t.Errorf("%s: a chave apareceu no erro: %v", caso.nome, err)
		}
	}
	if _, err := Novo("", "", ""); !errors.Is(err, ErrSemConfiguracao) {
		t.Errorf("sem nada: err = %v", err)
	}
}

func TestCriarLinkMandaAChaveEODestino(t *testing.T) {
	var autorizacao, caminho, metodo string
	var corpo map[string]string
	falso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		autorizacao, caminho, metodo = r.Header.Get("Authorization"), r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&corpo)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"codigo":"ab3k9xq","destino":"https://lakes777.github.io","ativo":true,"urlCurta":"https://b.exemplo/r/ab3k9xq"}`))
	}))
	defer falso.Close()

	link, err := novo(t, falso.URL+"/").CriarLink(ctx)

	if err != nil {
		t.Fatal(err)
	}
	if link != (Link{Codigo: "ab3k9xq", UrlCurta: "https://b.exemplo/r/ab3k9xq"}) {
		t.Errorf("link = %+v", link)
	}
	// Sem código: o Beacon sorteia um (o nome da empresa não aparece no link)
	if autorizacao != "Bearer "+chave || metodo != http.MethodPost || caminho != "/api/links" ||
		len(corpo) != 1 || corpo["destino"] != DestinoPadrao {
		t.Errorf("autorizacao = %q, %s %s, corpo = %v", autorizacao, metodo, caminho, corpo)
	}
}

func TestDestinoConfigurado(t *testing.T) {
	var corpo map[string]string
	falso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&corpo)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"codigo":"x","urlCurta":"https://b.exemplo/r/x"}`))
	}))
	defer falso.Close()
	c, err := Novo(falso.URL, chave, "https://lakes777.github.io/#projetos")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.CriarLink(ctx); err != nil {
		t.Fatal(err)
	}
	if corpo["destino"] != "https://lakes777.github.io/#projetos" {
		t.Errorf("destino = %q", corpo["destino"])
	}
}

func TestCliquesDoLinkEPorLink(t *testing.T) {
	falso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/links/ab3k9xq/estatisticas":
			if r.URL.Query().Get("dias") != "365" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"codigo":"ab3k9xq","total":3,"robos":2,"porDia":[{"dia":"2026-10-11","cliques":1},{"dia":"2026-10-12","cliques":2}]}`))
		case "/api/estatisticas":
			_, _ = w.Write([]byte(`{"total":4,"robos":0,"porDia":[],"porLink":[{"codigo":"ab3k9xq","cliques":3},{"codigo":"zz","cliques":0}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer falso.Close()
	c := novo(t, falso.URL)

	cliques, err := c.CliquesDoLink(ctx, "ab3k9xq")
	if err != nil {
		t.Fatal(err)
	}
	if cliques.Total != 3 || len(cliques.PorDia) != 2 || cliques.PorDia[1] != (CliquesNoDia{"2026-10-12", 2}) {
		t.Errorf("cliques = %+v", cliques)
	}

	porLink, err := c.CliquesPorLink(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(porLink) != 2 || porLink["ab3k9xq"] != 3 || porLink["zz"] != 0 {
		t.Errorf("porLink = %v", porLink)
	}
}

func TestRespostasDeErro(t *testing.T) {
	status := http.StatusNotFound
	falso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"title":"motivo do beacon"}`))
	}))
	defer falso.Close()
	c := novo(t, falso.URL)

	if _, err := c.CliquesDoLink(ctx, "sumiu"); !errors.Is(err, ErrNaoEncontrado) {
		t.Errorf("404: err = %v", err)
	}
	status = http.StatusUnauthorized
	if _, err := c.CriarLink(ctx); err == nil || !strings.Contains(err.Error(), "PURSUIT_BEACON_CHAVE") {
		t.Errorf("401: err = %v", err)
	}
	status = http.StatusInternalServerError
	if _, err := c.CriarLink(ctx); err == nil || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "motivo do beacon") {
		t.Errorf("500: err = %v", err)
	}
	// 201 sem código não pode virar um link vazio guardado na candidatura
	status = http.StatusCreated
	if _, err := c.CriarLink(ctx); err == nil {
		t.Error("201 sem código: aceitou")
	}
}

func TestErroDeRedeNaoMostraAChave(t *testing.T) {
	// Um servidor que demora mais que o prazo do pedido
	lento := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
	}))
	defer lento.Close()
	c := novo(t, lento.URL)
	comPrazo, cancelar := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancelar()

	_, err := c.CriarLink(comPrazo)

	if err == nil || strings.Contains(err.Error(), chave) {
		t.Errorf("err = %v", err)
	}
}

func TestNaoSegueRedirecionamento(t *testing.T) {
	// Um http -> https do proxy: seguir faria o POST virar GET sem corpo (e mandaria a chave de novo)
	var pedidos atomic.Int32
	falso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos.Add(1)
		http.Redirect(w, r, "/outro-lugar", http.StatusPermanentRedirect)
	}))
	defer falso.Close()

	_, err := novo(t, falso.URL).CriarLink(ctx)

	if err == nil || !strings.Contains(err.Error(), "308") || pedidos.Load() != 1 {
		t.Errorf("err = %v, pedidos = %d", err, pedidos.Load())
	}
}
