package engine

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"github.com/zishang520/engine.io-go-parser/packet"
	"github.com/zishang520/engine.io/v2/transports"
	"github.com/zishang520/engine.io/v2/types"
	"github.com/zishang520/engine.io/v2/utils"
)

// SocketWithUpgrade provides a WebSocket-like interface to connect to an Engine.IO server.
// The connection will be established with one of the available low-level transports,
// such as HTTP long-polling, WebSocket, or WebTransport.
//
// This implementation includes an upgrade mechanism that attempts to upgrade the initial
// transport to a better one after the connection is established.
//
// To enable tree-shaking, no transports are included by default. The `transports` option
// must be explicitly specified when creating a new socket.
//
// Example usage:
//
//	import (
//		"github.com/zishang520/engine.io-client-go/engine"
//		"github.com/zishang520/engine.io-client-go/transports"
//		"github.com/zishang520/engine.io/v2/types"
//	)
//
//	func main() {
//		opts := engine.DefaultSocketOptions()
//		opts.SetTransports(types.NewSet(transports.Polling, transports.WebSocket, transports.WebTransport))
//		socket := engine.NewSocket("http://localhost:8080", opts)
//		socket.On("open", func(...any) {
//			socket.Send("hello")
//		})
//	}
//
// See: [SocketWithoutUpgrade]
//
// See: [Socket]
type socketWithUpgrade struct {
	SocketWithoutUpgrade

	_upgrades *types.Set[string]
}

// MakeSocketWithUpgrade creates a new SocketWithUpgrade instance with default settings.
// It initializes the base SocketWithoutUpgrade and sets up the upgrades set.
func MakeSocketWithUpgrade() SocketWithUpgrade {
	s := &socketWithUpgrade{
		SocketWithoutUpgrade: MakeSocketWithoutUpgrade(),
		_upgrades:            types.NewSet[string](),
	}

	s.Prototype(s)

	return s
}

// NewSocketWithUpgrade creates a new SocketWithUpgrade instance with the specified URI and options.
// It initializes the socket and establishes the connection using the provided configuration.
func NewSocketWithUpgrade(uri string, opts SocketOptionsInterface) SocketWithUpgrade {
	s := MakeSocketWithUpgrade()

	s.Construct(uri, opts)

	return s
}

// OnOpen is called when the socket connection is established.
// If upgrade is enabled in the options, it will start probing for better transport options.
func (s *socketWithUpgrade) OnOpen() {
	s.SocketWithoutUpgrade.OnOpen()

	if SocketStateOpen == s.ReadyState() && s.Opts().Upgrade() {
		client_socket_log.Debug("starting upgrade probes")
		for _, upgrade := range s._upgrades.Keys() {
			s._probe(upgrade)
		}
	}
}

// _probe attempts to upgrade the current transport to a better one.
// It creates a new transport instance and tests its compatibility with the server.
// If successful, it will upgrade the connection to use the new transport.
func (s *socketWithUpgrade) _probe(name string) {
	client_socket_log.Debug(`probing transport "%s"`, name)
	transport := s.Proto().CreateTransport(name)
	var (
		failed  atomic.Bool
		cleanup func()
	)

	s.SetPriorWebsocketSuccess(false)

	onTransportOpen := func(...any) {
		if failed.Load() {
			return
		}

		client_socket_log.Debug(`probe transport "%s" opened`, name)
		transport.Send([]*packet.Packet{
			{
				Type: packet.PING,
				Data: types.NewStringBufferString("probe"),
			},
		})
		transport.Once("packet", func(msgs ...any) {
			if failed.Load() {
				return
			}
			msg := msgs[0].(*packet.Packet)
			sb := new(strings.Builder)
			io.Copy(sb, msg.Data)

			if msg.Type == packet.PONG && sb.String() == "probe" {
				client_socket_log.Debug(`probe transport "%s" pong`, name)
				s.SetUpgrading(true)
				s.Emit("upgrading", transport)
				if transport == nil {
					return
				}
				s.SetPriorWebsocketSuccess(transports.WEBSOCKET == transport.Name())
				client_socket_log.Debug(`pausing current transport "%s"`, s.Transport().Name())
				s.Transport().Pause(func() {
					if failed.Load() {
						return
					}
					if SocketStateClosed == s.ReadyState() {
						return
					}
					client_socket_log.Debug("changing transport and sending upgrade packet")

					cleanup()

					s.Proto().SetTransport(transport)
					transport.Send([]*packet.Packet{
						{
							Type: packet.UPGRADE,
						},
					})
					s.Emit("upgrade", transport)
					transport = nil
					s.SetUpgrading(false)
					s.Proto().Flush()
				})
			} else {
				client_socket_log.Debug(`probe transport "%s" failed`, name)
				s.Emit("upgradeError", errors.New("["+transport.Name()+"] probe error"))
			}
		})
	}

	freezeTransport := func() {
		if failed.Load() {
			return
		}

		// Any callback called by transport should be ignored since now
		failed.Store(true)

		cleanup()

		transport.Close()
		transport = nil
	}

	// Handle any error that happens while probing
	onerror := func(errs ...any) {
		e := fmt.Errorf("[%s] probe error: %w", transport.Name(), errs[0].(error))

		freezeTransport()

		client_socket_log.Debug(`probe transport "%s" failed because of error: %v`, name, e)

		s.Emit("upgradeError", e)
	}

	onTransportClose := func(...any) {
		onerror(errors.New("transport closed"))
	}

	// When the socket is closed while we're probing
	onclose := func(...any) {
		onerror(errors.New("socket closed"))
	}

	// When the socket is upgraded while we're probing
	onupgrade := func(to ...any) {
		if to, ok := to[0].(Transport); ok && to != nil && transport != nil && to.Name() != transport.Name() {
			client_socket_log.Debug(`"%s" works - aborting "%s"`, to.Name(), transport.Name())
			freezeTransport()
		}
	}

	// Remove all listeners on the transport and on self
	cleanup = func() {
		transport.RemoveListener("open", onTransportOpen)
		transport.RemoveListener("error", onerror)
		transport.RemoveListener("close", onTransportClose)
		s.RemoveListener("close", onclose)
		s.RemoveListener("upgrading", onupgrade)
	}

	transport.Once("open", onTransportOpen)
	transport.Once("error", onerror)
	transport.Once("close", onTransportClose)

	s.Once("close", onclose)
	s.Once("upgrading", onupgrade)

	if s._upgrades.Has(transports.WEBTRANSPORT) && name != transports.WEBTRANSPORT {
		// favor WebTransport
		utils.SetTimeout(func() {
			if !failed.Load() {
				transport.Open()
			}
		}, 200*time.Millisecond)
	} else {
		transport.Open()
	}
}

// OnHandshake is called when the initial handshake with the server is completed.
// It filters the available upgrades based on the server's capabilities and client configuration.
func (s *socketWithUpgrade) OnHandshake(data *HandshakeData) {
	s._upgrades = s._filterUpgrades(data.Upgrades)
	s.SocketWithoutUpgrade.OnHandshake(data)
}

// Filters upgrades, returning only those matching client transports.
func (s *socketWithUpgrade) _filterUpgrades(upgrades []string) *types.Set[string] {
	filteredUpgrades := types.NewSet[string]()
	for _, upgrade := range upgrades {
		if s.Transports().Has(upgrade) {
			filteredUpgrades.Add(upgrade)
		}
	}
	return filteredUpgrades
}
