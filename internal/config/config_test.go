package config

import "testing"

func TestPadroes(t *testing.T) {
	c, err := Carregar(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if c.Endereco != "127.0.0.1:8095" {
		t.Errorf("Endereco = %q", c.Endereco)
	}
	if c.BancoURL != bancoPadrao {
		t.Errorf("BancoURL = %q", c.BancoURL)
	}
}

func TestEspacosContamComoVazio(t *testing.T) {
	c, err := Carregar(func(string) string { return "   " })
	if err != nil {
		t.Fatal(err)
	}
	if c.Endereco != enderecoPadrao || c.BancoURL != bancoPadrao {
		t.Errorf("c = %+v", c)
	}
}

func TestBancoURLInvalida(t *testing.T) {
	for _, url := range []string{"mysql://u:s@h/b", "localhost:5435", "postgres://"} {
		_, err := Carregar(func(nome string) string {
			if nome == "PURSUIT_BANCO_URL" {
				return url
			}
			return ""
		})
		if err == nil {
			t.Errorf("aceitou %q", url)
		}
	}
}

func TestVariaveisTemPrioridade(t *testing.T) {
	ambiente := map[string]string{
		"PURSUIT_ENDERECO":  ":9000",
		"PURSUIT_BANCO_URL": "postgres://outro@banco/outro",
	}
	c, err := Carregar(func(nome string) string { return ambiente[nome] })
	if err != nil {
		t.Fatal(err)
	}
	if c.Endereco != ":9000" || c.BancoURL != "postgres://outro@banco/outro" {
		t.Errorf("c = %+v", c)
	}
}
