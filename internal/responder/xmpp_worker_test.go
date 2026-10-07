package responder

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dure-one/dure-mycart/internal/models"
)

func TestXMPPWorker_fetchMessages(t *testing.T) {
	t.Skip("Integration test - requires XMPP server")

	worker := &XMPPWorker{
		settings: &models.ResponderSettings{
			XMPPJID:      "test@localhost",
			XMPPPassword: "password",
			XMPPServer:   "localhost",
			XMPPPort:     5222,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := worker.fetchMessages(ctx)
	require.NoError(t, err)
}

func TestXMPPWorker_Start(t *testing.T) {
	t.Skip("Integration test - requires XMPP server")

	worker := &XMPPWorker{
		settings: &models.ResponderSettings{
			XMPPJID:      "test@localhost",
			XMPPPassword: "password",
			XMPPServer:   "localhost",
			XMPPPort:     5222,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := worker.Start(ctx)
	assert.NoError(t, err)
}

func TestXMPPConnection_DureCo_DirectTLS_ALPN(t *testing.T) {
	t.Skip("Integration test - requires live dure.co server and valid credentials")

	settings := &models.ResponderSettings{
		XMPPJID:    "admin@dure.co",
		XMPPServer: "dure.co",
		XMPPPort:   443,
	}

	worker := NewXMPPWorker(settings, nil)

	err := worker.Connect()
	require.NoError(t, err, "Failed to connect to dure.co with Direct TLS + ALPN")
	defer worker.Disconnect()

	t.Log("✓ Successfully connected to dure.co (port 443, Direct TLS, ALPN 'xmpp-client')")
}

func TestXMPPConnection_ConversationsIm_STARTTLS(t *testing.T) {
	t.Skip("conversations.im test requires valid credentials")

	settings := &models.ResponderSettings{
		XMPPJID:      "test@conversations.im",
		XMPPPassword: "testpass",
		XMPPServer:   "conversations.im",
		XMPPPort:     5222,
	}

	worker := NewXMPPWorker(settings, nil)

	err := worker.Connect()
	require.NoError(t, err, "Failed to connect to conversations.im with STARTTLS")
	defer worker.Disconnect()

	t.Log("✓ Successfully connected to conversations.im (port 5222, STARTTLS)")
}
