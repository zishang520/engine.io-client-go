package engine

import (
	"github.com/zishang520/engine.io/events"
	"github.com/zishang520/engine.io/packet"
	"github.com/zishang520/engine.io/types"
	"github.com/zishang520/engine.io/utils"
)

type TransportInterface interface {
	events.EventEmitter

	// Transport name.
	Name() string
	// Query
	Query() *utils.ParameterBag
	// Opens the transport.
	Open()
	// Closes the transport.
	Close()
	// Sends multiple packets.
	Send([]*packet.Packet)

	Pause(func())
}

type HandshakeData struct {
	Sid          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval uint64   `json:"pingInterval"`
	PingTimeout  uint64   `json:"pingTimeout"`
	MaxPayload   uint64   `json:"maxPayload"`
}
