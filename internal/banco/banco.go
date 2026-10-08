// Package banco abre a conexão com o Postgres e aplica as migrações.
package banco

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// As migrações vão dentro do binário (embed): a imagem de produção não precisa da pasta.
//
//go:embed migracoes/*.sql
var arquivos embed.FS

// Conectar abre o pool de conexões e confere que o banco responde.
func Conectar(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("configuração do banco: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("o banco não respondeu: %w", err)
	}
	return pool, nil
}

// Migrar aplica as migrações que faltam (goose). Rodar de novo não faz nada. Uma trava do
// Postgres (advisory lock) impede que duas instâncias subindo juntas migrem ao mesmo tempo.
func Migrar(ctx context.Context, pool *pgxpool.Pool) error {
	pasta, err := fs.Sub(arquivos, "migracoes")
	if err != nil {
		return err
	}
	// O goose usa database/sql; o stdlib do pgx empresta uma conexão do mesmo pool
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()
	trava, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}
	provedor, err := goose.NewProvider(goose.DialectPostgres, db, pasta, goose.WithSessionLocker(trava))
	if err != nil {
		return fmt.Errorf("migrações: %w", err)
	}
	if _, err := provedor.Up(ctx); err != nil {
		return fmt.Errorf("aplicar migrações: %w", err)
	}
	return nil
}
