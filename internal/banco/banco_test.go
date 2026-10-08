package banco_test

import (
	"context"
	"testing"

	"github.com/Lakes777/pursuit/internal/banco"
	"github.com/Lakes777/pursuit/internal/testebanco"
)

func TestMigrarDuasVezesNaoFazNada(t *testing.T) {
	pool := testebanco.Pool(t) // já migrado uma vez
	if err := banco.Migrar(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
}

func TestTabelaCandidaturaRecusaEmpresaVazia(t *testing.T) {
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)

	_, err := pool.Exec(context.Background(), "insert into candidatura (empresa, vaga) values ('   ', 'Back-end')")
	if err == nil {
		t.Fatal("aceitou empresa só com espaços")
	}
}

func TestTabelaCandidaturaAceitaOBasico(t *testing.T) {
	pool := testebanco.Pool(t)
	testebanco.Limpar(t, pool)

	var id int64
	err := pool.QueryRow(context.Background(),
		"insert into candidatura (empresa, vaga) values ('Nubank', 'Back-end Júnior') returning id").Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Errorf("id = %d", id)
	}
}

func TestConectarComBancoInexistenteFalha(t *testing.T) {
	_, err := banco.Conectar(context.Background(), "postgres://ninguem:errada@127.0.0.1:1/nada?connect_timeout=1")
	if err == nil {
		t.Fatal("conectou num banco que não existe")
	}
}
