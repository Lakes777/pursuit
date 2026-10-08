// Cada tela se registra aqui com o nome da seção do endereço (#/<secao>/<id>).
//
// registrarTela('quadro', {
//   titulo: 'Quadro',                         // título da aba do navegador
//   montar(container, { id }, ctx) { ... },   // desenha a tela dentro de container
// })
//
// montar pode devolver uma função de limpeza (chamada ao sair da tela). ctx traz:
//   ctx.api(metodo, caminho, corpo)  → a API (js/api.js)
//   ctx.etapas                       → a lista de /api/etapas (etapa, nome, final), na ordem do processo
//   ctx.navegar('#/quadro')          → troca de tela
//   ctx.avisar('Salvo.', 'ok'|'erro')→ aviso rápido no canto
//   ctx.atualizarNumeros()           → recarrega a faixa de números do topo (depois de mudar dados)

const telas = new Map()

export function registrarTela(secao, tela) {
  telas.set(secao, tela)
}

export function telaDa(secao) {
  return telas.get(secao)
}
