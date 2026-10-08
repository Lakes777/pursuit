// Fala com a API do Pursuit. O cookie da sessão vai sozinho (mesma origem, HttpOnly).

/** Erro de uma resposta da API (formato RFC 9457): titulo, status e, na validação, campos. */
export class ErroDaAPI extends Error {
  constructor(status, problema = {}) {
    super(problema.title || (status === 0 ? 'Sem conexão com o Pursuit' : `Erro ${status}`))
    this.status = status
    this.titulo = this.message
    this.detalhe = problema.detail || ''
    this.campos = problema.campos || {}
  }
}

/**
 * api('GET', '/api/candidaturas') → JSON (ou null no 204).
 * Um 401 fora do login avisa a página (evento "pursuit:sessao-expirou"), que volta à tela de entrada.
 */
export async function api(metodo, caminho, corpo) {
  const opcoes = { method: metodo, headers: { Accept: 'application/json' }, credentials: 'same-origin' }
  if (corpo !== undefined) {
    opcoes.headers['Content-Type'] = 'application/json'
    opcoes.body = JSON.stringify(corpo)
  }
  let resposta
  try {
    resposta = await fetch(caminho, opcoes)
  } catch {
    throw new ErroDaAPI(0)
  }
  if (resposta.status === 204) return null
  const dados = await resposta.json().catch(() => null)
  if (!resposta.ok) {
    if (resposta.status === 401 && caminho !== '/api/sessao') {
      window.dispatchEvent(new CustomEvent('pursuit:sessao-expirou'))
    }
    throw new ErroDaAPI(resposta.status, dados ?? {})
  }
  return dados
}
