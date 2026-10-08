// node --test (sem dependências). Fuso fixo para as datas: TZ=America/Sao_Paulo (no CI também).
import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  agruparPorEtapa, compararFichas, conferirFormulario, corpoDaMudanca, corpoDoFormulario, diasNaEtapa, errosDoPedido,
  fontesSugeridas, nomeDaModalidade, opcoesDeModalidade, primeiraMaiuscula, textoDiasNaEtapa, textoDoPasso, totalDeFichas,
} from '../static/js/quadro-logica.js'

const etapas = [
  { etapa: 'interesse', nome: 'Interesse', final: false },
  { etapa: 'enviada', nome: 'Candidatura enviada', final: false },
  { etapa: 'triagem', nome: 'Triagem', final: false },
  { etapa: 'contratado', nome: 'Contratado', final: true },
  { etapa: 'recusada', nome: 'Recusada', final: true },
]

const agora = new Date('2026-10-08T15:00:00-03:00')

function candidatura(id, etapa, etapaDesde, extra = {}) {
  return { id, empresa: `Empresa ${id}`, vaga: 'Go', etapa, etapaDesde, atualizadaEm: '2026-10-08T12:00:00-03:00', ...extra }
}

test('agruparPorEtapa: uma pasta por etapa, na ordem, com as finais separadas', () => {
  const lista = [
    candidatura(1, 'enviada', '2026-10-07T10:00:00-03:00'),
    candidatura(2, 'recusada', '2026-10-01T10:00:00-03:00'),
    candidatura(3, 'enviada', '2026-09-30T10:00:00-03:00'),
    candidatura(4, 'sonho', '2026-09-30T10:00:00-03:00'), // etapa desconhecida: fica de fora
  ]

  const { abertas, encerradas } = agruparPorEtapa(lista, etapas, new Set([3]), agora)

  assert.deepEqual(abertas.map((p) => p.etapa), ['interesse', 'enviada', 'triagem'])
  assert.deepEqual(encerradas.map((p) => p.etapa), ['contratado', 'recusada'])
  assert.equal(abertas[0].fichas.length, 0) // pasta vazia continua lá (é onde se solta uma ficha)
  assert.equal(abertas[1].nome, 'Candidatura enviada')
  // A mais parada primeiro
  assert.deepEqual(abertas[1].fichas.map((f) => f.id), [3, 1])
  assert.equal(abertas[1].fichas[0].dias, 8)
  assert.equal(abertas[1].fichas[0].parada, true)
  assert.equal(abertas[1].fichas[1].parada, false)
  assert.equal(totalDeFichas(abertas), 2)
  assert.equal(totalDeFichas(encerradas), 1)
})

test('agruparPorEtapa não altera a lista recebida', () => {
  const lista = [candidatura(1, 'enviada', '2026-10-07T10:00:00-03:00')]
  agruparPorEtapa(lista, etapas, new Set(), agora)
  assert.equal('dias' in lista[0], false)
})

test('compararFichas: empate na data vai pelo id', () => {
  const mesma = '2026-10-07T10:00:00-03:00'
  const fichas = [candidatura(9, 'enviada', mesma), candidatura(2, 'enviada', mesma), candidatura(5, 'enviada', '2026-10-01T10:00:00-03:00')]
  assert.deepEqual(fichas.sort(compararFichas).map((f) => f.id), [5, 2, 9])
})

test('diasNaEtapa conta pela última mudança de etapa, não pela edição', () => {
  const c = candidatura(1, 'enviada', '2026-10-01T18:00:00-03:00', { atualizadaEm: '2026-10-08T14:00:00-03:00' })
  assert.equal(diasNaEtapa(c, agora), 7)
  // Sem etapaDesde (API antiga), cai na última atualização
  assert.equal(diasNaEtapa({ atualizadaEm: '2026-10-06T10:00:00-03:00' }, agora), 2)
  // Relógio do celular atrasado: nunca negativo
  assert.equal(diasNaEtapa(candidatura(1, 'enviada', '2026-10-09T10:00:00-03:00'), agora), 0)
})

test('textoDiasNaEtapa', () => {
  assert.equal(textoDiasNaEtapa(0), 'entrou hoje nesta etapa')
  assert.equal(textoDiasNaEtapa(1), 'há 1 dia nesta etapa')
  assert.equal(textoDiasNaEtapa(12), 'há 12 dias nesta etapa')
})

test('fontes sugeridas: as padrão e as usadas, sem repetir', () => {
  const fontes = fontesSugeridas([{ fonte: ' linkedin ' }, { fonte: 'Catho' }, { fonte: null }, { fonte: 'catho' }, { fonte: '  ' }])
  assert.deepEqual(fontes, ['LinkedIn', 'Gupy', 'Indeed', 'Site da empresa', 'Indicação', 'Catho'])
  assert.deepEqual(fontesSugeridas(), ['LinkedIn', 'Gupy', 'Indeed', 'Site da empresa', 'Indicação'])
})

test('modalidades', () => {
  assert.equal(nomeDaModalidade('hibrido'), 'Híbrido')
  assert.equal(nomeDaModalidade(null), '')
  assert.deepEqual(opcoesDeModalidade().map(([v]) => v), ['', 'remoto', 'hibrido', 'presencial'])
})

test('conferirFormulario pede empresa e vaga', () => {
  assert.deepEqual(conferirFormulario({ empresa: '  ', vaga: '' }), { empresa: 'obrigatório', vaga: 'obrigatório' })
  assert.deepEqual(conferirFormulario({ empresa: 'Nubank', vaga: 'Go' }), {})
})

test('corpoDoFormulario da nova: dados, etapa, data e observação', () => {
  const corpo = corpoDoFormulario({
    empresa: ' Nubank ', vaga: 'Go', link: '', fonte: 'LinkedIn', modalidade: 'remoto', salario: '', anotacoes: 'linha 1\nlinha 2',
    etapa: 'enviada', em: '2026-10-06T10:00', observacao: '  ',
  }, true)
  assert.deepEqual(corpo, {
    empresa: 'Nubank', vaga: 'Go', link: null, fonte: 'LinkedIn', modalidade: 'remoto', salario: null, anotacoes: 'linha 1\nlinha 2',
    etapa: 'enviada', em: '2026-10-06T13:00:00.000Z',
  })
})

test('corpoDoFormulario da edição: só os campos de dados (o PUT recusa etapa e id)', () => {
  const corpo = corpoDoFormulario({ empresa: 'Nu', vaga: 'Go', etapa: 'triagem', em: '2026-10-06T10:00', observacao: 'x', id: '3' }, false)
  assert.deepEqual(Object.keys(corpo), ['empresa', 'vaga', 'link', 'fonte', 'modalidade', 'salario', 'anotacoes'])
  assert.equal(corpo.fonte, null) // campo vazio apaga o que havia
})

test('corpoDaMudanca: data vazia fica de fora (a API usa agora)', () => {
  assert.deepEqual(corpoDaMudanca('triagem', '', ''), { etapa: 'triagem' })
  assert.deepEqual(corpoDaMudanca('triagem', '2026-10-06T10:00', ' o RH ligou '),
    { etapa: 'triagem', em: '2026-10-06T13:00:00.000Z', observacao: 'o RH ligou' })
})

test('errosDoPedido separa os campos conhecidos da mensagem geral', () => {
  assert.deepEqual(errosDoPedido({ status: 422, campos: { em: 'não pode ser no futuro' } }, ['em', 'observacao']),
    { campos: { em: 'Não pode ser no futuro' }, geral: '' })
  assert.deepEqual(errosDoPedido({ status: 422, campos: { etapa: 'etapa desconhecida' } }, ['em']),
    { campos: {}, geral: 'Confira os dados: etapa: etapa desconhecida.' })
  assert.equal(errosDoPedido({ status: 409, message: 'A candidatura já está nessa etapa' }, ['em']).geral, 'A candidatura já está nessa etapa.')
  assert.match(errosDoPedido({ status: 404 }, []).geral, /não existe mais/)
  assert.equal(errosDoPedido({ status: 0, message: 'Sem conexão com o Pursuit' }, []).geral, 'Não foi possível salvar: Sem conexão com o Pursuit.')
})

test('textoDoPasso e primeiraMaiuscula', () => {
  const nome = (e) => etapas.find((x) => x.etapa === e)?.nome ?? e
  assert.equal(textoDoPasso({ de: null, para: 'interesse' }, nome), 'Início: Interesse')
  assert.equal(textoDoPasso({ de: 'enviada', para: 'triagem' }, nome), 'Candidatura enviada → Triagem')
  assert.equal(primeiraMaiuscula('óbvio'), 'Óbvio')
  assert.equal(primeiraMaiuscula(undefined), '')
})
