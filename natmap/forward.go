// Package forward contains a UDP packet forwarder.
// https://github.com/gwangyi/udp-forward
package natmap

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"
)

type connection struct {
	available  chan struct{}
	udp        *net.UDPConn
	lastActive time.Time
}

type Logger interface {
	Println(v ...any)
}

// Forwarder represents a UDP packet forwarder.
type Forwarder struct {
	src          *net.UDPAddr
	router       Router
	client       *net.UDPAddr
	listenerConn *net.UDPConn

	connections      map[string]*connection
	connectionsMutex *sync.RWMutex

	connectCallback    func(addr string)
	disconnectCallback func(addr string)

	timeout time.Duration

	closed bool

	bufferSize int

	logger Logger
}

// Router represents a router that gives the destination address.
type Router interface {
	Route(*net.UDPAddr) *net.UDPAddr
}

type staticRouter struct {
	*net.UDPAddr
}

func (r staticRouter) Route(*net.UDPAddr) *net.UDPAddr {
	return r.UDPAddr
}

type funcRouter func(*net.UDPAddr) *net.UDPAddr

func (r funcRouter) Route(incoming *net.UDPAddr) *net.UDPAddr {
	return r(incoming)
}

// config represents the configuration of Forwarder.
type config struct {
	listenerFactory func() (*net.UDPConn, error)
	router          Router
	timeout         time.Duration
	bufferSize      int
	logger          Logger
}

// Option gives the way to customize the forwarder.
type Option func(*config) error

// WithAddr lets the new forwarder listen from given address.
func WithAddr(src string) Option {
	return func(c *config) error {
		srcAddr, err := net.ResolveUDPAddr("udp", src)
		if err != nil {
			return err
		}
		c.listenerFactory = func() (*net.UDPConn, error) {
			return net.ListenUDP("udp", srcAddr)
		}
		return nil
	}
}

// WithConn lets the new forwarder to use given conn instead of new one.
func WithConn(conn *net.UDPConn) Option {
	return func(c *config) error {
		c.listenerFactory = func() (*net.UDPConn, error) {
			return conn, nil
		}
		return nil
	}
}

// WithDestination lets the new forwarder forward packets to the given address.
func WithDestination(dest string) Option {
	return func(c *config) error {
		destAddr, err := net.ResolveUDPAddr("udp", dest)
		if err != nil {
			return err
		}
		c.router = staticRouter{UDPAddr: destAddr}
		return nil
	}
}

// WithRouter lets the new forwarder forward packets according to the given router.
func WithRouter(router Router) Option {
	return func(c *config) error {
		c.router = router
		return nil
	}
}

// WithRouterFunc does the same as WithRouter, but with a function.
func WithRouterFunc(router func(*net.UDPAddr) *net.UDPAddr) Option {
	return WithRouter(funcRouter(router))
}

// WithTimeout sets the timeout.
// No interaction more than the timeout will remove the connection from the NAT
// table.
func WithTimeout(timeout time.Duration) Option {
	return func(c *config) error {
		c.timeout = timeout
		return nil
	}
}

// WithBufferSize sets the buffer size that is used by forwarding.
// Larger packet can be discarded.
func WithBufferSize(size int) Option {
	return func(c *config) error {
		c.bufferSize = size
		return nil
	}
}

// WithLogger sets a logger.
func WithLogger(logger Logger) Option {
	return func(c *config) error {
		c.logger = logger
		return nil
	}
}

type emptyLogger struct{}

func (emptyLogger) Println(v ...any) {}

// WithoutLogger lets forwarder not log anything.
func WithoutLogger() Option {
	return WithLogger(emptyLogger{})
}

// DefaultTimeout is the default timeout period of inactivity for convenience
// sake. It is equivelant to 5 minutes.
const DefaultTimeout = time.Minute * 5

// Forward forwards UDP packets from the src address to the dst address, with a
// timeout to "disconnect" clients after the timeout period of inactivity. It
// implements a reverse NAT and thus supports multiple seperate users. Forward
// is also asynchronous.
func forward(options ...Option) (*Forwarder, error) {
	config := &config{
		timeout:    DefaultTimeout,
		bufferSize: 4096,
		logger:     log.Default(),
	}

	options = append([]Option{WithAddr(":")}, options...)

	for _, opt := range options {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	forwarder := new(Forwarder)
	forwarder.connectCallback = func(addr string) {}
	forwarder.disconnectCallback = func(addr string) {}
	forwarder.connectionsMutex = new(sync.RWMutex)
	forwarder.connections = make(map[string]*connection)
	forwarder.timeout = config.timeout
	forwarder.router = config.router
	forwarder.bufferSize = config.bufferSize
	forwarder.logger = config.logger

	var err error
	forwarder.listenerConn, err = config.listenerFactory()
	if err != nil {
		return nil, err
	}
	forwarder.src, _ = forwarder.listenerConn.LocalAddr().(*net.UDPAddr)

	go forwarder.janitor()
	go forwarder.run()

	return forwarder, nil
}

func (f *Forwarder) run() {
	for {
		buf := make([]byte, f.bufferSize)
		oob := make([]byte, f.bufferSize)
		n, _, _, addr, err := f.listenerConn.ReadMsgUDP(buf, oob)
		if err != nil {
			f.logger.Println("forward: failed to read, terminating:", err)
			return
		}
		go f.handle(buf[:n], addr)
	}
}

func (f *Forwarder) janitor() {
	for !f.closed {
		time.Sleep(f.timeout)
		var keysToDelete []string

		f.connectionsMutex.RLock()
		for k, conn := range f.connections {
			if conn.lastActive.Before(time.Now().Add(-f.timeout)) {
				keysToDelete = append(keysToDelete, k)
			}
		}
		f.connectionsMutex.RUnlock()

		f.connectionsMutex.Lock()
		for _, k := range keysToDelete {
			f.connections[k].udp.Close()
			delete(f.connections, k)
		}
		f.connectionsMutex.Unlock()

		for _, k := range keysToDelete {
			f.disconnectCallback(k)
		}
	}
}

func (f *Forwarder) handle(data []byte, addr *net.UDPAddr) {
	f.connectionsMutex.Lock()
	conn, found := f.connections[addr.String()]
	if !found {
		f.connections[addr.String()] = &connection{
			available:  make(chan struct{}),
			udp:        nil,
			lastActive: time.Now(),
		}
	}
	f.connectionsMutex.Unlock()

	if !found {
		var udpConn *net.UDPConn
		var err error
		dst := f.router.Route(addr)
		if dst == nil {
			f.connectionsMutex.Lock()
			delete(f.connections, addr.String())
			f.connectionsMutex.Unlock()
			return
		}
		if dst.IP.To4()[0] == 127 {
			// log.Println("using local listener")
			laddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:")
			udpConn, err = net.DialUDP("udp", laddr, dst)
		} else {
			udpConn, err = net.DialUDP("udp", nil, dst)
		}
		if err != nil {
			f.logger.Println("udp-forward: failed to dial:", err)
			delete(f.connections, addr.String())
			return
		}

		f.connectionsMutex.Lock()
		f.connections[addr.String()].udp = udpConn
		f.connections[addr.String()].lastActive = time.Now()
		close(f.connections[addr.String()].available)
		f.connectionsMutex.Unlock()

		f.connectCallback(addr.String())

		_, _, err = udpConn.WriteMsgUDP(data, nil, nil)
		if err != nil {
			f.logger.Println("udp-forward: error sending initial packet to client", err)
		}

		for {
			// log.Println("in loop to read from NAT connection to servers")
			buf := make([]byte, f.bufferSize)
			oob := make([]byte, f.bufferSize)
			n, _, _, _, err := udpConn.ReadMsgUDP(buf, oob)
			if err != nil {
				f.connectionsMutex.Lock()
				udpConn.Close()
				delete(f.connections, addr.String())
				f.connectionsMutex.Unlock()
				f.disconnectCallback(addr.String())
				f.logger.Println("udp-forward: abnormal read, closing:", err)
				return
			}

			// log.Println("sent packet to client")
			_, _, err = f.listenerConn.WriteMsgUDP(buf[:n], nil, addr)
			if err != nil {
				f.logger.Println("udp-forward: error sending packet to client:", err)
			}
		}

		// unreachable
	}

	<-conn.available

	// log.Println("sent packet to server", conn.udp.RemoteAddr())
	_, _, err := conn.udp.WriteMsgUDP(data, nil, nil)
	if err != nil {
		f.logger.Println("udp-forward: error sending packet to server:", err)
	}

	shouldChangeTime := false
	f.connectionsMutex.RLock()
	if _, found := f.connections[addr.String()]; found {
		if f.connections[addr.String()].lastActive.Before(
			time.Now().Add(f.timeout / 4)) {
			shouldChangeTime = true
		}
	}
	f.connectionsMutex.RUnlock()

	if shouldChangeTime {
		f.connectionsMutex.Lock()
		// Make sure it still exists
		if _, found := f.connections[addr.String()]; found {
			connWrapper := f.connections[addr.String()]
			connWrapper.lastActive = time.Now()
			f.connections[addr.String()] = connWrapper
		}
		f.connectionsMutex.Unlock()
	}
}

// Close stops the forwarder.
func (f *Forwarder) Close() error {
	var errs error
	f.connectionsMutex.Lock()
	f.closed = true
	for _, conn := range f.connections {
		err := conn.udp.Close()
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}
	err := f.listenerConn.Close()
	if err != nil {
		errs = errors.Join(errs, err)
	}
	f.connectionsMutex.Unlock()
	return errs
}

// OnConnect can be called with a callback function to be called whenever a
// new client connects.
func (f *Forwarder) OnConnect(callback func(addr string)) {
	f.connectCallback = callback
}

// OnDisconnect can be called with a callback function to be called whenever a
// new client disconnects (after 5 minutes of inactivity).
func (f *Forwarder) OnDisconnect(callback func(addr string)) {
	f.disconnectCallback = callback
}

// Connected returns the list of connected clients in IP:port form.
func (f *Forwarder) Connected() []string {
	f.connectionsMutex.Lock()
	defer f.connectionsMutex.Unlock()
	results := make([]string, 0, len(f.connections))
	for key := range f.connections {
		results = append(results, key)
	}
	return results
}

// LocalAddr returns LocalAddr of listening connection.
func (f *Forwarder) LocalAddr() *net.UDPAddr {
	addr, _ := f.listenerConn.LocalAddr().(*net.UDPAddr)
	return addr
}
