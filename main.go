package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
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
)

func init() {
	flag.StringVar(&stun, "s", "stunserver.stunprotocol.org:3478", "stun")
	flag.StringVar(&localAddr, "l", "", "local addr")
	flag.StringVar(&port, "p", "8086", "port")
	flag.StringVar(&target, "d", "", "forward to target host")
	flag.BoolVar(&test, "t", false, "test server")
	flag.StringVar(&comm, "e", "", "run script for mapped address")
	flag.Parse()
}

func main() {
	ctx := context.Background()
	if localAddr == "" {
		s, err := natmap.GetLocalAddr()
		if err != nil {
			panic(err)
		}
		h, _, err := net.SplitHostPort(s)
		if err != nil {
			panic(err)
		}
		localAddr = h
	}
	portu, err := strconv.ParseUint(port, 10, 64)
	if err != nil {
		panic(err)
	}
	for {
		err := openPort(ctx, target, localAddr, uint16(portu), stun, func(s string) {
			fmt.Println(s)
			if comm != "" {
				h, p, err := net.SplitHostPort(s)
				if err != nil {
					panic(err)
				}
				c := exec.CommandContext(ctx, comm, localAddr, port, h, p)
				c.Stdin = os.Stdin
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				err = c.Run()
				if err != nil {
					log.Println(err)
				}
			}
		})
		if err != nil {
			log.Println(err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func openPort(ctx context.Context, target, localAddr string, portu uint16, stun string, finish func(string)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if target != "" {
		l, err := natmap.Forward(ctx, portu, target, func(s string) {
			log.Println(s)
		})
		if err != nil {
			return fmt.Errorf("openPort: %w", err)
		}
		defer l.Close()
	}
	if test {
		l, err := testServer(ctx, portu)
		if err != nil {
			return fmt.Errorf("openPort: %w", err)
		}
		defer l.Close()
	}
	errCh := make(chan error, 1)
	m, s, err := natmap.NatMap(ctx, stun, localAddr, uint16(portu), func(s error) {
		cancel()
		errCh <- ErrNatMap{err: s}
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

type ErrNatMap struct {
	err error
}

func (e ErrNatMap) Error() string {
	return e.err.Error()
}

func (e ErrNatMap) Unwrap() error {
	return e.err
}

func testServer(ctx context.Context, port uint16) (net.Listener, error) {
	s := http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		Addr:         "0.0.0.0:" + strconv.FormatUint(uint64(port), 10),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		}),
	}
	l, err := reuse.Listen(ctx, "tcp", "0.0.0.0:"+strconv.FormatUint(uint64(port), 10))
	if err != nil {
		return nil, fmt.Errorf("testServer: %w", err)
	}
	go func() {
		err = s.Serve(l)
		if err != nil {
			log.Println(err)
		}
	}()
	return l, nil
}
