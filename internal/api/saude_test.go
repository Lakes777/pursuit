package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Lakes777/pursuit/internal/testebanco"
)

var semLog = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestSaudeComOBancoDeVerdade(t *testing.T) {
	servidor := httptest.NewServer(Novo(testebanco.Pool(t), nil, semLog))
	defer servidor.Close()

	resposta, err := http.Get(servidor.URL + "/saude")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resposta.Body.Close() }()
	corpo, _ := io.ReadAll(resposta.Body)

	if resposta.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resposta.StatusCode)
	}
	if !strings.Contains(string(corpo), `"banco":"ok"`) {
		t.Errorf("corpo = %s", corpo)
	}
}

type bancoFora struct{}

func (bancoFora) Ping(context.Context) error { return errors.New("conexão recusada") }

func TestSaudeSemBancoResponde503(t *testing.T) {
	gravador := httptest.NewRecorder()
	Novo(bancoFora{}, nil, semLog).ServeHTTP(gravador, httptest.NewRequest(http.MethodGet, "/saude", nil))

	if gravador.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", gravador.Code)
	}
	if !strings.Contains(gravador.Body.String(), `"banco":"fora"`) {
		t.Errorf("corpo = %s", gravador.Body)
	}
}

func TestSaudeSoAceitaGet(t *testing.T) {
	gravador := httptest.NewRecorder()
	Novo(bancoFora{}, nil, semLog).ServeHTTP(gravador, httptest.NewRequest(http.MethodPost, "/saude", nil))

	if gravador.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d", gravador.Code)
	}
}
