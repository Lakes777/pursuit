// A ficha de uma candidatura: os dados, a etapa atual, o link do portfólio (Beacon), o histórico
// como linha do tempo e as ações (mudar etapa, editar, apagar).
import { ErroDaAPI } from '../api.js'
import { el, icone, trocar } from '../dom.js'
import { formatarDia, textoDosCliques, ultimoDiaComClique } from '../beacon-logica.js'
import { abrirMudancaDeEtapa, confirmar, fecharJanelas } from '../janelas.js'
import { formatarData, formatarDataHora, nomeDaEtapa } from '../logica.js'
import { diasNaEtapa, linkSeguro, nomeDaModalidade, textoDiasNaEtapa, textoDoPasso } from '../quadro-logica.js'
import { registrarTela } from './registro.js'

/** O aviso de candidatura que não existe (também usado pelo formulário de edição). */
export function naoEncontrada() {
  return el('div', { class: 'pasta nao-encontrada' },
    el('span', { class: 'orelha' }, 'Não encontrada'),
    el('h1', {}, 'Esta candidatura não existe'),
    el('p', {}, 'Talvez ela tenha sido apagada, ou o endereço esteja errado.'),
    el('a', { class: 'botao principal', href: '#/quadro' }, icone('voltar'), 'Voltar ao quadro'))
}

const voltar = () => el('a', { class: 'botao fantasma voltar', href: '#/quadro' }, icone('voltar'), 'Voltar ao quadro')

registrarTela('candidatura', {
  titulo: 'Candidatura',
  async montar(container, params, ctx) {
    let c
    // Sem o estado do Beacon, a ficha abre igual, só sem a parte do link
    let beacon = { ligado: false }
    try {
      if (!/^\d+$/.test(params.id ?? '')) throw new ErroDaAPI(404)
      ;[c, beacon] = await Promise.all([
        ctx.api('GET', `/api/candidaturas/${params.id}`),
        ctx.api('GET', '/api/beacon').catch(() => beacon),
      ])
    } catch (erro) {
      if (!(erro instanceof ErroDaAPI) || erro.status !== 404) throw erro
      if (ctx.atual(params.navegacao)) trocar(container, naoEncontrada())
      return
    }
    if (!ctx.atual(params.navegacao)) return
    document.title = `${c.empresa} · Pursuit`
    desenhar(c)

    function desenhar(c) {
      const etapa = ctx.etapas.find((e) => e.etapa === c.etapa)
      const nomeDe = (e) => nomeDaEtapa(ctx.etapas, e)

      const mudar = el('button', { type: 'button', class: 'botao principal' }, icone('seta'), 'Mudar etapa')
      const apagar = el('button', { type: 'button', class: 'botao perigo' }, icone('lixeira'), 'Apagar')
      mudar.addEventListener('click', async () => {
        const detalhe = await abrirMudancaDeEtapa(ctx, c)
        if (!detalhe || !ctx.atual(params.navegacao)) return
        ctx.avisar(`Etapa mudada para ${nomeDe(detalhe.etapa)}.`)
        ctx.atualizarNumeros()
        desenhar(detalhe)
        container.querySelector('.etapa-atual')?.focus()
      })
      apagar.addEventListener('click', async () => {
        const apagada = await confirmar({
          titulo: 'Apagar a candidatura?',
          texto: `${c.empresa} · ${c.vaga} e todo o histórico de etapas serão apagados. Não dá para desfazer.`,
          botao: 'Apagar',
          acao: () => ctx.api('DELETE', `/api/candidaturas/${c.id}`),
        })
        if (!apagada) return
        ctx.avisar(`Candidatura de ${c.empresa} apagada.`)
        ctx.atualizarNumeros()
        if (ctx.atual(params.navegacao)) ctx.navegar('#/quadro')
      })

      const dados = [
        ['Link', !c.link ? null
          : linkSeguro(c.link) ? el('a', { href: c.link, target: '_blank', rel: 'noopener noreferrer', class: 'link-vaga' }, c.link, icone('link'))
            : el('span', { class: 'link-vaga' }, c.link)],
        ['Fonte', c.fonte],
        ['Modalidade', nomeDaModalidade(c.modalidade)],
        ['Salário', c.salario],
        ['Cadastrada em', formatarData(c.criadaEm)],
      ]

      trocar(container,
        voltar(),
        el('div', { class: 'cabeca-candidatura' },
          el('div', { class: 'titulo-candidatura' },
            el('h1', {}, c.empresa),
            el('p', { class: 'vaga-candidatura' }, c.vaga)),
          el('div', { class: 'acoes-candidatura' },
            mudar,
            el('a', { class: 'botao', href: `#/editar/${c.id}` }, icone('lapis'), 'Editar'),
            apagar)),
        el('div', { class: 'detalhe-candidatura' },
          el('section', { class: 'pasta etapa-atual', tabindex: '-1', 'aria-labelledby': 'etapa-atual-titulo' },
            el('h2', { class: 'orelha', id: 'etapa-atual-titulo' }, 'Etapa atual'),
            el('p', { class: `nome-etapa${etapa?.final ? ' final' : ''}` }, nomeDe(c.etapa)),
            el('p', { class: 'desde-etapa' }, icone('relogio'),
              `${textoDiasNaEtapa(diasNaEtapa(c))} (desde ${formatarDataHora(c.etapaDesde ?? c.atualizadaEm)})`)),
          linkDoPortfolio(c),
          el('section', { class: 'pasta dados-candidatura', 'aria-labelledby': 'dados-titulo' },
            el('h2', { class: 'orelha', id: 'dados-titulo' }, 'Dados'),
            el('dl', { class: 'lista-dados' },
              dados.map(([rotulo, valor]) => [el('dt', {}, rotulo), el('dd', {}, valor || el('span', { class: 'sem-dado' }, 'Não informado'))])),
            el('h3', { class: 'titulo-anotacoes' }, 'Anotações'),
            c.anotacoes
              ? el('p', { class: 'anotacoes' }, c.anotacoes)
              : el('p', { class: 'sem-dado' }, 'Nenhuma anotação.')),
          el('section', { class: 'pasta historico', 'aria-labelledby': 'historico-titulo' },
            el('h2', { class: 'orelha', id: 'historico-titulo' }, 'Histórico'),
            el('ol', { class: 'linha-do-tempo' },
              c.historico.map((m) => el('li', {},
                el('p', { class: 'passo' }, textoDoPasso(m, nomeDe)),
                el('time', { datetime: m.em }, formatarDataHora(m.em)),
                m.observacao ? el('p', { class: 'observacao' }, m.observacao) : null))))))
    }

    /**
     * O link curto desta candidatura: sem link, o botão para criar; com link, o endereço para copiar
     * e quantas vezes foi aberto. Some de vez com o Beacon desligado e sem link guardado.
     */
    function linkDoPortfolio(candidatura) {
      const c = candidatura
      if (!c.beacon && !beacon.ligado) return null
      const secao = el('section', { class: 'pasta link-portfolio', 'aria-labelledby': 'link-portfolio-titulo' },
        el('h2', { class: 'orelha', id: 'link-portfolio-titulo' }, 'Link do portfólio'))
      if (!c.beacon) {
        const criar = el('button', { type: 'button', class: 'botao' }, icone('link'), 'Criar link')
        criar.addEventListener('click', async () => {
          criar.disabled = true
          try {
            const atualizada = await ctx.api('POST', `/api/candidaturas/${c.id}/beacon`)
            if (!ctx.atual(params.navegacao)) return
            ctx.avisar('Link criado. Use ele no currículo desta vaga.')
            secao.replaceWith(linkDoPortfolio(atualizada))
            container.querySelector('.link-portfolio .copiar')?.focus()
          } catch (erro) {
            if (erro.status !== 401 && ctx.atual(params.navegacao)) ctx.avisar(`Não foi possível criar o link: ${erro.message}.`, 'erro')
            criar.disabled = false
          }
        })
        secao.append(
          el('p', { class: 'explica-link' },
            'Um endereço curto só desta vaga, que leva ao seu portfólio. Ponha no currículo que mandar para ela: quando alguém abrir, aparece aqui.'),
          criar)
        return secao
      }
      const copiar = el('button', { type: 'button', class: 'botao copiar' }, 'Copiar')
      copiar.addEventListener('click', async () => {
        try {
          await navigator.clipboard.writeText(c.beacon.url)
          ctx.avisar('Link copiado.')
        } catch {
          // Sem permissão da área de transferência: seleciona o texto para copiar à mão
          getSelection()?.selectAllChildren(endereco)
        }
      })
      const endereco = el('code', { class: 'endereco-link' }, c.beacon.url)
      const cliques = el('p', { class: 'cliques-link', 'aria-live': 'polite' }, 'Contando os cliques...')
      secao.append(el('div', { class: 'linha-link' }, endereco, copiar), cliques)
      if (!beacon.ligado) {
        // O link guardado continua valendo; só a contagem, que passa pelo Beacon, está desligada
        trocar(cliques, el('span', { class: 'sem-dado' }, 'A contagem de cliques está desligada (sem o Beacon configurado).'))
        return secao
      }
      ctx.api('GET', `/api/candidaturas/${c.id}/beacon/cliques`).then((n) => {
        if (!ctx.atual(params.navegacao)) return
        const ultimo = ultimoDiaComClique(n.porDia)
        trocar(cliques, el('strong', { class: n.total ? 'aberto' : '' }, textoDosCliques(n.total)),
          ultimo ? ` · o último em ${formatarDia(ultimo)}` : null,
          el('span', { class: 'periodo-cliques' }, ` (últimos ${beacon.dias ?? 365} dias, sem contar robôs)`))
      }).catch((erro) => {
        if (erro.status !== 401 && ctx.atual(params.navegacao)) trocar(cliques, el('span', { class: 'sem-dado' }, `Não foi possível contar os cliques: ${erro.message}.`))
      })
      return secao
    }

    return fecharJanelas
  },
})
