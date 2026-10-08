// node --test (sem dependências): as funções puras das telas de números e lembretes.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  descricaoDasSemanas,
  diasDoPrazo,
  escalaDasBarras,
  larguraDoPasso,
  linhasDasFontes,
  plural,
  rotuloDaSemana,
  separarEtapas,
  tamanhoDaBarra,
  textosDoTempo,
} from '../static/js/numeros-logica.js'

test('rotuloDaSemana lê a data como texto, sem fuso', () => {
  assert.equal(rotuloDaSemana('2026-10-05'), '05/10')
  assert.equal(rotuloDaSemana('2026-12-28'), '28/12')
  // À meia-noite UTC seria o dia anterior em Brasília; aqui não pode mudar
  assert.equal(rotuloDaSemana('2026-01-01'), '01/01')
})

test('escalaDasBarras: o topo é o maior valor, nunca menor que 1', () => {
  assert.equal(escalaDasBarras([0, 3, 7, 2]), 7)
  assert.equal(escalaDasBarras([0, 0, 0]), 1)
  assert.equal(escalaDasBarras([]), 1)
})

test('tamanhoDaBarra: proporcional, zero some e o pequeno não some', () => {
  assert.equal(tamanhoDaBarra(7, 7, 150), 150)
  assert.equal(tamanhoDaBarra(0, 7, 150), 0)
  assert.equal(tamanhoDaBarra(1, 1000, 150), 2)
  assert.equal(tamanhoDaBarra(3, 6, 100, 2), 50)
})

test('larguraDoPasso do funil', () => {
  assert.equal(larguraDoPasso(5, 5), 100)
  assert.equal(larguraDoPasso(3, 5), 60)
  assert.equal(larguraDoPasso(1, 3), 33.3)
  assert.equal(larguraDoPasso(1, 200), 3)
  assert.equal(larguraDoPasso(0, 5), 0)
  // Sem nenhuma enviada não há base: tudo zero
  assert.equal(larguraDoPasso(0, 0), 0)
})

test('plural', () => {
  assert.equal(plural(1, 'candidatura'), '1 candidatura')
  assert.equal(plural(0, 'candidatura'), '0 candidaturas')
  assert.equal(plural(2, 'candidatura parada', 'candidaturas paradas'), '2 candidaturas paradas')
  assert.equal(diasDoPrazo(1), '1 dia')
  assert.equal(diasDoPrazo(7), '7 dias')
})

test('textosDoTempo sem respostas mostra traço e explica', () => {
  const t = textosDoTempo({ respondidas: 0, aguardando: 3, mediaDias: null, medianaDias: null })
  assert.equal(t.mediana, '–')
  assert.equal(t.media, '–')
  assert.equal(t.aguardando, '3')
  assert.match(t.resumo, /Ainda não há respostas/)
})

test('textosDoTempo com respostas', () => {
  const t = textosDoTempo({ respondidas: 2, aguardando: 1, mediaDias: 4.5, medianaDias: 3 })
  assert.equal(t.mediana, '3 dias')
  assert.equal(t.media, '4,5 dias')
  assert.equal(t.respondidas, '2')
  assert.match(t.resumo, /2 respostas/)
  assert.match(textosDoTempo({ respondidas: 1, aguardando: 0, mediaDias: 1, medianaDias: 1 }).resumo, /1 resposta,/)
})

test('linhasDasFontes mantém a ordem da API e nomeia a sem fonte', () => {
  const linhas = linhasDasFontes([
    { fonte: 'LinkedIn', enviadas: 2, entrevistas: 1, propostas: 0, contratados: 0, taxaDeEntrevista: 50 },
    { fonte: null, enviadas: 3, entrevistas: 0, propostas: 0, contratados: 0, taxaDeEntrevista: 0 },
    { fonte: 'Gupy', enviadas: 3, entrevistas: 1, propostas: 0, contratados: 0, taxaDeEntrevista: 33.3 },
  ])
  assert.deepEqual(
    linhas.map((l) => [l.nome, l.semFonte, l.taxa]),
    [
      ['LinkedIn', false, '50%'],
      ['Sem fonte', true, '0%'],
      ['Gupy', false, '33,3%'],
    ],
  )
})

test('separarEtapas pela lista de /api/etapas', () => {
  const etapas = [
    { etapa: 'enviada', final: false },
    { etapa: 'entrevista', final: false },
    { etapa: 'contratado', final: true },
    { etapa: 'recusada', final: true },
  ]
  const porEtapa = [
    { etapa: 'enviada', total: 2 },
    { etapa: 'entrevista', total: 0 },
    { etapa: 'contratado', total: 1 },
    { etapa: 'recusada', total: 4 },
    { etapa: 'nova', total: 1 },
  ]
  const { andamento, finais } = separarEtapas(porEtapa, etapas)
  assert.deepEqual(andamento.map((e) => e.etapa), ['enviada', 'entrevista', 'nova'])
  assert.deepEqual(finais.map((e) => e.etapa), ['contratado', 'recusada'])
})

test('descricaoDasSemanas nomeia cada semana e marca a atual', () => {
  const texto = descricaoDasSemanas([
    { inicio: '2026-09-28', total: 0 },
    { inicio: '2026-10-05', total: 2 },
  ])
  assert.equal(texto, 'Envios por semana, começando na segunda-feira. 28/09: 0; 05/10 (esta semana): 2.')
})
