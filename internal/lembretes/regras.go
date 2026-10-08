// Package lembretes avisa pelo Telegram as candidaturas paradas há dias na mesma etapa.
package lembretes

import (
	"fmt"
	"strings"
	"time"

	"github.com/Lakes777/pursuit/internal/candidaturas"
)

// Prazos: depois de quantos dias parada na etapa a candidatura merece um lembrete, e o que
// dizer. Etapas finais (contratado, recusada, desisti) não têm prazo.
var Prazos = map[candidaturas.Etapa]Prazo{
	candidaturas.Interesse:  {10, "marcada como interesse há %s: ainda vai se candidatar?"},
	candidaturas.Enviada:    {7, "enviada há %s, sem resposta: vale mandar uma mensagem ao recrutador."},
	candidaturas.Triagem:    {5, "na triagem há %s, sem notícias."},
	candidaturas.Entrevista: {5, "entrevista há %s, sem retorno: vale perguntar como está o processo."},
	candidaturas.Tecnica:    {5, "etapa técnica há %s, sem retorno."},
	candidaturas.Proposta:   {2, "proposta há %s: falta responder."},
}

// Prazo de uma etapa.
type Prazo struct {
	Dias  int
	Frase string // com %s para "8 dias"
}

// PrazoDaEtapa é uma linha da tabela de prazos da tela (GET /api/lembretes/prazos).
type PrazoDaEtapa struct {
	Etapa candidaturas.Etapa `json:"etapa"`
	Nome  string             `json:"nome"`
	Dias  int                `json:"dias"`
}

// Regras é o que a tela de lembretes mostra (GET /api/lembretes/prazos): os prazos e os números
// das regras, todos daqui, para a tela nunca dizer uma coisa e o Go fazer outra.
type Regras struct {
	Prazos      []PrazoDaEtapa `json:"prazos"`
	RepetirDias int            `json:"repetirDias"`
	HoraInicio  int            `json:"horaInicio"`
	HoraFim     int            `json:"horaFim"`
}

// AsRegras junta a tabela de prazos e os números das regras.
func AsRegras() Regras {
	return Regras{Prazos: TabelaDePrazos(), RepetirDias: RepetirDias, HoraInicio: HoraInicio, HoraFim: HoraFim}
}

// TabelaDePrazos devolve os prazos na ordem das etapas (candidaturas.Etapas), só as que têm
// prazo. A tela lê daqui: os números ficam num lugar só, sem cópia no JavaScript.
func TabelaDePrazos() []PrazoDaEtapa {
	tabela := []PrazoDaEtapa{}
	for _, info := range candidaturas.Etapas {
		if prazo, ok := Prazos[info.Etapa]; ok {
			tabela = append(tabela, PrazoDaEtapa{Etapa: info.Etapa, Nome: info.Nome, Dias: prazo.Dias})
		}
	}
	return tabela
}

// RepetirDias: uma candidatura que continua parada volta a ser lembrada a cada tantos dias.
const RepetirDias = 7

// Limite da mensagem: o Telegram recusa textos acima de 4096 caracteres. Com folga para o
// cabeçalho e o "e mais N"; as que não couberem ficam para o dia seguinte.
const limiteDaMensagem = 3800

// Parada é uma candidatura que merece lembrete.
type Parada struct {
	CandidaturaID int64              `json:"candidaturaId"`
	HistoricoID   int64              `json:"-"`
	Empresa       string             `json:"empresa"`
	Vaga          string             `json:"vaga"`
	Etapa         candidaturas.Etapa `json:"etapa"`
	Desde         time.Time          `json:"desde"`
	Dias          int                `json:"dias"`
	Texto         string             `json:"texto"`
}

// precisaLembrar: parada há pelo menos o prazo da etapa, e não lembrada nos últimos 7 dias
// (por esta mesma mudança de etapa). Os dias são do calendário de Brasília, não horas corridas:
// enviada no dia 1º às 18h está "há 7 dias" no dia 8 às 9h, e o lembrete sai às 9h, não às 18h
// (ou, com a regra de uma mensagem por dia, só no dia 9).
func precisaLembrar(etapa candidaturas.Etapa, desde, ultimoLembrete, agora time.Time) (int, bool) {
	prazo, ok := Prazos[etapa]
	if !ok {
		return 0, false
	}
	dias := diasEntre(desde, agora)
	if dias < prazo.Dias {
		return dias, false
	}
	return dias, ultimoLembrete.IsZero() || ultimoLembrete.Year() < 2 || diasEntre(ultimoLembrete, agora) >= RepetirDias
}

// diasEntre conta as viradas de dia no calendário de Brasília (de 1º às 23h a 2 às 0h = 1 dia).
func diasEntre(de, ate time.Time) int {
	a, b := de.In(brasilia), ate.In(brasilia)
	inicio := time.Date(a.Year(), a.Month(), a.Day(), 12, 0, 0, 0, time.UTC)
	fim := time.Date(b.Year(), b.Month(), b.Day(), 12, 0, 0, 0, time.UTC)
	return int(fim.Sub(inicio).Hours() / 24)
}

func textoDaParada(empresa, vaga string, etapa candidaturas.Etapa, dias int) string {
	duracao := fmt.Sprintf("%d dias", dias)
	if dias == 1 {
		duracao = "1 dia"
	}
	return fmt.Sprintf("%s (%s): %s", empresa, vaga, fmt.Sprintf(Prazos[etapa].Frase, duracao))
}

// mensagem junta as paradas numa mensagem só, até o limite do Telegram (as mais antigas
// primeiro). Devolve também quantas couberam: só essas são gravadas como lembradas, e as
// outras entram na mensagem do dia seguinte.
func mensagem(paradas []Parada) (string, int) {
	var b strings.Builder
	if len(paradas) == 1 {
		b.WriteString("Pursuit: 1 candidatura parada.\n")
	} else {
		fmt.Fprintf(&b, "Pursuit: %d candidaturas paradas.\n", len(paradas))
	}
	couberam := 0
	for _, p := range paradas {
		linha := "\n- " + p.Texto
		// A primeira sempre entra (empresa e vaga têm no máximo 200 letras cada)
		if couberam > 0 && b.Len()+len(linha) > limiteDaMensagem {
			break
		}
		b.WriteString(linha)
		couberam++
	}
	if resto := len(paradas) - couberam; resto > 0 {
		fmt.Fprintf(&b, "\n\nE mais %d, que ficam para amanhã.", resto)
	}
	return b.String(), couberam
}
