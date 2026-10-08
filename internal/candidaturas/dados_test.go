package candidaturas

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func texto(s string) *string { return &s }

func TestNormalizarLimpaEspacosEVazios(t *testing.T) {
	d, err := Dados{
		Empresa: "  Nubank ", Vaga: "Back-end\n", Link: texto(" https://vagas.nubank.com/1 "),
		Fonte: texto("   "), Modalidade: texto(" Remoto "), Anotacoes: texto(""),
	}.normalizar()
	if err != nil {
		t.Fatal(err)
	}
	if d.Empresa != "Nubank" || d.Vaga != "Back-end" || *d.Link != "https://vagas.nubank.com/1" {
		t.Errorf("d = %+v", d)
	}
	if d.Fonte != nil || d.Anotacoes != nil {
		t.Errorf("texto vazio deveria virar nil: %+v", d)
	}
	if *d.Modalidade != "remoto" {
		t.Errorf("modalidade = %q", *d.Modalidade)
	}
}

func campos(t *testing.T, err error) map[string]string {
	t.Helper()
	var validacao *ErroDeValidacao
	if !errors.As(err, &validacao) {
		t.Fatalf("esperava erro de validação, veio %v", err)
	}
	return validacao.Campos
}

func TestNormalizarApontaCadaCampo(t *testing.T) {
	_, err := Dados{
		Empresa: " ", Vaga: strings.Repeat("a", 201), Link: texto("vagas.com/1"),
		Modalidade: texto("lua"), Salario: texto(strings.Repeat("9", 101)),
	}.normalizar()

	c := campos(t, err)
	esperado := map[string]string{
		"empresa": "obrigatório", "vaga": "no máximo 200 caracteres",
		"link":       "use um endereço completo, começando com http:// ou https://",
		"modalidade": "use remoto, hibrido ou presencial", "salario": "no máximo 100 caracteres",
	}
	for campo, mensagem := range esperado {
		if c[campo] != mensagem {
			t.Errorf("%s = %q, esperado %q", campo, c[campo], mensagem)
		}
	}
	if len(c) != len(esperado) {
		t.Errorf("campos a mais: %v", c)
	}
}

func TestNULERecusado(t *testing.T) {
	_, err := Dados{Empresa: "A\x00B", Vaga: "x", Fonte: texto("Linked\x00In")}.normalizar()

	c := campos(t, err)
	if c["empresa"] != mensagemNUL || c["fonte"] != mensagemNUL {
		t.Errorf("campos = %v", c)
	}
}

func TestLimiteContaLetrasENaoBytes(t *testing.T) {
	// "ç" ocupa 2 bytes: 200 deles passam do limite em bytes, mas não em letras
	if _, err := (Dados{Empresa: strings.Repeat("ç", 200), Vaga: "x"}).normalizar(); err != nil {
		t.Fatal(err)
	}
}

func TestLinkSoHttpOuHttps(t *testing.T) {
	for _, link := range []string{"ftp://a.com", "javascript:alert(1)", "https://", "//a.com"} {
		if _, err := (Dados{Empresa: "a", Vaga: "b", Link: texto(link)}).normalizar(); err == nil {
			t.Errorf("aceitou %q", link)
		}
	}
}

func TestConferirData(t *testing.T) {
	agora := time.Date(2026, 10, 8, 15, 0, 0, 0, time.UTC)
	ultima := agora.Add(-48 * time.Hour)

	if err := conferirData(agora.Add(-time.Hour), agora, ultima); err != nil {
		t.Errorf("passado depois da última: %v", err)
	}
	if err := conferirData(agora.Add(30*time.Second), agora, ultima); err != nil {
		t.Errorf("dentro da folga do relógio: %v", err)
	}
	if c := campos(t, conferirData(agora.Add(2*time.Minute), agora, ultima)); c["em"] != "não pode ser no futuro" {
		t.Errorf("futuro: %v", c)
	}
	c := campos(t, conferirData(ultima.Add(-time.Hour), agora, ultima))
	if c["em"] != "não pode ser antes da última mudança de etapa (06/10/2026 12:00)" {
		t.Errorf("antes da última: %v", c)
	}
}

func TestEscaparLike(t *testing.T) {
	if got := escaparLike(`100%_a\b`); got != `100\%\_a\\b` {
		t.Errorf("escaparLike = %q", got)
	}
}

func TestEtapaValida(t *testing.T) {
	if !Tecnica.Valida() || Etapa("Tecnica").Valida() || Etapa("").Valida() {
		t.Error("Valida errada")
	}
}
