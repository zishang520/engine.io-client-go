package engine

import (
	"io"
	"net/http"

	"github.com/zishang520/engine.io-go-parser/packet"
	"github.com/zishang520/engine.io/v2/types"
)

// SocketWithoutUpgrade provides a WebSocket-like interface to connect to an Engine.IO server.
// This implementation maintains a single transport connection without attempting to upgrade
// to more efficient transports after initial connection.
//
// Key features:
// - Single transport connection (no upgrade mechanism)
// - Event-based communication
// - Support for binary data
// - Tree-shaking friendly (transports must be explicitly included)
//
// Example usage:
//
//	socket := NewSocketWithoutUpgrade("http://localhost:8080", &SocketOptions{
//	  Transports: types.NewSet[string](transports.WEBSOCKET),
//	})
//
//	socket.On(SocketStateOpen, func() {
//	  socket.Send("hello")
//	})
//
// See: [SocketWithUpgrade] for an implementation with transport upgrade support
// See: [Socket] for the recommended high-level interface
type SocketWithoutUpgrade interface {
	// Extends EventEmitter for event-based communication
	types.EventEmitter

	// Prototype methods for interface implementation
	Prototype(SocketWithoutUpgrade)
	Proto() SocketWithoutUpgrade

	// Setters for internal state
	// Protected: Internal state management
	SetPriorWebsocketSuccess(bool)
	SetUpgrading(bool)

	// Getters for socket properties
	Id() string
	Transport() Transport
	ReadyState() SocketState
	WriteBuffer() *types.Slice[*packet.Packet]

	// Protected: Internal access methods
	Opts() SocketOptionsInterface
	Transports() *types.Set[string]
	Upgrading() bool
	CookieJar() http.CookieJar
	PriorWebsocketSuccess() bool
	Protocol() int

	// Core socket methods
	Construct(string, SocketOptionsInterface)
	// Protected: Transport management
	CreateTransport(string) Transport
	SetTransport(Transport)
	// Protected: Event handlers
	OnOpen()
	OnHandshake(*HandshakeData)
	// Protected: Buffer management
	Flush()
	HasPingExpired() bool
	// Public: Data transmission
	Write(io.Reader, *packet.Options, func()) SocketWithoutUpgrade
	Send(io.Reader, *packet.Options, func()) SocketWithoutUpgrade
	Close() SocketWithoutUpgrade
}

// SocketWithUpgrade extends SocketWithoutUpgrade to add transport upgrade capabilities.
// This implementation will attempt to upgrade the initial transport to a more efficient one
// after the connection is established.
//
// Key features:
// - Automatic transport upgrade mechanism
// - Fallback to lower-level transports if upgrade fails
// - Event-based communication
// - Support for binary data
//
// Example usage:
//
//	socket := NewSocketWithUpgrade("http://localhost:8080", &SocketOptions{
//	  Transports: types.NewSet[string](transports.WEBSOCKET),
//	})
//
//	socket.On("open", func() {
//	  socket.Send("hello")
//	})
//
// See: [SocketWithoutUpgrade] for the base implementation without upgrade support
// See: [Socket] for the recommended high-level interface
type SocketWithUpgrade interface {
	SocketWithoutUpgrade
}

// Socket provides the recommended high-level interface for Engine.IO client connections.
// This interface extends SocketWithUpgrade to provide the most feature-complete implementation
// with automatic transport upgrades and optimal performance.
//
// Key features:
// - Automatic transport upgrade mechanism
// - Multiple transport support (WebSocket, WebTransport, Polling)
// - Event-based communication
// - Support for binary data
// - Automatic reconnection
//
// Example usage:
//
//	socket := NewSocket("http://localhost:8080", DefaultSocketOptions())
//
//	socket.On("open", func() {
//	  socket.Send("hello")
//	})
//
// See: [SocketWithoutUpgrade] for a simpler implementation without transport upgrade
// See: [SocketWithUpgrade] for the base implementation with upgrade support
type Socket interface {
	SocketWithUpgrade
}
