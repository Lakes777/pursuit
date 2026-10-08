-- +goose Up
-- Conta única (só eu uso), mas numa tabela: o nome entra no login e a senha fica só como hash argon2id.
create table usuario (
    id bigint generated always as identity primary key,
    nome text not null unique check (nome ~ '^[a-z0-9._-]{1,50}$'),
    senha_hash text not null,
    criado_em timestamptz not null default now()
);

-- Sessões do lado do servidor: o cookie leva um código aleatório e aqui fica só o sha256 dele.
-- Quem vê o banco não consegue montar um cookie válido. Apagar as linhas derruba as sessões.
create table sessao (
    token_hash bytea primary key,
    usuario_id bigint not null references usuario (id) on delete cascade,
    criada_em timestamptz not null default now(),
    expira_em timestamptz not null
);

create index sessao_por_expiracao on sessao (expira_em);

-- +goose Down
drop table sessao;
drop table usuario;
