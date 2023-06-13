package reuse

import (
	"context"
	"fmt"
	"net"

	"github.com/libp2p/go-reuseport"
)

func DialContext(ctx context.Context, network, laddr, raddr string) (net.Conn, error) {
	nla, err := reuseport.ResolveAddr(network, laddr)
	if err != nil {
		return nil, fmt.Errorf("resolving local addr: %w", err)
	}
	d := net.Dialer{
		Control:   reuseport.Control,
		LocalAddr: nla,
	}
	return d.DialContext(ctx, network, raddr)
}

var listenConfig = net.ListenConfig{
	Control: reuseport.Control,
}

func Listen(ctx context.Context, network, address string) (net.Listener, error) {
	return listenConfig.Listen(ctx, network, address)
}
