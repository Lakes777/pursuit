// Package testebanco sobe um Postgres de verdade num contêiner (Testcontainers) para os testes,
// um por pacote de teste, já com as migrações aplicadas. Cada pacote chama Encerrar no TestMain.
//
// Os testes que usam o banco NÃO podem usar t.Parallel(): Limpar apaga as tabelas, e um teste
// rodando ao mesmo tempo perderia os dados no meio.
package testebanco

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Lakes777/pursuit/internal/banco"
)

var (
	uma       sync.Once
	conteiner *postgres.PostgresContainer
	pool      *pgxpool.Pool
	erro      error
)

// Pool devolve a conexão com o banco dos testes. O contêiner sobe no primeiro uso e é
// apagado sozinho no fim (pelo Ryuk do Testcontainers).
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	uma.Do(func() {
		ctx := context.Background()
		conteiner, erro = postgres.Run(ctx, "postgres:17-alpine",
			postgres.WithDatabase("pursuit"),
			postgres.WithUsername("pursuit"),
			postgres.WithPassword("pursuit"),
			testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
		)
		if erro != nil {
			return
		}
		var url string
		if url, erro = conteiner.ConnectionString(ctx, "sslmode=disable"); erro != nil {
			return
		}
		if pool, erro = banco.Conectar(ctx, url); erro != nil {
			return
		}
		erro = banco.Migrar(ctx, pool)
	})
	if erro != nil {
		t.Fatalf("banco dos testes: %v", erro)
	}
	return pool
}

// Encerrar fecha o pool e apaga o contêiner (se subiu). Para o TestMain, depois do m.Run().
// O Ryuk do Testcontainers apagaria sozinho, mas aqui fica explícito e mais rápido.
func Encerrar() {
	if pool != nil {
		pool.Close()
	}
	if conteiner != nil {
		_ = conteiner.Terminate(context.Background())
	}
}

// Limpar apaga os dados das tabelas, para cada teste começar do zero.
func Limpar(t testing.TB, p *pgxpool.Pool) {
	t.Helper()
	if _, err := p.Exec(context.Background(), "truncate candidatura, etapa_historico, sessao, usuario restart identity cascade"); err != nil {
		t.Fatalf("limpar o banco: %v", err)
	}
}
