-- name: UltimaMudancaDeCada :many
-- A última mudança de etapa de cada candidatura e quando ela foi lembrada pela última vez
-- (ano 1 = nunca). As regras de "parada há quantos dias" ficam no Go (pacote lembretes).
with ultima as (
    select distinct on (candidatura_id) candidatura_id, id as historico_id, para, em
    from etapa_historico
    order by candidatura_id, em desc, id desc
)
select c.id, c.empresa, c.vaga, u.historico_id, u.para as etapa, u.em,
    coalesce((select max(l.enviado_em) from lembrete l where l.historico_id = u.historico_id),
        '0001-01-01T00:00:00Z'::timestamptz)::timestamptz as ultimo_lembrete
from ultima u
join candidatura c on c.id = u.candidatura_id
order by u.em, c.id;

-- name: TravarLembretes :exec
-- Trava até o fim da transação: duas instâncias da API não montam o mesmo lembrete ao mesmo tempo.
select pg_advisory_xact_lock(7271001);

-- name: LembreteEnviadoDesde :one
select exists (select 1 from lembrete where enviado_em >= @desde) as enviado;

-- name: RegistrarLembrete :one
insert into lembrete (candidatura_id, historico_id, enviado_em)
values (@candidatura_id, @historico_id, @enviado_em)
returning id;

-- name: DesfazerLembretes :exec
delete from lembrete where id = any(@ids::bigint[]);
