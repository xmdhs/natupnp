package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/xmdhs/natupnp/natmap"
)

var (
	stun      string
	localAddr string
	port      string
	test      bool
)

func init() {
	flag.StringVar(&stun, "s", "stun.sipnet.com:3478", "stun")
	flag.StringVar(&localAddr, "l", "", "local addr")
	flag.StringVar(&port, "p", "8086", "port")
	flag.BoolVar(&test, "t", false, "test server")
	flag.Parse()
}

func main() {
	if test {
		go testServer(port)
	}
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

	m, s, err := natmap.NatMap(context.Background(), "stun.sipnet.com:3478", localAddr, uint16(portu), func(s string) {
		log.Println(s)
	})
	if err != nil {
		panic(err)
	}
	defer m.Close()
	fmt.Println(s)
	os.Stdin.Read(make([]byte, 1))
}

func testServer(port string) {
	s := http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		Addr:         "0.0.0.0:" + port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		}),
	}
	s.ListenAndServe()
}
