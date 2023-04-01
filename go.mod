module github.com/foxcpp/maddy

go 1.18

replace github.com/emersion/go-smtp => github.com/dioad/go-smtp v0.16.0

require (
	blitiri.com.ar/go/spf v1.5.1
	github.com/GehirnInc/crypt v0.0.0-20200316065508-bb7000b8a962
	github.com/caddyserver/certmagic v0.17.2
	github.com/emersion/go-imap v1.2.2-0.20220928192137-6fac715be9cf
	github.com/emersion/go-imap-compress v0.0.0-20201103190257-14809af1d1b9
	github.com/emersion/go-imap-sortthread v1.2.0
	github.com/emersion/go-message v0.16.0
	github.com/emersion/go-milter v0.3.3
	github.com/emersion/go-msgauth v0.6.6
	github.com/emersion/go-sasl v0.0.0-20220912192320-0145f2c60ead
	github.com/emersion/go-smtp v0.16.0
	github.com/foxcpp/go-dovecot-sasl v0.0.0-20200522223722-c4699d7a24bf
	github.com/foxcpp/go-imap-backend-tests v0.0.0-20220105184719-e80aa29a5e16
	github.com/foxcpp/go-imap-i18nlevel v0.0.0-20200208001533-d6ec88553005
	github.com/foxcpp/go-imap-mess v0.0.0-20230108134257-b7ec3a649613
	github.com/foxcpp/go-imap-namespace v0.0.0-20200802091432-08496dd8e0ed
	github.com/foxcpp/go-imap-sql v0.5.1-0.20230313080458-c0176dad679c
	github.com/foxcpp/go-mockdns v1.0.0
	github.com/foxcpp/go-mtasts v0.0.0-20191219193356-62bc3f1f74b8
	github.com/go-ldap/ldap/v3 v3.4.4
	github.com/go-sql-driver/mysql v1.7.0
	github.com/google/uuid v1.3.0
	github.com/hashicorp/go-hclog v1.4.0
	github.com/johannesboyne/gofakes3 v0.0.0-20210704111953-6a9f95c2941c
	github.com/lib/pq v1.10.6
	github.com/libdns/alidns v1.0.3-0.20220501125541-4a895238a95d
	github.com/libdns/cloudflare v0.1.1-0.20221006221909-9d3ab3c3cddd
	github.com/libdns/digitalocean v0.0.0-20220518195853-a541bc8aa80f
	github.com/libdns/gandi v1.0.3-0.20220921161957-dcd0274d2c79
	github.com/libdns/googleclouddns v1.1.0
	github.com/libdns/hetzner v0.0.1
	github.com/libdns/leaseweb v0.3.1
	github.com/libdns/libdns v0.2.2-0.20221006221142-3ef90aee33fd
	github.com/libdns/metaname v0.3.0
	github.com/libdns/namecheap v0.0.0-20211109042440-fc7440785c8e
	github.com/libdns/namedotcom v0.3.3
	github.com/libdns/route53 v1.3.0
	github.com/libdns/vultr v0.0.0-20220906182619-5ea9da3d9625
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/miekg/dns v1.1.50
	github.com/minio/minio-go/v7 v7.0.47
	github.com/netauth/netauth v0.6.2-0.20220831214440-1df568cd25d6
	github.com/prometheus/client_golang v1.14.0
	github.com/urfave/cli/v2 v2.24.3
	go.uber.org/zap v1.24.0
	golang.org/x/crypto v0.5.0
	golang.org/x/net v0.7.0
	golang.org/x/sync v0.1.0
	golang.org/x/text v0.7.0
)

require (
	cloud.google.com/go/compute v1.18.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	github.com/Azure/go-ntlmssp v0.0.0-20221128193559-754e69321358 // indirect
	github.com/aws/aws-sdk-go v1.44.40 // indirect
	github.com/aws/aws-sdk-go-v2 v1.17.4 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.18.12 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.13.12 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.22 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.28 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.22 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.29 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.22 // indirect
	github.com/aws/aws-sdk-go-v2/service/route53 v1.27.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.12.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.14.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.18.3 // indirect
	github.com/aws/smithy-go v1.13.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/digitalocean/godo v1.96.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/emersion/go-textwrapper v0.0.0-20200911093747-65d896831594 // indirect
	github.com/fatih/color v1.14.1 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.4 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.1 // indirect
	github.com/googleapis/gax-go/v2 v2.7.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.2 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/klauspost/cpuid/v2 v2.2.3 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mholt/acmez v1.0.4 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/netauth/protocol v0.0.0-20210918062754-7fee492ffcbd // indirect
	github.com/pelletier/go-toml/v2 v2.0.6 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.39.0 // indirect
	github.com/prometheus/procfs v0.9.0 // indirect
	github.com/rs/xid v1.4.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryszard/goskiplist v0.0.0-20150312221310-2dfbae5fcf46 // indirect
	github.com/shabbyrobe/gocovmerge v0.0.0-20180507124511-f6ea450bfb63 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/spf13/afero v1.9.3 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.15.0 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	github.com/vultr/govultr/v2 v2.17.2 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	golang.org/x/mod v0.7.0 // indirect
	golang.org/x/oauth2 v0.4.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.5.0 // indirect
	google.golang.org/api v0.109.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230202175211-008b39050e57 // indirect
	google.golang.org/grpc v1.52.3 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gotest.tools v2.2.0+incompatible // indirect
)

replace github.com/emersion/go-imap => github.com/foxcpp/go-imap v1.0.0-beta.1.0.20220623182312-df940c324887
