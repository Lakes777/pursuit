// Fase 7, parte A: a preencher.
import { el } from '../dom.js'
import { registrarTela } from './registro.js'

registrarTela('quadro', {
  titulo: 'Quadro',
  montar(container) {
    container.append(el('p', { class: 'vazio' }, 'Em construção.'))
  },
})
