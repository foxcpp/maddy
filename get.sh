#!/bin/bash

REQUIRED_GOVERSION=1.13.0

# This will make sure the variable safe to use after 'set -u'
# if it is not defined, the empty value is fine.
DESTDIR=$DESTDIR

if [ "$GOVERSION" == "" ]; then
    GOVERSION=1.13.4
fi
if [ "$MADDYVERSION" == "" ]; then
    MADDYVERSION=master
fi
if [ "$PREFIX" == "" ]; then
    PREFIX=/usr/local
fi
if [ "$SYSTEMDUNITS" == "" ]; then
    SYSTEMDUNITS=$PREFIX/lib/systemd
fi
if [ "$CONFDIR" == "" ]; then
    CONFDIR=/etc/maddy
fi
if [ "$SUDO" == "" ]; then
    SUDO=sudo
fi
if [ "$NO_RUN" == "" ]; then
    NO_RUN=0
fi
if [ "$HOSTNAME" == "" ]; then
    HOSTNAME=$(uname -n)
fi

export CGO_CFLAGS="-g -O2 -D_FORTIFY_SOURCE=2 $CFLAGS"
export CGO_CXXFLAGS="-g -O2 -D_FORTIFY_SOURCE=2 $CXXFLAGS"
export LDFLAGS="-Wl,-z,relro,-z,now $LDFLAGS"
export CGO_LDFLAGS=$LDFLAGS

set -euo pipefail
IFS=$'\n'

ensure_go_toolchain() {
    if ! command -v go >/dev/null; then
        downloadgo=1
    else
        SYSGOVERSION=$(go version | cut -f3 -d ' ' | grep -Po "([0-9]+\.){2}[0-9]+")
        SYSGOMAJOR=$(cut -f1 -d. <<<"$SYSGOVERSION")
        SYSGOMINOR=$(cut -f2 -d. <<<"$SYSGOVERSION")
        SYSGOPATCH=$(cut -f3 -d. <<<"$SYSGOVERSION")
        WANTEDGOMAJOR=$(cut -f1 -d. <<<$REQUIRED_GOVERSION)
        WANTEDGOMINOR=$(cut -f2 -d. <<<$REQUIRED_GOVERSION)
        WANTEDGOPATCH=$(cut -f3 -d. <<<$REQUIRED_GOVERSION)

        downloadgo=0
        if [ "$SYSGOMAJOR" -ne "$WANTEDGOMAJOR" ]; then
            downloadgo=1
        fi
        if [ "$SYSGOMINOR" -lt "$WANTEDGOMINOR" ]; then
            downloadgo=1
        fi
        if [ "$SYSGOPATCH" -lt "$WANTEDGOPATCH" ]; then
            downloadgo=1
        fi

        if [ $downloadgo -eq 0 ]; then
            echo "Using system Go toolchain ($SYSGOVERSION, $(command -v go))." >&2
        fi
    fi

    if [ $downloadgo -eq 1 ]; then
        echo "Downloading Go $GOVERSION toolchain..." >&2
        if ! [ -e go$GOVERSION ]; then
            if ! [ -e go$GOVERSION.linux-amd64.tar.gz ]; then
                wget -q "https://dl.google.com/go/go$GOVERSION.linux-amd64.tar.gz"
            fi
            tar xf go$GOVERSION.linux-amd64.tar.gz
            mv go go$GOVERSION
        fi
        export GOROOT=$PWD/go$GOVERSION
        export PATH=go$GOVERSION/bin:$PATH
    fi
}

download_and_compile() {
    export GOPATH="$PWD/gopath:$(go env GOPATH)"
    export GOBIN="$PWD/gopath/bin"

    echo 'Downloading and compiling maddy...' >&2

    export GO111MODULE=on

    go get -trimpath -buildmode=pie \
        -ldflags "-extldflags $LDFLAGS \
            -X \"github.com/foxcpp/maddy.DefaultLibexecDirectory=$PREFIX/lib/maddy\" \
            -X \"github.com/foxcpp/maddy.ConfigDirectory=$CONFDIR\"" \
        github.com/foxcpp/maddy/cmd/{maddy,maddyctl}@$MADDYVERSION
}

source_dir() {
    maddy_version_tag=$("$PWD/gopath/bin/maddy" -v | cut -f2 -d ' ')
    echo "$PWD/gopath/pkg/mod/github.com/foxcpp/maddy@$maddy_version_tag"
}

install_executables() {
    echo 'Installing maddy...' >&2

    $SUDO mkdir -p "$DESTDIR/$PREFIX/bin"
    $SUDO cp --remove-destination "$PWD/gopath/bin/maddy" "$PWD/gopath/bin/maddyctl" "$DESTDIR/$PREFIX/bin/"
}

install_dist() {
    echo 'Installing dist files...' >&2

    $SUDO bash "$(source_dir)/dist/install.sh"

    $SUDO sed -Ei "s!/usr/bin!$PREFIX/bin!g;\
        s!/usr/lib/maddy!$PREFIX/lib/maddy!g;\
        s!/etc/maddy!$CONFDIR!g" "$DESTDIR/$SYSTEMDUNITS/system/maddy.service" "$DESTDIR/$SYSTEMDUNITS/system/maddy@.service"
}

install_man() {
    set +e
    if ! command -v scdoc &>/dev/null; then
        echo 'No scdoc utility found. Skipping man pages installation.' >&2
        set -e
        return
    fi
    if ! command -v gzip &>/dev/null; then
        echo 'No gzip utility found. Skipping man pages installation.' >&2
        set -e
        return
    fi
    set -e

    echo 'Installing man pages...' >&2
    for f in "$(source_dir)"/docs/man/*.1.scd; do
        scdoc < "$f" | gzip > /tmp/maddy-tmp.gz
        $SUDO install -Dm 0644 /tmp/maddy-tmp.gz "$DESTDIR/$PREFIX/share/man/man1/$(basename -s .scd "$f").gz"
    done
    for f in "$(source_dir)"/docs/man/*.5.scd; do
        scdoc < "$f" | gzip > /tmp/maddy-tmp.gz
        $SUDO install -Dm 0644 /tmp/maddy-tmp.gz "$DESTDIR/$PREFIX/share/man/man5/$(basename -s .scd "$f").gz"
    done
    rm /tmp/maddy-tmp.gz

}

create_user() {
    echo 'Creating maddy user and group...' >&2

    $SUDO useradd -UMr -s /sbin/nologin maddy || true
}

install_config() {
    echo 'Using configuration path:' $CONFDIR/maddy.conf
    if ! [ -e "$DESTDIR/$CONFDIR/maddy.conf" ]; then
        echo 'Installing default configuration...' >&2

        install "$(source_dir)/maddy.conf" /tmp/maddy.conf

        host=$HOSTNAME
        set +e # premit to fail if stdin is /dev/null (in package.sh)
        read -rp "What's your domain, btw? [$host] > " DOMAIN
        set -e
        if [ "$DOMAIN" = "" ]; then
            DOMAIN=$host
        fi
        echo 'Good, I will put that into configuration for you.' >&2

        sed -Ei "s/^\\$\\(primary_domain\) = .+$/$\(primary_domain\) = $DOMAIN/" /tmp/maddy.conf
        sed -Ei "s/^\\$\\(hostname\) = .+$/$\(hostname\) = $DOMAIN/" /tmp/maddy.conf
        sed -Ei "s!/etc/maddy!$CONFDIR!g" /tmp/maddy.conf

        $SUDO install -Dm 0644 /tmp/maddy.conf "$DESTDIR/$CONFDIR/maddy.conf"
        rm /tmp/maddy.conf
    else
        echo "Configuration already exists in /etc/maddy/maddy.conf, skipping defaults installation." >&2
    fi
}

run() {
    mkdir -p maddy-setup/
    cd maddy-setup/

    ensure_go_toolchain
    download_and_compile
    install_executables
    install_dist
    install_man
    install_config
    create_user
    $SUDO systemctl daemon-reload

    echo "Okay, almost ready." >&2
    echo "It's up to you to figure out TLS certificates and DNS stuff, though." >&2
    echo "Here is the tutorial to help you:" >&2
    echo "https://github.com/foxcpp/maddy/wiki/Tutorial:-Setting-up-a-mail-server-with-maddy" >&2
}

if [ "$NO_RUN" != "1" ]; then
    run
fi
