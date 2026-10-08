// Início da página: confere a sessão, mostra o login ou o app, e troca de tela pelo endereço (#/...).
import { api, ErroDaAPI } from './api.js'
import { el, trocar } from './dom.js'
import { dias, lerRota, porcentagem } from './logica.js'
import { telaDa } from './telas/registro.js'
// Cada arquivo registra as próprias telas
import './telas/quadro.js'
import './telas/candidatura.js'
import './telas/formulario.js'
import './telas/numeros.js'
import './telas/lembretes.js'

const $ = (id) => document.getElementById(id)
let etapas = []
let limparTela = null
// Contador de navegação: uma resposta que chega depois de o usuário já ter mudado de tela é ignorada
let navegacao = 0

const ctx = {
  api,
  get etapas() {
    return etapas
  },
  navegar(hash) {
    if (location.hash === hash) mostrarTela()
    else location.hash = hash
  },
  avisar,
  atualizarNumeros,
  /** true se esta montagem ainda é a tela atual (use depois de cada await). */
  atual: (n) => n === navegacao,
}

function avisar(texto, tipo = 'ok') {
  const aviso = el('div', { class: `aviso-rapido ${tipo === 'erro' ? 'erro' : ''}` }, texto)
  $('avisos').append(aviso)
  setTimeout(() => aviso.remove(), tipo === 'erro' ? 7000 : 3500)
}

async function iniciar() {
  try {
    await api('GET', '/api/sessao')
    await abrirApp()
  } catch (erro) {
    if (erro instanceof ErroDaAPI && erro.status === 401) mostrarEntrada()
    else mostrarFalha(erro)
  } finally {
    $('carregando').hidden = true
  }
}

function mostrarFalha(erro) {
  $('carregando').hidden = false
  $('carregando').textContent = `Não foi possível abrir o Pursuit: ${erro.message}. Recarregue a página.`
}

function mostrarEntrada() {
  if (limparTela) limparTela()
  limparTela = null
  $('app').hidden = true
  $('entrada').hidden = false
  $('usuario').focus()
}

$('form-entrada').addEventListener('submit', async (evento) => {
  evento.preventDefault()
  const botao = evento.submitter ?? evento.target.querySelector('button')
  const erro = $('erro-entrada')
  erro.hidden = true
  botao.disabled = true
  try {
    await api('POST', '/api/sessao', { usuario: $('usuario').value, senha: $('senha').value })
    $('senha').value = ''
    $('entrada').hidden = true
    await abrirApp()
  } catch (e) {
    erro.textContent = e.status === 401 ? 'Usuário ou senha incorretos.' : e.message
    erro.hidden = false
  } finally {
    botao.disabled = false
  }
})

$('sair').addEventListener('click', async () => {
  try {
    await api('DELETE', '/api/sessao')
  } finally {
    mostrarEntrada()
  }
})

window.addEventListener('pursuit:sessao-expirou', () => {
  avisar('A sessão expirou. Entre de novo.', 'erro')
  mostrarEntrada()
})

async function abrirApp() {
  etapas = await api('GET', '/api/etapas')
  $('app').hidden = false
  atualizarNumeros()
  await mostrarTela()
}

/** A faixa do topo: em andamento, quantas chegam à entrevista, resposta mediana. */
async function atualizarNumeros() {
  const faixa = $('numeros-topo')
  try {
    const n = await api('GET', '/api/numeros')
    const entrevista = n.funil.find((p) => p.etapa === 'entrevista')
    trocar(
      faixa,
      el('div', {}, el('b', {}, String(n.emAndamento)), el('small', {}, 'em andamento')),
      el('div', {}, el('b', {}, porcentagem(entrevista?.taxa ?? null)), el('small', {}, 'chegam à entrevista')),
      el('div', {}, el('b', {}, dias(n.respostas.medianaDias)), el('small', {}, 'resposta (mediana)')),
    )
  } catch {
    trocar(faixa)
  }
}

async function mostrarTela() {
  if ($('app').hidden) return
  const rota = lerRota(location.hash)
  const tela = telaDa(rota.secao) ?? telaDa('quadro')
  const esta = ++navegacao
  if (limparTela) limparTela()
  limparTela = null

  for (const link of document.querySelectorAll('.menu a')) {
    if (link.dataset.secao === rota.secao) link.setAttribute('aria-current', 'page')
    else link.removeAttribute('aria-current')
  }
  document.title = tela.titulo ? `${tela.titulo} · Pursuit` : 'Pursuit'
  const conteudo = $('conteudo')
  trocar(conteudo)
  try {
    const limpeza = await tela.montar(conteudo, { id: rota.id, navegacao: esta }, ctx)
    if (typeof limpeza === 'function') {
      // Se o usuário já trocou de tela enquanto esta carregava, limpa na hora
      if (esta === navegacao) limparTela = limpeza
      else limpeza()
    }
  } catch (erro) {
    // Um erro de uma tela que o usuário já deixou não apaga a tela nova
    if (esta !== navegacao) return
    if (erro instanceof ErroDaAPI && erro.status === 401) return
    trocar(conteudo, el('p', { class: 'erro' }, `Não foi possível abrir esta tela: ${erro.message}`))
  }
}

window.addEventListener('hashchange', mostrarTela)
iniciar()
