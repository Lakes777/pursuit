// Package beacon fala com o Beacon (github.com/Lakes777/beacon), o meu encurtador de links: cria um
// link curto por candidatura, que leva ao portfólio, e lê quantas vezes ele foi aberto. A API do
// Beacon pede login; o Pursuit entra com a chave de integração (Authorization: Bearer), que fica só
// aqui no servidor e nunca vai para o navegador.
package beacon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DiasDeCliques: o período das contas de cliques. É o máximo que o Beacon aceita: uma candidatura
// pode durar meses, e o que importa é se o link foi aberto alguma vez.
const DiasDeCliques = 365

// DestinoPadrao: para onde os links levam, se PURSUIT_BEACON_DESTINO não disser outro.
const DestinoPadrao = "https://andrelagos.dev"

var (
	// ErrSemConfiguracao: faltou o endereço ou a chave (a integração fica desligada).
	ErrSemConfiguracao = errors.New("beacon sem endereço ou chave")
	// ErrNaoEncontrado: o Beacon não tem esse link (apagado lá pelo painel, por exemplo).
	ErrNaoEncontrado = errors.New("link não encontrado no beacon")
)

// Link é o que o Beacon devolve ao criar: o código e o endereço curto que vai no currículo.
type Link struct {
	Codigo   string `json:"codigo"`
	UrlCurta string `json:"urlCurta"`
}

// Cliques de um link no período (sem robôs: prévias do LinkedIn, do WhatsApp...).
type Cliques struct {
	Total  int            `json:"total"`
	PorDia []CliquesNoDia `json:"porDia"`
}

// CliquesNoDia: dia no formato 2026-10-12, no horário de Brasília.
type CliquesNoDia struct {
	Dia     string `json:"dia"`
	Cliques int    `json:"cliques"`
}

// Cliente do Beacon.
type Cliente struct {
	endereco string
	chave    string
	destino  string
	http     *http.Client
}

// Novo valida a configuração. Endereço e chave vazios = ErrSemConfiguracao; destino vazio = DestinoPadrao.
func Novo(endereco, chave, destino string) (*Cliente, error) {
	endereco, chave, destino = strings.TrimSpace(endereco), strings.TrimSpace(chave), strings.TrimSpace(destino)
	if endereco == "" || chave == "" {
		return nil, ErrSemConfiguracao
	}
	u, err := url.Parse(endereco)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("endereço do Beacon inválido: use um endereço completo, com https://")
	}
	// A chave vai em cada pedido: por http, ela passaria aberta pela rede. Só no próprio computador
	if u.Scheme == "http" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" {
		return nil, errors.New("endereço do Beacon com http: use https (a chave vai em cada pedido)")
	}
	if destino == "" {
		destino = DestinoPadrao
	}
	if d, err := url.Parse(destino); err != nil || (d.Scheme != "http" && d.Scheme != "https") || d.Host == "" {
		return nil, errors.New("destino dos links do Beacon inválido: use um endereço completo, com https://")
	}
	// O Beacon só aceita chaves a partir de 32 caracteres; a mesma regra aqui avisa no deploy, e não
	// no primeiro clique em "Criar link". O valor nunca vai para a mensagem de erro.
	if len(chave) < 32 || strings.ContainsAny(chave, " \"'") {
		return nil, errors.New("chave do Beacon em formato inválido (gere com openssl rand -hex 32)")
	}
	return &Cliente{endereco: strings.TrimRight(endereco, "/"), chave: chave, destino: destino,
		http: &http.Client{
			Timeout: 10 * time.Second,
			// Sem seguir redirecionamentos: um 301/308 transformaria o POST num GET sem corpo, e o
			// erro viraria um confuso "respondeu 200". Melhor falhar dizendo o código.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}}, nil
}

// CriarLink cria um link com código aleatório (o Beacon sorteia 7 caracteres): um código com o nome
// da empresa diria a qualquer um que visse o link para onde eu me candidatei.
func (c *Cliente) CriarLink(ctx context.Context) (Link, error) {
	var link Link
	err := c.pedir(ctx, http.MethodPost, "/api/links", map[string]string{"destino": c.destino}, http.StatusCreated, &link)
	if err == nil && (link.Codigo == "" || link.UrlCurta == "") {
		err = errors.New("beacon: resposta sem código ou endereço")
	}
	return link, err
}

// CliquesDoLink: os cliques de um link no período.
func (c *Cliente) CliquesDoLink(ctx context.Context, codigo string) (Cliques, error) {
	var cliques Cliques
	err := c.pedir(ctx, http.MethodGet,
		fmt.Sprintf("/api/links/%s/estatisticas?dias=%d", url.PathEscape(codigo), DiasDeCliques), nil, http.StatusOK, &cliques)
	return cliques, err
}

// CliquesPorLink: quantos cliques cada link do Beacon teve no período, num pedido só (para o quadro
// marcar as candidaturas abertas sem um pedido por cartão).
func (c *Cliente) CliquesPorLink(ctx context.Context) (map[string]int, error) {
	var resumo struct {
		PorLink []struct {
			Codigo  string `json:"codigo"`
			Cliques int    `json:"cliques"`
		} `json:"porLink"`
	}
	if err := c.pedir(ctx, http.MethodGet, fmt.Sprintf("/api/estatisticas?dias=%d", DiasDeCliques), nil, http.StatusOK, &resumo); err != nil {
		return nil, err
	}
	porLink := make(map[string]int, len(resumo.PorLink))
	for _, l := range resumo.PorLink {
		porLink[l.Codigo] = l.Cliques
	}
	return porLink, nil
}

func (c *Cliente) pedir(ctx context.Context, metodo, caminho string, corpo any, esperado int, resposta any) error {
	var leitor io.Reader
	if corpo != nil {
		bruto, err := json.Marshal(corpo)
		if err != nil {
			return err
		}
		leitor = bytes.NewReader(bruto)
	}
	pedido, err := http.NewRequestWithContext(ctx, metodo, c.endereco+caminho, leitor)
	if err != nil {
		return errors.New("beacon: endereço inválido")
	}
	pedido.Header.Set("Authorization", "Bearer "+c.chave)
	pedido.Header.Set("Accept", "application/json")
	if corpo != nil {
		pedido.Header.Set("Content-Type", "application/json")
	}
	r, err := c.http.Do(pedido)
	if err != nil {
		// A chave vai no cabeçalho, não na URL, mas o erro só leva o motivo, como no Telegram
		var erroDeURL *url.Error
		if errors.As(err, &erroDeURL) {
			err = erroDeURL.Err
		}
		return fmt.Errorf("beacon: %w", err)
	}
	defer func() { _ = r.Body.Close() }()
	switch r.StatusCode {
	case esperado:
	case http.StatusNotFound:
		return ErrNaoEncontrado
	case http.StatusUnauthorized:
		return errors.New("beacon recusou a chave (401): confira PURSUIT_BEACON_CHAVE e Integracao__Chave")
	default:
		detalhe, _ := io.ReadAll(io.LimitReader(r.Body, 500))
		return fmt.Errorf("beacon respondeu %d: %s", r.StatusCode, detalhe)
	}
	if resposta == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(resposta); err != nil {
		return fmt.Errorf("beacon: resposta inválida: %w", err)
	}
	return nil
}
