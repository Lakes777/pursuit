package candidaturas

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/Lakes777/pursuit/internal/banco/bd"
	"github.com/Lakes777/pursuit/internal/beacon"
)

// LinkDoBeacon é o link curto da candidatura: o endereço que vai no currículo mandado para a vaga.
type LinkDoBeacon struct {
	Codigo string `json:"codigo"`
	URL    string `json:"url"`
}

// Encurtador é o que CriarLinkDoBeacon usa do Beacon (o *beacon.Cliente serve; nos testes, um falso).
type Encurtador interface {
	CriarLink(ctx context.Context) (beacon.Link, error)
}

// ErrBeacon embrulha uma falha ao falar com o Beacon (fora do ar, chave recusada): a API responde
// 502, e não 500, porque o problema está do outro lado.
var ErrBeacon = errors.New("falha ao falar com o Beacon")

// CriarLinkDoBeacon dá à candidatura um link curto, se ela ainda não tem (se tem, devolve o mesmo:
// o link pode já estar num currículo enviado e não pode mudar).
//
// O pedido ao Beacon fica fora de transação, como o Telegram nos lembretes: segurar uma linha
// travada enquanto espera a rede travaria a candidatura junto. Em troca, dois pedidos ao mesmo tempo
// podem criar dois links no Beacon: só o primeiro a gravar fica, e o outro sobra lá sem uso (a chave
// do Pursuit só lê e cria links no Beacon, de propósito, então não tem como apagá-lo).
func (s *Servico) CriarLinkDoBeacon(ctx context.Context, id int64, encurtador Encurtador) (Candidatura, error) {
	q := bd.New(s.pool)
	linha, err := q.BuscarCandidatura(ctx, id)
	if err != nil {
		return Candidatura{}, naoEncontrada(err)
	}
	if linha.BeaconCodigo == nil {
		link, err := encurtador.CriarLink(ctx)
		if err != nil {
			return Candidatura{}, fmt.Errorf("%w: %w", ErrBeacon, err)
		}
		// Depois que o Beacon criou o link, gravar vale mais que respeitar o cancelamento: se a aba
		// fechar agora, o link fica guardado, em vez de sobrar no Beacon sem nunca aparecer aqui
		gravar := context.WithoutCancel(ctx)
		linha, err = q.DefinirBeacon(gravar, bd.DefinirBeaconParams{ID: id, BeaconCodigo: &link.Codigo, BeaconUrl: &link.UrlCurta})
		if errors.Is(err, pgx.ErrNoRows) {
			// Outro pedido gravou antes (ou a candidatura foi apagada no meio): vale o que está no banco
			linha, err = q.BuscarCandidatura(gravar, id)
		}
		if err != nil {
			return Candidatura{}, naoEncontrada(err)
		}
	}
	ultima, err := q.UltimaMudanca(ctx, id)
	if err != nil {
		return Candidatura{}, err
	}
	return converter(linha, ultima), nil
}

// linkDoBeacon: nil quando a candidatura não tem link (a migração garante os dois juntos).
func linkDoBeacon(linha bd.Candidatura) *LinkDoBeacon {
	if linha.BeaconCodigo == nil || linha.BeaconUrl == nil {
		return nil
	}
	return &LinkDoBeacon{Codigo: *linha.BeaconCodigo, URL: *linha.BeaconUrl}
}
