package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	eimap "github.com/emersion/go-imap"
	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/urfave/cli"
)

func msgsAdd(ctx *cli.Context) error {
	if err := connectToDB(ctx); err != nil {
		return err
	}

	if !ctx.GlobalBool("unsafe") {
		return errors.New("Error: Refusing to edit mailboxes without --unsafe")
	}

	username := ctx.Args().First()
	if username == "" {
		return errors.New("Error: USERNAME is required")
	}
	name := ctx.Args().Get(1)
	if name == "" {
		return errors.New("Error: MAILBOX is required")
	}

	u, err := backend.GetUser(username)
	if err != nil {
		return err
	}

	mbox, err := u.GetMailbox(name)
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

	status, err := mbox.Status([]eimap.StatusItem{eimap.StatusUidNext})
	if err != nil {
		return err
	}

	if err := mbox.CreateMessage(flags, date, &buf); err != nil {
		return err
	}

	fmt.Println(status.UidNext)

	return nil
}

func msgsRemove(ctx *cli.Context) error {
	if err := connectToDB(ctx); err != nil {
		return err
	}

	if !ctx.GlobalBool("unsafe") {
		return errors.New("Error: Refusing to edit mailboxes without --unsafe")
	}

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

	seq, err := eimap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := backend.GetUser(username)
	if err != nil {
		return err
	}

	mbox, err := u.GetMailbox(name)
	if err != nil {
		return err
	}

	if !ctx.Bool("yes") {
		if !Confirmation("Are you sure you want to delete these messages?", false) {
			return errors.New("Cancelled")
		}
	}

	mboxB := mbox.(*imapsql.Mailbox)
	if err := mboxB.DelMessages(ctx.Bool("uid"), seq); err != nil {
		return err
	}

	return nil
}

func msgsCopy(ctx *cli.Context) error {
	if err := connectToDB(ctx); err != nil {
		return err
	}

	if !ctx.GlobalBool("unsafe") {
		return errors.New("Error: Refusing to edit mailboxes without --unsafe")
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

	seq, err := eimap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := backend.GetUser(username)
	if err != nil {
		return err
	}

	srcMbox, err := u.GetMailbox(srcName)
	if err != nil {
		return err
	}

	return srcMbox.CopyMessages(ctx.Bool("uid"), seq, tgtName)
}

func msgsMove(ctx *cli.Context) error {
	if err := connectToDB(ctx); err != nil {
		return err
	}

	if !ctx.GlobalBool("unsafe") {
		return errors.New("Error: Refusing to edit mailboxes without --unsafe")
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

	seq, err := eimap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := backend.GetUser(username)
	if err != nil {
		return err
	}

	srcMbox, err := u.GetMailbox(srcName)
	if err != nil {
		return err
	}

	moveMbox := srcMbox.(*imapsql.Mailbox)

	return moveMbox.MoveMessages(ctx.Bool("uid"), seq, tgtName)
}

func msgsList(ctx *cli.Context) error {
	if err := connectToDB(ctx); err != nil {
		return err
	}

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

	seq, err := eimap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := backend.GetUser(username)
	if err != nil {
		return err
	}

	mbox, err := u.GetMailbox(mboxName)
	if err != nil {
		return err
	}

	ch := make(chan *eimap.Message, 10)
	go func() {
		err = mbox.ListMessages(ctx.Bool("uid"), seq, []eimap.FetchItem{eimap.FetchEnvelope, eimap.FetchInternalDate, eimap.FetchRFC822Size, eimap.FetchFlags, eimap.FetchUid}, ch)
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

func msgsDump(ctx *cli.Context) error {
	if err := connectToDB(ctx); err != nil {
		return err
	}

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

	seq, err := eimap.ParseSeqSet(seqset)
	if err != nil {
		return err
	}

	u, err := backend.GetUser(username)
	if err != nil {
		return err
	}

	mbox, err := u.GetMailbox(mboxName)
	if err != nil {
		return err
	}

	ch := make(chan *eimap.Message, 10)
	go func() {
		err = mbox.ListMessages(ctx.Bool("uid"), seq, []eimap.FetchItem{eimap.FetchRFC822}, ch)
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
