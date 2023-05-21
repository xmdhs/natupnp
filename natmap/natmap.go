package natmap

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/xmdhs/natupnp/reuse"
	"github.com/xmdhs/natupnp/stun"
	"github.com/xmdhs/natupnp/upnp"
)

type Map struct {
	cancel func()
}

func NatMap(ctx context.Context, stunAddr string, host string, port uint16, log func(string)) (*Map, string, error) {
	m := Map{}
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	err := upnp.AddPortMapping(ctx, "", port, "TCP", port, host, true, "github.com/xmdhs/natupnp", 0)
	if err != nil {
		return nil, "", fmt.Errorf("NatMap: %w", err)
	}
	go keepalive(ctx, port, log)

	stunConn, err := reuse.DialContext(ctx, "tcp", "0.0.0.0:"+strconv.Itoa(int(port)), stunAddr)
	if err != nil {
		return nil, "", fmt.Errorf("NatMap: %w", err)
	}
	mapAddr, err := stun.GetMappedAddress(ctx, stunConn)
	if err != nil {
		return nil, "", fmt.Errorf("NatMap: %w", err)
	}

	return &m, fmt.Sprintf("%v:%v", mapAddr.IP.String(), mapAddr.Port), nil
}

func (m Map) Close() error {
	m.cancel()
	return nil
}

func keepalive(ctx context.Context, port uint16, log func(string)) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return reuse.DialContext(ctx, "tcp", "0.0.0.0:"+strconv.Itoa(int(port)), addr)
	}
	c := http.Client{Transport: tr}
	for {
		reqs, err := http.NewRequestWithContext(ctx, "GET", "http://connect.rom.miui.com/generate_204", nil)
		if err != nil {
			panic(err)
		}
		rep, err := c.Do(reqs)
		if err != nil {
			c.CloseIdleConnections()
			log(err.Error())
			continue
		}
		rep.Body.Close()
		time.Sleep(10 * time.Second)
	}
}

func GetLocalAddr() (string, error) {
	l, err := net.Dial("udp4", "223.5.5.5:53")
	if err != nil {
		return "", fmt.Errorf("getLocal: %w", err)
	}
	defer l.Close()
	return l.LocalAddr().String(), nil
}
