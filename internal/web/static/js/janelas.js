// Janelas (<dialog>) do quadro e da ficha: mudar de etapa e confirmar antes de apagar.
// Tudo dentro da página (nada de alert/confirm), com o foco preso na janela pelo showModal e o
// Esc fechando. Cada janela some do DOM ao fechar.
import { el, trocar } from './dom.js'
import { dataAPIParaLocal, nomeDaEtapa } from './logica.js'
import { corpoDaMudanca, errosDoPedido, etapaSugerida, MENSAGEM_DATA_INCOMPLETA } from './quadro-logica.js'

const abertas = new Set()
let sequencia = 0

/** Fecha as janelas abertas (ao sair da tela pelo endereço, com uma janela ainda aberta). */
export function fecharJanelas() {
  for (const janela of abertas) janela.close()
}

/** Cria a janela, abre e devolve-a; ao fechar, ela sai do DOM e o foco volta para onde estava. */
function abrirJanela(...filhos) {
  const antes = document.activeElement
  const janela = el('dialog', {}, ...filhos)
  // Com um pedido a caminho, o Esc não fecha: senão a tela mostraria o estado antigo de algo que
  // o servidor já mudou (uma candidatura já apagada, uma ficha já movida)
  janela.addEventListener('cancel', (evento) => {
    if (janela.dataset.enviando) evento.preventDefault()
  })
  janela.addEventListener('close', () => {
    abertas.delete(janela)
    janela.remove()
    if (antes instanceof HTMLElement && antes.isConnected) antes.focus()
  })
  document.body.append(janela)
  abertas.add(janela)
  janela.showModal()
  return janela
}

/**
 * Um campo com rótulo, ajuda opcional e o lugar do erro embaixo (ligados ao controle pelo
 * aria-describedby): { bloco, controle, mostrarErro(texto) }. Também usado pelo formulário.
 */
export function campo(rotulo, controle, ajuda = null) {
  const idErro = `${controle.id}-erro`
  const erro = el('p', { class: 'erro-campo', id: idErro, hidden: true })
  const idAjuda = ajuda ? `${controle.id}-ajuda` : null
  controle.setAttribute('aria-describedby', [idAjuda, idErro].filter(Boolean).join(' '))
  const obrigatorio = controle.hasAttribute('required')
  const bloco = el('div', { class: 'campo' },
    el('label', { for: controle.id }, rotulo, obrigatorio ? el('span', { class: 'obrigatorio', 'aria-hidden': 'true' }, ' *') : null),
    controle,
    ajuda ? el('p', { class: 'ajuda', id: idAjuda }, ajuda) : null,
    erro)
  return {
    bloco,
    controle,
    mostrarErro(mensagem) {
      bloco.classList.toggle('com-erro', Boolean(mensagem))
      erro.hidden = !mensagem
      erro.textContent = mensagem || ''
      if (mensagem) controle.setAttribute('aria-invalid', 'true')
      else controle.removeAttribute('aria-invalid')
    },
  }
}

/**
 * Janela "Mover para <etapa>". Com destino (a pasta onde a ficha foi solta), só pede a data e a
 * observação; sem destino (botão "Mover"), pede também a etapa. Resolve com o detalhe que a API
 * devolveu, ou null se a pessoa cancelar.
 */
export function abrirMudancaDeEtapa(ctx, candidatura, destino = null) {
  return new Promise((resolver) => {
    const n = ++sequencia
    const titulo = el('h2', { id: `mover-titulo-${n}` })
    const contexto = el('p', { class: 'janela-contexto' }, `${candidatura.empresa} · ${candidatura.vaga}`)
    const geral = el('p', { class: 'erro', role: 'alert', hidden: true })

    const opcoes = ctx.etapas.filter((e) => e.etapa !== candidatura.etapa)
    // A próxima etapa do processo, que é a mudança mais comum; sem uma (etapa final), nada vem
    // escolhido: um Enter apressado não pode mandar a candidatura de volta ao começo
    const sugerida = destino ?? etapaSugerida(ctx.etapas, candidatura.etapa)
    const escolha = campo('Para qual etapa', el('select', { id: `mover-etapa-${n}`, name: 'etapa', required: true },
      sugerida ? null : el('option', { value: '' }, 'Escolha a etapa'),
      opcoes.map((e) => el('option', { value: e.etapa }, e.nome))))
    escolha.controle.value = sugerida ?? ''
    const data = campo('Quando', el('input', { id: `mover-em-${n}`, name: 'em', type: 'datetime-local', max: dataAPIParaLocal(null) }),
      'Deixe vazio para agora.')
    const observacao = campo('Observação', el('textarea', { id: `mover-obs-${n}`, name: 'observacao', maxlength: 2000, rows: 3 }),
      'Opcional. Ex.: o RH ligou, entrevista com o tech lead.')
    const campos = { etapa: escolha, em: data, observacao }

    const atualizarTitulo = () => {
      titulo.textContent = escolha.controle.value ? `Mover para ${nomeDaEtapa(ctx.etapas, escolha.controle.value)}` : 'Mover para outra etapa'
    }
    escolha.controle.addEventListener('change', atualizarTitulo)
    atualizarTitulo()

    const salvar = el('button', { type: 'submit', class: 'botao principal' }, 'Mover')
    const cancelar = el('button', { type: 'button', class: 'botao' }, 'Cancelar')
    let resultado = null

    const formulario = el('form', { novalidate: true },
      titulo, contexto,
      el('div', { class: 'campos-janela' }, destino ? null : escolha.bloco, data.bloco, observacao.bloco),
      geral,
      el('div', { class: 'botoes-janela' }, cancelar, salvar))

    formulario.addEventListener('submit', async (evento) => {
      evento.preventDefault()
      if (salvar.disabled) return
      for (const c of Object.values(campos)) c.mostrarErro('')
      geral.hidden = true
      if (!escolha.controle.value) {
        escolha.mostrarErro('Escolha a etapa.')
        return escolha.controle.focus()
      }
      // Data digitada pela metade: o campo fica vazio para o JavaScript, e a API registraria "agora"
      if (data.controle.validity.badInput) {
        data.mostrarErro(MENSAGEM_DATA_INCOMPLETA)
        return data.controle.focus()
      }
      salvar.disabled = true
      cancelar.disabled = true
      janela.dataset.enviando = 'sim'
      try {
        resultado = await ctx.api('POST', `/api/candidaturas/${candidatura.id}/etapas`,
          corpoDaMudanca(escolha.controle.value, data.controle.value, observacao.controle.value))
        janela.close()
      } catch (erro) {
        if (erro.status === 401) return janela.close()
        const { campos: porCampo, geral: mensagem } = errosDoPedido(erro, destino ? ['em', 'observacao'] : Object.keys(campos))
        for (const [nome, texto] of Object.entries(porCampo)) campos[nome].mostrarErro(texto)
        if (mensagem) {
          geral.textContent = mensagem
          geral.hidden = false
        }
        const primeiro = Object.keys(porCampo)[0]
        if (primeiro) campos[primeiro].controle.focus()
      } finally {
        salvar.disabled = false
        cancelar.disabled = false
        delete janela.dataset.enviando
      }
    })
    cancelar.addEventListener('click', () => janela.close())

    const janela = abrirJanela(formulario)
    janela.setAttribute('aria-labelledby', titulo.id)
    janela.addEventListener('close', () => resolver(resultado))
    // O foco começa no primeiro campo (a etapa, ou a data quando a ficha foi solta numa pasta)
    ;(destino ? data : escolha).controle.focus()
  })
}

/**
 * Pergunta antes de uma ação sem volta. acao() roda com o botão desabilitado; se ela falhar, o
 * erro aparece na própria janela. Resolve true se a ação deu certo, false se cancelou.
 */
export function confirmar({ titulo, texto, botao, acao }) {
  return new Promise((resolver) => {
    const n = ++sequencia
    const cabecalho = el('h2', { id: `confirmar-titulo-${n}` }, titulo)
    const geral = el('p', { class: 'erro', role: 'alert', hidden: true })
    const sim = el('button', { type: 'button', class: 'botao perigo' }, botao)
    const nao = el('button', { type: 'button', class: 'botao' }, 'Cancelar')
    let feito = false

    const janela = abrirJanela(cabecalho, el('p', {}, texto), geral, el('div', { class: 'botoes-janela' }, nao, sim))
    janela.setAttribute('aria-labelledby', cabecalho.id)
    janela.addEventListener('close', () => resolver(feito))
    nao.addEventListener('click', () => janela.close())
    sim.addEventListener('click', async () => {
      if (sim.disabled) return
      sim.disabled = true
      nao.disabled = true
      janela.dataset.enviando = 'sim'
      geral.hidden = true
      try {
        await acao()
        feito = true
        janela.close()
      } catch (erro) {
        if (erro.status === 401) return janela.close()
        trocar(geral, `Não foi possível: ${erro.message}.`)
        geral.hidden = false
      } finally {
        sim.disabled = false
        nao.disabled = false
        delete janela.dataset.enviando
      }
    })
    // Numa ação sem volta, o foco começa no "Cancelar": um Enter apressado não apaga nada
    nao.focus()
  })
}
