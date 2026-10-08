package contas

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parâmetros do argon2id recomendados pela OWASP (19 MiB de memória, 2 passadas, 1 thread):
// caro para quem tenta milhões de senhas, leve para um login de vez em quando.
const (
	memoriaKiB = 19 * 1024
	passadas   = 2
	threads    = 1
	tamSal     = 16
	tamHash    = 32
)

// hashDeSenha devolve o hash no formato PHC, que guarda junto os parâmetros e o sal:
// $argon2id$v=19$m=19456,t=2,p=1$<sal>$<hash>
func hashDeSenha(senha string) (string, error) {
	sal := make([]byte, tamSal)
	if _, err := rand.Read(sal); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(senha), sal, passadas, memoriaKiB, threads, tamHash)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoriaKiB, passadas, threads, b64.EncodeToString(sal), b64.EncodeToString(hash)), nil
}

var errHashInvalido = errors.New("hash de senha em formato desconhecido")

// No máximo 2 conferências ao mesmo tempo: cada uma usa 19 MiB, e muitos logins juntos (de
// muitos IPs, que o limite por IP não segura) acabariam com a memória de uma VM pequena.
// Os outros esperam a vez, ou desistem se o pedido for cancelado.
var vagasDoArgon2 = make(chan struct{}, 2)

// conferirSenhaNaVez espera uma vaga e confere.
func conferirSenhaNaVez(ctx context.Context, senha, guardado string) (bool, error) {
	select {
	case vagasDoArgon2 <- struct{}{}:
		defer func() { <-vagasDoArgon2 }()
	case <-ctx.Done():
		return false, ctx.Err()
	}
	return conferirSenha(senha, guardado)
}

// conferirSenha recalcula o hash com os parâmetros guardados e compara em tempo constante
// (sem parar no primeiro byte diferente, o que daria uma pista pelo tempo de resposta).
func conferirSenha(senha, guardado string) (bool, error) {
	partes := strings.Split(guardado, "$")
	if len(partes) != 6 || partes[1] != "argon2id" {
		return false, errHashInvalido
	}
	var versao int
	var memoria, tempo uint32
	var paralelo uint8
	if _, err := fmt.Sscanf(partes[2], "v=%d", &versao); err != nil || versao != argon2.Version {
		return false, errHashInvalido
	}
	if _, err := fmt.Sscanf(partes[3], "m=%d,t=%d,p=%d", &memoria, &tempo, &paralelo); err != nil {
		return false, errHashInvalido
	}
	b64 := base64.RawStdEncoding
	sal, err := b64.DecodeString(partes[4])
	if err != nil {
		return false, errHashInvalido
	}
	esperado, err := b64.DecodeString(partes[5])
	if err != nil || len(esperado) == 0 {
		return false, errHashInvalido
	}
	calculado := argon2.IDKey([]byte(senha), sal, tempo, memoria, paralelo, uint32(len(esperado))) //nolint:gosec // tamanho do hash guardado (32)
	return subtle.ConstantTimeCompare(calculado, esperado) == 1, nil
}

// hashDeMentira é conferido quando o nome não existe: o login leva o mesmo tempo com nome
// certo ou errado, e o tempo não entrega quais nomes existem.
var hashDeMentira = func() string {
	h, err := hashDeSenha("senha-que-ninguem-usa")
	if err != nil {
		panic(err)
	}
	return h
}()
