package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func pedir(t *testing.T, caminho string, cab map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, caminho, nil)
	for k, v := range cab {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, r)
	return w
}

func TestRaizServeOIndexComCabecalhosDeSeguranca(t *testing.T) {
	w := pedir(t, "/", nil)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<title>Pursuit</title>") {
		t.Fatalf("status %d", w.Code)
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
		t.Errorf("Content-Type = %s", w.Header().Get("Content-Type"))
	}
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe-inline") {
		t.Errorf("CSP = %s", csp)
	}
	if w.Header().Get("X-Frame-Options") != "DENY" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("cabeçalhos = %v", w.Header())
	}
}

func TestETagDa304(t *testing.T) {
	primeira := pedir(t, "/css/pursuit.css", nil)
	etag := primeira.Header().Get("ETag")
	if primeira.Code != http.StatusOK || etag == "" || primeira.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("primeira: %d %v", primeira.Code, primeira.Header())
	}

	if segunda := pedir(t, "/css/pursuit.css", map[string]string{"If-None-Match": etag}); segunda.Code != http.StatusNotModified {
		t.Errorf("segunda: %d", segunda.Code)
	}
}

func TestTiposDosArquivos(t *testing.T) {
	casos := map[string]string{
		"/js/app.js": "text/javascript", "/css/pursuit.css": "text/css",
		"/fontes/familjen-grotesk.woff2": "font/woff2", "/img/pursuit.svg": "image/svg+xml",
	}
	for caminho, tipo := range casos {
		if w := pedir(t, caminho, nil); w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), tipo) {
			t.Errorf("%s: %d %s", caminho, w.Code, w.Header().Get("Content-Type"))
		}
	}
}

func TestNaoSaiDaPasta(t *testing.T) {
	for _, caminho := range []string{"/nada.html", "/../web.go", "/%2e%2e/web.go", "/static/index.html"} {
		if w := pedir(t, caminho, nil); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", caminho, w.Code)
		}
	}
}
