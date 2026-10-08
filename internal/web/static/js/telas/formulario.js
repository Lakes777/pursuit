// Formulário de candidatura: nova (#/nova, com a etapa inicial) e editar (#/editar/<id>, só os
// dados: a etapa muda pela janela de mudança, que guarda o histórico).
import { ErroDaAPI } from '../api.js'
import { el, icone, trocar } from '../dom.js'
import { campo as campoComErro } from '../janelas.js'
import { dataAPIParaLocal } from '../logica.js'
import { conferirFormulario, corpoDoFormulario, errosDoPedido, fontesSugeridas, opcoesDeModalidade } from '../quadro-logica.js'
import { naoEncontrada } from './candidatura.js'
import { registrarTela } from './registro.js'

registrarTela('nova', {
  titulo: 'Nova candidatura',
  montar: (container, params, ctx) => montarFormulario(container, params, ctx, null),
})

registrarTela('editar', {
  titulo: 'Editar candidatura',
  montar: (container, params, ctx) => montarFormulario(container, params, ctx, params.id ?? ''),
})

/** id null = nova; senão, edita a candidatura com esse id. */
async function montarFormulario(container, params, ctx, id) {
  const nova = id === null
  const [lista, atual] = await Promise.all([
    // A lista só serve para sugerir as fontes já usadas: sem ela, ficam as padrão
    ctx.api('GET', '/api/candidaturas').catch(() => []),
    nova ? null : buscar(ctx, id),
  ])
  if (!ctx.atual(params.navegacao)) return
  if (!nova && !atual) {
    trocar(container, naoEncontrada())
    return
  }
  if (atual) document.title = `Editar ${atual.empresa} · Pursuit`

  const campos = {}
  const texto = (nome, rotulo, atributos = {}, ajuda = null) =>
    campo(campos, nome, rotulo, el('input', { id: `f-${nome}`, name: nome, value: atual?.[nome] ?? '', ...atributos }), ajuda)

  const fontes = el('datalist', { id: 'f-fontes' }, fontesSugeridas(lista).map((f) => el('option', { value: f })))
  const modalidade = el('select', { id: 'f-modalidade', name: 'modalidade' },
    opcoesDeModalidade().map(([valor, nome]) => el('option', { value: valor }, nome)))
  modalidade.value = atual?.modalidade ?? ''
  const anotacoes = el('textarea', { id: 'f-anotacoes', name: 'anotacoes', maxlength: 10000, rows: 6 })
  anotacoes.value = atual?.anotacoes ?? ''

  const blocoDados = el('fieldset', { class: 'pasta grupo-form' },
    el('legend', { class: 'orelha' }, 'Vaga'),
    el('div', { class: 'grade-form' },
      texto('empresa', 'Empresa', { required: true, 'aria-required': 'true', maxlength: 200, autocomplete: 'organization' }),
      texto('vaga', 'Vaga', { required: true, 'aria-required': 'true', maxlength: 200, autocomplete: 'off' }),
      texto('link', 'Link da vaga', { type: 'url', inputmode: 'url', maxlength: 2000, placeholder: 'https://', autocomplete: 'off' }),
      texto('fonte', 'Fonte', { list: 'f-fontes', maxlength: 100, autocomplete: 'off' }, 'Onde encontrou a vaga. Escolha uma ou escreva outra.'),
      campo(campos, 'modalidade', 'Modalidade', modalidade),
      texto('salario', 'Salário', { maxlength: 100, autocomplete: 'off' }, 'Texto livre, ex.: R$ 6.000 + VR.'),
      el('div', { class: 'largo' }, campo(campos, 'anotacoes', 'Anotações', anotacoes)),
    ),
    fontes)

  let blocoEtapa = null
  if (nova) {
    const etapa = el('select', { id: 'f-etapa', name: 'etapa' }, ctx.etapas.map((e) => el('option', { value: e.etapa }, e.nome)))
    // O caso mais comum: cadastrar logo depois de mandar a candidatura
    etapa.value = ctx.etapas.some((e) => e.etapa === 'enviada') ? 'enviada' : (ctx.etapas[0]?.etapa ?? '')
    blocoEtapa = el('fieldset', { class: 'pasta grupo-form' },
      el('legend', { class: 'orelha' }, 'Etapa inicial'),
      el('div', { class: 'grade-form' },
        campo(campos, 'etapa', 'Etapa', etapa),
        campo(campos, 'em', 'Quando', el('input', { id: 'f-em', name: 'em', type: 'datetime-local', max: dataAPIParaLocal(null) }),
          'Deixe vazio para agora. Pode ser no passado.'),
        el('div', { class: 'largo' },
          campo(campos, 'observacao', 'Observação', el('textarea', { id: 'f-observacao', name: 'observacao', maxlength: 2000, rows: 3 }),
            'Opcional. Ex.: pela Gupy, indicação do Fulano.'))))
  }

  const geral = el('p', { class: 'erro', role: 'alert', hidden: true })
  const salvar = el('button', { type: 'submit', class: 'botao principal' }, nova ? 'Cadastrar' : 'Salvar')
  const cancelar = el('a', { class: 'botao', href: nova ? '#/quadro' : `#/candidatura/${atual.id}` }, 'Cancelar')
  const formulario = el('form', { class: 'formulario-candidatura', novalidate: true },
    blocoDados, blocoEtapa, geral, el('div', { class: 'botoes-form' }, cancelar, salvar))

  trocar(container,
    el('a', { class: 'botao fantasma voltar', href: nova ? '#/quadro' : `#/candidatura/${atual.id}` },
      icone('voltar'), nova ? 'Voltar ao quadro' : 'Voltar à candidatura'),
    el('h1', { class: 'titulo-form' }, nova ? 'Nova candidatura' : `Editar ${atual.empresa}`),
    el('p', { class: 'sub-form' }, 'Os campos com * são obrigatórios.'),
    formulario)
  campos.empresa.controle.focus()

  let enviando = false
  formulario.addEventListener('submit', async (evento) => {
    evento.preventDefault()
    if (enviando) return // sem duplo envio (Enter repetido, clique duplo)
    const valores = Object.fromEntries(Object.entries(campos).map(([nome, c]) => [nome, c.controle.value]))
    for (const c of Object.values(campos)) c.mostrarErro('')
    geral.hidden = true

    const locais = conferirFormulario(valores)
    if (Object.keys(locais).length) return mostrarErros(errosDoPedido({ status: 422, campos: locais }, Object.keys(campos)))

    enviando = true
    salvar.disabled = true
    try {
      const corpo = corpoDoFormulario(valores, nova)
      const salva = nova
        ? await ctx.api('POST', '/api/candidaturas', corpo)
        : await ctx.api('PUT', `/api/candidaturas/${atual.id}`, corpo)
      ctx.avisar(nova ? `Candidatura de ${salva.empresa} cadastrada.` : 'Alterações salvas.')
      ctx.atualizarNumeros()
      if (ctx.atual(params.navegacao)) ctx.navegar(`#/candidatura/${salva.id}`)
    } catch (erro) {
      if (erro.status === 401 || !ctx.atual(params.navegacao)) return
      mostrarErros(errosDoPedido(erro, Object.keys(campos)))
    } finally {
      enviando = false
      salvar.disabled = false
    }
  })

  function mostrarErros({ campos: porCampo, geral: mensagem }) {
    for (const [nome, texto] of Object.entries(porCampo)) campos[nome].mostrarErro(texto)
    if (mensagem) {
      geral.textContent = mensagem
      geral.hidden = false
    }
    // O foco vai para o primeiro campo com erro, na ordem da tela
    const primeiro = Object.keys(campos).find((nome) => porCampo[nome])
    if (primeiro) campos[primeiro].controle.focus()
    else if (mensagem) geral.scrollIntoView({ block: 'nearest' })
  }
}

/** A candidatura para editar, ou null se ela não existe. */
async function buscar(ctx, id) {
  if (!/^\d+$/.test(id)) return null
  try {
    return await ctx.api('GET', `/api/candidaturas/${id}`)
  } catch (erro) {
    if (erro instanceof ErroDaAPI && erro.status === 404) return null
    throw erro
  }
}

/** campo() das janelas, guardado em campos[nome] para mostrar os erros da API. */
function campo(campos, nome, rotulo, controle, ajuda = null) {
  campos[nome] = campoComErro(rotulo, controle, ajuda)
  return campos[nome].bloco
}
