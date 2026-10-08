-- name: BuscarUsuarioPorNome :one
select * from usuario where nome = @nome;

-- name: ContarOutrasContas :one
select count(*) from usuario where nome <> @nome;

-- name: DefinirSenha :one
-- Cria a conta ou troca a senha de uma que já existe.
insert into usuario (nome, senha_hash) values (@nome, @senha_hash)
on conflict (nome) do update set senha_hash = excluded.senha_hash
returning *;

-- name: CriarSessao :exec
insert into sessao (token_hash, usuario_id, expira_em) values (@token_hash, @usuario_id, @expira_em);

-- name: BuscarSessao :one
select s.usuario_id, u.nome, s.expira_em
from sessao s join usuario u on u.id = s.usuario_id
where s.token_hash = @token_hash and s.expira_em > @agora;

-- name: RenovarSessao :exec
update sessao set expira_em = @expira_em where token_hash = @token_hash;

-- name: ApagarSessao :exec
delete from sessao where token_hash = @token_hash;

-- name: ApagarSessoesDoUsuario :exec
delete from sessao where usuario_id = @usuario_id;

-- name: ApagarSessoesVencidas :execrows
delete from sessao where expira_em <= @agora;
