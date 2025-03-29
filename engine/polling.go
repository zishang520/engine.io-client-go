package engine

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"

	"github.com/zishang520/engine.io-client-go/request"
	"github.com/zishang520/engine.io-go-parser/packet"
	"github.com/zishang520/engine.io-go-parser/parser"
	"github.com/zishang520/engine.io/v2/transports"
	"github.com/zishang520/engine.io/v2/types"
)

// polling implements the HTTP long-polling transport for Engine.IO.
// This transport uses regular HTTP requests to simulate real-time communication
// by keeping connections open until data is available or a timeout occurs.
type polling struct {
	Transport

	// client is the HTTP client used for making requests
	client *request.HTTPClient

	// _polling indicates whether a polling request is currently in progress
	_polling atomic.Bool
}

// Name returns the identifier for the polling transport.
func (p *polling) Name() string {
	return transports.POLLING
}

// MakePolling creates a new polling transport instance with default settings.
// This is the factory function for creating a new polling transport.
func MakePolling() Polling {
	s := &polling{
		Transport: MakeTransport(),
	}

	s._polling.Store(false)

	s.Prototype(s)

	return s
}

// NewPolling creates a new polling transport instance with the specified socket and options.
//
// Parameters:
//   - socket: The parent socket instance
//   - opts: The socket options configuration
//
// Returns: A new Polling transport instance
func NewPolling(socket Socket, opts SocketOptionsInterface) Polling {
	s := MakePolling()

	s.Construct(socket, opts)

	return s
}

// Construct initializes the polling transport with the given socket and options.
// This sets up the HTTP client with appropriate configuration for long-polling.
func (p *polling) Construct(socket Socket, opts SocketOptionsInterface) {
	p.Transport.Construct(socket, opts)

	p.client = request.NewHTTPClient(
		request.WithLogger(NewLog("HTTPClient")),
		request.WithTimeout(p.Opts().RequestTimeout()),
		request.WithCookieJar(p.Socket().CookieJar()),
		request.WithTransport(request.NewTransport(p.Opts().TLSClientConfig(), p.Opts().QUICConfig())),
	)
}

// DoOpen initiates the polling transport by starting the polling cycle.
// This method triggers the initial polling request to establish the connection.
func (p *polling) DoOpen() {
	p._poll()
}

// Pause temporarily suspends the polling transport.
// This is used during transport upgrades to prevent packet loss.
//
// Parameters:
//   - onPause: Callback function to be called when the transport is paused
func (p *polling) Pause(onPause func()) {
	p.SetReadyState(TransportStatePausing)

	pause := func() {
		client_polling_log.Debug("paused")
		p.SetReadyState(TransportStatePaused)
		onPause()
	}

	if p._polling.Load() || !p.Writable() {
		var total atomic.Uint32
		if p._polling.Load() {
			client_polling_log.Debug("we are currently polling - waiting to pause")
			total.Add(1)
			p.Once("pollComplete", func(...any) {
				client_polling_log.Debug("pre-pause polling complete")
				if total.Add(^uint32(0)) == 0 {
					pause()
				}
			})
		}
		if !p.Writable() {
			total.Add(1)
			p.Once("drain", func(...any) {
				client_polling_log.Debug("pre-pause writing complete")
				if total.Add(^uint32(0)) == 0 {
					pause()
				}
			})
		}
	} else {
		pause()
	}
}

// _poll starts a new polling cycle.
// This method sets up the polling state and initiates a new polling request.
func (p *polling) _poll() {
	client_polling_log.Debug("polling")
	p._polling.Store(true)
	go p.doPoll()
	p.Emit("poll")
}

// _onPacket handles incoming packets from the polling transport.
// This method processes different packet types and updates the transport state accordingly.
func (p *polling) _onPacket(data *packet.Packet) {
	// if its the first message we consider the transport open
	if TransportStateOpening == p.ReadyState() && data.Type == packet.OPEN {
		p.OnOpen()
	}

	// if its a close packet, we close the ongoing requests
	if packet.CLOSE == data.Type {
		p.OnClose(errors.New("transport closed by the server"))
		return
	}

	// otherwise bypass onData and handle the message
	p.OnPacket(data)
}

// OnData processes incoming data from the polling transport.
// This method decodes the payload and handles each packet in the payload.
func (p *polling) OnData(data types.BufferInterface) {
	client_polling_log.Debug("polling got data %#v", data)

	packets, _ := parser.Parserv4().DecodePayload(data)
	// decode payload
	for _, data := range packets {
		p._onPacket(data)
	}

	// if an event did not trigger closing
	if readyState := p.ReadyState(); TransportStateClosed != readyState {
		// if we got data we're not polling
		p._polling.Store(false)
		p.Emit("pollComplete")

		if TransportStateOpen == readyState {
			p._poll()
		} else {
			client_polling_log.Debug(`ignoring poll - transport state "%s"`, readyState)
		}
	}
}

// DoClose gracefully closes the polling transport.
// This method ensures that a close packet is sent to the server before closing the connection.
func (p *polling) DoClose() {
	defer p.client.Close()

	cleanup := func(...any) {
		client_polling_log.Debug("writing close packet")
		p.Write([]*packet.Packet{
			{
				Type: packet.CLOSE,
			},
		})
	}

	if TransportStateOpen == p.ReadyState() {
		client_polling_log.Debug("transport open - closing")
		cleanup()
	} else {
		// in case we're trying to close while
		// handshaking is in progress (GH-164)
		client_polling_log.Debug("transport not open - deferring close")
		p.Once("open", cleanup)
	}
}

// Write sends packets over the polling transport.
// This method encodes the packets and sends them to the server.
func (p *polling) Write(packets []*packet.Packet) {
	p.SetWritable(false)

	data, _ := parser.Parserv4().EncodePayload(packets)
	go p.doWrite(data, func() {
		p.SetWritable(true)
		p.Emit("drain")
	})
}

// uri generates the URI for the polling transport connection.
// This method constructs the appropriate URL with query parameters.
func (p *polling) uri() *url.URL {
	schema := "http"
	if p.Opts().Secure() {
		schema = "https"
	}

	query := url.Values{}
	for k, vs := range p.Query() {
		for _, v := range vs {
			query.Add(k, v)
		}
	}

	if p.Opts().TimestampRequests() {
		query.Set(p.Opts().TimestampParam(), request.RandomString())
	}

	if !p.SupportsBinary() && !query.Has("sid") {
		query.Set("b64", "1")
	}

	return p.CreateUri(schema, query)
}

// doPoll performs the actual HTTP request to poll for data from the server.
// This method handles the HTTP GET request and error handling.
func (p *polling) doPoll() {
	res, err := p._fetch(nil)
	if err != nil {
		p.OnError("fetch read error", err, nil)
		return
	}
	defer res.Body.Close()

	if !res.Ok() {
		p.OnError("fetch read error", res.Err, res.Request.Context())
		return
	}

	data, err := types.NewStringBufferReader(res.Body)
	if err != nil {
		p.OnError("fetch read error", err, nil)
		return
	}

	p.OnData(data)
}

// doWrite performs the actual HTTP request to write data to the server.
// This method handles the HTTP POST request and error handling.
func (p *polling) doWrite(data types.BufferInterface, fn func()) {
	res, err := p._fetch(data)
	if err != nil {
		p.OnError("fetch write error", err, nil)
		return
	}
	defer res.Body.Close()

	if !res.Ok() {
		p.OnError("fetch write error", res.Err, res.Request.Context())
		return
	}

	fn()
}

// _fetch performs the actual HTTP request with the given data.
// This method handles the HTTP request configuration and execution.
func (p *polling) _fetch(data io.Reader) (res *request.Response, err error) {
	headers := http.Header{}
	for k, vs := range p.Opts().ExtraHeaders() {
		for _, v := range vs {
			headers.Add(k, v)
		}
	}

	if data != nil {
		headers.Set("Content-Type", "text/plain;charset=UTF-8")

		res, err = p.client.Post(p.uri().String(), &request.Options{
			Body:    data,
			Headers: headers,
		})
	} else {
		res, err = p.client.Get(p.uri().String(), &request.Options{
			Headers: headers,
		})
	}

	return
}
