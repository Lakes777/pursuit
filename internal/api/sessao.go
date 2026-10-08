package api

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/Lakes777/pursuit/internal/contas"
)

const nomeDoCookie = "pursuit_sessao"

type rotasDeSessao struct {
	contas       *contas.Servico
	limite       *contas.Limite
	cookieSeguro bool
	proxies      []netip.Prefix
	log          *slog.Logger
}

type entradaLogin struct {
	Usuario string `json:"usuario"`
	Senha   string `json:"senha"`
}

func (s rotasDeSessao) entrar(w http.ResponseWriter, r *http.Request) {
	if !s.limite.Permitir(ipDoPedido(r, s.proxies), time.Now()) {
		w.Header().Set("Retry-After", "60")
		responderProblema(w, Problema{Titulo: "Tentativas demais", Status: http.StatusTooManyRequests,
			Detalhe: "espere um minuto e tente de novo"})
		return
	}
	var entrada entradaLogin
	if !lerJSON(w, r, &entrada) {
		return
	}
	token, usuario, err := s.contas.Entrar(r.Context(), entrada.Usuario, entrada.Senha)
	if errors.Is(err, contas.ErrCredenciais) {
		responderProblema(w, Problema{Titulo: "Nome ou senha incorretos", Status: http.StatusUnauthorized})
		return
	}
	if err != nil {
		s.log.Error("erro no login", "erro", err)
		responderProblema(w, Problema{Titulo: "Erro interno", Status: http.StatusInternalServerError})
		return
	}
	s.gravarCookie(w, token, time.Now().Add(contas.Duracao))
	responderJSON(w, http.StatusOK, usuario)
}

func (s rotasDeSessao) quemSou(w http.ResponseWriter, r *http.Request) {
	responderJSON(w, http.StatusOK, usuarioDoPedido(r))
}

func (s rotasDeSessao) sair(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(nomeDoCookie); err == nil {
		if err := s.contas.Sair(r.Context(), cookie.Value); err != nil {
			s.log.Error("erro ao sair", "erro", err)
			responderProblema(w, Problema{Titulo: "Erro interno", Status: http.StatusInternalServerError})
			return
		}
	}
	s.apagarCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// gravarCookie: HttpOnly (o JavaScript da página não lê, então um XSS não rouba a sessão),
// SameSite=Strict (outro site não consegue fazer o navegador mandar o cookie: proteção contra
// CSRF) e Secure em produção (só por HTTPS). Path=/api: a página em si não precisa dele.
func (s rotasDeSessao) gravarCookie(w http.ResponseWriter, token string, expira time.Time) {
	//nolint:gosec // Secure vem da configuração: ligado por padrão, desligado só sem HTTPS no computador
	http.SetCookie(w, &http.Cookie{
		Name: nomeDoCookie, Value: token, Path: "/api", Expires: expira,
		HttpOnly: true, Secure: s.cookieSeguro, SameSite: http.SameSiteStrictMode,
	})
}

// apagarCookie: MaxAge negativo manda o navegador esquecer o cookie na hora.
func (s rotasDeSessao) apagarCookie(w http.ResponseWriter) {
	//nolint:gosec // Secure vem da configuração, como em gravarCookie
	http.SetCookie(w, &http.Cookie{
		Name: nomeDoCookie, Value: "", Path: "/api", MaxAge: -1,
		HttpOnly: true, Secure: s.cookieSeguro, SameSite: http.SameSiteStrictMode,
	})
}

type chaveDoUsuario struct{}

// exigirLogin deixa o pedido passar só com uma sessão válida, e guarda o usuário no context.
func (s rotasDeSessao) exigirLogin(proximo http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(nomeDoCookie)
		token := ""
		if err == nil {
			token = cookie.Value
		}
		usuario, renovada, err := s.contas.Sessao(r.Context(), token)
		if errors.Is(err, contas.ErrSemSessao) {
			responderProblema(w, Problema{Titulo: "Faça login", Status: http.StatusUnauthorized})
			return
		}
		if err != nil {
			s.log.Error("erro ao conferir a sessão", "erro", err)
			responderProblema(w, Problema{Titulo: "Erro interno", Status: http.StatusInternalServerError})
			return
		}
		if renovada {
			s.gravarCookie(w, token, time.Now().Add(contas.Duracao))
		}
		proximo(w, r.WithContext(context.WithValue(r.Context(), chaveDoUsuario{}, usuario)))
	}
}

func usuarioDoPedido(r *http.Request) contas.Usuario {
	usuario, _ := r.Context().Value(chaveDoUsuario{}).(contas.Usuario)
	return usuario
}

// ipDoPedido devolve o IP de quem fez o pedido, para o limite de tentativas. Atrás do Caddy,
// toda conexão vem do IP dele, e o limite viraria um só para todo mundo: por isso, quando a
// conexão vem de um proxy confiável, vale o X-Forwarded-For, lido da direita para a esquerda
// (o último endereço que não é proxy é quem falou com o Caddy). De qualquer outra origem o
// cabeçalho é ignorado: senão cada tentativa poderia inventar um IP novo e fugir do limite.
func ipDoPedido(r *http.Request, proxies []netip.Prefix) netip.Addr {
	ip := ipDaConexao(r.RemoteAddr)
	if !confiavel(ip, proxies) {
		return ip
	}
	repassados := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(repassados) - 1; i >= 0; i-- {
		candidato, err := netip.ParseAddr(strings.TrimSpace(repassados[i]))
		if err != nil {
			return ip // cabeçalho estragado: fica o IP do proxy (um limite só, mas nunca um IP inventado)
		}
		candidato = candidato.Unmap()
		if !confiavel(candidato, proxies) {
			return candidato
		}
	}
	return ip
}

func ipDaConexao(remoto string) netip.Addr {
	host, _, err := net.SplitHostPort(remoto)
	if err != nil {
		host = remoto
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.IPv4Unspecified()
	}
	return ip.Unmap()
}

func confiavel(ip netip.Addr, proxies []netip.Prefix) bool {
	for _, p := range proxies {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
