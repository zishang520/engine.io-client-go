package engine

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"

	ws "github.com/gorilla/websocket"
	"github.com/zishang520/engine.io-client-go/request"
	"github.com/zishang520/engine.io-go-parser/packet"
	"github.com/zishang520/engine.io-go-parser/parser"
	"github.com/zishang520/engine.io/v2/transports"
	"github.com/zishang520/engine.io/v2/types"
)

// websocket implements the WebSocket transport for Engine.IO.
// This transport provides full-duplex communication over a single TCP connection
// using the WebSocket protocol.
type websocket struct {
	Transport

	// dialer is the WebSocket dialer used to establish connections
	dialer *ws.Dialer

	// socket is the WebSocket connection instance
	socket *types.WebSocketConn

	// mu is a mutex to protect concurrent access to the WebSocket connection
	mu sync.Mutex
}

// Name returns the identifier for the WebSocket transport.
func (w *websocket) Name() string {
	return transports.WEBSOCKET
}

// MakeWebSocket creates a new WebSocket transport instance with default settings.
// This is the factory function for creating a new WebSocket transport.
func MakeWebSocket() WebSocket {
	s := &websocket{
		Transport: MakeTransport(),
	}

	s.Prototype(s)

	return s
}

// NewWebSocket creates a new WebSocket transport instance with the specified socket and options.
//
// Parameters:
//   - socket: The parent socket instance
//   - opts: The socket options configuration
//
// Returns: A new WebSocket transport instance
func NewWebSocket(socket Socket, opts SocketOptionsInterface) WebSocket {
	s := MakeWebSocket()

	s.Construct(socket, opts)

	return s
}

// Construct initializes the WebSocket transport with the given socket and options.
// This sets up the WebSocket dialer with appropriate configuration for the connection.
func (w *websocket) Construct(socket Socket, opts SocketOptionsInterface) {
	w.Transport.Construct(socket, opts)

	w.dialer = &ws.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		TLSClientConfig:   w.Opts().TLSClientConfig(),
		Subprotocols:      w.Opts().Protocols(),
		EnableCompression: w.Opts().PerMessageDeflate() != nil,
		Jar:               w.Socket().CookieJar(),
	}
}

// DoOpen initiates the WebSocket connection.
// This method establishes the WebSocket connection and sets up event listeners.
func (w *websocket) DoOpen() {
	headers := http.Header{}
	for k, vs := range w.Opts().ExtraHeaders() {
		for _, v := range vs {
			headers.Add(k, v)
		}
	}
	socket, _, err := w.dialer.Dial(w.uri().String(), headers)
	if err != nil {
		w.Emit("error", err)
		return
	}
	w.socket = &types.WebSocketConn{EventEmitter: types.NewEventEmitter(), Conn: socket}

	w.addEventListeners()
}

// _init handles the WebSocket message reading loop.
// This method processes incoming WebSocket messages and handles different message types.
func (w *websocket) _init() {
	for {
		mt, message, err := w.socket.NextReader()
		if err != nil {
			if ws.IsUnexpectedCloseError(err) || errors.Is(err, net.ErrClosed) {
				w.socket.Emit("close")
			} else {
				w.socket.Emit("error", err)
			}
			return
		}

		switch mt {
		case ws.BinaryMessage:
			read := types.NewBytesBuffer(nil)
			if _, err := read.ReadFrom(message); err != nil {
				if errors.Is(err, net.ErrClosed) {
					w.socket.Emit("close")
				} else {
					w.socket.Emit("error", err)
				}
			} else {
				w.OnData(read)
			}
		case ws.TextMessage:
			read := types.NewStringBuffer(nil)
			if _, err := read.ReadFrom(message); err != nil {
				if errors.Is(err, net.ErrClosed) {
					w.socket.Emit("close")
				} else {
					w.socket.Emit("error", err)
				}
			} else {
				w.OnData(read)
			}
		case ws.CloseMessage:
			w.socket.Emit("close")
			if c, ok := message.(io.Closer); ok {
				c.Close()
			}
			return
		case ws.PingMessage:
		case ws.PongMessage:
		}
		if c, ok := message.(io.Closer); ok {
			c.Close()
		}
	}
}

// addEventListeners sets up event handlers for the WebSocket connection.
// This method configures error and close event handlers and starts the message reading loop.
func (w *websocket) addEventListeners() {
	w.socket.On("error", func(errs ...any) {
		w.OnError("websocket error", errs[0].(error), nil)
	})
	w.socket.Once("close", func(...any) {
		w.OnClose(NewTransportError("websocket connection closed", nil, nil).Err())
	})

	go w._init()

	w.OnOpen()
}

// Write sends packets over the WebSocket connection.
// This method handles packet encoding and WebSocket message framing.
func (w *websocket) Write(packets []*packet.Packet) {
	w.SetWritable(false)

	go func() {
		// fake drain
		// defer to next tick to allow Socket to clear writeBuffer
		defer func() {
			w.SetWritable(true)
			w.Emit("drain")
		}()

		w.mu.Lock()
		defer w.mu.Unlock()

		// encodePacket efficient as it uses websocket framing
		// no need for encodePayload
		for _, packet := range packets {
			// always creates a new object since ws modifies it
			compress := false
			if packet.Options != nil {
				compress = packet.Options.Compress

				if w.Opts().PerMessageDeflate() == nil && packet.Options.WsPreEncodedFrame != nil {
					mt := ws.BinaryMessage
					if _, ok := packet.Options.WsPreEncodedFrame.(*types.StringBuffer); ok {
						mt = ws.TextMessage
					}
					pm, err := ws.NewPreparedMessage(mt, packet.Options.WsPreEncodedFrame.Bytes())
					if err != nil {
						client_websocket_log.Debug(`Send Error "%s"`, err.Error())
						if errors.Is(err, net.ErrClosed) {
							w.socket.Emit("close")
						} else {
							w.socket.Emit("error", err)
						}
						return
					}
					if err := w.socket.WritePreparedMessage(pm); err != nil {
						client_websocket_log.Debug(`Send Error "%s"`, err.Error())
						if errors.Is(err, net.ErrClosed) {
							w.socket.Emit("close")
						} else {
							w.socket.Emit("error", err)
						}
						return
					}
					return
				}
			}

			data, err := parser.Parserv4().EncodePacket(packet, w.SupportsBinary())
			if err != nil {
				client_websocket_log.Debug(`Send Error "%s"`, err.Error())
				if errors.Is(err, net.ErrClosed) {
					w.socket.Emit("close")
				} else {
					w.socket.Emit("error", err)
				}
				return
			}
			w.doWrite(data, compress)
		}
	}()
}

// doWrite performs the actual WebSocket write operation.
// This method handles message compression and WebSocket message framing.
func (w *websocket) doWrite(data types.BufferInterface, compress bool) {
	if perMessageDeflate := w.Opts().PerMessageDeflate(); perMessageDeflate != nil {
		if data.Len() < perMessageDeflate.Threshold {
			compress = false
		}
	}
	client_websocket_log.Debug(`writing %#v`, data)

	w.socket.EnableWriteCompression(compress)
	mt := ws.BinaryMessage
	if _, ok := data.(*types.StringBuffer); ok {
		mt = ws.TextMessage
	}
	write, err := w.socket.NextWriter(mt)
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			w.socket.Emit("close")
		} else {
			w.socket.Emit("error", err)
		}
		return
	}
	defer func() {
		if err := write.Close(); err != nil {
			if errors.Is(err, net.ErrClosed) {
				w.socket.Emit("close")
			} else {
				w.socket.Emit("error", err)
			}
			return
		}
	}()
	if _, err := io.Copy(write, data); err != nil {
		if errors.Is(err, net.ErrClosed) {
			w.socket.Emit("close")
		} else {
			w.socket.Emit("error", err)
		}
		return
	}
}

// DoClose gracefully closes the WebSocket connection.
// This method ensures proper cleanup of the WebSocket connection.
func (w *websocket) DoClose() {
	if w.socket != nil {
		defer w.socket.Close()
	}
}

// Generates uri for connection.
func (w *websocket) uri() *url.URL {
	schema := "ws"
	if w.Opts().Secure() {
		schema = "wss"
	}

	query := url.Values{}
	for k, vs := range w.Query() {
		for _, v := range vs {
			query.Add(k, v)
		}
	}

	if w.Opts().TimestampRequests() {
		query.Set(w.Opts().TimestampParam(), request.RandomString())
	}

	if !w.SupportsBinary() {
		query.Set("b64", "1")
	}

	return w.CreateUri(schema, query)
}
