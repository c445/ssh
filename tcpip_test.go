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

var sampleServerResponse = []byte("Hello world")
var altSampleServerResponse = []byte("Better world")

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
		LocalPortForwardingCallback: func(ctx Context, pdf *PortForwardDestination) bool {
			addr := net.JoinHostPort(pdf.Host, strconv.Itoa(pdf.Port))
			if addr != l.Addr().String() {
				panic("unexpected destinationHost: " + addr)
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
		LocalPortForwardingCallback: func(ctx Context, pdf *PortForwardDestination) bool {
			addr := net.JoinHostPort(pdf.Host, strconv.Itoa(pdf.Port))
			if addr != l.Addr().String() {
				panic("unexpected destinationHost: " + addr)
			}
			tmp := strings.Split(alternativeDestination.Addr().String(), ":")
			pdf.Host = tmp[0]
			p, err := strconv.Atoi(tmp[1])
			if err != nil {
				t.Fatalf("cannot convert port %s", tmp[1])
			}
			pdf.Port = p
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
