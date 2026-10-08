// Funções puras (sem DOM nem rede), testadas com `node --test` (logica.test.js).

const fusoDoNavegador = undefined // datas no fuso de quem está usando (Brasília, no meu caso)

/** "08/10/2026" */
export function formatarData(iso) {
  return new Date(iso).toLocaleDateString('pt-BR', { timeZone: fusoDoNavegador })
}

/** "08/10/2026 às 14:30" */
export function formatarDataHora(iso) {
  const d = new Date(iso)
  const hora = d.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit', timeZone: fusoDoNavegador })
  return `${formatarData(iso)} às ${hora}`
}

/** Dias de calendário entre a data e agora (de ontem 23h para hoje 0h = 1), como os lembretes. */
export function diasDesde(iso, agora = new Date()) {
  const de = new Date(iso)
  const inicio = Date.UTC(de.getFullYear(), de.getMonth(), de.getDate())
  const fim = Date.UTC(agora.getFullYear(), agora.getMonth(), agora.getDate())
  return Math.round((fim - inicio) / 86_400_000)
}

/** "hoje", "ontem", "há 8 dias" (e "amanhã"/"em 3 dias" para datas futuras). */
export function textoHa(dias) {
  if (dias === 0) return 'hoje'
  if (dias === 1) return 'ontem'
  if (dias === -1) return 'amanhã'
  return dias > 0 ? `há ${dias} dias` : `em ${-dias} dias`
}

/** 38 → "38%", 37.5 → "37,5%", null → "–" */
export function porcentagem(valor) {
  if (valor === null || valor === undefined) return '–'
  return `${numero(valor)}%`
}

/** 4.5 → "4,5", 1234 → "1.234" */
export function numero(valor, casas = 1) {
  return Number(valor).toLocaleString('pt-BR', { maximumFractionDigits: casas })
}

/** 1 → "1 dia", 4.5 → "4,5 dias", null → "–" */
export function dias(valor) {
  if (valor === null || valor === undefined) return '–'
  return valor === 1 ? '1 dia' : `${numero(valor)} dias`
}

/** O nome de uma etapa para a tela ("tecnica" → "Etapa técnica"), pela lista de /api/etapas. */
export function nomeDaEtapa(etapas, etapa) {
  return etapas.find((e) => e.etapa === etapa)?.nome ?? etapa
}

/**
 * O valor de um <input type="datetime-local"> ("2026-10-06T10:00", na hora local) para o formato
 * que a API espera (RFC 3339 com fuso). Vazio → undefined (a API usa "agora").
 */
export function dataLocalParaAPI(valor) {
  if (!valor) return undefined
  const d = new Date(valor)
  if (Number.isNaN(d.getTime())) return undefined
  return d.toISOString()
}

/** O contrário: um instante da API para o valor de um <input type="datetime-local">. */
export function dataAPIParaLocal(iso, agora) {
  const d = iso ? new Date(iso) : agora ?? new Date()
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`
}

/** Lê a rota do endereço: "#/candidatura/12" → { secao: "candidatura", id: "12" }. */
export function lerRota(hash) {
  const partes = (hash || '').replace(/^#\/?/, '').split('/').filter(Boolean)
  return { secao: partes[0] || 'quadro', id: partes[1] ?? null }
}
