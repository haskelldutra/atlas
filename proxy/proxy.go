package proxy

import (
	"atlas/balancer"
	"atlas/inspect"
	"atlas/metrics"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Proxy struct {
	Backend []string
	Inspect inspect.InspectHTTPRequest
}

func NewProxy(backend []string, inspect *inspect.InspectHTTPRequest) *Proxy {
	return &Proxy{
		Backend: backend,
		Inspect: *inspect,
	}
}

func (p *Proxy) Server(w http.ResponseWriter, r *http.Request) {
	backend, _ := balancer.BalancerBackend(p.Backend)

	remote, err := url.Parse(backend)
	if err != nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	metrics.RequestCounter.Inc()

	if p.Inspect.InspectRequest(w, r) {
		metrics.RequestBlockedCounter.Inc()
		return
	}

	r.Host = remote.Host
	r.URL.Host = remote.Host
	r.URL.Scheme = remote.Scheme
	r.RequestURI = ""

	client := http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Do(r)

	if err != nil {
		metrics.RequestFailedCounter.Inc()
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}

	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		metrics.RequestFailedCounter.Inc()
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}
}
