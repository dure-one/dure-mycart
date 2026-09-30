package responder

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shurco/mycart/internal/models"
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
