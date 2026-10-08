package api

import (
	"os"
	"testing"

	"github.com/Lakes777/pursuit/internal/testebanco"
)

// O Postgres dos testes sobe no primeiro uso e é apagado aqui, depois de todos os testes do pacote.
func TestMain(m *testing.M) {
	codigo := m.Run()
	testebanco.Encerrar()
	os.Exit(codigo)
}
