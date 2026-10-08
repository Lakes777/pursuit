// Funções puras das telas de números e lembretes (sem DOM nem rede), testadas com `node --test`
// (testes/numeros.test.js). Os gráficos só desenham o que estas funções calculam.
import { dias, porcentagem } from './logica.js'

/** "2026-10-05" (segunda-feira que a API manda) → "05/10". Lê o texto, sem passar por fuso. */
export function rotuloDaSemana(inicio) {
  const [, mes, dia] = String(inicio).split('-')
  return `${dia}/${mes}`
}

/**
 * Escala das barras: o topo é o maior valor (pelo menos 1, para não dividir por zero), então a
 * barra mais alta encosta no topo e o único rótulo de escala ("máximo") é um valor que existe.
 */
export function escalaDasBarras(valores) {
  return Math.max(1, ...valores.map((v) => Number(v) || 0))
}

/**
 * Altura (ou largura) de uma barra em unidades do gráfico. Um valor acima de zero nunca some
 * (fica com pelo menos `minimo`); zero dá zero (o gráfico desenha só a marca da base).
 */
export function tamanhoDaBarra(valor, maximo, util, minimo = 2) {
  if (!valor || valor <= 0) return 0
  return Math.max(minimo, (valor / maximo) * util)
}

/** Largura de um passo do funil em % do primeiro (as enviadas). Zero vira 0; o resto, pelo menos 3%. */
export function larguraDoPasso(total, primeiro) {
  if (!total || !primeiro) return 0
  return Math.round(Math.max(3, Math.min(100, (total / primeiro) * 100)) * 10) / 10
}

/** 1 → "1 candidatura", 3 → "3 candidaturas" (plural regular: só acrescenta "s", ou o plural dado). */
export function plural(n, singular, varias = `${singular}s`) {
  return `${n} ${n === 1 ? singular : varias}`
}

/**
 * O resumo do tempo de resposta para a tela. Sem nenhuma resposta, a mediana e a média vêm null
 * (mostradas como "–") e a frase explica o porquê.
 */
export function textosDoTempo(respostas) {
  const { respondidas, aguardando, mediaDias, medianaDias } = respostas
  const resumo =
    respondidas === 0
      ? 'Ainda não há respostas: o tempo aparece quando uma empresa responder a uma candidatura enviada.'
      : `Calculado sobre ${plural(respondidas, 'resposta')}, do envio até a primeira mudança que veio da empresa.`
  return {
    mediana: dias(medianaDias),
    media: dias(mediaDias),
    respondidas: String(respondidas),
    aguardando: String(aguardando),
    resumo,
  }
}

/** As linhas da tabela de fontes, na ordem da API; fonte null vira "Sem fonte". */
export function linhasDasFontes(porFonte) {
  return porFonte.map((f) => ({
    nome: f.fonte ?? 'Sem fonte',
    semFonte: f.fonte === null || f.fonte === undefined,
    enviadas: f.enviadas,
    entrevistas: f.entrevistas,
    propostas: f.propostas,
    contratados: f.contratados,
    taxa: porcentagem(f.taxaDeEntrevista),
  }))
}

/**
 * Separa a lista porEtapa da API em andamento e finais, pela lista de /api/etapas (que diz quais
 * são finais). Uma etapa que a lista não conhece fica em andamento.
 */
export function separarEtapas(porEtapa, etapas) {
  const finais = new Set(etapas.filter((e) => e.final).map((e) => e.etapa))
  return {
    andamento: porEtapa.filter((e) => !finais.has(e.etapa)),
    finais: porEtapa.filter((e) => finais.has(e.etapa)),
  }
}

/** O texto alternativo do gráfico de semanas: "05/10: 2; 28/09: 0; ...", da mais antiga à atual. */
export function descricaoDasSemanas(porSemana) {
  const partes = porSemana.map((s, i) => {
    const rotulo = rotuloDaSemana(s.inicio)
    const atual = i === porSemana.length - 1 ? ' (esta semana)' : ''
    return `${rotulo}${atual}: ${s.total}`
  })
  return `Envios por semana, começando na segunda-feira. ${partes.join('; ')}.`
}

/** "3 dias" ou "1 dia", para a tabela de prazos. */
export function diasDoPrazo(n) {
  return plural(n, 'dia')
}
