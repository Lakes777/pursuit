// Funções puras do quadro, da ficha e do formulário (sem DOM nem rede), testadas com
// `node --test` (quadro.test.js).
import { dataLocalParaAPI, diasDesde } from './logica.js'

/** As fontes que aparecem como sugestão no formulário, antes das já usadas. */
export const FONTES_PADRAO = ['LinkedIn', 'Gupy', 'Indeed', 'Site da empresa', 'Indicação']

/** Os campos de dados (os únicos que o PUT aceita), na ordem do formulário. */
export const CAMPOS_DE_DADOS = ['empresa', 'vaga', 'link', 'fonte', 'modalidade', 'salario', 'anotacoes']

const MODALIDADES = { remoto: 'Remoto', hibrido: 'Híbrido', presencial: 'Presencial' }

/** "hibrido" → "Híbrido"; vazio → "" */
export function nomeDaModalidade(modalidade) {
  if (!modalidade) return ''
  return MODALIDADES[modalidade] ?? modalidade
}

/** As opções do select de modalidade: [valor, texto], começando pela vazia. */
export function opcoesDeModalidade() {
  return [['', 'Não informada'], ...Object.entries(MODALIDADES)]
}

/** Dias na etapa atual (pela última mudança de etapa; sem ela, pela última atualização). */
export function diasNaEtapa(candidatura, agora = new Date()) {
  const desde = candidatura.etapaDesde ?? candidatura.atualizadaEm ?? candidatura.criadaEm
  return Math.max(0, diasDesde(desde, agora))
}

/** 0 → "entrou hoje nesta etapa", 1 → "há 1 dia nesta etapa", 8 → "há 8 dias nesta etapa" */
export function textoDiasNaEtapa(dias) {
  if (!(dias > 0)) return 'entrou hoje nesta etapa'
  return dias === 1 ? 'há 1 dia nesta etapa' : `há ${dias} dias nesta etapa`
}

/**
 * Ordem das fichas dentro de uma pasta: a mais parada primeiro (a que entrou na etapa há mais
 * tempo); empate pela de id menor (cadastrada antes), para a ordem não pular entre recargas.
 */
export function compararFichas(a, b) {
  const da = new Date(a.etapaDesde ?? a.atualizadaEm).getTime()
  const db = new Date(b.etapaDesde ?? b.atualizadaEm).getTime()
  return da - db || a.id - b.id
}

/**
 * Separa as candidaturas em pastas, uma por etapa, na ordem de /api/etapas:
 * { abertas: [{ etapa, nome, final, fichas }], encerradas: [...] }.
 * Cada ficha é a candidatura com { dias, parada }. paradas: os ids que /api/lembretes devolveu.
 * Todas as etapas viram pasta, mesmo vazias (são onde se solta uma ficha arrastada).
 */
export function agruparPorEtapa(candidaturas, etapas, paradas = new Set(), agora = new Date()) {
  const pastas = new Map(etapas.map((e) => [e.etapa, { etapa: e.etapa, nome: e.nome, final: e.final, fichas: [] }]))
  for (const c of candidaturas) {
    const pasta = pastas.get(c.etapa)
    if (!pasta) continue // etapa que esta versão da página não conhece
    pasta.fichas.push({ ...c, dias: diasNaEtapa(c, agora), parada: paradas.has(c.id) })
  }
  const todas = [...pastas.values()]
  for (const pasta of todas) pasta.fichas.sort(compararFichas)
  return { abertas: todas.filter((p) => !p.final), encerradas: todas.filter((p) => p.final) }
}

/** Quantas fichas há numa lista de pastas. */
export function totalDeFichas(pastas) {
  return pastas.reduce((soma, p) => soma + p.fichas.length, 0)
}

/** As sugestões de fonte: as padrão e as já usadas, sem repetir (maiúsculas e espaços não contam). */
export function fontesSugeridas(candidaturas = []) {
  const vistas = new Map()
  for (const fonte of [...FONTES_PADRAO, ...candidaturas.map((c) => c.fonte)]) {
    const limpa = (fonte ?? '').trim()
    const chave = limpa.toLocaleLowerCase('pt-BR')
    if (limpa && !vistas.has(chave)) vistas.set(chave, limpa)
  }
  return [...vistas.values()]
}

/** Confere no navegador o que dá para conferir antes de enviar: { campo: mensagem }. */
export function conferirFormulario(valores) {
  const erros = {}
  if (!(valores.empresa ?? '').trim()) erros.empresa = 'obrigatório'
  if (!(valores.vaga ?? '').trim()) erros.vaga = 'obrigatório'
  return erros
}

/**
 * O corpo do pedido a partir dos valores do formulário (texto de cada campo).
 * Campo opcional vazio vai como null (no PUT, apaga o que havia). Na edição vão só os campos de
 * dados (o PUT recusa etapa e id); na nova, também a etapa inicial, a data e a observação.
 */
export function corpoDoFormulario(valores, nova) {
  const corpo = {}
  for (const campo of CAMPOS_DE_DADOS) {
    const valor = (valores[campo] ?? '').trim()
    corpo[campo] = valor === '' && campo !== 'empresa' && campo !== 'vaga' ? null : valor
  }
  if (nova) {
    if (valores.etapa) corpo.etapa = valores.etapa
    const em = dataLocalParaAPI(valores.em)
    if (em) corpo.em = em
    const observacao = (valores.observacao ?? '').trim()
    if (observacao) corpo.observacao = observacao
  }
  return corpo
}

/** O corpo do POST /api/candidaturas/{id}/etapas. Data vazia = agora (a API decide). */
export function corpoDaMudanca(etapa, dataLocal, observacao) {
  const corpo = { etapa }
  const em = dataLocalParaAPI(dataLocal)
  if (em) corpo.em = em
  const obs = (observacao ?? '').trim()
  if (obs) corpo.observacao = obs
  return corpo
}

/**
 * Separa o erro da API em mensagens por campo (os que o formulário mostra) e uma geral:
 * { campos: { em: "..." }, geral: "..." }. O 409 da mudança de etapa vira uma frase clara.
 */
export function errosDoPedido(erro, camposConhecidos) {
  const campos = {}
  const sobra = []
  for (const [campo, mensagem] of Object.entries(erro?.campos ?? {})) {
    if (camposConhecidos.includes(campo)) campos[campo] = primeiraMaiuscula(mensagem)
    else sobra.push(`${campo}: ${mensagem}`)
  }
  let geral = ''
  if (erro?.status === 409) geral = 'A candidatura já está nessa etapa.'
  else if (erro?.status === 404) geral = 'Esta candidatura não existe mais (talvez tenha sido apagada).'
  else if (sobra.length) geral = `Confira os dados: ${sobra.join('; ')}.`
  else if (!Object.keys(campos).length) geral = erro?.message ? `Não foi possível salvar: ${erro.message}.` : 'Não foi possível salvar.'
  return { campos, geral }
}

/** "obrigatório" → "Obrigatório" */
export function primeiraMaiuscula(texto) {
  const t = String(texto ?? '')
  return t.charAt(0).toLocaleUpperCase('pt-BR') + t.slice(1)
}

/** O texto de um passo do histórico: "Interesse → Triagem", ou "Início: Interesse" no primeiro. */
export function textoDoPasso(mudanca, nomeDe) {
  if (!mudanca.de) return `Início: ${nomeDe(mudanca.para)}`
  return `${nomeDe(mudanca.de)} → ${nomeDe(mudanca.para)}`
}
