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
	if !c.CookieSeguro || c.LoginPorMinuto != 10 {
		t.Errorf("c = %+v", c)
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

func TestNumerosEBooleanosInvalidos(t *testing.T) {
	for nome, valor := range map[string]string{
		"PURSUIT_COOKIE_SEGURO": "talvez", "PURSUIT_LOGIN_POR_MINUTO": "0",
	} {
		if _, err := Carregar(func(n string) string {
			if n == nome {
				return valor
			}
			return ""
		}); err == nil {
			t.Errorf("aceitou %s=%s", nome, valor)
		}
	}
}

func TestProxiesConfiaveis(t *testing.T) {
	c, err := Carregar(func(n string) string {
		if n == "PURSUIT_PROXY_CONFIAVEL" {
			return " 172.20.0.0/16, 10.0.0.7 "
		}
		return ""
	})
	if err != nil || len(c.ProxiesConfiaveis) != 2 || c.ProxiesConfiaveis[0].String() != "172.20.0.0/16" ||
		c.ProxiesConfiaveis[1].String() != "10.0.0.7/32" {
		t.Errorf("c = %+v, err = %v", c.ProxiesConfiaveis, err)
	}
	if _, err := Carregar(func(n string) string {
		if n == "PURSUIT_PROXY_CONFIAVEL" {
			return "caddy"
		}
		return ""
	}); err == nil {
		t.Error("aceitou um nome no lugar de uma faixa")
	}
}

func TestVariaveisTemPrioridade(t *testing.T) {
	ambiente := map[string]string{
		"PURSUIT_ENDERECO":         ":9000",
		"PURSUIT_BANCO_URL":        "postgres://outro@banco/outro",
		"PURSUIT_COOKIE_SEGURO":    "false",
		"PURSUIT_LOGIN_POR_MINUTO": "3",
	}
	c, err := Carregar(func(nome string) string { return ambiente[nome] })
	if err != nil {
		t.Fatal(err)
	}
	if c.Endereco != ":9000" || c.BancoURL != "postgres://outro@banco/outro" || c.CookieSeguro ||
		c.LoginPorMinuto != 3 {
		t.Errorf("c = %+v", c)
	}
}
