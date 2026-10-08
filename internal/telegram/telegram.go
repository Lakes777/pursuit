// Package telegram manda mensagens pela API de bots do Telegram, com o token do Sidekick: a
// mensagem chega na conversa com o bot, como os avisos do Vigil.
package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// O formato dos tokens do BotFather. Também barra aspas e espaços de um .env mal escrito.
var formatoDoToken = regexp.MustCompile(`^\d+:[A-Za-z0-9_-]+$`)

// ErrSemConfiguracao: faltou o token ou o chat.
var ErrSemConfiguracao = errors.New("telegram sem token ou chat")

// Cliente do Telegram.
type Cliente struct {
	endereco string
	token    string
	chat     string
	http     *http.Client
}

// Novo valida o token e o chat. Endereço vazio = a API de verdade.
func Novo(endereco, token, chat string) (*Cliente, error) {
	token, chat = strings.TrimSpace(token), strings.TrimSpace(chat)
	if token == "" || chat == "" {
		return nil, ErrSemConfiguracao
	}
	if !formatoDoToken.MatchString(token) {
		// O valor nunca vai para a mensagem de erro: é a senha do bot
		return nil, errors.New("token do Telegram em formato inválido")
	}
	if endereco == "" {
		endereco = "https://api.telegram.org"
	}
	return &Cliente{endereco: strings.TrimRight(endereco, "/"), token: token, chat: chat,
		http: &http.Client{Timeout: 10 * time.Second}}, nil
}

// Enviar manda o texto (sem formatação: o que a pessoa digitou não vira HTML nem Markdown).
func (c *Cliente) Enviar(ctx context.Context, texto string) error {
	corpo := url.Values{"chat_id": {c.chat}, "text": {texto}}.Encode()
	pedido, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endereco+"/bot"+c.token+"/sendMessage",
		strings.NewReader(corpo))
	if err != nil {
		return errors.New("telegram: endereço inválido")
	}
	pedido.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resposta, err := c.http.Do(pedido)
	if err != nil {
		// O erro do http traz a URL inteira, com o token: devolve só o motivo
		var erroDeURL *url.Error
		if errors.As(err, &erroDeURL) {
			err = erroDeURL.Err
		}
		return fmt.Errorf("telegram: %w", err)
	}
	defer func() { _ = resposta.Body.Close() }()
	if resposta.StatusCode != http.StatusOK {
		// O corpo explica o erro (ex.: "chat not found") e não tem o token
		detalhe, _ := io.ReadAll(io.LimitReader(resposta.Body, 500))
		return fmt.Errorf("telegram recusou (%d): %s", resposta.StatusCode, detalhe)
	}
	return nil
}
