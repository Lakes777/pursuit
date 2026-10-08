package candidaturas

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// Dados são os campos que a pessoa preenche (a etapa muda por outro caminho, com histórico).
type Dados struct {
	Empresa    string  `json:"empresa"`
	Vaga       string  `json:"vaga"`
	Link       *string `json:"link"`
	Fonte      *string `json:"fonte"`
	Modalidade *string `json:"modalidade"`
	Salario    *string `json:"salario"`
	Anotacoes  *string `json:"anotacoes"`
}

// ErroDeValidacao diz o problema de cada campo, para a tela mostrar ao lado dele.
type ErroDeValidacao struct {
	Campos map[string]string
}

func (e *ErroDeValidacao) Error() string {
	return fmt.Sprintf("dados inválidos: %v", e.Campos)
}

func (e *ErroDeValidacao) adicionar(campo, mensagem string) {
	if e.Campos == nil {
		e.Campos = map[string]string{}
	}
	e.Campos[campo] = mensagem
}

func (e *ErroDeValidacao) ouNil() error {
	if len(e.Campos) == 0 {
		return nil
	}
	return e
}

var modalidades = map[string]bool{"remoto": true, "hibrido": true, "presencial": true}

// normalizar tira os espaços das pontas, troca texto vazio por nil e confere os limites
// (os mesmos das constraints do banco, para o erro chegar com o nome do campo).
func (d Dados) normalizar() (Dados, error) {
	var erros ErroDeValidacao
	d.Empresa = strings.TrimSpace(d.Empresa)
	d.Vaga = strings.TrimSpace(d.Vaga)
	obrigatorio(&erros, "empresa", d.Empresa, 200)
	obrigatorio(&erros, "vaga", d.Vaga, 200)
	d.Link = opcional(&erros, "link", d.Link, 2000)
	d.Fonte = opcional(&erros, "fonte", d.Fonte, 100)
	d.Salario = opcional(&erros, "salario", d.Salario, 100)
	d.Anotacoes = opcional(&erros, "anotacoes", d.Anotacoes, 10000)
	d.Modalidade = opcional(&erros, "modalidade", d.Modalidade, 20)
	if d.Modalidade != nil {
		*d.Modalidade = strings.ToLower(*d.Modalidade)
		if !modalidades[*d.Modalidade] {
			erros.adicionar("modalidade", "use remoto, hibrido ou presencial")
		}
	}
	if d.Link != nil && erros.Campos["link"] == "" && !linkValido(*d.Link) {
		erros.adicionar("link", "use um endereço completo, começando com http:// ou https://")
	}
	return d, erros.ouNil()
}

// O Postgres não guarda o caractere NUL em text: sem esta checagem, ele viraria um erro 500.
const mensagemNUL = "tem um caractere inválido"

func obrigatorio(erros *ErroDeValidacao, campo, valor string, maximo int) {
	switch {
	case valor == "":
		erros.adicionar(campo, "obrigatório")
	case strings.ContainsRune(valor, 0):
		erros.adicionar(campo, mensagemNUL)
	case utf8.RuneCountInString(valor) > maximo:
		erros.adicionar(campo, fmt.Sprintf("no máximo %d caracteres", maximo))
	}
}

func opcional(erros *ErroDeValidacao, campo string, valor *string, maximo int) *string {
	if valor == nil {
		return nil
	}
	limpo := strings.TrimSpace(*valor)
	if limpo == "" {
		return nil
	}
	switch {
	case strings.ContainsRune(limpo, 0):
		erros.adicionar(campo, mensagemNUL)
	case utf8.RuneCountInString(limpo) > maximo:
		erros.adicionar(campo, fmt.Sprintf("no máximo %d caracteres", maximo))
	}
	return &limpo
}

func linkValido(link string) bool {
	u, err := url.Parse(link)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// Folga para o relógio do celular um pouco adiantado em relação ao servidor.
const folgaDoRelogio = time.Minute

// conferirData recusa uma data no futuro ou antes de "desde" (a última mudança registrada).
func conferirData(em, agora, desde time.Time) error {
	var erros ErroDeValidacao
	switch {
	case em.After(agora.Add(folgaDoRelogio)):
		erros.adicionar("em", "não pode ser no futuro")
	case em.Before(desde):
		erros.adicionar("em", "não pode ser antes da última mudança de etapa ("+
			desde.In(brasilia).Format("02/01/2006 15:04")+")")
	}
	return erros.ouNil()
}

var brasilia = carregarBrasilia()

func carregarBrasilia() *time.Location {
	local, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		// Sem a base de fusos (imagem mínima sem tzdata): Brasília não tem horário de verão desde 2019
		return time.FixedZone("BRT", -3*60*60)
	}
	return local
}
