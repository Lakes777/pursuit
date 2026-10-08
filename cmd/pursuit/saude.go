package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// conferirSaude chama GET /saude na própria API (o healthcheck do Docker: a imagem não tem curl
// nem wget, só o binário). Sai com erro se a API não responder 200 em 3 s.
func conferirSaude(ctx context.Context, endereco string) error {
	host, porta, err := net.SplitHostPort(endereco)
	if err != nil {
		return err
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	ctx, cancelar := context.WithTimeout(ctx, 3*time.Second)
	defer cancelar()
	pedido, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, porta)+"/saude", nil)
	if err != nil {
		return err
	}
	resposta, err := http.DefaultClient.Do(pedido)
	if err != nil {
		return err
	}
	defer func() { _ = resposta.Body.Close() }()
	if resposta.StatusCode != http.StatusOK {
		return fmt.Errorf("a API respondeu %d", resposta.StatusCode)
	}
	return nil
}
