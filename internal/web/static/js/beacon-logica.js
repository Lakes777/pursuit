// O link curto do Beacon de cada candidatura: textos e contas puras (testadas com node --test).

/** "Ainda não foi aberto", "Aberto 1 vez", "Aberto 3 vezes". */
export function textoDosCliques(total) {
  if (!total) return 'Ainda não foi aberto'
  return `Aberto ${total.toLocaleString('pt-BR')} ${total === 1 ? 'vez' : 'vezes'}`
}

/** O último dia com clique ("2026-10-12"), ou null. porDia vem em ordem, do mais antigo ao mais novo. */
export function ultimoDiaComClique(porDia = []) {
  for (let i = porDia.length - 1; i >= 0; i--) if (porDia[i].cliques > 0) return porDia[i].dia
  return null
}

/** "2026-10-12" → "12/10/2026" (o dia já vem no horário de Brasília: sem passar por Date, sem fuso). */
export function formatarDia(dia) {
  const [ano, mes, d] = dia.split('-')
  return `${d}/${mes}/${ano}`
}

/** Quantos cliques o link da candidatura teve, pelo resumo do Beacon ({ codigo: cliques }); 0 sem link. */
export function cliquesDaCandidatura(c, porLink = {}) {
  return c.beacon ? (porLink[c.beacon.codigo] ?? 0) : 0
}
