-- +goose Up
-- O link curto do Beacon de cada candidatura (o que vai no currículo mandado para aquela vaga).
-- Um por candidatura, e um código não serve a duas candidaturas.
alter table candidatura
    add column beacon_codigo text unique check (length(beacon_codigo) between 1 and 64),
    add column beacon_url text check (length(beacon_url) between 1 and 2000),
    -- Os dois juntos: um código sem endereço (ou o contrário) não teria como aparecer na tela
    add constraint beacon_completo check ((beacon_codigo is null) = (beacon_url is null));

-- +goose Down
alter table candidatura
    drop constraint beacon_completo,
    drop column beacon_url,
    drop column beacon_codigo;
