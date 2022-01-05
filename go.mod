module github.com/foxcpp/maddy

go 1.14

require (
	blitiri.com.ar/go/spf v1.2.0
	github.com/GehirnInc/crypt v0.0.0-20200316065508-bb7000b8a962
	github.com/caddyserver/certmagic v0.14.1
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/emersion/go-imap v1.0.6
	github.com/emersion/go-imap-compress v0.0.0-20201103190257-14809af1d1b9
	github.com/emersion/go-imap-sortthread v1.2.0
	github.com/emersion/go-imap-specialuse v0.0.0-20161227184202-ba031ced6a62 // indirect
	github.com/emersion/go-message v0.15.0
	github.com/emersion/go-milter v0.3.2
	github.com/emersion/go-msgauth v0.6.5
	github.com/emersion/go-sasl v0.0.0-20200509203442-7bfe0ed36a21
	github.com/emersion/go-smtp v0.15.1-0.20210705155248-26eb4814e227
	github.com/foxcpp/go-dovecot-sasl v0.0.0-20200522223722-c4699d7a24bf
	github.com/foxcpp/go-imap-backend-tests v0.0.0-20220105184719-e80aa29a5e16
	github.com/foxcpp/go-imap-i18nlevel v0.0.0-20200208001533-d6ec88553005
	github.com/foxcpp/go-imap-mess v0.0.0-20220105225909-b3469f4a4315
	github.com/foxcpp/go-imap-namespace v0.0.0-20200802091432-08496dd8e0ed
	github.com/foxcpp/go-imap-sql v0.5.1-0.20220105233636-946daf36ce81
	github.com/foxcpp/go-mockdns v0.0.0-20201212160233-ede2f9158d15
	github.com/foxcpp/go-mtasts v0.0.0-20191219193356-62bc3f1f74b8
	github.com/go-ldap/ldap/v3 v3.3.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/google/uuid v1.2.0
	github.com/johannesboyne/gofakes3 v0.0.0-20210704111953-6a9f95c2941c
	github.com/klauspost/compress v1.11.13 // indirect
	github.com/lib/pq v1.10.0
	github.com/libdns/alidns v1.0.2
	github.com/libdns/cloudflare v0.1.0
	github.com/libdns/digitalocean v0.0.0-20210310230526-186c4ebd2215
	github.com/libdns/gandi v1.0.2
	github.com/libdns/googleclouddns v1.0.1
	github.com/libdns/hetzner v0.0.1
	github.com/libdns/leaseweb v0.2.1
	github.com/libdns/libdns v0.2.1
	github.com/libdns/metaname v0.3.0
	github.com/libdns/namedotcom v0.3.3
	github.com/libdns/route53 v1.1.1
	github.com/libdns/vultr v0.0.0-20201128180404-1d5ee21ea62f
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/miekg/dns v1.1.42
	github.com/minio/minio-go/v7 v7.0.12
	github.com/pierrec/lz4 v2.6.0+incompatible // indirect
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.20.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/urfave/cli v1.22.5
	go.uber.org/zap v1.17.0
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/text v0.3.7
)

replace github.com/emersion/go-imap => github.com/foxcpp/go-imap v1.0.0-beta.1.0.20220105164802-1e767d4cfd62
