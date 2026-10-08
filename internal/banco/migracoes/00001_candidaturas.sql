-- +goose Up
-- Uma vaga em que me candidatei (ou pretendo). A etapa e o histórico de etapas chegam na fase 2.
create table candidatura (
    id bigint generated always as identity primary key,
    empresa text not null check (length(btrim(empresa)) between 1 and 200),
    vaga text not null check (length(btrim(vaga)) between 1 and 200),
    link text check (length(link) <= 2000),
    criada_em timestamptz not null default now()
);

-- +goose Down
drop table candidatura;
