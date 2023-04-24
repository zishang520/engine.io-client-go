package engine

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zishang520/engine.io-client-go/config"
	"github.com/zishang520/engine.io/log"
	"github.com/zishang520/engine.io/packet"
	"github.com/zishang520/engine.io/parser"
	"github.com/zishang520/engine.io/types"
	"github.com/zishang520/engine.io/utils"
)

var client_websocket_log = log.NewLog("engine.io-client:websocket")

type WS struct {
	*Transport

	ws    *websocket.Conn
	mu_ws sync.RWMutex
}

// WebSocket transport constructor.
func NewWS(opts config.SocketOptionsInterface) *WS {
	p := &WS{}
	p.Transport = NewTransport(opts)
	p.supportsBinary = !opts.ForceBase64()

	p.doOpen = p._doOpen
	p.doClose = p._doClose
	p.write = p._write
	return p
}

// Transport name.
func (w *WS) Name() string {
	return "websocket"
}

// Opens socket.
func (w *WS) _doOpen() {
	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		Subprotocols:     w.opts.Protocols(),
	}
	c, _, err := dialer.Dial(w.uri(), w.opts.ExtraHeaders())
	if err != nil {
		w.Emit("error", err)
		return
	}
	w.mu_ws.Lock()
	w.ws = c
	w.mu_ws.Unlock()
	w.addEventListeners()
}

// Adds event listeners to the socket
func (w *WS) addEventListeners() {
	w.onOpen()
	go func() {
		w.mu_ws.RLock()
		ws := w.ws
		w.mu_ws.RUnlock()

		if ws == nil {
			return
		}

		for {
			mt, message, err := ws.NextReader()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err) {
					w.onClose(errors.New("websocket connection closed: " + err.Error()))
				} else {
					w.onError("websocket error", err)
				}
				break
			}
			switch mt {
			case websocket.BinaryMessage:
				read := types.NewBytesBuffer(nil)
				if _, err := read.ReadFrom(message); err != nil {
					w.onError("websocket error", err)
				} else {
					w.onData(read)
				}
			case websocket.TextMessage:
				read := types.NewStringBuffer(nil)
				if _, err := read.ReadFrom(message); err != nil {
					w.onError("websocket error", err)
				} else {
					w.onData(read)
				}
			case websocket.CloseMessage:
				w.onClose(errors.New("websocket connection closed"))
				if c, ok := message.(io.Closer); ok {
					c.Close()
				}
				break
			case websocket.PingMessage:
			case websocket.PongMessage:
			}
			if c, ok := message.(io.Closer); ok {
				c.Close()
			}
		}
	}()
}

// Writes data to socket.
func (w *WS) _write(packets []*packet.Packet) {
	w.setWritable(false)
	// defer to allow Socket to clear writeBuffer
	defer func() {
		w.setWritable(true)
		w.Emit("drain")
	}()
	// encodePacket efficient as it uses WS framing
	// no need for encodePayload
	w.mu_ws.RLock()
	ws := w.ws
	w.mu_ws.RUnlock()

	if ws == nil {
		return
	}

	for _, packet := range packets {
		if data, err := parser.Parserv4().EncodePacket(packet, w.supportsBinary); err == nil {
			compress := false
			if packet.Options != nil {
				compress = packet.Options.Compress
			}
			if w.perMessageDeflate != nil {
				if data.Len() < w.perMessageDeflate.Threshold {
					compress = false
				}
			}
			w.send(ws, data, compress)
		}
	}
}

func (w *WS) send(ws *websocket.Conn, data types.BufferInterface, compress bool) {
	ws.EnableWriteCompression(compress)
	mt := websocket.BinaryMessage
	if _, ok := data.(*types.StringBuffer); ok {
		mt = websocket.TextMessage
	}
	// Sometimes the websocket has already been closed but the client didn't
	// have a chance of informing us about it yet, in that case send will
	// throw an error
	write, err := ws.NextWriter(mt)
	if err != nil {
		client_websocket_log.Debug("websocket send error: %s", err.Error())
		w.onError("websocket send error", err)
		return
	}
	defer func() {
		if err := write.Close(); err != nil {
			client_websocket_log.Debug("websocket send error: %s", err.Error())
			w.onError("websocket send error", err)
			return
		}
	}()
	if _, err := io.Copy(write, data); err != nil {
		client_websocket_log.Debug("websocket send error: %s", err.Error())
		w.onError("websocket send error", err)
		return
	}
}

// Closes socket.
func (w *WS) _doClose() {
	w.mu_ws.Lock()
	defer w.mu_ws.Unlock()

	if w.ws != nil {
		w.ws.Close()
		w.ws = nil
	}
}

// Generates uri for connection.
func (w *WS) uri() string {
	_url := &url.URL{
		Path:   w.opts.Path(),
		Scheme: "ws",
	}
	if w.opts.Secure() {
		_url.Scheme = "wss"
	}
	query := url.Values(w.query.All())
	// cache busting is forced
	if false != w.opts.TimestampRequests() {
		query.Set(w.opts.TimestampParam(), utils.YeastDate())
	}
	if !w.supportsBinary {
		query.Set("b64", "1")
	}
	_url.RawQuery = query.Encode()
	host := ""
	if strings.Index(w.opts.Hostname(), ":") > -1 {
		host += "[" + w.opts.Hostname() + "]"
	} else {
		host += w.opts.Hostname()
	}
	port := ""
	// avoid port if default for schema
	if w.opts.Port() != "" && (("wss" == _url.Scheme && w.opts.Port() != "443") || ("ws" == _url.Scheme && w.opts.Port() != "80")) {
		port = ":" + w.opts.Port()
	}
	_url.Host = host + port
	return _url.String()
}
