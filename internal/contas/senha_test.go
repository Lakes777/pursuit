package contas

import (
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestSenhaCertaEErrada(t *testing.T) {
	hash, err := hashDeSenha("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("hash = %s", hash)
	}
	if ok, err := conferirSenha("correct horse battery staple", hash); !ok || err != nil {
		t.Errorf("senha certa: %v %v", ok, err)
	}
	if ok, _ := conferirSenha("correct horse battery stapler", hash); ok {
		t.Error("aceitou senha errada")
	}
}

func TestMesmaSenhaDaHashesDiferentes(t *testing.T) {
	a, _ := hashDeSenha("mesma senha de sempre")
	b, _ := hashDeSenha("mesma senha de sempre")
	if a == b {
		t.Error("o sal deveria mudar o hash")
	}
}

func TestHashEmFormatoDesconhecido(t *testing.T) {
	for _, h := range []string{"", "texto", "$bcrypt$x$y$z$w", "$argon2id$v=18$m=1,t=1,p=1$c2Fs$aGFzaA"} {
		if _, err := conferirSenha("x", h); err == nil {
			t.Errorf("aceitou %q", h)
		}
	}
}

func TestSenhaNova(t *testing.T) {
	if ConferirSenhaNova("curta") == nil || ConferirSenhaNova(strings.Repeat("a", 129)) == nil {
		t.Error("limites")
	}
	if ConferirSenhaNova("ççççççççççç") == nil { // 11 letras (22 bytes): ainda curta
		t.Error("contou bytes em vez de letras")
	}
	if err := ConferirSenhaNova("uma frase longa"); err != nil {
		t.Error(err)
	}
}

func TestNome(t *testing.T) {
	if n, err := NormalizarNome("  Andre.Lagos "); err != nil || n != "andre.lagos" {
		t.Errorf("n = %q, err = %v", n, err)
	}
	for _, nome := range []string{"", "andré", "com espaço", strings.Repeat("a", 51)} {
		if _, err := NormalizarNome(nome); err == nil {
			t.Errorf("aceitou %q", nome)
		}
	}
}

func TestLimitePorIP(t *testing.T) {
	l := NovoLimite(2)
	agora := time.Date(2026, 10, 8, 15, 0, 0, 0, time.UTC)
	ip := netip.MustParseAddr("200.1.2.3")

	for i := range 2 {
		if !l.Permitir(ip, agora) {
			t.Fatalf("a tentativa %d deveria passar", i+1)
		}
	}
	if l.Permitir(ip, agora) {
		t.Error("a 3ª deveria ser barrada")
	}
	if !l.Permitir(netip.MustParseAddr("200.1.2.4"), agora) {
		t.Error("outro IP tem o próprio limite")
	}
	if !l.Permitir(ip, agora.Add(30*time.Second)) {
		t.Error("depois de meio minuto volta 1 tentativa (2 por minuto)")
	}
}

func TestIPv6ContaPeloPrefixo64(t *testing.T) {
	l := NovoLimite(1)
	agora := time.Now()

	l.Permitir(netip.MustParseAddr("2804:14c:1::1"), agora)

	if l.Permitir(netip.MustParseAddr("2804:14c:1::ffff"), agora) {
		t.Error("o mesmo /64 deveria dividir o limite")
	}
	if !l.Permitir(netip.MustParseAddr("2804:14c:2::1"), agora) {
		t.Error("outro /64 tem o próprio limite")
	}
	if !l.Permitir(netip.MustParseAddr("::ffff:200.1.2.3"), agora) {
		t.Error("IPv4 dentro de IPv6 conta como IPv4")
	}
}

func TestLimiteEsqueceIPsParados(t *testing.T) {
	l := NovoLimite(1)
	agora := time.Now()
	l.Permitir(netip.MustParseAddr("200.1.2.3"), agora)

	l.Permitir(netip.MustParseAddr("200.1.2.4"), agora.Add(11*time.Minute))

	if len(l.baldes) != 1 {
		t.Errorf("baldes = %d", len(l.baldes))
	}
}
