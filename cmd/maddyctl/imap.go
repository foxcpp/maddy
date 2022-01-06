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

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/maddy/cmd/maddyctl/clitools"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/urfave/cli"
)

func FormatAddress(addr *imap.Address) string {
	return fmt.Sprintf("%s <%s@%s>", addr.PersonalName, addr.MailboxName, addr.HostName)
}

func FormatAddressList(addrs []*imap.Address) string {
	res := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		res = append(res, FormatAddress(addr))
	}
	return strings.Join(res, ", ")
}

func mboxesList(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	mboxes, err := u.ListMailboxes(ctx.Bool("subscribed,s"))
	if err != nil {
		return err
	}

	if len(mboxes) == 0 && !ctx.GlobalBool("quiet") {
		fmt.Fprintln(os.Stderr, "No mailboxes.")
	}

	for _, info := range mboxes {
		if len(info.Attributes) != 0 {
			fmt.Print(info.Name, "\t", info.Attributes, "\n")
		} else {
			fmt.Println(info.Name)
		}
	}

	return nil
}

func mboxesCreate(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return errors.New("Error: NAME is required")
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	if ctx.IsSet("special") {
		attr := "\\" + strings.Title(ctx.String("special"))

		suu, ok := u.(SpecialUseUser)
		if !ok {
			return errors.New("Error: storage backend does not support SPECIAL-USE IMAP extension")
		}

		return suu.CreateMailboxSpecial(name, attr)
	}

	return u.CreateMailbox(name)
}

func mboxesRemove(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return errors.New("Error: NAME is required")
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	if !ctx.Bool("yes,y") {
		status, err := u.Status(name, []imap.StatusItem{imap.StatusMessages})
		if err != nil {
			return err
		}

		if status.Messages != 0 {
			fmt.Fprintf(os.Stderr, "Mailbox %s contains %d messages.\n", name, status.Messages)
		}

		if !clitools.Confirmation("Are you sure you want to delete that mailbox?", false) {
			return errors.New("Cancelled")
		}
	}

	return u.DeleteMailbox(name)
}

func mboxesRename(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	oldName := ctx.Args().Get(1)
	if oldName == "" {
		return errors.New("Error: OLDNAME is required")
	}
	newName := ctx.Args().Get(2)
	if newName == "" {
		return errors.New("Error: NEWNAME is required")
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	return u.RenameMailbox(oldName, newName)
}

func msgsAdd(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return errors.New("Error: MAILBOX is required")
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	flags := ctx.StringSlice("flag")
	if flags == nil {
		flags = []string{}
	}

	date := time.Now()
	if ctx.IsSet("date") {
		date = time.Unix(ctx.Int64("date"), 0)
	}

	buf := bytes.Buffer{}
	if _, err := io.Copy(&buf, os.Stdin); err != nil {
		return err
	}

	if buf.Len() == 0 {
		return errors.New("Error: Empty message, refusing to continue")
	}

	status, err := u.Status(name, []imap.StatusItem{imap.StatusUidNext})
	if err != nil {
		return err
	}

	if err := u.CreateMessage(name, flags, date, &buf, nil); err != nil {
		return err
	}

	// TODO: Use APPENDUID
	fmt.Println(status.UidNext)

	return nil
}

func msgsRemove(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return errors.New("Error: MAILBOX is required")
	}
	seqset := ctx.Args().Get(2)
	if seqset == "" {
		return errors.New("Error: SEQSET is required")
	}

	seq, err := imap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	_, mbox, err := u.GetMailbox(name, true, nil)
	if err != nil {
		return err
	}

	if !ctx.Bool("yes") {
		if !clitools.Confirmation("Are you sure you want to delete these messages?", false) {
			return errors.New("Cancelled")
		}
	}

	mboxB := mbox.(*imapsql.Mailbox)
	return mboxB.DelMessages(ctx.Bool("uid"), seq)
}

func msgsCopy(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	srcName := ctx.Args().Get(1)
	if srcName == "" {
		return errors.New("Error: SRCMAILBOX is required")
	}
	seqset := ctx.Args().Get(2)
	if seqset == "" {
		return errors.New("Error: SEQSET is required")
	}
	tgtName := ctx.Args().Get(3)
	if tgtName == "" {
		return errors.New("Error: TGTMAILBOX is required")
	}

	seq, err := imap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	_, srcMbox, err := u.GetMailbox(srcName, true, nil)
	if err != nil {
		return err
	}

	return srcMbox.CopyMessages(ctx.Bool("uid"), seq, tgtName)
}

func msgsMove(be module.Storage, ctx *cli.Context) error {
	if ctx.Bool("y,yes") || !clitools.Confirmation("Currently, it is unsafe to remove messages from mailboxes used by connected clients, continue?", false) {
		return errors.New("Cancelled")
	}

	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	srcName := ctx.Args().Get(1)
	if srcName == "" {
		return errors.New("Error: SRCMAILBOX is required")
	}
	seqset := ctx.Args().Get(2)
	if seqset == "" {
		return errors.New("Error: SEQSET is required")
	}
	tgtName := ctx.Args().Get(3)
	if tgtName == "" {
		return errors.New("Error: TGTMAILBOX is required")
	}

	seq, err := imap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	_, srcMbox, err := u.GetMailbox(srcName, true, nil)
	if err != nil {
		return err
	}

	moveMbox := srcMbox.(*imapsql.Mailbox)

	return moveMbox.MoveMessages(ctx.Bool("uid"), seq, tgtName)
}

func msgsList(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	mboxName := ctx.Args().Get(1)
	if mboxName == "" {
		return errors.New("Error: MAILBOX is required")
	}
	seqset := ctx.Args().Get(2)
	if seqset == "" {
		seqset = "1:*"
	}

	seq, err := imap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	_, mbox, err := u.GetMailbox(mboxName, true, nil)
	if err != nil {
		return err
	}

	ch := make(chan *imap.Message, 10)
	go func() {
		err = mbox.ListMessages(ctx.Bool("uid"), seq, []imap.FetchItem{imap.FetchEnvelope, imap.FetchInternalDate, imap.FetchRFC822Size, imap.FetchFlags, imap.FetchUid}, ch)
	}()

	for msg := range ch {
		if !ctx.Bool("full") {
			fmt.Printf("UID %d: %s - %s\n  %v, %v\n\n", msg.Uid, FormatAddressList(msg.Envelope.From), msg.Envelope.Subject, msg.Flags, msg.Envelope.Date)
			continue
		}

		fmt.Println("- Server meta-data:")
		fmt.Println("UID:", msg.Uid)
		fmt.Println("Sequence number:", msg.SeqNum)
		fmt.Println("Flags:", msg.Flags)
		fmt.Println("Body size:", msg.Size)
		fmt.Println("Internal date:", msg.InternalDate.Unix(), msg.InternalDate)
		fmt.Println("- Envelope:")
		if len(msg.Envelope.From) != 0 {
			fmt.Println("From:", FormatAddressList(msg.Envelope.From))
		}
		if len(msg.Envelope.To) != 0 {
			fmt.Println("To:", FormatAddressList(msg.Envelope.To))
		}
		if len(msg.Envelope.Cc) != 0 {
			fmt.Println("CC:", FormatAddressList(msg.Envelope.Cc))
		}
		if len(msg.Envelope.Bcc) != 0 {
			fmt.Println("BCC:", FormatAddressList(msg.Envelope.Bcc))
		}
		if msg.Envelope.InReplyTo != "" {
			fmt.Println("In-Reply-To:", msg.Envelope.InReplyTo)
		}
		if msg.Envelope.MessageId != "" {
			fmt.Println("Message-Id:", msg.Envelope.MessageId)
		}
		if !msg.Envelope.Date.IsZero() {
			fmt.Println("Date:", msg.Envelope.Date.Unix(), msg.Envelope.Date)
		}
		if msg.Envelope.Subject != "" {
			fmt.Println("Subject:", msg.Envelope.Subject)
		}
		fmt.Println()
	}
	return err
}

func msgsDump(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	mboxName := ctx.Args().Get(1)
	if mboxName == "" {
		return errors.New("Error: MAILBOX is required")
	}
	seqset := ctx.Args().Get(2)
	if seqset == "" {
		seqset = "*"
	}

	seq, err := imap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	_, mbox, err := u.GetMailbox(mboxName, true, nil)
	if err != nil {
		return err
	}

	ch := make(chan *imap.Message, 10)
	go func() {
		err = mbox.ListMessages(ctx.Bool("uid"), seq, []imap.FetchItem{imap.FetchRFC822}, ch)
	}()

	for msg := range ch {
		for _, v := range msg.Body {
			if _, err := io.Copy(os.Stdout, v); err != nil {
				return err
			}
		}
	}
	return err
}

func msgsFlags(be module.Storage, ctx *cli.Context) error {
	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return errors.New("Error: MAILBOX is required")
	}
	seqStr := ctx.Args().Get(2)
	if seqStr == "" {
		return errors.New("Error: SEQ is required")
	}

	seq, err := imap.ParseSeqSet(seqStr)
	if err != nil {
		return err
	}

	u, err := be.GetIMAPAcct(username)
	if err != nil {
		return err
	}

	_, mbox, err := u.GetMailbox(name, true, nil)
	if err != nil {
		return err
	}

	flags := ctx.Args()[3:]
	if len(flags) == 0 {
		return errors.New("Error: at least once FLAG is required")
	}

	var op imap.FlagsOp
	switch ctx.Command.Name {
	case "add-flags":
		op = imap.AddFlags
	case "rem-flags":
		op = imap.RemoveFlags
	case "set-flags":
		op = imap.SetFlags
	default:
		panic("unknown command: " + ctx.Command.Name)
	}

	return mbox.UpdateMessagesFlags(ctx.IsSet("uid"), seq, op, true, flags)
}
