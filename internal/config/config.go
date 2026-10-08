// Package config lê a configuração das variáveis de ambiente, com padrões para rodar no computador.
package config

import (
	"errors"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config é tudo o que o Pursuit precisa para subir.
type Config struct {
	// Endereco é onde o servidor escuta. Localmente só 127.0.0.1; no contêiner, ":8095".
	Endereco string
	// BancoURL é a conexão com o Postgres (formato postgres://usuario:senha@host:porta/banco).
	BancoURL string
	// CookieSeguro: o cookie da sessão só vai por HTTPS. Ligado por padrão; desligar só no
	// computador, onde não há HTTPS (PURSUIT_COOKIE_SEGURO=false).
	CookieSeguro bool
	// LoginPorMinuto: tentativas de login por minuto por IP.
	LoginPorMinuto int
	// Telegram dos lembretes (o token do Sidekick e o meu chat). Sem eles, os lembretes ficam desligados.
	TelegramToken string
	TelegramChat  string
	TelegramURL   string // vazio = api.telegram.org
	// ProxiesConfiaveis: de onde o X-Forwarded-For vale (a rede do Caddy, em produção)
	ProxiesConfiaveis []netip.Prefix
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
	seguro, err := strconv.ParseBool(valorOuPadrao(ler("PURSUIT_COOKIE_SEGURO"), "true"))
	if err != nil {
		return Config{}, errors.New("PURSUIT_COOKIE_SEGURO deve ser true ou false")
	}
	c.CookieSeguro = seguro
	porMinuto, err := strconv.Atoi(valorOuPadrao(ler("PURSUIT_LOGIN_POR_MINUTO"), "10"))
	if err != nil || porMinuto < 1 {
		return Config{}, errors.New("PURSUIT_LOGIN_POR_MINUTO deve ser um número a partir de 1")
	}
	c.LoginPorMinuto = porMinuto
	c.TelegramToken = strings.TrimSpace(ler("PURSUIT_TELEGRAM_TOKEN"))
	c.TelegramChat = strings.TrimSpace(ler("PURSUIT_TELEGRAM_CHAT"))
	c.TelegramURL = strings.TrimSpace(ler("PURSUIT_TELEGRAM_URL"))
	for _, item := range strings.Split(ler("PURSUIT_PROXY_CONFIAVEL"), ",") {
		if item = strings.TrimSpace(item); item == "" {
			continue
		}
		prefixo, err := netip.ParsePrefix(item)
		if err != nil {
			ip, errIP := netip.ParseAddr(item)
			if errIP != nil {
				return Config{}, errors.New("PURSUIT_PROXY_CONFIAVEL: use faixas como 172.20.0.0/16, separadas por vírgula")
			}
			prefixo = netip.PrefixFrom(ip, ip.BitLen())
		}
		c.ProxiesConfiaveis = append(c.ProxiesConfiaveis, prefixo.Masked())
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
