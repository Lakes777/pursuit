// Package web serve a interface (HTML, CSS e JavaScript puros, sem etapa de build), embutida no
// binário: a imagem de produção continua sendo só o executável.
package web

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed static
var arquivos embed.FS

// cabecalhos de segurança de toda resposta da interface:
//   - CSP só 'self': a página só carrega o que vem do próprio Pursuit (nada de CDN), e um XSS
//     não consegue rodar script de fora nem estilo inline;
//   - nosniff, DENY (ninguém põe a página num iframe) e sem Referer para os links das vagas.
var cabecalhos = map[string]string{
	"Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
		"font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'",
	"X-Content-Type-Options": "nosniff",
	"X-Frame-Options":        "DENY",
	"Referrer-Policy":        "no-referrer",
}

// Handler serve os arquivos de static/. Cada arquivo tem um ETag (sha256 do conteúdo, calculado
// uma vez): com Cache-Control no-cache, o navegador sempre pergunta, mas recebe 304 sem baixar de
// novo o que não mudou, e uma versão nova aparece na hora.
func Handler() http.Handler {
	raiz, err := fs.Sub(arquivos, "static")
	if err != nil {
		panic(err)
	}
	etags := map[string]string{}
	_ = fs.WalkDir(raiz, ".", func(caminho string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		conteudo, err := fs.ReadFile(raiz, caminho)
		if err != nil {
			return err
		}
		soma := sha256.Sum256(conteudo)
		etags[caminho] = `"` + hex.EncodeToString(soma[:8]) + `"`
		return nil
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for nome, valor := range cabecalhos {
			w.Header().Set(nome, valor)
		}
		caminho := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if caminho == "" {
			caminho = "index.html"
		}
		etag, existe := etags[caminho]
		if !existe {
			http.NotFound(w, r)
			return
		}
		conteudo, err := fs.ReadFile(raiz, caminho)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "no-cache")
		// ServeContent cuida do If-None-Match (304), do Content-Type pela extensão e do HEAD
		http.ServeContent(w, r, caminho, time.Time{}, bytes.NewReader(conteudo))
	})
}
