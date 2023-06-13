package natmap

import (
	"context"
	"fmt"
	"io"
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

func NatMap(ctx context.Context, stunAddr string, host string, port uint16, log func(error)) (*Map, string, error) {
	m := Map{}
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	err := upnp.AddPortMapping(ctx, "", port, "TCP", port, host, true, "github.com/xmdhs/natupnp", 0)
	if err != nil {
		return nil, "", fmt.Errorf("NatMap: %w", err)
	}
	stunConn, err := reuse.DialContext(ctx, "tcp4", "0.0.0.0:"+strconv.Itoa(int(port)), stunAddr)
	if err != nil {
		return nil, "", fmt.Errorf("NatMap: %w", err)
	}
	defer stunConn.Close()
	mapAddr, err := stun.GetMappedAddress(ctx, stunConn)
	if err != nil {
		return nil, "", fmt.Errorf("NatMap: %w", err)
	}
	go keepalive(ctx, port, log)
	return &m, fmt.Sprintf("%v:%v", mapAddr.IP.String(), mapAddr.Port), nil
}

func (m Map) Close() error {
	m.cancel()
	return nil
}

func keepalive(ctx context.Context, port uint16, log func(error)) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return reuse.DialContext(ctx, "tcp", "0.0.0.0:"+strconv.Itoa(int(port)), addr)
	}
	c := http.Client{Transport: tr, Timeout: 5 * time.Second}
	for {
		func() {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			reqs, err := http.NewRequestWithContext(ctx, "GET", "http://connect.rom.miui.com/generate_204", nil)
			if err != nil {
				panic(err)
			}
			defer time.Sleep(10 * time.Second)

			rep, err := c.Do(reqs)
			if err != nil {
				c.CloseIdleConnections()
				log(err)
				return
			}
			defer rep.Body.Close()
		}()
		select {
		case <-ctx.Done():
			return
		default:
		}
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

func Forward(ctx context.Context, port uint16, target string, log func(string)) error {
	l, err := reuse.Listen(ctx, "tcp", "0.0.0.0:"+strconv.FormatUint(uint64(port), 10))
	if err != nil {
		return fmt.Errorf("Forward: %w", err)
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			c, err := l.Accept()
			if err != nil {
				log(err.Error())
				continue
			}
			var d net.Dialer
			tc, err := d.DialContext(ctx, "tcp", target)
			if err != nil {
				log(err.Error())
				continue
			}
			go copy(c, tc)
			go copy(tc, c)
		}
	}()
	return nil
}

func copy(dst io.WriteCloser, src io.ReadCloser) (written int64, err error) {
	defer dst.Close()
	defer src.Close()
	return io.Copy(dst, src)
}
