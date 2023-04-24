package engine

import (
	"sync"

	"github.com/zishang520/engine.io-client-go/config"
	"github.com/zishang520/engine.io/errors"
	"github.com/zishang520/engine.io/events"
	"github.com/zishang520/engine.io/log"
	"github.com/zishang520/engine.io/packet"
	"github.com/zishang520/engine.io/parser"
	"github.com/zishang520/engine.io/types"
	"github.com/zishang520/engine.io/utils"
)

var client_transport_log = log.NewLog("engine.io-client:transport")

type Transport struct {
	events.EventEmitter

	opts config.SocketOptionsInterface

	httpCompression   *types.HttpCompression
	perMessageDeflate *types.PerMessageDeflate

	supportsBinary bool
	query          *utils.ParameterBag
	_readyState    string
	_writable      bool

	mu_readyState sync.RWMutex
	mu_writable   sync.RWMutex

	doOpen  func()
	doClose func()
	write   func([]*packet.Packet)
}

func (t *Transport) SetPerMessageDeflate(perMessageDeflate *types.PerMessageDeflate) {
	t.perMessageDeflate = perMessageDeflate
}
func (t *Transport) PerMessageDeflate() *types.PerMessageDeflate {
	return t.perMessageDeflate
}

// Transport abstract constructor.
func NewTransport(opts config.SocketOptionsInterface) *Transport {
	t := &Transport{}

	t.EventEmitter = events.New()
	t._writable = false
	t.opts = opts
	t.query = opts.Query()
	t._readyState = ""

	t.doOpen = t._doOpen
	t.doClose = t._doClose
	t.write = t._write

	return t
}

func (t *Transport) Query() *utils.ParameterBag {
	return t.query
}

func (t *Transport) setReadyState(readyState string) {
	t.mu_readyState.Lock()
	defer t.mu_readyState.Unlock()

	t._readyState = readyState
}
func (t *Transport) readyState() string {
	t.mu_readyState.RLock()
	defer t.mu_readyState.RUnlock()

	return t._readyState
}

func (t *Transport) setWritable(writable bool) {
	t.mu_writable.Lock()
	defer t.mu_writable.Unlock()

	t._writable = writable
}
func (t *Transport) writable() bool {
	t.mu_writable.RLock()
	defer t.mu_writable.RUnlock()

	return t._writable
}

// Emits an error.
func (t *Transport) onError(reason string, description error) {
	t.Emit("error", errors.NewTransportError(reason, description).Err())
}

// Opens the transport.
func (t *Transport) Open() {
	if readyState := t.readyState(); "closed" == readyState || "" == readyState {
		t.setReadyState("opening")
		t.doOpen()
	}
}

// Closes the transport.
func (t *Transport) Close() {
	if readyState := t.readyState(); "opening" == readyState || "open" == readyState {
		t.doClose()
		t.onClose(nil)
	}
}

// Sends multiple packets.
func (t *Transport) Send(packets []*packet.Packet) {
	if "open" == t.readyState() {
		t.write(packets)
	} else {
		// this might happen if the transport was silently closed in the beforeunload event handler
		client_transport_log.Debug("transport is not open, discarding packets")
	}
}

// Called upon open
func (t *Transport) onOpen() {
	t.setReadyState("open")
	t.setWritable(true)
	t.Emit("open")
}

// Called with data.
func (t *Transport) onData(data types.BufferInterface) {
	p, _ := parser.Parserv4().DecodePacket(data)
	t.onPacket(p)
}

// Called with a decoded packet.
func (t *Transport) onPacket(data *packet.Packet) {
	t.Emit("packet", data)
}

// Called upon close.
func (t *Transport) onClose(details error) {
	t.setReadyState("closed")
	t.Emit("close", details)
}

func (t *Transport) HasPause() bool {
	return false
}

func (t *Transport) Pause(func())            {}
func (t *Transport) _doOpen()                {}
func (t *Transport) _doClose()               {}
func (t *Transport) _write([]*packet.Packet) {}
