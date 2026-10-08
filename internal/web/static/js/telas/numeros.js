// Números: o funil, o tempo de resposta, os envios por semana, as fontes e as etapas de agora,
// a partir de GET /api/numeros. Gráficos em SVG desenhados à mão (sem biblioteca).
import { el, icone, svg, trocar } from '../dom.js'
import { porcentagem } from '../logica.js'
import {
  descricaoDasSemanas,
  escalaDasBarras,
  larguraDoPasso,
  linhasDasFontes,
  plural,
  rotuloDaSemana,
  separarEtapas,
  tamanhoDaBarra,
  textosDoTempo,
} from '../numeros-logica.js'
import { registrarTela } from './registro.js'

registrarTela('numeros', {
  titulo: 'Números',
  async montar(container, params, ctx) {
    container.append(el('p', { class: 'vazio' }, 'Carregando os números…'))
    const n = await ctx.api('GET', '/api/numeros')
    if (!ctx.atual(params.navegacao)) return

    if (n.total === 0) {
      trocar(container, cabecalho(n), vazio())
      return
    }
    trocar(
      container,
      cabecalho(n),
      el(
        'div',
        { class: 'numeros-grade' },
        funil(n.funil),
        tempo(n.respostas),
        semanas(n.porSemana),
        etapasAgora(n.porEtapa, ctx.etapas),
        fontes(n.porFonte),
      ),
    )
    // No celular o gráfico rola: começa mostrando a semana atual (a última, à direita)
    const rolagem = container.querySelector('.rolagem-grafico')
    if (rolagem) rolagem.scrollLeft = rolagem.scrollWidth
  },
})

function cabecalho(n) {
  return el(
    'header',
    { class: 'cabecalho-tela' },
    el('h1', {}, 'Números'),
    el('p', { class: 'sub' }, n.total === 0 ? 'Nenhuma candidatura ainda.' : `${plural(n.total, 'candidatura')}, ${n.emAndamento} em andamento.`),
  )
}

/** Uma pasta com a orelha (o título da seção). */
function pasta(titulo, classe, ...filhos) {
  const id = `secao-${titulo.normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/\W+/g, '-').toLowerCase()}`
  return el('section', { class: `pasta ${classe}`, 'aria-labelledby': id }, el('h2', { class: 'orelha', id }, titulo), ...filhos)
}

function vazio() {
  return pasta(
    'Ainda não há números',
    'numeros-vazio',
    el('p', {}, 'Os números aparecem quando houver candidaturas cadastradas. Aqui vão estar:'),
    el(
      'ul',
      {},
      el('li', {}, 'o funil, das enviadas até a contratação, com a taxa de cada passo;'),
      el('li', {}, 'quanto tempo as empresas levam para responder (mediana e média);'),
      el('li', {}, 'quantas candidaturas saíram em cada uma das últimas 12 semanas;'),
      el('li', {}, 'quais fontes (LinkedIn, Gupy...) rendem mais entrevistas.'),
    ),
    el('a', { class: 'botao principal', href: '#/nova' }, icone('mais'), 'Cadastrar a primeira candidatura'),
  )
}

/**
 * O funil como pastas que vão encolhendo: cada passo é uma pasta com a largura proporcional às
 * enviadas, com o nome, o total e a taxa ao lado. É uma lista: o leitor de tela lê o texto.
 */
function funil(passos) {
  const primeiro = passos[0]?.total ?? 0
  const lista = el(
    'ol',
    { class: 'funil' },
    passos.map((p) => {
      const barra = el('span', { class: `funil-pasta ${p.total === 0 ? 'zerada' : ''}`, 'aria-hidden': 'true' })
      barra.style.width = `${larguraDoPasso(p.total, primeiro)}%`
      return el(
        'li',
        { class: 'funil-passo' },
        el(
          'div',
          { class: 'funil-texto' },
          el('span', { class: 'funil-nome' }, p.nome),
          el('span', { class: 'funil-valores numero' }, el('b', {}, String(p.total)), ' · ', porcentagem(p.taxa)),
        ),
        el('div', { class: 'funil-trilho' }, barra),
      )
    }),
  )
  const nota =
    primeiro === 0
      ? 'Nenhuma candidatura enviada ainda: o funil começa quando uma for marcada como enviada.'
      : 'Cada passo conta as que chegaram pelo menos até ali (pelo histórico); a taxa é sobre as enviadas.'
  return pasta('Funil', 'larga', lista, el('p', { class: 'nota' }, nota))
}

function tempo(respostas) {
  const t = textosDoTempo(respostas)
  const ficha = (valor, rotulo, classe = '') =>
    el('div', { class: `ficha-numero ${classe}` }, el('b', { class: 'numero' }, valor), el('small', {}, rotulo))
  return pasta(
    'Tempo de resposta',
    '',
    el(
      'div',
      { class: 'fichas-numero' },
      ficha(t.mediana, 'mediana', 'destaque'),
      ficha(t.media, 'média'),
      ficha(t.respondidas, 'respondidas'),
      ficha(t.aguardando, 'aguardando resposta', respostas.aguardando > 0 ? 'atencao' : ''),
    ),
    el('p', { class: 'nota' }, t.resumo),
  )
}

// Medidas do gráfico de semanas (em unidades do viewBox)
const G = { largura: 480, altura: 210, topo: 22, base: 176, esquerda: 6, direita: 6 }

/** Barras das 12 semanas; a atual (a última) em verde, os zeros com uma marca na base. */
function semanas(porSemana) {
  const maximo = escalaDasBarras(porSemana.map((s) => s.total))
  const util = G.base - G.topo
  const passo = (G.largura - G.esquerda - G.direita) / porSemana.length
  const larguraBarra = passo * 0.62
  const ultima = porSemana.length - 1

  const barras = porSemana.map((s, i) => {
    const atual = i === ultima
    const x = G.esquerda + i * passo + (passo - larguraBarra) / 2
    const meio = x + larguraBarra / 2
    const h = tamanhoDaBarra(s.total, maximo, util)
    const classe = atual ? 'semana atual' : 'semana'
    return svg(
      'g',
      { class: classe },
      s.total > 0
        ? svg('rect', { x, y: G.base - h, width: larguraBarra, height: h, rx: 2 })
        : svg('rect', { class: 'zero', x, y: G.base - 2, width: larguraBarra, height: 2 }),
      svg('text', { class: 'valor', x: meio, y: G.base - h - 6, 'text-anchor': 'middle' }, String(s.total)),
      svg('text', { class: 'rotulo', x: meio, y: G.base + 17, 'text-anchor': 'middle' }, rotuloDaSemana(s.inicio)),
      atual ? svg('text', { class: 'rotulo forte', x: meio, y: G.base + 31, 'text-anchor': 'middle' }, 'atual') : null,
    )
  })

  const grafico = svg(
    'svg',
    { viewBox: `0 0 ${G.largura} ${G.altura}`, role: 'img', 'aria-label': descricaoDasSemanas(porSemana), class: 'grafico-semanas' },
    svg('line', { class: 'base', x1: G.esquerda, x2: G.largura - G.direita, y1: G.base, y2: G.base }),
    ...barras,
  )
  const total = porSemana.reduce((soma, s) => soma + s.total, 0)
  const atual = porSemana[ultima]?.total ?? 0
  return pasta(
    'Envios por semana',
    '',
    el('div', { class: 'rolagem-grafico', tabindex: 0, 'aria-label': 'Gráfico de envios por semana (role para os lados)' }, grafico),
    el(
      'p',
      { class: 'nota' },
      `${plural(total, 'envio')} nas últimas 12 semanas; ${atual} nesta. Cada barra é uma semana, de segunda a domingo (a data é a segunda-feira).`,
    ),
  )
}

/** Quantas candidaturas estão em cada etapa agora; as encerradas num grupo à parte. */
function etapasAgora(porEtapa, etapas) {
  const { andamento, finais } = separarEtapas(porEtapa, etapas)
  const maximo = escalaDasBarras(porEtapa.map((e) => e.total))
  const grupo = (titulo, lista) =>
    el(
      'div',
      { class: 'grupo-etapas' },
      el('h3', {}, titulo),
      el(
        'ul',
        { class: 'etapas-agora' },
        lista.map((e) => {
          const barra = el('span', { class: 'etapa-barra', 'aria-hidden': 'true' })
          barra.style.width = `${tamanhoDaBarra(e.total, maximo, 100, 2)}%`
          return el(
            'li',
            { class: e.total === 0 ? 'zerada' : '' },
            el('span', { class: 'etapa-nome' }, e.nome),
            el('span', { class: 'etapa-trilho' }, barra),
            el('b', { class: 'numero' }, String(e.total)),
          )
        }),
      ),
    )
  return pasta('Etapas agora', '', grupo('Em andamento', andamento), grupo('Encerradas', finais))
}

function fontes(porFonte) {
  const linhas = linhasDasFontes(porFonte)
  const conteudo =
    linhas.length === 0
      ? el('p', { class: 'nota' }, 'Nenhuma candidatura enviada ainda. As fontes aparecem quando houver envios.')
      : el(
          'div',
          { class: 'rolagem-tabela' },
          el(
            'table',
            { class: 'tabela-numeros' },
            el('caption', { class: 'so-leitor' }, 'Resultado por fonte das vagas'),
            el(
              'thead',
              {},
              el(
                'tr',
                {},
                el('th', { scope: 'col' }, 'Fonte'),
                ['Enviadas', 'Entrevistas', 'Propostas', 'Contratados', 'Taxa de entrevista'].map((t) => el('th', { scope: 'col', class: 'num' }, t)),
              ),
            ),
            el(
              'tbody',
              {},
              linhas.map((l) =>
                el(
                  'tr',
                  {},
                  el('th', { scope: 'row', class: l.semFonte ? 'sem-fonte' : '' }, l.nome),
                  [l.enviadas, l.entrevistas, l.propostas, l.contratados].map((v) => el('td', { class: 'num' }, String(v))),
                  el('td', { class: 'num forte' }, l.taxa),
                ),
              ),
            ),
          ),
        )
  return pasta(
    'Fontes',
    'larga',
    conteudo,
    linhas.length > 0 ? el('p', { class: 'nota' }, 'Taxa de entrevista: das enviadas por essa fonte, quantas chegaram à entrevista.') : null,
  )
}
