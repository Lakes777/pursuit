// node --test (sem dependências). Fuso fixo para as datas: TZ=America/Sao_Paulo (no CI também).
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { dataAPIParaLocal, dataLocalParaAPI, dias, diasDesde, formatarData, lerRota, nomeDaEtapa, numero, porcentagem, textoHa } from '../static/js/logica.js'

test('diasDesde conta viradas de dia, não horas', () => {
  const agora = new Date('2026-10-08T00:30:00-03:00')
  assert.equal(diasDesde('2026-10-07T23:30:00-03:00', agora), 1)
  assert.equal(diasDesde('2026-10-08T00:10:00-03:00', agora), 0)
  assert.equal(diasDesde('2026-10-01T18:00:00-03:00', new Date('2026-10-08T09:00:00-03:00')), 7)
})

test('textoHa', () => {
  assert.equal(textoHa(0), 'hoje')
  assert.equal(textoHa(1), 'ontem')
  assert.equal(textoHa(8), 'há 8 dias')
  assert.equal(textoHa(-1), 'amanhã')
  assert.equal(textoHa(-3), 'em 3 dias')
})

test('números no formato brasileiro', () => {
  assert.equal(porcentagem(38), '38%')
  assert.equal(porcentagem(37.5), '37,5%')
  assert.equal(porcentagem(null), '–')
  assert.equal(numero(1234), '1.234')
  assert.equal(dias(1), '1 dia')
  assert.equal(dias(4.5), '4,5 dias')
  assert.equal(dias(null), '–')
})

test('formatarData usa o fuso local', () => {
  // 02h UTC do dia 9 ainda é dia 8 em Brasília
  assert.equal(formatarData('2026-10-09T02:00:00Z'), '08/10/2026')
})

test('datas do formulário vão e voltam', () => {
  assert.equal(dataLocalParaAPI('2026-10-06T10:00'), '2026-10-06T13:00:00.000Z')
  assert.equal(dataLocalParaAPI(''), undefined)
  assert.equal(dataLocalParaAPI('lixo'), undefined)
  assert.equal(dataAPIParaLocal('2026-10-06T13:00:00Z'), '2026-10-06T10:00')
})

test('nomeDaEtapa', () => {
  const etapas = [{ etapa: 'tecnica', nome: 'Etapa técnica' }]
  assert.equal(nomeDaEtapa(etapas, 'tecnica'), 'Etapa técnica')
  assert.equal(nomeDaEtapa(etapas, 'nova'), 'nova')
})

test('lerRota', () => {
  assert.deepEqual(lerRota(''), { secao: 'quadro', id: null })
  assert.deepEqual(lerRota('#/numeros'), { secao: 'numeros', id: null })
  assert.deepEqual(lerRota('#/candidatura/12'), { secao: 'candidatura', id: '12' })
})
