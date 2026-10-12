package candidaturas

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/Lakes777/pursuit/internal/beacon"
)

// encurtadorFalso faz o papel do Beacon: cada pedido ganha um código novo.
type encurtadorFalso struct {
	mu      sync.Mutex
	criados []string
	falha   error
}

func (e *encurtadorFalso) CriarLink(context.Context) (beacon.Link, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.falha != nil {
		return beacon.Link{}, e.falha
	}
	codigo := fmt.Sprintf("c%d", len(e.criados)+1)
	e.criados = append(e.criados, codigo)
	return beacon.Link{Codigo: codigo, UrlCurta: "https://b.exemplo/r/" + codigo}, nil
}

func TestCriarLinkDoBeaconGuardaNaCandidatura(t *testing.T) {
	s, _ := novoServico(t)
	d := criar(t, s, "Nubank")
	falso := &encurtadorFalso{}

	c, err := s.CriarLinkDoBeacon(ctx, d.ID, falso)

	if err != nil {
		t.Fatal(err)
	}
	if c.Beacon == nil || *c.Beacon != (LinkDoBeacon{Codigo: "c1", URL: "https://b.exemplo/r/c1"}) {
		t.Fatalf("beacon = %+v", c.Beacon)
	}
	// Fica guardado: a ficha e a lista mostram o mesmo link
	buscada, err := s.Buscar(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	lista, err := s.Listar(ctx, Filtro{})
	if err != nil {
		t.Fatal(err)
	}
	if buscada.Beacon == nil || buscada.Beacon.Codigo != "c1" || lista[0].Beacon == nil || lista[0].Beacon.Codigo != "c1" {
		t.Errorf("buscada = %+v, lista = %+v", buscada.Beacon, lista[0].Beacon)
	}
	if c.EtapaDesde.IsZero() || !c.EtapaDesde.Equal(d.EtapaDesde) {
		t.Errorf("etapaDesde = %v, esperado %v", c.EtapaDesde, d.EtapaDesde)
	}
}

func TestSegundoPedidoDevolveOMesmoLink(t *testing.T) {
	s, _ := novoServico(t)
	d := criar(t, s, "iFood")
	falso := &encurtadorFalso{}
	if _, err := s.CriarLinkDoBeacon(ctx, d.ID, falso); err != nil {
		t.Fatal(err)
	}

	c, err := s.CriarLinkDoBeacon(ctx, d.ID, falso)

	if err != nil {
		t.Fatal(err)
	}
	// O link pode já estar num currículo: não muda, e o Beacon nem é chamado de novo
	if c.Beacon.Codigo != "c1" || len(falso.criados) != 1 {
		t.Errorf("beacon = %+v, criados = %v", c.Beacon, falso.criados)
	}
}

func TestEditarNaoMexeNoLink(t *testing.T) {
	s, _ := novoServico(t)
	d := criar(t, s, "Itaú")
	if _, err := s.CriarLinkDoBeacon(ctx, d.ID, &encurtadorFalso{}); err != nil {
		t.Fatal(err)
	}

	c, err := s.Editar(ctx, d.ID, Dados{Empresa: "Itaú Unibanco", Vaga: "Go"})

	if err != nil {
		t.Fatal(err)
	}
	if c.Beacon == nil || c.Beacon.Codigo != "c1" {
		t.Errorf("beacon = %+v", c.Beacon)
	}
}

func TestCandidaturaInexistenteNemChamaOBeacon(t *testing.T) {
	s, _ := novoServico(t)
	falso := &encurtadorFalso{}

	_, err := s.CriarLinkDoBeacon(ctx, 999999, falso)

	if !errors.Is(err, ErrNaoEncontrada) || len(falso.criados) != 0 {
		t.Errorf("err = %v, criados = %v", err, falso.criados)
	}
}

func TestBeaconForaDoArNaoGuardaNada(t *testing.T) {
	s, _ := novoServico(t)
	d := criar(t, s, "Stone")

	_, err := s.CriarLinkDoBeacon(ctx, d.ID, &encurtadorFalso{falha: errors.New("conexão recusada")})

	if !errors.Is(err, ErrBeacon) {
		t.Fatalf("err = %v", err)
	}
	buscada, err := s.Buscar(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if buscada.Beacon != nil {
		t.Errorf("beacon = %+v", buscada.Beacon)
	}
}

func TestPedidosAoMesmoTempoFicamComUmLinkSo(t *testing.T) {
	s, _ := novoServico(t)
	d := criar(t, s, "Mercado Livre")
	falso := &encurtadorFalso{}

	var espera sync.WaitGroup
	resultados := make([]Candidatura, 10)
	erros := make([]error, 10)
	for i := range 10 {
		espera.Go(func() { resultados[i], erros[i] = s.CriarLinkDoBeacon(ctx, d.ID, falso) })
	}
	espera.Wait()

	buscada, err := s.Buscar(ctx, d.ID)
	if err != nil || buscada.Beacon == nil {
		t.Fatalf("buscada = %+v, err = %v", buscada.Beacon, err)
	}
	// Todos devolvem o link que ficou guardado, mesmo os que perderam a corrida
	for i := range 10 {
		if erros[i] != nil || resultados[i].Beacon == nil || resultados[i].Beacon.Codigo != buscada.Beacon.Codigo {
			t.Errorf("pedido %d: beacon = %+v, err = %v", i, resultados[i].Beacon, erros[i])
		}
	}
}
