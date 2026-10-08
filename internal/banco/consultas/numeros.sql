-- O "alcance" de cada candidatura é o nível mais adiantado que ela já atingiu no histórico. Os
-- níveis vêm do Go (numeros.niveis, montado a partir de candidaturas.Etapas) em duas listas do
-- mesmo tamanho, @etapas e @niveis: o nível de uma etapa é (@niveis)[posição dela em @etapas].
-- A ordem fica num lugar só. Etapa sem nível (desisti) dá null, que o max ignora.

-- name: Funil :many
-- Quantas candidaturas chegaram a cada nível (pular uma etapa conta como ter passado por ela).
with alcance as (
    select h.candidatura_id,
        max((@niveis::int[])[array_position(@etapas::text[], h.para)]) as maximo
    from etapa_historico h
    group by h.candidatura_id
)
select p.nivel::int as nivel, count(*) filter (where a.maximo >= p.nivel) as total
-- distinct: recusada tem o mesmo nível de enviada, e sem ele o nível repetido contaria em dobro
from (select distinct n as nivel from unnest(@niveis::int[]) as n) p
cross join alcance a
group by p.nivel
order by p.nivel;

-- name: PorEtapaAtual :many
select etapa, count(*) as total from candidatura group by etapa;

-- name: PorFonte :many
-- Fontes iguais sem diferenciar maiúsculas nem espaços nas pontas ("LinkedIn" e " linkedin ");
-- o nome mostrado é a grafia mais usada. Sem fonte = ''.
with alcance as (
    select c.id, nullif(lower(btrim(c.fonte)), '') as chave, btrim(c.fonte) as fonte,
        max((@niveis::int[])[array_position(@etapas::text[], h.para)]) as maximo
    from candidatura c
    join etapa_historico h on h.candidatura_id = c.id
    group by c.id
)
select coalesce(mode() within group (order by fonte), '')::text as fonte,
    count(*) filter (where maximo >= @nivel_enviada::int) as enviadas,
    count(*) filter (where maximo >= @nivel_entrevista::int) as entrevistas,
    count(*) filter (where maximo >= @nivel_proposta::int) as propostas,
    count(*) filter (where maximo >= @nivel_contratado::int) as contratados
from alcance
group by chave
having count(*) filter (where maximo >= @nivel_enviada::int) > 0
order by enviadas desc, chave nulls last;

-- name: TempoDeResposta :one
-- Do primeiro "enviada" até a primeira mudança depois dele que veio da empresa (qualquer etapa,
-- menos voltar para interesse/enviada ou "desisti", que é decisão minha). Em dias. "Depois" é
-- por (em, id): duas mudanças no mesmo instante ficam na ordem em que foram registradas.
with envio as (
    select distinct on (candidatura_id) candidatura_id, em as enviada_em, id as enviada_id
    from etapa_historico where para = 'enviada'
    order by candidatura_id, em, id
),
resposta as (
    select e.candidatura_id, e.enviada_em, c.etapa,
        (select min(h.em) from etapa_historico h
         where h.candidatura_id = e.candidatura_id and (h.em, h.id) > (e.enviada_em, e.enviada_id)
           and h.para not in ('interesse', 'enviada', 'desisti')) as respondida_em
    from envio e join candidatura c on c.id = e.candidatura_id
)
select count(respondida_em) as respondidas,
    count(*) filter (where respondida_em is null and etapa = 'enviada') as aguardando,
    -- 0 quando nenhuma foi respondida (aí o Go devolve null)
    coalesce(avg(extract(epoch from respondida_em - enviada_em)) / 86400, 0)::float8 as media_dias,
    coalesce(percentile_cont(0.5) within group (order by extract(epoch from respondida_em - enviada_em)) / 86400, 0)::float8 as mediana_dias
from resposta;

-- name: EnviadasPorSemana :many
-- O primeiro envio de cada candidatura (voltar para "enviada" não conta de novo), em semanas
-- de segunda a domingo no horário de Brasília.
select p.semana::date as semana, count(*) as total
from (
    select date_trunc('week', min(em) at time zone 'America/Sao_Paulo') as semana
    from etapa_historico
    where para = 'enviada'
    group by candidatura_id
    having min(em) >= @desde
) p
group by p.semana
order by p.semana;
