-- +goose Up
-- Cada lembrete mandado pelo Telegram, ligado à mudança de etapa que deixou a candidatura parada.
-- Mudar de etapa cria outra linha no histórico, e a contagem recomeça.
create table lembrete (
    id bigint generated always as identity primary key,
    candidatura_id bigint not null references candidatura (id) on delete cascade,
    historico_id bigint not null references etapa_historico (id) on delete cascade,
    enviado_em timestamptz not null default now()
);

create index lembrete_por_historico on lembrete (historico_id, enviado_em desc);
create index lembrete_por_envio on lembrete (enviado_em);

-- +goose Down
drop table lembrete;
