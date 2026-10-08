package telegram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const token = "123:abc_DEF-ghi"

func TestEnviaChatETexto(t *testing.T) {
	var recebido url.Values
	var caminho string
	falso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caminho = r.URL.Path
		corpo, _ := io.ReadAll(r.Body)
		recebido, _ = url.ParseQuery(string(corpo))
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer falso.Close()
	c, err := Novo(falso.URL, token, "42")
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Enviar(context.Background(), "Nubank & iFood <b>"); err != nil {
		t.Fatal(err)
	}

	if caminho != "/bot"+token+"/sendMessage" || recebido.Get("chat_id") != "42" ||
		recebido.Get("text") != "Nubank & iFood <b>" || recebido.Has("parse_mode") {
		t.Errorf("caminho = %s, recebido = %v", caminho, recebido)
	}
}

func TestRecusaTrazOMotivo(t *testing.T) {
	falso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: chat not found"}`))
	}))
	defer falso.Close()
	c, _ := Novo(falso.URL, token, "42")

	err := c.Enviar(context.Background(), "oi")

	if err == nil || !strings.Contains(err.Error(), "chat not found") {
		t.Errorf("err = %v", err)
	}
}

func TestErroDeRedeNaoMostraOToken(t *testing.T) {
	falso := httptest.NewServer(http.NotFoundHandler())
	endereco := falso.URL
	falso.Close() // ninguém responde mais nesse endereço
	c, _ := Novo(endereco, token, "42")

	err := c.Enviar(context.Background(), "oi")

	if err == nil || strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "abc_DEF") {
		t.Errorf("err = %v", err)
	}
}

func TestTempoEsgotadoNaoMostraOToken(t *testing.T) {
	// Demora mais que o limite do cliente (e não espera a desconexão: sem ler o corpo, o servidor
	// não percebe que o cliente foi embora, e o Close() do teste ficaria preso)
	falso := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer falso.Close()
	c, _ := Novo(falso.URL, token, "42")
	c.http.Timeout = 50 * time.Millisecond

	err := c.Enviar(context.Background(), "oi")

	if err == nil || strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "abc_DEF") {
		t.Errorf("err = %v", err)
	}
}

func TestConfiguracao(t *testing.T) {
	if _, err := Novo("", "", "42"); !errors.Is(err, ErrSemConfiguracao) {
		t.Errorf("sem token: %v", err)
	}
	if _, err := Novo("", token, " "); !errors.Is(err, ErrSemConfiguracao) {
		t.Errorf("sem chat: %v", err)
	}
	for _, ruim := range []string{`"123:abc"`, "123:abc def", "abc"} {
		_, err := Novo("", ruim, "42")
		if err == nil || strings.Contains(err.Error(), ruim) {
			t.Errorf("token %q: %v", ruim, err)
		}
	}
	c, err := Novo("", " "+token+" ", "42")
	if err != nil || c.endereco != "https://api.telegram.org" {
		t.Errorf("c = %+v, err = %v", c, err)
	}
}
