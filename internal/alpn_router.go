package app

import (
	"crypto/tls"
	"io"
	"net"
	"os"
	"sync"
)

// ALPNRouter wraps a TLS listener and routes connections based on ALPN protocol.
// HTTP connections (http/1.1, h2) are passed to the underlying application,
// while XMPP connections (xmpp-client, xmpp-server) are proxied to xmpp-proxy.
type ALPNRouter struct {
	listener      net.Listener
	xmppC2STarget string
	xmppS2STarget string
	acceptChan    chan acceptResult
	once          sync.Once
}

type acceptResult struct {
	conn net.Conn
	err  error
}

// NewALPNRouter creates a new ALPN-based connection router.
func NewALPNRouter(listener net.Listener) *ALPNRouter {
	return &ALPNRouter{
		listener:      listener,
		xmppC2STarget: getEnvOrDefault("XMPP_C2S_TARGET", "127.0.0.1:5222"),
		xmppS2STarget: getEnvOrDefault("XMPP_S2S_TARGET", "127.0.0.1:5269"),
		acceptChan:    make(chan acceptResult, 10),
	}
}

// Accept implements net.Listener interface. It accepts the next connection
// and routes it based on the negotiated ALPN protocol.
func (r *ALPNRouter) Accept() (net.Conn, error) {
	r.once.Do(func() {
		go r.acceptLoop()
	})

	result := <-r.acceptChan
	return result.conn, result.err
}

// acceptLoop continuously accepts connections and routes them based on ALPN.
func (r *ALPNRouter) acceptLoop() {
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			r.acceptChan <- acceptResult{nil, err}
			return
		}

		tlsConn, ok := conn.(*tls.Conn)
		if !ok {
			// Non-TLS connection, pass through to HTTP handler
			r.acceptChan <- acceptResult{conn, nil}
			continue
		}

		// Force TLS handshake to read ALPN protocol
		if err := tlsConn.Handshake(); err != nil {
			tlsConn.Close()
			logger().Warn().Err(err).Msg("TLS handshake failed")
			continue
		}

		alpn := tlsConn.ConnectionState().NegotiatedProtocol

		switch alpn {
		case "xmpp-client":
			logger().Info().
				Str("remote", tlsConn.RemoteAddr().String()).
				Str("alpn", alpn).
				Msg("Routing to XMPP C2S")
			go r.proxyXMPP(tlsConn, r.xmppC2STarget, "c2s")

		case "xmpp-server":
			logger().Info().
				Str("remote", tlsConn.RemoteAddr().String()).
				Str("alpn", alpn).
				Msg("Routing to XMPP S2S")
			go r.proxyXMPP(tlsConn, r.xmppS2STarget, "s2s")

		default:
			// HTTP/1.1, h2, or other protocols - pass to Fiber
			logger().Debug().
				Str("remote", tlsConn.RemoteAddr().String()).
				Str("alpn", alpn).
				Msg("Routing to HTTP handler")
			r.acceptChan <- acceptResult{tlsConn, nil}
		}
	}
}

// proxyXMPP proxies a TLS connection to the XMPP backend (xmpp-proxy).
// The TLS connection is terminated here and forwarded as plaintext to xmpp-proxy,
// which then forwards to Prosody with PROXY protocol headers.
func (r *ALPNRouter) proxyXMPP(client net.Conn, target, connType string) {
	defer client.Close()

	backend, err := net.Dial("tcp", target)
	if err != nil {
		logger().Error().
			Err(err).
			Str("target", target).
			Str("type", connType).
			Msg("Failed to connect to XMPP backend")
		return
	}
	defer backend.Close()

	logger().Debug().
		Str("client", client.RemoteAddr().String()).
		Str("backend", target).
		Str("type", connType).
		Msg("XMPP proxy established")

	// Bidirectional copy between client and backend
	done := make(chan error, 2)

	// Client → Backend
	go func() {
		_, err := io.Copy(backend, client)
		done <- err
	}()

	// Backend → Client
	go func() {
		_, err := io.Copy(client, backend)
		done <- err
	}()

	// Wait for either direction to complete
	err = <-done

	if err != nil && err != io.EOF {
		logger().Debug().
			Err(err).
			Str("type", connType).
			Msg("XMPP proxy connection closed with error")
	} else {
		logger().Debug().
			Str("type", connType).
			Msg("XMPP proxy connection closed")
	}
}

// Close closes the underlying listener.
func (r *ALPNRouter) Close() error {
	return r.listener.Close()
}

// Addr returns the listener's network address.
func (r *ALPNRouter) Addr() net.Addr {
	return r.listener.Addr()
}

// getEnvOrDefault returns the environment variable value or a default if not set.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
