package engine

import (
	"context"
	"net/url"

	"github.com/zishang520/engine.io-go-parser/packet"
	"github.com/zishang520/engine.io/v2/transports"
	"github.com/zishang520/engine.io/v2/types"
)

// Transport defines the interface for all transport implementations in Engine.IO.
// It provides a common set of methods that all transport types must implement.
type Transport interface {
	// Extends EventEmitter for event-based communication
	types.EventEmitter

	// Prototype methods for interface implementation
	Prototype(Transport)
	Proto() Transport

	// Setters for transport state and configuration
	SetWritable(bool)
	// Protected: Internal state management
	SetReadyState(TransportState)

	// Getters for transport properties
	// Abstract: Must be implemented by specific transport types
	Name() string
	Query() url.Values
	Writable() bool

	// Protected: Internal access methods
	Opts() SocketOptionsInterface
	SupportsBinary() bool
	ReadyState() TransportState
	Socket() Socket

	// Core transport methods
	Construct(Socket, SocketOptionsInterface)
	// Protected: Error handling
	OnError(string, error, context.Context) Transport
	Open() Transport
	Close() Transport
	Send([]*packet.Packet)
	// Protected: Event handlers
	OnOpen()
	OnData(types.BufferInterface)
	OnPacket(*packet.Packet)
	OnClose(error)
	Pause(func())

	// Protected: URI handling
	CreateUri(string, url.Values) *url.URL

	// Protected: Abstract methods that must be implemented by specific transports
	DoOpen()
	DoClose()
	Write([]*packet.Packet)
}

// Polling represents the HTTP long-polling transport type.
// This transport uses regular HTTP requests to simulate real-time communication.
type Polling interface {
	Transport
}

// WebSocket represents the WebSocket transport type.
// This transport provides full-duplex communication over a single TCP connection.
type WebSocket interface {
	Transport
}

// WebTransport represents the WebTransport transport type.
// This transport provides low-latency, bidirectional communication using the QUIC protocol.
//
// WebTransport is a modern transport that offers several advantages:
// - Lower latency than WebSocket
// - Better multiplexing support
// - Built-in congestion control
// - Support for unreliable datagrams
//
// See: https://developer.mozilla.org/en-US/docs/Web/API/WebTransport
// See: https://caniuse.com/webtransport
type WebTransport interface {
	Transport
}

// WebSocketBuilder implements the transport builder pattern for WebSocket connections.
type WebSocketBuilder struct{}

// New creates a new WebSocket transport instance.
//
// Parameters:
//   - socket: The parent socket instance
//   - opts: The socket options configuration
//
// Returns: A new WebSocket transport instance
func (*WebSocketBuilder) New(socket Socket, opts SocketOptionsInterface) Transport {
	return NewWebSocket(socket, opts)
}

// Name returns the identifier for the WebSocket transport type.
func (*WebSocketBuilder) Name() string {
	return transports.WEBSOCKET
}

// WebTransportBuilder implements the transport builder pattern for WebTransport connections.
type WebTransportBuilder struct{}

// New creates a new WebTransport instance.
//
// Parameters:
//   - socket: The parent socket instance
//   - opts: The socket options configuration
//
// Returns: A new WebTransport instance
func (*WebTransportBuilder) New(socket Socket, opts SocketOptionsInterface) Transport {
	return NewWebTransport(socket, opts)
}

// Name returns the identifier for the WebTransport transport type.
func (*WebTransportBuilder) Name() string {
	return transports.WEBTRANSPORT
}

// PollingBuilder implements the transport builder pattern for HTTP long-polling connections.
type PollingBuilder struct{}

// New creates a new Polling transport instance.
//
// Parameters:
//   - socket: The parent socket instance
//   - opts: The socket options configuration
//
// Returns: A new Polling transport instance
func (*PollingBuilder) New(socket Socket, opts SocketOptionsInterface) Transport {
	return NewPolling(socket, opts)
}

// Name returns the identifier for the Polling transport type.
func (*PollingBuilder) Name() string {
	return transports.POLLING
}
