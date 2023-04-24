package engine

import (
	"github.com/zishang520/engine.io-client-go/config"
)

type transports struct {
	New func(config.SocketOptionsInterface) TransportInterface
}

var _transports map[string]*transports = map[string]*transports{
	"polling": &transports{
		// Polling polymorphic New.
		New: func(opts config.SocketOptionsInterface) TransportInterface {
			return NewPolling(opts)
		},
	},

	"websocket": &transports{
		New: func(opts config.SocketOptionsInterface) TransportInterface {
			return NewWS(opts)
		},
	},
}

func Transports() map[string]*transports {
	return _transports
}
