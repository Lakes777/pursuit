// Package config lê a configuração das variáveis de ambiente, com padrões para rodar no computador.
package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

// Config é tudo o que o Pursuit precisa para subir.
type Config struct {
	// Endereco é onde o servidor escuta. Localmente só 127.0.0.1; no contêiner, ":8095".
	Endereco string
	// BancoURL é a conexão com o Postgres (formato postgres://usuario:senha@host:porta/banco).
	BancoURL string
}

// Padrões para o compose.yaml da raiz (Postgres na porta 5435).
const (
	enderecoPadrao = "127.0.0.1:8095"
	bancoPadrao    = "postgres://pursuit:pursuit@localhost:5435/pursuit" //nolint:gosec // senha só do banco local do compose
)

// Carregar lê PURSUIT_ENDERECO e PURSUIT_BANCO_URL. A função de leitura vem por parâmetro
// (os.Getenv no programa) para os testes não precisarem mexer no ambiente de verdade.
func Carregar(ler func(string) string) (Config, error) {
	c := Config{
		Endereco: valorOuPadrao(ler("PURSUIT_ENDERECO"), enderecoPadrao),
		BancoURL: valorOuPadrao(ler("PURSUIT_BANCO_URL"), bancoPadrao),
	}
	u, err := url.Parse(c.BancoURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		// Sem repetir o valor no erro: a URL traz a senha do banco
		return Config{}, errors.New("PURSUIT_BANCO_URL inválida: use postgres://usuario:senha@host:porta/banco")
	}
	return c, nil
}

// DoAmbiente é Carregar com as variáveis do processo.
func DoAmbiente() (Config, error) {
	return Carregar(os.Getenv)
}

// Um valor só com espaços (comum num .env mal escrito) conta como vazio.
func valorOuPadrao(valor, padrao string) string {
	valor = strings.TrimSpace(valor)
	if valor == "" {
		return padrao
	}
	return valor
}
