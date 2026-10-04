package app

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

// ALPNRouter routes incoming TLS connections based on negotiated ALPN protocol.
// - xmpp-client → Prosody C2S (with PROXY protocol v1)
// - xmpp-server → Prosody S2S (with PROXY protocol v1)
// - http/1.1, acme-tls/1 → Fiber (pass through)
type ALPNRouter struct {
	listener    net.Listener
	connChan    chan net.Conn
	closeCh     chan struct{}
	closeOnce   sync.Once
	prosodyC2S  string // Prosody C2S backend address
	prosodyS2S  string // Prosody S2S backend address
}

// NewALPNRouter creates a new ALPN-based connection router.
func NewALPNRouter(ln net.Listener) *ALPNRouter {
	// Get Prosody backend addresses from environment
	prosodyC2S := os.Getenv("XMPP_PROXY_PROSODY_C2S")
	if prosodyC2S == "" {
		prosodyC2S = "127.0.0.1:5222" // Default C2S port
	}

	prosodyS2S := os.Getenv("XMPP_PROXY_PROSODY_S2S")
	if prosodyS2S == "" {
		prosodyS2S = "127.0.0.1:5269" // Default S2S port
	}

	router := &ALPNRouter{
		listener:   ln,
		connChan:   make(chan net.Conn),
		closeCh:    make(chan struct{}),
		prosodyC2S: prosodyC2S,
		prosodyS2S: prosodyS2S,
	}

	go router.route()
	return router
}

// Accept returns the next HTTP connection (XMPP connections are proxied separately).
func (r *ALPNRouter) Accept() (net.Conn, error) {
	select {
	case conn := <-r.connChan:
		return conn, nil
	case <-r.closeCh:
		return nil, net.ErrClosed
	}
}

// Close closes the underlying listener.
func (r *ALPNRouter) Close() error {
	var err error
	r.closeOnce.Do(func() {
		close(r.closeCh)
		err = r.listener.Close()
	})
	return err
}

// Addr returns the underlying listener's address.
func (r *ALPNRouter) Addr() net.Addr {
	return r.listener.Addr()
}

// route accepts connections and routes based on ALPN protocol.
func (r *ALPNRouter) route() {
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			select {
			case <-r.closeCh:
				return
			default:
				logger().Err(err).Msg("ALPN router accept error")
				continue
			}
		}

		go r.handleConnection(conn)
	}
}

// handleConnection inspects ALPN and routes to appropriate backend.
func (r *ALPNRouter) handleConnection(conn net.Conn) {
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		logger().Warn().Msg("Non-TLS connection on ALPN router")
		conn.Close()
		return
	}

	// Force TLS handshake to complete so we can read negotiated protocol
	if err := tlsConn.Handshake(); err != nil {
		logger().Err(err).Msg("TLS handshake failed")
		conn.Close()
		return
	}

	proto := tlsConn.ConnectionState().NegotiatedProtocol

	switch proto {
	case "xmpp-client":
		r.proxyToXMPP(tlsConn, r.prosodyC2S, "C2S")
	case "xmpp-server":
		r.proxyToXMPP(tlsConn, r.prosodyS2S, "S2S")
	case "http/1.1", "acme-tls/1", "":
		// HTTP traffic → pass to Fiber
		select {
		case r.connChan <- tlsConn:
		case <-r.closeCh:
			tlsConn.Close()
		}
	default:
		logger().Warn().Str("protocol", proto).Msg("Unknown ALPN protocol")
		tlsConn.Close()
	}
}

// proxyToXMPP forwards connection to Prosody with PROXY protocol v1 header.
func (r *ALPNRouter) proxyToXMPP(clientConn *tls.Conn, backend, connType string) {
	defer clientConn.Close()

	// Connect to Prosody backend
	backendConn, err := net.Dial("tcp", backend)
	if err != nil {
		logger().Err(err).Str("backend", backend).Str("type", connType).Msg("Failed to connect to Prosody")
		return
	}
	defer backendConn.Close()

	// Send PROXY protocol v1 header to preserve client IP
	if err := sendProxyProtocol(backendConn, clientConn); err != nil {
		logger().Err(err).Str("type", connType).Msg("Failed to send PROXY protocol header")
		return
	}

	logger().Debug().
		Str("client", clientConn.RemoteAddr().String()).
		Str("backend", backend).
		Str("type", connType).
		Msg("ALPN routed XMPP connection")

	// Bidirectional copy
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(backendConn, clientConn)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(clientConn, backendConn)
		done <- struct{}{}
	}()

	<-done // Wait for one direction to finish
}

// sendProxyProtocol sends PROXY protocol v1 header.
// Format: PROXY TCP4 <client-ip> <proxy-ip> <client-port> <proxy-port>\r\n
func sendProxyProtocol(backend net.Conn, client net.Conn) error {
	clientAddr := client.RemoteAddr().(*net.TCPAddr)
	proxyAddr := client.LocalAddr().(*net.TCPAddr)

	// Determine protocol family
	family := "TCP4"
	if clientAddr.IP.To4() == nil {
		family = "TCP6"
	}

	header := fmt.Sprintf("PROXY %s %s %s %d %d\r\n",
		family,
		clientAddr.IP.String(),
		proxyAddr.IP.String(),
		clientAddr.Port,
		proxyAddr.Port,
	)

	_, err := backend.Write([]byte(header))
	return err
}
