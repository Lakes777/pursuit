package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConferirSaude(t *testing.T) {
	status := http.StatusOK
	falso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/saude" {
			t.Errorf("caminho = %s", r.URL.Path)
		}
		w.WriteHeader(status)
	}))
	defer falso.Close()
	endereco := strings.TrimPrefix(falso.URL, "http://")

	if err := conferirSaude(context.Background(), endereco); err != nil {
		t.Errorf("no ar: %v", err)
	}
	status = http.StatusServiceUnavailable
	if err := conferirSaude(context.Background(), endereco); err == nil {
		t.Error("503 deveria dar erro")
	}
	// ":porta" (como no contêiner) vira 127.0.0.1
	porta := endereco[strings.LastIndex(endereco, ":"):]
	status = http.StatusOK
	if err := conferirSaude(context.Background(), porta); err != nil {
		t.Errorf(":porta: %v", err)
	}
}
