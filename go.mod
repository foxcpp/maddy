module github.com/foxcpp/maddy

go 1.17

require (
	blitiri.com.ar/go/spf v1.4.0
	github.com/GehirnInc/crypt v0.0.0-20200316065508-bb7000b8a962
	github.com/caddyserver/certmagic v0.16.1
	github.com/emersion/go-imap v1.2.1
	github.com/emersion/go-imap-compress v0.0.0-20201103190257-14809af1d1b9
	github.com/emersion/go-imap-sortthread v1.2.0
	github.com/emersion/go-message v0.16.0
	github.com/emersion/go-milter v0.3.3
	github.com/emersion/go-msgauth v0.6.6
	github.com/emersion/go-sasl v0.0.0-20211008083017-0b9dcfb154ac
	github.com/emersion/go-smtp v0.15.1-0.20220119142625-1c322d2783aa
	github.com/foxcpp/go-dovecot-sasl v0.0.0-20200522223722-c4699d7a24bf
	github.com/foxcpp/go-imap-backend-tests v0.0.0-20220105184719-e80aa29a5e16
	github.com/foxcpp/go-imap-i18nlevel v0.0.0-20200208001533-d6ec88553005
	github.com/foxcpp/go-imap-mess v0.0.0-20220105225909-b3469f4a4315
	github.com/foxcpp/go-imap-namespace v0.0.0-20200802091432-08496dd8e0ed
	github.com/foxcpp/go-imap-sql v0.5.1-0.20220623181604-c20be1a387b4
	github.com/foxcpp/go-mockdns v1.0.0
	github.com/foxcpp/go-mtasts v0.0.0-20191219193356-62bc3f1f74b8
	github.com/go-ldap/ldap/v3 v3.4.3
	github.com/go-sql-driver/mysql v1.6.0
	github.com/google/uuid v1.3.0
	github.com/johannesboyne/gofakes3 v0.0.0-20210704111953-6a9f95c2941c
	github.com/lib/pq v1.10.6
	github.com/libdns/alidns v1.0.2
	github.com/libdns/cloudflare v0.1.0
	github.com/libdns/digitalocean v0.0.0-20220518195853-a541bc8aa80f
	github.com/libdns/gandi v1.0.2
	github.com/libdns/googleclouddns v1.0.2
	github.com/libdns/hetzner v0.0.1
	github.com/libdns/leaseweb v0.2.1
	github.com/libdns/libdns v0.2.1
	github.com/libdns/metaname v0.3.0
	github.com/libdns/namecheap v0.0.0-20211109042440-fc7440785c8e
	github.com/libdns/namedotcom v0.3.3
	github.com/libdns/route53 v1.1.2
	github.com/libdns/vultr v0.0.0-20211122184636-cd4cb5c12e51
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/miekg/dns v1.1.50
	github.com/minio/minio-go/v7 v7.0.29
	github.com/prometheus/client_golang v1.12.2
	github.com/urfave/cli/v2 v2.10.2
	go.uber.org/zap v1.21.0
	golang.org/x/crypto v0.0.0-20220622213112-05595931fe9d
	golang.org/x/net v0.0.0-20220622184535-263ec571b305
	golang.org/x/sync v0.0.0-20220601150217-0de741cfad7f
	golang.org/x/text v0.3.7
)

require (
	cloud.google.com/go/compute v1.7.0 // indirect
	// Do not upgrade go-ntlmssp - newer version are incompatible with go-ldap.
	github.com/Azure/go-ntlmssp v0.0.0-20211209120228-48547f28849e // indirect
	github.com/aws/aws-sdk-go v1.44.40 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/digitalocean/godo v1.81.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/emersion/go-textwrapper v0.0.0-20200911093747-65d896831594 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.4 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.1.0 // indirect
	github.com/googleapis/gax-go/v2 v2.4.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.15.6 // indirect
	github.com/klauspost/cpuid/v2 v2.0.14 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/mholt/acmez v1.0.2 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.35.0 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/rs/xid v1.4.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryszard/goskiplist v0.0.0-20150312221310-2dfbae5fcf46 // indirect
	github.com/shabbyrobe/gocovmerge v0.0.0-20180507124511-f6ea450bfb63 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/vultr/govultr/v2 v2.17.2 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/oauth2 v0.0.0-20220622183110-fd043fe589d2 // indirect
	golang.org/x/sys v0.0.0-20220622161953-175b2fd9d664 // indirect
	golang.org/x/tools v0.1.11 // indirect
	google.golang.org/api v0.85.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20220622171453-ea41d75dfa0f // indirect
	google.golang.org/grpc v1.47.0 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/ini.v1 v1.66.6 // indirect
	gotest.tools v2.2.0+incompatible // indirect
)

replace github.com/emersion/go-imap => github.com/foxcpp/go-imap v1.0.0-beta.1.0.20220105164802-1e767d4cfd62
