package natmap

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/xmdhs/natupnp/reuse"
)

func NatMapUdp(ctx context.Context, stunAddr string, laddr netip.AddrPort, log func(error)) (*Map, netip.AddrPort, error) {
	m := Map{}
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	mapAddr, err := getPubulicPort(ctx, stunAddr, laddr, false)
	if err != nil {
		return nil, netip.AddrPort{}, fmt.Errorf("NatMap: %w", err)
	}
	go keepaliveUDP(ctx, laddr, log)
	return &m, mapAddr, nil

}

func keepaliveUDP(ctx context.Context, laddr netip.AddrPort, log func(error)) {
	raddr := "223.5.5.5:53"
	if laddr.Addr().Is6() {
		raddr = "[2400:3200::1]:53"
	}
	r := net.Resolver{
		PreferGo: true,
		Dial: func(context context.Context, network, address string) (net.Conn, error) {
			conn, err := reuse.DialContext(context, "udp", laddr.String(), raddr)
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

type logger struct {
	log func(string)
}

func (l logger) Println(v ...any) {
	build := &strings.Builder{}
	fmt.Fprint(build, v...)
	l.log(build.String())
}

func ForwardUdp(ctx context.Context, laddr netip.AddrPort, target string, log func(string)) (io.Closer, error) {
	lc, err := reuse.ListenPacket(ctx, "udp", laddr.String())
	if err != nil {
		return nil, err
	}

	f, err := forward(WithLogger(logger{log}), WithConn(lc.(*net.UDPConn)), WithDestination(target))
	if err != nil {
		return nil, fmt.Errorf("ForwardUdp: %w", err)
	}
	return f, nil
}
