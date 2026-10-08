// Package contas cuida da conta, da senha e das sessões (login por cookie).
package contas

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Lakes777/pursuit/internal/banco/bd"
)

// Duracao da sessão sem uso; cada uso empurra o fim para frente (sessão deslizante).
const Duracao = 7 * 24 * time.Hour

// Só renova quando já passou 1 h da última renovação: evita uma escrita no banco por pedido.
const intervaloDeRenovacao = time.Hour

var (
	// ErrCredenciais: nome ou senha errados (a mesma mensagem para os dois, de propósito).
	ErrCredenciais = errors.New("nome ou senha incorretos")
	// ErrSemSessao: cookie ausente, desconhecido ou vencido.
	ErrSemSessao = errors.New("sem sessão")
)

// Usuario logado.
type Usuario struct {
	ID   int64  `json:"-"`
	Nome string `json:"usuario"`
}

// Servico de contas e sessões.
type Servico struct {
	pool  *pgxpool.Pool
	agora func() time.Time
}

// NovoServico usa o relógio de verdade.
func NovoServico(pool *pgxpool.Pool) *Servico {
	return &Servico{pool: pool, agora: time.Now}
}

var formatoDoNome = regexp.MustCompile(`^[a-z0-9._-]{1,50}$`)

// NormalizarNome: o nome não diferencia maiúsculas ("Andre" e "andre" são a mesma conta).
func NormalizarNome(nome string) (string, error) {
	nome = strings.ToLower(strings.TrimSpace(nome))
	if !formatoDoNome.MatchString(nome) {
		return "", errors.New("o nome tem de 1 a 50 caracteres: letras sem acento, números, ponto, hífen ou sublinhado")
	}
	return nome, nil
}

// ConferirSenhaNova: de 12 a 128 caracteres. Sem regra de "maiúscula e símbolo": tamanho
// é o que mais pesa (recomendação do NIST), e uma frase longa é fácil de lembrar.
func ConferirSenhaNova(senha string) error {
	switch n := utf8.RuneCountInString(senha); {
	case n < 12:
		return errors.New("a senha precisa de pelo menos 12 caracteres")
	case n > 128:
		return errors.New("a senha pode ter no máximo 128 caracteres")
	}
	return nil
}

// ErrOutraConta: o Pursuit tem uma conta só, e já existe uma com outro nome.
var ErrOutraConta = errors.New("já existe uma conta com outro nome (o Pursuit tem uma conta só)")

// DefinirSenha cria a conta ou troca a senha, e derruba todas as sessões dela (quem trocou a
// senha porque desconfiou de algo não quer ninguém mais logado). Conta única: as candidaturas
// não são separadas por pessoa, então uma segunda conta veria tudo da primeira.
func (s *Servico) DefinirSenha(ctx context.Context, nome, senha string) error {
	nome, err := NormalizarNome(nome)
	if err != nil {
		return err
	}
	if err := ConferirSenhaNova(senha); err != nil {
		return err
	}
	hash, err := hashDeSenha(senha)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := bd.New(tx)
		// A trava da tabela faz dois definir-senha com nomes diferentes ao mesmo tempo não
		// criarem duas contas
		if _, err := tx.Exec(ctx, "lock table usuario in share row exclusive mode"); err != nil {
			return err
		}
		outras, err := q.ContarOutrasContas(ctx, nome)
		if err != nil {
			return err
		}
		if outras > 0 {
			return ErrOutraConta
		}
		usuario, err := q.DefinirSenha(ctx, bd.DefinirSenhaParams{Nome: nome, SenhaHash: hash})
		if err != nil {
			return err
		}
		return q.ApagarSessoesDoUsuario(ctx, usuario.ID)
	})
}

// Entrar confere nome e senha e abre uma sessão. Devolve o token para o cookie: ele só
// existe aqui e no navegador; o banco guarda o sha256.
func (s *Servico) Entrar(ctx context.Context, nome, senha string) (string, Usuario, error) {
	q := bd.New(s.pool)
	nome, errNome := NormalizarNome(nome)
	usuario, err := q.BuscarUsuarioPorNome(ctx, nome)
	guardado := usuario.SenhaHash
	if errNome != nil || errors.Is(err, pgx.ErrNoRows) {
		guardado = hashDeMentira
	} else if err != nil {
		return "", Usuario{}, err
	}
	// Uma senha gigante faria o argon2 trabalhar à toa; a conferência acontece mesmo assim
	// com uma vazia, para o tempo ser o mesmo
	if utf8.RuneCountInString(senha) > 128 {
		senha = ""
	}
	certa, err := conferirSenhaNaVez(ctx, senha, guardado)
	if err != nil {
		return "", Usuario{}, err
	}
	if !certa || guardado == hashDeMentira {
		return "", Usuario{}, ErrCredenciais
	}

	token, err := novoToken()
	if err != nil {
		return "", Usuario{}, err
	}
	err = q.CriarSessao(ctx, bd.CriarSessaoParams{
		TokenHash: hashDoToken(token), UsuarioID: usuario.ID, ExpiraEm: s.agora().Add(Duracao),
	})
	if err != nil {
		return "", Usuario{}, err
	}
	return token, Usuario{ID: usuario.ID, Nome: usuario.Nome}, nil
}

// Sessao devolve quem é o dono do token e renova a sessão. renovada diz se o fim foi empurrado
// (aí a API manda o cookie de novo, com a validade nova).
func (s *Servico) Sessao(ctx context.Context, token string) (usuario Usuario, renovada bool, err error) {
	if token == "" {
		return Usuario{}, false, ErrSemSessao
	}
	q := bd.New(s.pool)
	agora := s.agora()
	hash := hashDoToken(token)
	linha, err := q.BuscarSessao(ctx, bd.BuscarSessaoParams{TokenHash: hash, Agora: agora})
	if errors.Is(err, pgx.ErrNoRows) {
		return Usuario{}, false, ErrSemSessao
	}
	if err != nil {
		return Usuario{}, false, err
	}
	usuario = Usuario{ID: linha.UsuarioID, Nome: linha.Nome}
	novoFim := agora.Add(Duracao)
	if novoFim.Sub(linha.ExpiraEm) < intervaloDeRenovacao {
		return usuario, false, nil
	}
	if err := q.RenovarSessao(ctx, bd.RenovarSessaoParams{TokenHash: hash, ExpiraEm: novoFim}); err != nil {
		return Usuario{}, false, err
	}
	return usuario, true, nil
}

// Sair apaga a sessão (não é erro se ela já não existia).
func (s *Servico) Sair(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return bd.New(s.pool).ApagarSessao(ctx, hashDoToken(token))
}

// LimparSessoesVencidas roda de hora em hora numa goroutine, até o ctx ser cancelado
// (no desligamento do servidor).
func (s *Servico) LimparSessoesVencidas(ctx context.Context, cada time.Duration, log *slog.Logger) {
	relogio := time.NewTicker(cada)
	defer relogio.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-relogio.C:
			apagadas, err := bd.New(s.pool).ApagarSessoesVencidas(ctx, s.agora())
			if err != nil && ctx.Err() == nil {
				log.Warn("limpeza de sessões falhou", "erro", err)
			} else if apagadas > 0 {
				log.Info("sessões vencidas apagadas", "quantidade", apagadas)
			}
		}
	}
}

// novoToken: 32 bytes aleatórios (256 bits), em base64 para caber no cookie.
func novoToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("gerar token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashDoToken: sha256 basta (sem sal nem argon2) porque o token já é aleatório e longo;
// não dá para adivinhar como uma senha.
func hashDoToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
