// Fase 7, parte B: a preencher.
import { el } from '../dom.js'
import { registrarTela } from './registro.js'

registrarTela('lembretes', {
  titulo: 'Lembretes',
  montar(container) {
    container.append(el('p', { class: 'vazio' }, 'Em construção.'))
  },
})
