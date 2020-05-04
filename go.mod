module github.com/foxcpp/maddy

go 1.13

require (
	blitiri.com.ar/go/spf v0.0.0-20191018194539-a683815bdae8
	github.com/GehirnInc/crypt v0.0.0-20200316065508-bb7000b8a962
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/emersion/go-imap v1.0.4
	github.com/emersion/go-imap-appendlimit v0.0.0-20190308131241-25671c986a6a
	github.com/emersion/go-imap-compress v0.0.0-20170105185004-f036eda44681
	github.com/emersion/go-imap-idle v0.0.0-20190519112320-2704abd7050e
	github.com/emersion/go-imap-move v0.0.0-20190710073258-6e5a51a5b342
	github.com/emersion/go-imap-specialuse v0.0.0-20161227184202-ba031ced6a62
	github.com/emersion/go-imap-unselect v0.0.0-20171113212723-b985794e5f26
	github.com/emersion/go-message v0.11.3-0.20200429151259-c5125629c3f8
	github.com/emersion/go-milter v0.1.0
	github.com/emersion/go-msgauth v0.4.1-0.20200429175443-e4c87369d72f
	github.com/emersion/go-sasl v0.0.0-20191210011802-430746ea8b9b
	github.com/emersion/go-smtp v0.12.2-0.20200219094142-f9be832b5554
	github.com/foxcpp/go-dovecot-sasl v0.0.0-20200504194015-e35592c01a2c
	github.com/foxcpp/go-imap-i18nlevel v0.0.0-20200208001533-d6ec88553005
	github.com/foxcpp/go-imap-sql v0.4.1-0.20200426175844-c3172a53940a
	github.com/foxcpp/go-mockdns v0.0.0-20200503193630-ff72b88723f2
	github.com/foxcpp/go-mtasts v0.0.0-20191219193356-62bc3f1f74b8
	github.com/go-sql-driver/mysql v1.5.0
	github.com/google/uuid v1.1.1
	github.com/lib/pq v1.4.0
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/miekg/dns v1.1.29
	github.com/pkg/errors v0.9.1 // indirect
	github.com/stretchr/testify v1.4.0 // indirect
	github.com/urfave/cli v1.22.4
	golang.org/x/crypto v0.0.0-20200423211502-4bdfaf469ed5
	golang.org/x/net v0.0.0-20200425230154-ff2c4b7c35a0
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	golang.org/x/sys v0.0.0-20200420163511-1957bb5e6d1f // indirect
	golang.org/x/text v0.3.2
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

replace github.com/emersion/go-milter => github.com/foxcpp/go-milter v0.1.1-0.20200502214548-312001df0b51
