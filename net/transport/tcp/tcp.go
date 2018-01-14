package tcp

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/nikkolasg/dsign/key"
	"github.com/nikkolasg/dsign/net/transport"
)

type tcpTransport struct {
	// the listener of incoming connections
	listener net.Listener
	// the close channel used to indicate to the listener we want to quit.
	quit chan bool
	// quitListener is a channel to indicate to the closing function that the
	// listener has actually really quit.
	quitListener chan bool
	listening    bool

	// closed tells the listen routine to return immediately if a
	// Stop() has been called.
	closed bool

	sync.Mutex
}

func NewTcpTransport() *tcpTransport {
	return &tcpTransport{
		quit:         make(chan bool),
		quitListener: make(chan bool),
	}
}

func (t *tcpTransport) Dial(id *key.Identity) (net.Conn, error) {
	if _, _, err := net.SplitHostPort(id.Address); err != nil {
		return nil, err
	}
	return net.Dial("tcp", id.Address)
}

func (t *tcpTransport) Listen(id *key.Identity, h transport.Handler) error {
	if _, _, err := net.SplitHostPort(id.Address); err != nil {
		return err
	}
	t.Lock()
	if t.closed == true {
		t.Unlock()
		return transport.ErrTransportClosed
	}
	var err error
	t.listener, err = net.Listen("tcp", id.Address)
	if err != nil {
		t.Unlock()
		return err
	}
	t.listening = true
	t.Unlock()
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.quit:
				t.quitListener <- true
				return nil
			default:
			}
			continue
		}
		h(id, conn)
	}
}

func (t *tcpTransport) Close() error {
	// lets see if we launched a listening routing
	t.Lock()
	defer t.Unlock()

	close(t.quit)

	if t.listener != nil {
		if err := t.listener.Close(); err != nil {
			if !strings.Contains("closed", err.Error()) {
				return err
			}
		}
	}
	var stop bool
	if t.listening {
		for !stop {
			select {
			case <-t.quitListener:
				stop = true
			case <-time.After(time.Millisecond * 50):
				continue
			}
		}
	}

	t.quit = make(chan bool)
	t.listening = false
	t.closed = true
	return nil
}
