// O quadro: uma pasta por etapa em andamento, com as fichas das candidaturas dentro; as
// encerradas (contratado, recusada, desisti) ficam numa área recolhível embaixo.
// Para mudar de etapa: arrastar a ficha para outra pasta (mouse) ou o botão "Mover" da ficha
// (teclado e celular). Os dois abrem a mesma janela, que pede a data e a observação.
import { el, icone, trocar } from '../dom.js'
import { fecharJanelas, abrirMudancaDeEtapa } from '../janelas.js'
import { nomeDaEtapa } from '../logica.js'
import { agruparPorEtapa, textoDiasNaEtapa, totalDeFichas } from '../quadro-logica.js'
import { registrarTela } from './registro.js'

// Lembrados entre uma visita e outra ao quadro (só nesta aba)
let buscaGuardada = ''
let encerradasAbertas = false

const TIPO_ARRASTO = 'application/x-pursuit-candidatura'

registrarTela('quadro', {
  titulo: 'Quadro',
  async montar(container, params, ctx) {
    let candidaturas = []
    let arrastada = null // a candidatura sendo arrastada (o dragover não pode ler o dataTransfer)
    let pedido = 0 // a resposta de uma busca antiga, que chega depois da nova, é ignorada
    let espera = null

    const campoBusca = el('input', {
      id: 'busca-quadro', type: 'search', name: 'busca', value: buscaGuardada, autocomplete: 'off',
      placeholder: 'Buscar por empresa ou vaga', maxlength: 200,
    })
    const formBusca = el('form', { class: 'busca-quadro', role: 'search' },
      el('label', { for: 'busca-quadro', class: 'so-leitor' }, 'Buscar por empresa ou vaga'),
      icone('busca'), campoBusca)
    const resumo = el('p', { class: 'resumo-quadro', 'aria-live': 'polite' })
    const area = el('div', { class: 'area-quadro' })
    trocar(container,
      el('div', { class: 'topo-quadro' }, el('h1', {}, 'Quadro'), formBusca, resumo),
      area)

    formBusca.addEventListener('submit', (evento) => {
      evento.preventDefault()
      clearTimeout(espera)
      recarregar()
    })
    campoBusca.addEventListener('input', () => {
      clearTimeout(espera)
      espera = setTimeout(recarregar, 250)
    })

    /** carregar() depois que a tela já está montada: um erro vira aviso, sem apagar o quadro. */
    function recarregar(focar) {
      return carregar(focar).catch((erro) => {
        if (erro.status !== 401 && ctx.atual(params.navegacao)) ctx.avisar(`Não foi possível carregar o quadro: ${erro.message}.`, 'erro')
      })
    }

    /** Busca as candidaturas e as paradas e redesenha. focar: id da ficha que recebe o foco. */
    async function carregar(focar = null) {
      const meu = ++pedido
      const busca = campoBusca.value.trim()
      buscaGuardada = campoBusca.value
      const caminho = busca ? `/api/candidaturas?busca=${encodeURIComponent(busca)}` : '/api/candidaturas'
      const [lista, paradas] = await Promise.all([
        ctx.api('GET', caminho),
        // Sem os lembretes, o quadro ainda funciona (só não marca as paradas)
        ctx.api('GET', '/api/lembretes').catch(() => []),
      ])
      if (!ctx.atual(params.navegacao) || meu !== pedido) return
      candidaturas = lista
      desenhar(busca, new Set(paradas.map((p) => p.candidaturaId)))
      if (focar) area.querySelector(`.ficha[data-id="${focar}"] .ficha-link`)?.focus()
    }

    function desenhar(busca, paradas) {
      if (!candidaturas.length) {
        trocar(resumo)
        trocar(area, busca ? semResultado(busca) : quadroVazio())
        return
      }
      const { abertas, encerradas } = agruparPorEtapa(candidaturas, ctx.etapas, paradas)
      const emAndamento = totalDeFichas(abertas)
      const quantasParadas = abertas.reduce((s, p) => s + p.fichas.filter((f) => f.parada).length, 0)
      trocar(resumo,
        busca ? `${candidaturas.length} encontrada${candidaturas.length === 1 ? '' : 's'} · ` : '',
        `${emAndamento} em andamento`,
        quantasParadas ? el('span', { class: 'etiqueta parada' }, `${quantasParadas} parada${quantasParadas === 1 ? '' : 's'}`) : null)

      const detalhes = el('details', { class: 'encerradas', open: encerradasAbertas },
        el('summary', {}, 'Encerradas', el('span', { class: 'contagem numero' }, String(totalDeFichas(encerradas)))),
        el('div', { class: 'quadro quadro-encerradas' }, encerradas.map(pasta)))
      detalhes.addEventListener('toggle', () => {
        encerradasAbertas = detalhes.open
      })
      trocar(area,
        el('div', { class: 'quadro' }, abertas.map(pasta)),
        detalhes)
    }

    function pasta(p) {
      const elemento = el('section', { class: 'pasta pasta-etapa', dataset: { etapa: p.etapa } },
        el('h2', { class: 'orelha', title: p.nome }, el('span', { class: 'nome-orelha' }, p.nome), el('span', { class: 'contagem numero' }, String(p.fichas.length))),
        p.fichas.length
          ? el('ul', { class: 'fichas' }, p.fichas.map((f) => el('li', {}, ficha(f))))
          : el('p', { class: 'pasta-vazia' }, 'Nenhuma candidatura aqui.'))
      soltarEm(elemento, p.etapa)
      return elemento
    }

    function ficha(f) {
      const link = el('a', { class: 'ficha-link', href: `#/candidatura/${f.id}`, draggable: 'false' }, f.empresa)
      const mover = el('button', { type: 'button', class: 'botao fantasma mover' },
        icone('seta'), 'Mover', el('span', { class: 'so-leitor' }, ` ${f.empresa} para outra etapa`))
      mover.addEventListener('click', () => mudar(f, null))
      const elemento = el('article', { class: `ficha${f.parada ? ' parada' : ''}`, draggable: 'true', dataset: { id: f.id } },
        el('h3', { class: 'ficha-empresa' }, link),
        el('p', { class: 'ficha-vaga' }, f.vaga),
        el('p', { class: 'ficha-dias' }, icone('relogio'), textoDiasNaEtapa(f.dias),
          f.parada ? el('span', { class: 'etiqueta parada' }, 'Parada') : null),
        mover)
      elemento.addEventListener('dragstart', (evento) => {
        arrastada = f
        evento.dataTransfer.effectAllowed = 'move'
        evento.dataTransfer.setData(TIPO_ARRASTO, String(f.id))
        evento.dataTransfer.setData('text/plain', `${f.empresa} · ${f.vaga}`)
        elemento.classList.add('arrastando')
        area.classList.add('arrastando')
      })
      elemento.addEventListener('dragend', () => {
        arrastada = null
        elemento.classList.remove('arrastando')
        area.classList.remove('arrastando')
        for (const alvo of area.querySelectorAll('.alvo')) alvo.classList.remove('alvo')
      })
      return elemento
    }

    /** Liga a pasta ao arrastar e soltar: ela aceita qualquer ficha de outra etapa. */
    function soltarEm(elemento, etapa) {
      const aceita = (evento) => arrastada && arrastada.etapa !== etapa && evento.dataTransfer.types.includes(TIPO_ARRASTO)
      elemento.addEventListener('dragover', (evento) => {
        if (!aceita(evento)) return
        evento.preventDefault()
        evento.dataTransfer.dropEffect = 'move'
        elemento.classList.add('alvo')
      })
      elemento.addEventListener('dragleave', (evento) => {
        if (!elemento.contains(evento.relatedTarget)) elemento.classList.remove('alvo')
      })
      elemento.addEventListener('drop', (evento) => {
        if (!aceita(evento)) return
        evento.preventDefault()
        elemento.classList.remove('alvo')
        const f = arrastada
        arrastada = null
        mudar(f, etapa)
      })
    }

    async function mudar(f, destino) {
      const detalhe = await abrirMudancaDeEtapa(ctx, f, destino)
      if (!detalhe || !ctx.atual(params.navegacao)) return
      ctx.avisar(`${f.empresa} foi para ${nomeDaEtapa(ctx.etapas, detalhe.etapa)}.`)
      ctx.atualizarNumeros()
      if (ctx.etapas.find((e) => e.etapa === detalhe.etapa)?.final) encerradasAbertas = true
      await recarregar(f.id)
    }

    function quadroVazio() {
      return el('div', { class: 'pasta quadro-vazio' },
        el('span', { class: 'orelha' }, 'Arquivo vazio'),
        el('h2', {}, 'Nenhuma candidatura ainda'),
        el('p', {}, 'Cadastre a primeira vaga em que se candidatou (ou que pretende tentar). Cada uma vira uma ficha na pasta da etapa em que está, e o quadro avisa quando alguma fica parada.'),
        el('a', { class: 'botao principal', href: '#/nova' }, icone('mais'), 'Cadastrar a primeira candidatura'))
    }

    function semResultado(busca) {
      const limpar = el('button', { type: 'button', class: 'botao' }, icone('x'), 'Limpar a busca')
      limpar.addEventListener('click', () => {
        campoBusca.value = ''
        recarregar()
        campoBusca.focus()
      })
      return el('div', { class: 'vazio' }, el('p', {}, `Nenhuma candidatura com "${busca}" na empresa ou na vaga.`), limpar)
    }

    await carregar()
    return () => {
      clearTimeout(espera)
      fecharJanelas()
    }
  },
})
