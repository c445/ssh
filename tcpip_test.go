package ssh

import (
	"bytes"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

var (
	sampleServerResponse    = []byte("Hello world")
	altSampleServerResponse = []byte("Better world")
)

func sampleSocketServer(reply []byte) net.Listener {
	l := newLocalListener()

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		conn.Write(reply)
		conn.Close()
	}()

	return l
}

func newTestSessionWithForwarding(t *testing.T, forwardingEnabled bool) (net.Listener, *gossh.Client, func()) {
	l := sampleSocketServer(sampleServerResponse)

	_, client, cleanup := newTestSession(t, &Server{
		Handler: func(s Session) {},
		LocalPortForwardingCallback: func(ctx Context, pfd *PortForwardDestination) bool {
			addr := pfd.String()
			if addr != l.Addr().String() {
				t.Fatal("unexpected destinationHost: " + addr)
			}
			return forwardingEnabled
		},
	}, nil)

	return l, client, func() {
		cleanup()
		l.Close()
	}
}

func newTestSessionWithForwardingAndOverwriteDestination(t *testing.T, forwardingEnabled bool) (net.Listener, *gossh.Client, func()) {
	l := sampleSocketServer(sampleServerResponse)
	alternativeDestination := sampleSocketServer(altSampleServerResponse)

	_, client, cleanup := newTestSession(t, &Server{
		Handler: func(s Session) {},
		LocalPortForwardingCallback: func(ctx Context, pfd *PortForwardDestination) bool {
			addr := pfd.String()
			if addr != l.Addr().String() {
				t.Fatal("unexpected destinationHost: " + addr)
			}
			host, port, err := net.SplitHostPort(alternativeDestination.Addr().String())
			if err != nil {
				t.Fatalf("cannot split addr(%s): %v", alternativeDestination.Addr(), err)
			}
			pfd.Host = host
			pfd.Port, err = strconv.Atoi(port)
			if err != nil {
				t.Fatalf("cannot convert port(%s): %v", port, err)
			}
			return forwardingEnabled
		},
	}, nil)

	return l, client, func() {
		cleanup()
		l.Close()
	}
}

func TestLocalPortForwardingWorks(t *testing.T) {
	t.Parallel()

	l, client, cleanup := newTestSessionWithForwarding(t, true)
	defer cleanup()

	conn, err := client.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("Error connecting to %v: %v", l.Addr().String(), err)
	}
	result, err := ioutil.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, sampleServerResponse) {
		t.Fatalf("result = %#v; want %#v", result, sampleServerResponse)
	}
}

func TestLocalPortForwardingWithOverwriteWorks(t *testing.T) {
	t.Parallel()

	l, client, cleanup := newTestSessionWithForwardingAndOverwriteDestination(t, true)
	defer cleanup()

	conn, err := client.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("Error connecting to %v: %v", l.Addr().String(), err)
	}
	result, err := ioutil.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, altSampleServerResponse) {
		t.Fatalf("result = %#v; want %#v", result, sampleServerResponse)
	}
}

func TestLocalPortForwardingRespectsCallback(t *testing.T) {
	t.Parallel()

	l, client, cleanup := newTestSessionWithForwarding(t, false)
	defer cleanup()

	_, err := client.Dial("tcp", l.Addr().String())
	if err == nil {
		t.Fatalf("Expected error connecting to %v but it succeeded", l.Addr().String())
	}
	if !strings.Contains(err.Error(), "port forwarding is disabled") {
		t.Fatalf("Expected permission error but got %#v", err)
	}
}
