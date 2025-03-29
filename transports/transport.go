package transports

import (
	"github.com/zishang520/engine.io-client-go/engine"
)

type (
	TransportCtor = engine.TransportCtor

	WebSocketBuilder    = engine.WebSocketBuilder
	WebTransportBuilder = engine.WebTransportBuilder
	PollingBuilder      = engine.PollingBuilder
)

var (
	Polling      TransportCtor = &PollingBuilder{}
	WebSocket    TransportCtor = &WebSocketBuilder{}
	WebTransport TransportCtor = &WebTransportBuilder{}
)
