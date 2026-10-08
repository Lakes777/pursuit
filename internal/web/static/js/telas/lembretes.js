// Lembretes: as candidaturas paradas que entrariam na próxima mensagem do Telegram
// (GET /api/lembretes), como funciona e os prazos por etapa (GET /api/lembretes/prazos).
import { el, icone, trocar } from '../dom.js'
import { nomeDaEtapa, textoHa } from '../logica.js'
import { diasDoPrazo, plural } from '../numeros-logica.js'
import { registrarTela } from './registro.js'

registrarTela('lembretes', {
  titulo: 'Lembretes',
  async montar(container, params, ctx) {
    container.append(el('p', { class: 'vazio' }, 'Carregando os lembretes…'))
    const [paradas, regrasDaAPI] = await Promise.all([ctx.api('GET', '/api/lembretes'), ctx.api('GET', '/api/lembretes/prazos')])
    if (!ctx.atual(params.navegacao)) return

    trocar(
      container,
      el(
        'header',
        { class: 'cabecalho-tela' },
        el('h1', {}, 'Lembretes'),
        el(
          'p',
          { class: 'sub' },
          paradas.length === 0 ? 'Nada parado além do prazo.' : `${plural(paradas.length, 'candidatura parada', 'candidaturas paradas')} além do prazo.`,
        ),
      ),
      el('div', { class: 'lembretes-grade' }, listaDeParadas(paradas, ctx.etapas, regrasDaAPI.repetirDias), regras(regrasDaAPI, ctx.etapas)),
    )
  },
})

function listaDeParadas(paradas, etapas, repetirDias) {
  const secao = el('section', { class: 'pasta paradas', 'aria-labelledby': 'secao-paradas' }, el('h2', { class: 'orelha', id: 'secao-paradas' }, 'Paradas agora'))
  if (paradas.length === 0) {
    secao.append(
      el(
        'div',
        { class: 'tudo-em-dia' },
        el('p', { class: 'tudo-em-dia-titulo' }, 'Tudo em dia.'),
        el(
          'p',
          {},
          `Nenhuma candidatura passou do prazo da etapa. Uma que já foi lembrada nos últimos ${repetirDias} dias só volta a aparecer aqui quando completar esse tempo.`,
        ),
      ),
    )
    return secao
  }
  secao.append(
    el('p', { class: 'nota' }, 'É o que entraria na próxima mensagem do Telegram, das mais antigas para as mais recentes.'),
    el(
      'ul',
      { class: 'lista-paradas' },
      paradas.map((p) =>
        el(
          'li',
          { class: 'ficha-parada' },
          el(
            'div',
            { class: 'parada-topo' },
            el('a', { class: 'parada-empresa', href: `#/candidatura/${p.candidaturaId}` }, p.empresa),
            el('span', { class: 'etiqueta parada' }, icone('relogio'), textoHa(p.dias)),
          ),
          el('p', { class: 'parada-vaga' }, p.vaga, el('span', { class: 'etiqueta' }, nomeDaEtapa(etapas, p.etapa))),
          el('p', { class: 'parada-texto' }, el('span', { class: 'so-leitor' }, 'Texto do lembrete: '), p.texto),
          el('a', { class: 'parada-abrir', href: `#/candidatura/${p.candidaturaId}` }, 'Abrir a candidatura', icone('seta')),
        ),
      ),
    ),
  )
  return secao
}

function regras({ prazos, repetirDias, horaInicio, horaFim }, etapas) {
  const finais = etapas.filter((e) => e.final).map((e) => e.nome.toLowerCase())
  return el(
    'section',
    { class: 'pasta', 'aria-labelledby': 'secao-regras' },
    el('h2', { class: 'orelha', id: 'secao-regras' }, 'Como funciona'),
    el(
      'ul',
      { class: 'regras' },
      el('li', {}, `Uma mensagem por dia, no máximo, pelo Telegram, juntando todas as paradas. Sai entre ${horaInicio}h e ${horaFim}h (horário de Brasília), nunca de madrugada.`),
      el('li', {}, 'Os dias contam pelo calendário, não por horas: o que mudou às 18h de um dia completa 1 dia à meia-noite.'),
      el('li', {}, `Se a candidatura continuar parada, o lembrete se repete a cada ${repetirDias} dias.`),
      el('li', {}, `Mudar de etapa recomeça a contagem. Etapas encerradas (${finais.join(', ')}) não são lembradas.`),
    ),
    el(
      'table',
      { class: 'tabela-prazos' },
      el('caption', {}, 'Prazo por etapa'),
      el('thead', {}, el('tr', {}, el('th', { scope: 'col' }, 'Etapa'), el('th', { scope: 'col', class: 'num' }, 'Lembra depois de'))),
      el(
        'tbody',
        {},
        prazos.map((p) => el('tr', {}, el('th', { scope: 'row' }, p.nome), el('td', { class: 'num' }, diasDoPrazo(p.dias)))),
      ),
    ),
  )
}
