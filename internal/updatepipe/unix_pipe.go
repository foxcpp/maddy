package updatepipe

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/emersion/go-imap/backend"
	"github.com/foxcpp/maddy/internal/log"
)

// UnixSockPipe implements the UpdatePipe interface by serializating updates
// to/from a Unix domain socket. Due to the way Unix sockets work, only one
// Listen goroutine can be running.
//
// The socket is stream-oriented and consists of the following messages:
//		OBJ_ID;TYPE_NAME;USER;MAILBOX;JSON_SERIALIZED_INTERNAL_OBJECT\n
//
// Where TYPE_NAME is one of the folow: ExpungeUpdate, MailboxUpdate,
// MessageUpdate.
// And OBJ_ID is Process ID and UnixSockPipe address concated as a string.
// It is used to deduplicate updates sent to Push and recevied via Listen.
//
// The SockPath field specifies the socket path to use. The actual socket
// is initialized on the first call to Listen or (Init)Push.
type UnixSockPipe struct {
	SockPath string
	Log      log.Logger

	listener net.Listener
	sender   net.Conn
}

var _ P = &UnixSockPipe{}

func (usp *UnixSockPipe) myID() string {
	return fmt.Sprintf("%d-%p", os.Getpid(), usp)
}

func (usp *UnixSockPipe) readUpdates(conn net.Conn, updCh chan<- backend.Update) {
	scnr := bufio.NewScanner(conn)
	for scnr.Scan() {
		id, upd, err := parseUpdate(scnr.Text())
		if err != nil {
			usp.Log.Error("malformed update received", err, "str", scnr.Text())
		}

		// It is our own update, skip.
		if id == usp.myID() {
			continue
		}

		updCh <- upd
	}
}

func (usp *UnixSockPipe) Wrap(upd <-chan backend.Update) chan backend.Update {
	ourUpds := make(chan backend.Update, cap(upd))

	return ourUpds
}

func (usp *UnixSockPipe) Listen(upd chan<- backend.Update) error {
	l, err := net.Listen("unix", usp.SockPath)
	if err != nil {
		return err
	}
	usp.listener = l
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go usp.readUpdates(conn, upd)
		}
	}()
	return nil
}

func (usp *UnixSockPipe) InitPush() error {
	sock, err := net.Dial("unix", usp.SockPath)
	if err != nil {
		return err
	}

	usp.sender = sock
	return nil
}

func (usp *UnixSockPipe) Push(upd backend.Update) error {
	if usp.sender == nil {
		if err := usp.InitPush(); err != nil {
			return err
		}
	}

	updStr, err := formatUpdate(usp.myID(), upd)
	if err != nil {
		return err
	}

	_, err = io.WriteString(usp.sender, updStr)
	return err
}

func (usp *UnixSockPipe) Close() error {
	if usp.sender != nil {
		usp.sender.Close()
	}
	if usp.listener != nil {
		usp.listener.Close()
		os.Remove(usp.SockPath)
	}
	return nil
}
