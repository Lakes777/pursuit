// node --test (sem dependências).
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { cliquesDaCandidatura, formatarDia, textoDosCliques, ultimoDiaComClique } from '../static/js/beacon-logica.js'

test('texto dos cliques no singular e no plural', () => {
  assert.equal(textoDosCliques(0), 'Ainda não foi aberto')
  assert.equal(textoDosCliques(undefined), 'Ainda não foi aberto')
  assert.equal(textoDosCliques(1), 'Aberto 1 vez')
  assert.equal(textoDosCliques(3), 'Aberto 3 vezes')
  assert.equal(textoDosCliques(1500), 'Aberto 1.500 vezes')
})

test('último dia com clique', () => {
  assert.equal(ultimoDiaComClique([]), null)
  assert.equal(ultimoDiaComClique(undefined), null)
  assert.equal(ultimoDiaComClique([{ dia: '2026-10-10', cliques: 0 }, { dia: '2026-10-11', cliques: 0 }]), null)
  assert.equal(ultimoDiaComClique([
    { dia: '2026-10-10', cliques: 2 }, { dia: '2026-10-11', cliques: 1 }, { dia: '2026-10-12', cliques: 0 },
  ]), '2026-10-11')
})

test('dia sem passar pelo fuso', () => {
  // Com new Date("2026-10-01") em Brasília, viraria 30/09 (meia-noite UTC)
  assert.equal(formatarDia('2026-10-01'), '01/10/2026')
})

test('cliques de cada candidatura pelo resumo', () => {
  const porLink = { ab3k9xq: 4, zz: 0 }
  assert.equal(cliquesDaCandidatura({ beacon: { codigo: 'ab3k9xq' } }, porLink), 4)
  assert.equal(cliquesDaCandidatura({ beacon: { codigo: 'novo' } }, porLink), 0)
  assert.equal(cliquesDaCandidatura({ beacon: null }, porLink), 0)
  assert.equal(cliquesDaCandidatura({ beacon: { codigo: 'ab3k9xq' } }), 0)
})
