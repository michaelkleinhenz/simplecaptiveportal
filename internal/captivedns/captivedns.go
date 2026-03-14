package captivedns

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/miekg/dns"
)

const (
	defaultGateway = "10.42.0.1"
	port           = 53
)

// Server is a DNS server that resolves all queries to the gateway IP (for captive portal).
type Server struct {
	gateway net.IP
	udp     *dns.Server
	tcp     *dns.Server
	log     *slog.Logger
}

// New creates a captive DNS server that responds with gatewayIP for all A/AAAA queries.
func New(gatewayIP string, log *slog.Logger) *Server {
	if gatewayIP == "" {
		gatewayIP = defaultGateway
	}
	ip := net.ParseIP(gatewayIP)
	if ip == nil {
		ip = net.ParseIP(defaultGateway)
	}
	// Ensure IPv4 for A records
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	s := &Server{gateway: ip, log: log}
	handler := dns.HandlerFunc(s.serve)
	addr := fmt.Sprintf(":%d", port)
	s.udp = &dns.Server{Addr: addr, Net: "udp", Handler: handler}
	s.tcp = &dns.Server{Addr: addr, Net: "tcp", Handler: handler}
	return s
}

func (s *Server) serve(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	for _, q := range r.Question {
		switch q.Qtype {
		case dns.TypeA:
			if s.gateway.To4() != nil {
				m.Answer = append(m.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
					A:   s.gateway.To4(),
				})
			}
		case dns.TypeAAAA:
			// Return minimal IPv6 (e.g. ::1 or discard) so clients don't wait; or skip
			// Many clients will fall back to A if AAAA fails; for captive we only need A
		}
	}
	if err := w.WriteMsg(m); err != nil && s.log != nil {
		s.log.Debug("captive dns write", "err", err)
	}
}

// Start starts the DNS server (UDP and TCP) in the background.
func (s *Server) Start() {
	go func() {
		if err := s.udp.ListenAndServe(); err != nil && s.log != nil {
			s.log.Debug("captive dns udp", "err", err)
		}
	}()
	go func() {
		if err := s.tcp.ListenAndServe(); err != nil && s.log != nil {
			s.log.Debug("captive dns tcp", "err", err)
		}
	}()
}

// Shutdown stops the DNS server.
func (s *Server) Shutdown() error {
	_ = s.udp.Shutdown()
	_ = s.tcp.Shutdown()
	return nil
}
