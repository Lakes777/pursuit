-- +goose Up
-- Os dados de cada candidatura e a etapa em que ela está. A lista de etapas fica numa
-- constraint (e não numa tabela) porque só muda junto com o código que a entende.
alter table candidatura
    add column fonte text check (length(fonte) <= 100),
    add column modalidade text check (modalidade in ('remoto', 'hibrido', 'presencial')),
    add column salario text check (length(salario) <= 100),
    add column anotacoes text check (length(anotacoes) <= 10000),
    add column etapa text not null default 'interesse' check (etapa in (
        'interesse', 'enviada', 'triagem', 'entrevista', 'tecnica', 'proposta',
        'contratado', 'recusada', 'desisti')),
    add column atualizada_em timestamptz not null default now();

-- Cada mudança de etapa, inclusive a primeira (de = null). "em" pode ser uma data passada,
-- para registrar depois algo que aconteceu antes (ex.: "me candidatei na segunda").
create table etapa_historico (
    id bigint generated always as identity primary key,
    candidatura_id bigint not null references candidatura (id) on delete cascade,
    de text,
    para text not null,
    em timestamptz not null default now(),
    observacao text check (length(observacao) <= 2000)
);

-- As candidaturas que já existiam ganham a primeira linha do histórico (a etapa do default)
insert into etapa_historico (candidatura_id, de, para, em)
select id, null, etapa, criada_em from candidatura;

create index etapa_historico_por_candidatura on etapa_historico (candidatura_id, em, id);
create index candidatura_por_atualizacao on candidatura (atualizada_em desc, id desc);

-- +goose Down
drop table etapa_historico;
alter table candidatura
    drop column fonte, drop column modalidade, drop column salario, drop column anotacoes,
    drop column etapa, drop column atualizada_em;
