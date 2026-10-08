// Monta elementos sem innerHTML: o texto vindo do banco (empresa, anotações) entra sempre como
// texto, nunca como HTML, então não há como um XSS se infiltrar por ele.

/**
 * el("a", { class: "botao", href: "#/quadro", onclick: fn, dataset: { id: 3 } }, "Texto", outroElemento)
 * Atributos: class, dataset, on<evento>; o resto vira setAttribute. Filhos: texto, elementos,
 * listas ou null/false (ignorados).
 */
export function el(tag, atributos = {}, ...filhos) {
  const elemento = document.createElement(tag)
  for (const [nome, valor] of Object.entries(atributos ?? {})) {
    if (valor === null || valor === undefined || valor === false) continue
    if (nome === 'class') elemento.className = valor
    else if (nome === 'dataset') Object.assign(elemento.dataset, valor)
    else if (nome.startsWith('on') && typeof valor === 'function') elemento.addEventListener(nome.slice(2), valor)
    else if (valor === true) elemento.setAttribute(nome, '')
    else elemento.setAttribute(nome, String(valor))
  }
  adicionar(elemento, filhos)
  return elemento
}

function adicionar(pai, filhos) {
  for (const filho of filhos) {
    if (filho === null || filho === undefined || filho === false) continue
    if (Array.isArray(filho)) adicionar(pai, filho)
    else pai.append(filho instanceof Node ? filho : String(filho))
  }
}

const SVG = 'http://www.w3.org/2000/svg'

/** Um ícone do sprite do index.html: icone("mais"). */
export function icone(nome, classe = '') {
  const svg = document.createElementNS(SVG, 'svg')
  svg.setAttribute('aria-hidden', 'true')
  if (classe) svg.setAttribute('class', classe)
  const usar = document.createElementNS(SVG, 'use')
  usar.setAttribute('href', `#i-${nome}`)
  svg.append(usar)
  return svg
}

/** Cria um elemento SVG (para gráficos): svg("rect", { x: 0, width: 10 }). */
export function svg(tag, atributos = {}, ...filhos) {
  const elemento = document.createElementNS(SVG, tag)
  for (const [nome, valor] of Object.entries(atributos)) {
    if (valor !== null && valor !== undefined) elemento.setAttribute(nome, String(valor))
  }
  for (const filho of filhos) if (filho !== null && filho !== undefined) elemento.append(filho)
  return elemento
}

/** Troca todo o conteúdo de um elemento. */
export function trocar(pai, ...filhos) {
  pai.replaceChildren()
  adicionar(pai, filhos)
}
