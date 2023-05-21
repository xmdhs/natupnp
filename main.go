package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/xmdhs/natupnp/natmap"
)

func main() {
	m, s, err := natmap.NatMap(context.Background(), "stun.sipnet.com:3478", "192.168.23.104", 9999, func(s string) {
		log.Println(s)
	})
	if err != nil {
		panic(err)
	}

	defer m.Close()

	fmt.Println(s)

	os.Stdin.Read(make([]byte, 1))
}
