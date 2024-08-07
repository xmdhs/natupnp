package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/xmdhs/natupnp/natmap"
	"github.com/xmdhs/natupnp/reuse"
)

var (
	stun      string
	localAddr string
	port      string
	test      bool
	target    string
	comm      string
	udp       bool
)

func init() {
	flag.StringVar(&stun, "s", "turn.cloudflare.com:3478", "stun")
	flag.StringVar(&localAddr, "l", "", "local addr")
	flag.StringVar(&port, "p", "8086", "port")
	flag.StringVar(&target, "d", "", "forward to target host")
	flag.BoolVar(&test, "t", false, "test server (only tcp)")
	flag.StringVar(&comm, "e", "", "run script for mapped address")
	flag.BoolVar(&udp, "u", false, "udp")
	flag.Parse()
}

func main() {
	ctx := context.Background()
	if localAddr == "" {
		s, err := natmap.GetLocalAddr()
		if err != nil {
			panic(err)
		}
		h, _, err := net.SplitHostPort(s.String())
		if err != nil {
			panic(err)
		}
		localAddr = h
	}
	portu, err := strconv.ParseUint(port, 10, 64)
	if err != nil {
		panic(err)
	}
	laddrPort := netip.AddrPortFrom(netip.MustParseAddr(localAddr), uint16(portu))

	for {
		err := openPort(ctx, target, laddrPort, stun, func(s netip.AddrPort) {
			fmt.Println(s)
			if comm != "" {
				c := exec.CommandContext(ctx, comm, localAddr, port, s.Addr().String(), strconv.Itoa(int(s.Port())))
				c.Stdin = os.Stdin
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				err = c.Run()
				if err != nil {
					log.Println(err)
				}
			}
		}, udp, test)
		if err != nil {
			log.Println(err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func openPort(ctx context.Context, target string, laddr netip.AddrPort,
	stun string, finish func(netip.AddrPort), udp bool, testserver bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if target != "" {
		var forward func(ctx context.Context, laddr netip.AddrPort, target string, log func(string)) (io.Closer, error)
		if udp {
			forward = natmap.ForwardUdp
		} else {
			forward = natmap.Forward
		}
		l, err := forward(ctx, laddr, target, func(s string) {
			log.Println(s)
		})
		if err != nil {
			return fmt.Errorf("openPort: %w", err)
		}
		defer l.Close()
	}
	if testserver {
		l, err := testServer(ctx, laddr)
		if err != nil {
			return fmt.Errorf("openPort: %w", err)
		}
		defer l.Close()
	}
	errCh := make(chan error, 1)
	var nmap func(ctx context.Context, stunAddr string, laddr netip.AddrPort, log func(error)) (*natmap.Map, netip.AddrPort, error)
	if udp {
		nmap = natmap.NatMapUdp
	} else {
		nmap = natmap.NatMap
	}

	m, s, err := nmap(ctx, stun, laddr, func(s error) {
		cancel()
		select {
		case errCh <- s:
		default:
		}
	})
	if err != nil {
		return fmt.Errorf("openPort: %w", err)
	}
	defer m.Close()

	finish(s)

	err = <-errCh
	if err != nil {
		return fmt.Errorf("openPort: %w", err)
	}
	return nil
}

func testServer(ctx context.Context, laddr netip.AddrPort) (net.Listener, error) {
	s := http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		Addr:         laddr.String(),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		}),
	}
	l, err := reuse.Listen(ctx, "tcp", laddr.String())
	if err != nil {
		return nil, fmt.Errorf("testServer: %w", err)
	}
	go func() {
		err = s.Serve(l)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Println(err)
		}
	}()
	return l, nil
}
