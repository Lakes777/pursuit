-- Consultas só do modo demonstração (PURSUIT_DEMO=true, numa instância com banco próprio).

-- name: TravarDemo :exec
-- Trava até o fim da transação: a semeadura da partida e a do cron de hora em hora não se
-- atropelam (uma espera a outra terminar).
select pg_advisory_xact_lock(7271002);

-- name: ContarCandidaturas :one
select count(*) from candidatura;

-- name: ApagarCandidaturas :exec
-- O histórico e os lembretes vão junto (on delete cascade). "restart identity" faz os ids
-- voltarem a 1: o link de uma ficha da demonstração continua o mesmo depois de cada reposição.
truncate candidatura, etapa_historico, lembrete restart identity;

-- name: InserirCandidaturaDemo :one
-- Com as datas e o link do Beacon já preenchidos: os exemplos têm passado (criada há semanas,
-- mexida há dias), que o cadastro normal não permite escolher.
insert into candidatura (empresa, vaga, link, fonte, modalidade, salario, anotacoes, etapa,
                         criada_em, atualizada_em, beacon_codigo, beacon_url)
values (@empresa, @vaga, @link, @fonte, @modalidade, @salario, @anotacoes, @etapa,
        @criada_em, @atualizada_em, @beacon_codigo, @beacon_url)
returning id;
