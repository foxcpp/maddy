/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package pop3

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/emersion/go-imap"
	imapbackend "github.com/emersion/go-imap/backend"
	_ "github.com/emersion/go-message/charset"
	"github.com/foxcpp/go-imap-mess"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	tls2 "github.com/foxcpp/maddy/framework/config/tls"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/auth"
	"github.com/foxcpp/maddy/internal/authz"
	"github.com/foxcpp/maddy/internal/proxy_protocol"
	"github.com/kiwiz/popgun"
	pop3backend "github.com/kiwiz/popgun/backends"
)

type Session struct {
	imapbackend.User
	Mailbox      imapbackend.Mailbox
	deletedItems *imap.SeqSet
}

type Endpoint struct {
	addrs         []string
	serv          *popgun.Server
	listeners     []net.Listener
	proxyProtocol *proxy_protocol.ProxyProtocol
	Store         module.Storage

	tlsConfig      *tls.Config
	listenersWg    sync.WaitGroup
	lockMutex      sync.Mutex
	activeUsersMap map[string]bool

	saslAuth auth.SASLAuth

	storageNormalize authz.NormalizeFunc
	storageMap       module.Table

	Log log.Logger
}

func New(modName string, addrs []string) (module.Module, error) {
	endp := &Endpoint{
		addrs: addrs,
		Log:   log.Logger{Name: modName},
		saslAuth: auth.SASLAuth{
			Log: log.Logger{Name: modName + "/sasl"},
		},
		activeUsersMap: make(map[string]bool),
	}

	return endp, nil
}

func (endp *Endpoint) Init(cfg *config.Map) error {
	var (
		insecureAuth bool
		debug        bool
		errors       bool
	)

	cfg.Callback("auth", func(m *config.Map, node config.Node) error {
		return endp.saslAuth.AddProvider(m, node)
	})
	cfg.Bool("sasl_login", false, false, &endp.saslAuth.EnableLogin)
	cfg.Custom("storage", false, true, nil, modconfig.StorageDirective, &endp.Store)
	cfg.Custom("tls", true, true, nil, tls2.TLSDirective, &endp.tlsConfig)
	cfg.Custom("proxy_protocol", false, false, nil, proxy_protocol.ProxyProtocolDirective, &endp.proxyProtocol)
	cfg.Bool("insecure_auth", false, false, &insecureAuth)
	cfg.Bool("errors", false, false, &errors)
	cfg.Bool("debug", true, false, &debug)
	config.EnumMapped(cfg, "storage_map_normalize", false, false, authz.NormalizeFuncs, authz.NormalizeAuto,
		&endp.storageNormalize)
	modconfig.Table(cfg, "storage_map", false, false, nil, &endp.storageMap)
	config.EnumMapped(cfg, "auth_map_normalize", true, false, authz.NormalizeFuncs, authz.NormalizeAuto,
		&endp.saslAuth.AuthNormalize)
	modconfig.Table(cfg, "auth_map", true, false, nil, &endp.saslAuth.AuthMap)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	endp.saslAuth.Log.Debug = endp.Log.Debug

	addresses := make([]config.Endpoint, 0, len(endp.addrs))
	for _, addr := range endp.addrs {
		saddr, err := config.ParseEndpoint(addr)
		if err != nil {
			return fmt.Errorf("pop3: invalid address: %s", addr)
		}
		addresses = append(addresses, saddr)
	}

	endp.serv = popgun.NewServer(endp, endp)
	if errors {
		endp.serv.ErrorLog = &endp.Log
	}
	if debug {
		endp.serv.DebugLog = &endp.Log
	}
	endp.serv.AllowInsecureAuth = insecureAuth

	return endp.setupListeners(addresses)
}

func (endp *Endpoint) setupListeners(addresses []config.Endpoint) error {
	for _, addr := range addresses {
		var l net.Listener
		var err error
		l, err = net.Listen(addr.Network(), addr.Address())
		if err != nil {
			return fmt.Errorf("pop3: %v", err)
		}
		endp.Log.Printf("listening on %v", addr)

		if addr.IsTLS() {
			if endp.tlsConfig == nil {
				return errors.New("pop3: can't bind on POPS endpoint without TLS configuration")
			}
			l = tls.NewListener(l, endp.tlsConfig)
		}

		if endp.proxyProtocol != nil {
			l = proxy_protocol.NewListener(l, endp.proxyProtocol, endp.Log)
		}

		endp.listeners = append(endp.listeners, l)

		endp.listenersWg.Add(1)
		go func() {
			if err := endp.serv.Serve(l); err != nil && !strings.HasSuffix(err.Error(), "use of closed network connection") {
				endp.Log.Printf("pop3: failed to serve %s: %s", addr, err)
			}
			endp.listenersWg.Done()
		}()
	}

	if endp.serv.AllowInsecureAuth {
		endp.Log.Println("authentication over unencrypted connections is allowed, this is insecure configuration and should be used only for testing!")
	}

	return nil
}

func (endp *Endpoint) Name() string {
	return "pop3"
}

func (endp *Endpoint) InstanceName() string {
	return "pop3"
}

func (endp *Endpoint) Close() error {
	for _, l := range endp.listeners {
		l.Close()
	}
	endp.listenersWg.Wait()
	return nil
}

func (endp *Endpoint) getSession(user pop3backend.User) (*Session, error) {
	sess, ok := user.(*Session)
	if !ok {
		return nil, fmt.Errorf("internal server error")
	}

	return sess, nil
}

func (endp *Endpoint) usernameForStorage(ctx context.Context, saslUsername string) (string, error) {
	saslUsername, err := endp.storageNormalize(saslUsername)
	if err != nil {
		return "", err
	}

	if endp.storageMap == nil {
		return saslUsername, nil
	}

	mapped, ok, err := endp.storageMap.Lookup(ctx, saslUsername)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", imapbackend.ErrInvalidCredentials
	}

	if saslUsername != mapped {
		endp.Log.DebugMsg("using mapped username for storage", "username", saslUsername, "mapped_username", mapped)
	}

	return mapped, nil
}

// interface implementation for popgun.Authorizator
func (endp *Endpoint) Authorize(conn net.Conn, user, pass string) (pop3backend.User, error) {
	// saslAuth handles AuthMap calling.
	err := endp.saslAuth.AuthPlain(user, pass)
	if err != nil {
		endp.Log.Error("authentication failed", err, "username", user, "src_ip", conn.RemoteAddr())
		return nil, imapbackend.ErrInvalidCredentials
	}

	storageUsername, err := endp.usernameForStorage(context.TODO(), user)
	if err != nil {
		if errors.Is(err, imapbackend.ErrInvalidCredentials) {
			return nil, err
		}
		endp.Log.Error("authentication failed due to an internal error", err, "username", user, "src_ip", conn.RemoteAddr())
		return nil, fmt.Errorf("internal server error")
	}

	imapUser, err := endp.Store.GetOrCreateIMAPAcct(storageUsername)
	if err != nil {
		return nil, err
	}

	_, mailbox, err := imapUser.GetMailbox(imap.InboxName, true, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to get maildrop")
	}

	return &Session{
		imapUser,
		mailbox,
		&imap.SeqSet{},
	}, nil
}

// interface implementation for popgun.Backend
func (endp *Endpoint) Stat(user pop3backend.User) (messages, octets int, err error) {
	sess, err := endp.getSession(user)
	if err != nil {
		return 0, 0, err
	}

	msgChan := make(chan *imap.Message)
	errChan := make(chan error, 1)

	go func() {
		errChan <- sess.Mailbox.ListMessages(true, nil, []imap.FetchItem{imap.FetchRFC822Size}, msgChan)
	}()

	count := 0
	size := 0
	for msg := range msgChan {
		count += 1
		size += int(msg.Size)
	}

	err = <-errChan
	if err != nil && err != mess.ErrNoMessages {
		return 0, 0, err
	}

	return count, size, nil
}

// List of sizes of all messages in bytes (octets)
func (endp *Endpoint) List(user pop3backend.User) (octets []int, err error) {
	sess, err := endp.getSession(user)
	if err != nil {
		return nil, err
	}

	msgChan := make(chan *imap.Message)
	errChan := make(chan error, 1)

	seqset := imap.SeqSet{}
	seqset.AddNum(0)
	go func() {
		errChan <- sess.Mailbox.ListMessages(true, &seqset, []imap.FetchItem{imap.FetchRFC822Size}, msgChan)
	}()

	items := make([]int, 0)
	for msg := range msgChan {
		items = append(items, int(msg.Size))
	}

	err = <-errChan
	if err != nil && err != mess.ErrNoMessages {
		return nil, err
	}

	return items, nil
}

// Returns whether message exists and if yes, then return size of the message in bytes (octets)
func (endp *Endpoint) ListMessage(user pop3backend.User, msgId int) (exists bool, octets int, err error) {
	sess, err := endp.getSession(user)
	if err != nil {
		return false, 0, err
	}

	msgChan := make(chan *imap.Message, 1)
	errChan := make(chan error, 1)

	seqset := imap.SeqSet{}
	seqset.AddNum(uint32(msgId))
	go func() {
		errChan <- sess.Mailbox.ListMessages(true, &seqset, []imap.FetchItem{imap.FetchRFC822Size}, msgChan)
	}()

	var msg *imap.Message
	msg = <-msgChan

	err = <-errChan
	if err != nil && err != mess.ErrNoMessages {
		return false, 0, err
	}

	if msg == nil {
		return false, 0, nil
	}

	return true, int(msg.Size), nil
}

// Retrieve whole message by ID - note that message ID is a message position returned
// by List() function, so be sure to keep that order unchanged while client is connected
// See Lock() function for more details
func (endp *Endpoint) Retr(user pop3backend.User, msgId int) (message string, err error) {
	sess, err := endp.getSession(user)
	if err != nil {
		return "", err
	}

	msgChan := make(chan *imap.Message)
	errChan := make(chan error, 1)

	seqset := imap.SeqSet{}
	seqset.AddNum(uint32(msgId))
	go func() {
		errChan <- sess.Mailbox.ListMessages(true, &seqset, []imap.FetchItem{imap.FetchRFC822Size}, msgChan)
	}()

	var msg *imap.Message
	msg = <-msgChan

	err = <-errChan
	if err != nil && err != mess.ErrNoMessages {
		return "", err
	}

	if msg == nil {
		return "", fmt.Errorf("not found")
	}

	return strconv.FormatUint(uint64(msg.Uid), 10), nil
}

// Delete message by message ID - message should be just marked as deleted until
// Update() is called. Be aware that after Dele() is called, functions like List() etc.
// should ignore all these messages even if Update() hasn't been called yet
func (endp *Endpoint) Dele(user pop3backend.User, msgId int) error {
	sess, err := endp.getSession(user)
	if err != nil {
		return err
	}

	seqset := imap.SeqSet{}
	seqset.AddNum(uint32(msgId))
	err = sess.Mailbox.UpdateMessagesFlags(true, &seqset, imap.SetFlags, false, []string{imap.DeletedFlag})
	if err != nil {
		return err
	}

	sess.deletedItems.AddNum(uint32(msgId))
	return nil
}

// Undelete all messages marked as deleted in single connection
func (endp *Endpoint) Rset(user pop3backend.User) error {
	sess, err := endp.getSession(user)
	if err != nil {
		return err
	}

	err = sess.Mailbox.UpdateMessagesFlags(true, sess.deletedItems, imap.RemoveFlags, false, []string{imap.DeletedFlag})
	if err != nil {
		return fmt.Errorf("pop3: internal server error")
	}

	sess.deletedItems = &imap.SeqSet{}
	return nil
}

// List of unique IDs of all message, similar to List(), but instead of size there
// is a unique ID which persists the same across all connections. Uid (unique id) is
// used to allow client to be able to keep messages on the server.
func (endp *Endpoint) Uidl(user pop3backend.User) (uids []string, err error) {
	sess, err := endp.getSession(user)
	if err != nil {
		return nil, err
	}

	msgChan := make(chan *imap.Message)
	errChan := make(chan error, 1)

	go func() {
		errChan <- sess.Mailbox.ListMessages(false, nil, nil, msgChan)
	}()

	items := make([]string, 0)
	for msg := range msgChan {
		items = append(items, strconv.FormatUint(uint64(msg.Uid), 10))
	}

	err = <-errChan
	if err != nil && err != mess.ErrNoMessages {
		return nil, err
	}

	return items, nil
}

// Similar to ListMessage, but returns unique ID by message ID instead of size.
func (endp *Endpoint) UidlMessage(user pop3backend.User, msgId int) (exists bool, uid string, err error) {
	sess, err := endp.getSession(user)
	if err != nil {
		return false, "", err
	}

	msgChan := make(chan *imap.Message, 1)
	errChan := make(chan error, 1)

	seqset := imap.SeqSet{}
	seqset.AddNum(uint32(msgId))

	go func() {
		errChan <- sess.Mailbox.ListMessages(true, &seqset, nil, msgChan)
	}()

	var msg *imap.Message
	msg = <-msgChan

	err = <-errChan
	if err != nil && err != mess.ErrNoMessages {
		return false, "", err
	}

	if msg == nil {
		return false, "", nil
	}

	return true, strconv.FormatUint(uint64(msg.Uid), 10), nil
}

// Write all changes to persistent storage, i.e. delete all messages marked as deleted.
func (endp *Endpoint) Update(user pop3backend.User) error {
	sess, err := endp.getSession(user)
	if err != nil {
		return err
	}

	return sess.Mailbox.Expunge()
}

// If the POP3 server issues a positive response, then the
// response given is multi-line.  After the initial +OK, the
// POP3 server sends the headers of the message, the blank
// line separating the headers from the body, and then the
// number of lines of the indicated message's body, being
// careful to byte-stuff the termination character (as with
// all multi-line responses).
// Note that if the number of lines requested by the POP3
// client is greater than than the number of lines in the
// body, then the POP3 server sends the entire message.
func (endp *Endpoint) Top(user pop3backend.User, msgId int, n int) (lines []string, err error) {
	return nil, fmt.Errorf("pop3: unimplemented")
}

// Lock is called immediately after client is connected. The best way what to use Lock() for
// is to read all the messages into cache after client is connected. If another user
// tries to lock the storage, you should return an error to avoid data race.
func (endp *Endpoint) Lock(user pop3backend.User) error {
	endp.lockMutex.Lock()
	defer endp.lockMutex.Unlock()

	backendUser, ok := user.(imapbackend.User)
	if !ok {
		return fmt.Errorf("pop3: internal server error")
	}
	username := backendUser.Username()

	// Only one simultaneous connection is supported
	if endp.activeUsersMap[username] {
		return fmt.Errorf("pop3: internal server error")
	}

	endp.activeUsersMap[username] = true

	return nil
}

// Release lock on storage, Unlock() is called after client is disconnected.
func (endp *Endpoint) Unlock(user pop3backend.User) error {
	endp.lockMutex.Lock()
	defer endp.lockMutex.Unlock()

	backendUser, ok := user.(imapbackend.User)
	if !ok {
		return fmt.Errorf("pop3: internal server error")
	}

	username := backendUser.Username()

	// Not locked
	if !endp.activeUsersMap[username] {
		return fmt.Errorf("pop3: internal server error")
	}

	err := endp.Rset(user)
	if err != nil {
		return err
	}

	err = backendUser.Logout()
	if err != nil {
		return err
	}

	endp.activeUsersMap[username] = false

	return nil
}

func init() {
	module.RegisterEndpoint("pop3", New)
}
