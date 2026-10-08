-- name: CriarCandidatura :one
insert into candidatura (empresa, vaga, link, fonte, modalidade, salario, anotacoes, etapa)
values (@empresa, @vaga, @link, @fonte, @modalidade, @salario, @anotacoes, @etapa)
returning *;

-- name: BuscarCandidatura :one
select * from candidatura where id = @id;

-- name: TravarCandidatura :one
-- Para mudar a etapa: trava a linha até o fim da transação, e duas mudanças ao mesmo
-- tempo na mesma candidatura acontecem uma depois da outra.
select * from candidatura where id = @id for update;

-- name: ListarCandidaturas :many
-- Filtros opcionais: etapa exata e busca por pedaço do nome da empresa ou da vaga
-- (a busca chega com % e _ já escapados, para valerem como texto).
-- etapa_desde: quando ela entrou na etapa atual (a última linha do histórico); a tela mostra
-- "há N dias nesta etapa". Sem histórico (linha inserida à mão), vale a data do cadastro.
select sqlc.embed(candidatura),
       coalesce(ultima.em, candidatura.criada_em)::timestamptz as etapa_desde
from candidatura
cross join lateral (
    select max(h.em) as em from etapa_historico h where h.candidatura_id = candidatura.id
) ultima
where (sqlc.narg(etapa)::text is null or etapa = sqlc.narg(etapa))
  and (sqlc.narg(busca)::text is null
       or empresa ilike '%' || sqlc.narg(busca) || '%'
       or vaga ilike '%' || sqlc.narg(busca) || '%')
order by candidatura.atualizada_em desc, candidatura.id desc;

-- name: EditarCandidatura :one
update candidatura
set empresa = @empresa, vaga = @vaga, link = @link, fonte = @fonte, modalidade = @modalidade,
    salario = @salario, anotacoes = @anotacoes, atualizada_em = now()
where id = @id
returning *;

-- name: MudarEtapa :one
update candidatura set etapa = @etapa, atualizada_em = now()
where id = @id
returning *;

-- name: ApagarCandidatura :execrows
delete from candidatura where id = @id;

-- name: RegistrarEtapa :one
insert into etapa_historico (candidatura_id, de, para, em, observacao)
values (@candidatura_id, @de, @para, @em, @observacao)
returning *;

-- name: HistoricoDaCandidatura :many
select * from etapa_historico where candidatura_id = @candidatura_id order by em, id;

-- name: UltimaMudanca :one
-- Sem histórico (linha inserida à mão), o ano 1: o time.Time do Go não representa '-infinity'.
select coalesce(max(em), '0001-01-01T00:00:00Z'::timestamptz)::timestamptz as em
from etapa_historico where candidatura_id = @candidatura_id;
