package contas

import (
	"net/netip"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limite conta as tentativas de login por IP (token bucket): N por minuto, com rajada de N.
// Seguro para várias goroutines (cada pedido HTTP roda na sua).
type Limite struct {
	porMinuto int
	mu        sync.Mutex
	baldes    map[netip.Prefix]*balde
	limpeza   time.Time
}

type balde struct {
	limitador *rate.Limiter
	visto     time.Time
}

// NovoLimite com n tentativas por minuto por IP.
func NovoLimite(porMinuto int) *Limite {
	return &Limite{porMinuto: max(1, porMinuto), baldes: map[netip.Prefix]*balde{}}
}

// Permitir diz se o IP ainda pode tentar agora (e conta a tentativa).
func (l *Limite) Permitir(ip netip.Addr, agora time.Time) bool {
	chave := chaveDoIP(ip)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limparAntigos(agora)
	b, ok := l.baldes[chave]
	if !ok {
		b = &balde{limitador: rate.NewLimiter(rate.Every(time.Minute/time.Duration(l.porMinuto)), l.porMinuto)}
		l.baldes[chave] = b
	}
	b.visto = agora
	return b.limitador.AllowN(agora, 1)
}

// limparAntigos tira, uma vez por minuto, os IPs parados há mais de 10 min (o balde
// deles já estaria cheio de novo): o mapa não cresce para sempre.
func (l *Limite) limparAntigos(agora time.Time) {
	if agora.Sub(l.limpeza) < time.Minute {
		return
	}
	l.limpeza = agora
	for chave, b := range l.baldes {
		if agora.Sub(b.visto) > 10*time.Minute {
			delete(l.baldes, chave)
		}
	}
}

// chaveDoIP: IPv4 conta sozinho; IPv6 conta pelo /64, porque quem tem um IPv6 costuma ter
// a faixa /64 inteira e trocaria de endereço a cada tentativa.
func chaveDoIP(ip netip.Addr) netip.Prefix {
	ip = ip.Unmap()
	if ip.Is4() {
		return netip.PrefixFrom(ip, 32)
	}
	prefixo, _ := ip.Prefix(64)
	return prefixo
}
