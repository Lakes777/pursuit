package contas

import (
	"context"
	"errors"

	"github.com/Lakes777/pursuit/internal/banco/bd"
)

// NomeDoVisitante é a conta fixa da demonstração: quem visita entra nela sem senha.
const NomeDoVisitante = "visitante"

// semSenha é o "hash" da conta do visitante. Não é um hash argon2id válido, então nenhuma senha
// confere com ele (como o "!" do /etc/shadow, que trava a senha de uma conta): pelo formulário de
// senha, o visitante é tratado como um nome que não existe. Só a rota do visitante abre sessão nela.
const semSenha = "!sem-senha"

var (
	// ErrSemDemo: pediram a conta do visitante com o modo demonstração desligado.
	ErrSemDemo = errors.New("o modo demonstração está desligado")
	// ErrNomeReservado: na demonstração, "visitante" é a conta sem senha e não pode ganhar uma.
	ErrNomeReservado = errors.New(`na demonstração, o nome "visitante" é da conta de exemplo e não recebe senha`)
)

// ComVisitante devolve uma cópia do serviço com a conta do visitante ligada (PURSUIT_DEMO=true).
// Desligada (o padrão), EntrarComoVisitante recusa e nada do resto muda.
func (s *Servico) ComVisitante() *Servico {
	copia := *s
	copia.visitante = true
	return &copia
}

// EntrarComoVisitante abre uma sessão na conta do visitante, criando a conta se ainda não
// existe (a semeadura também cria; aqui é para a demonstração funcionar mesmo antes dela).
func (s *Servico) EntrarComoVisitante(ctx context.Context) (string, Usuario, error) {
	if !s.visitante {
		return "", Usuario{}, ErrSemDemo
	}
	q := bd.New(s.pool)
	usuario, err := q.DefinirSenha(ctx, bd.DefinirSenhaParams{Nome: NomeDoVisitante, SenhaHash: semSenha})
	if err != nil {
		return "", Usuario{}, err
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

// PrepararVisitante cria (ou mantém) a conta do visitante, sem senha utilizável, e derruba as
// sessões dela. A semeadura chama dentro da própria transação: quem estava vendo a demonstração
// entra de novo e encontra os dados de volta ao original, sem uma tela meio apagada.
func PrepararVisitante(ctx context.Context, q *bd.Queries) error {
	usuario, err := q.DefinirSenha(ctx, bd.DefinirSenhaParams{Nome: NomeDoVisitante, SenhaHash: semSenha})
	if err != nil {
		return err
	}
	return q.ApagarSessoesDoUsuario(ctx, usuario.ID)
}
