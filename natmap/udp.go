package natmap

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"time"

	"github.com/xmdhs/natupnp/reuse"
)

func NatMapUdp(ctx context.Context, stunAddr string, host string, port uint16, log func(error)) (*Map, netip.AddrPort, error) {
	m := Map{}
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	mapAddr, err := getPubulicPort(ctx, stunAddr, host, port, false)
	if err != nil {
		return nil, netip.AddrPort{}, fmt.Errorf("NatMap: %w", err)
	}
	go keepaliveUDP(ctx, port, log)
	return &m, mapAddr, nil

}

func keepaliveUDP(ctx context.Context, port uint16, log func(error)) {
	r := net.Resolver{
		PreferGo: true,
		Dial: func(context context.Context, network, address string) (net.Conn, error) {
			conn, err := reuse.DialContext(context, "udp", "0.0.0.0:"+strconv.Itoa(int(port)), "223.5.5.5:53")
			if err != nil {
				return nil, err
			}
			return conn, nil
		},
	}
	for {
		func() {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			_, err := r.LookupNetIP(ctx, "ip4", "baidu.com")
			if err != nil {
				log(err)
				return
			}
			defer time.Sleep(10 * time.Second)
		}()
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
