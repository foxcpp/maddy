#!/bin/bash

REQUIRED_GOVERSION=1.13.0

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
if [ "$CONFPATH" == "" ]; then
    CONFPATH=/etc/maddy/maddy.conf
fi
if [ "$SUDO" == "" ]; then
    SUDO=$SUDO
fi

export CGO_CFLAGS="-g -O2 -D_FORTIFY_SOURCE=2 $CFLAGS"
export CGO_CXXFLAGS="-g -O2 -D_FORTIFY_SOURCE=2 $CXXFLAGS"
export LDFLAGS="-Wl,-z,relro,-z,now $LDFLAGS"
export CGO_LDFLAGS=$LDFLAGS

set -euo pipefail
IFS=$'\n'

ensure_go_toolchain() {
    if ! which go >/dev/null; then
        download=1
    else
        SYSGOVERSION=`go version | grep -Po "([0-9]+\.){2}[0-9]+"`
        SYSGOMAJOR=`cut -f1 -d. <<<$SYSGOVERSION`
        SYSGOMINOR=`cut -f2 -d. <<<$SYSGOVERSION`
        SYSGOPATCH=`cut -f3 -d. <<<$SYSGOVERSION`
        WANTEDGOMAJOR=`cut -f1 -d. <<<$REQUIRED_GOVERSION`
        WANTEDGOMINOR=`cut -f2 -d. <<<$REQUIRED_GOVERSION`
        WANTEDGOPATCH=`cut -f3 -d. <<<$REQUIRED_GOVERSION`

        downloadgo=0
        if [ $SYSGOMAJOR -ne $WANTEDGOMAJOR ]; then
            downloadgo=1
        fi
        if [ $SYSGOMINOR -lt $WANTEDGOMINOR ]; then
            downloadgo=1
        fi
        if [ $SYSGOPATCH -lt $WANTEDGOPATCH ]; then
            downloadgo=1
        fi

        if [ $downloadgo -eq 0 ]; then
            echo "Using system Go toolchain ($SYSGOVERSION, `which go`)." >&2
        fi
    fi

    if [ $downloadgo -eq 1 ]; then
        echo "Downloading Go $GOVERSION toolchain..." >&2
        if ! [ -e go$GOVERSION ]; then
            if ! [ -e go$GOVERSION.linux-amd64.tar.gz ]; then
                wget -q 'https://dl.google.com/go/go1.13.3.linux-amd64.tar.gz'
            fi
            tar xf go$GOVERSION.linux-amd64.tar.gz
            mv go go$GOVERSION
        fi
        export GOROOT=$PWD/go$GOVERSION
        export PATH=go$GOVERSION/bin:$PATH
    fi
}

download_and_compile() {
    export GOPATH="$PWD/gopath"
    export GOBIN="$GOPATH/bin"

    echo 'Downloading and compiling maddy...' >&2

    export GO111MODULE=on

    go get -trimpath -buildmode=pie -ldflags "-extldflags $LDFLAGS" github.com/foxcpp/maddy/cmd/{maddy,maddyctl}@$MADDYVERSION
}

install_executables() {
    echo 'Installing maddy...' >&2

    $SUDO mkdir -p "$PREFIX/bin"
    $SUDO cp --remove-destination "$GOPATH/bin/maddy" "$GOPATH/bin/maddyctl" "$PREFIX/bin/"
}

install_systemd() {
    echo 'Downloading and installing systemd unit files...' >&2

    wget -q "https://raw.githubusercontent.com/foxcpp/maddy/$MADDYVERSION/dist/systemd/maddy.service" -O maddy.service
    wget -q "https://raw.githubusercontent.com/foxcpp/maddy/$MADDYVERSION/dist/systemd/maddy@.service" -O maddy@.service

    sed -Ei "s!/usr/bin!$PREFIX/bin!g" maddy.service maddy@.service

    $SUDO mkdir -p "$SYSTEMDUNITS/system/"
    $SUDO cp maddy.service maddy@.service "$SYSTEMDUNITS/system/"
    $SUDO systemctl daemon-reload
}

create_user() {
    echo 'Creating maddy user and group...' >&2

    $SUDO useradd -UMr -s /sbin/nologin maddy || true
}

install_config() {
    echo 'Using configuration path:' $CONFPATH
    if ! [ -e "$CONFPATH" ]; then
        echo 'Downloading and installing default configuration...' >&2

        wget -q "https://raw.githubusercontent.com/foxcpp/maddy/$MADDYVERSION/maddy.conf" -O maddy.conf
        $SUDO mkdir -p /etc/maddy/

        host=`hostname`
        read -p "What's your domain, btw? [$host] > " DOMAIN
        if [ "$DOMAIN" = "" ]; then
            DOMAIN=$host
        fi
        echo 'Good, I will put that into configuration for you.' >&2

        sed -Ei "s/^\\$\\(primary_domain\) = .+$/$\(primary_domain\) = $DOMAIN/" maddy.conf
        sed -Ei "s/^\\$\\(hostname\) = .+$/$\(hostname\) = $DOMAIN/" maddy.conf

        $SUDO cp maddy.conf $CONFPATH
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
    install_systemd
    install_config
    create_user

    echo "Okay, almost ready." >&2
    echo "It's up to you to figure out TLS certificates and DNS stuff, though." >&2
    echo "Here is the tutorial to help you:" >&2
    echo "https://github.com/foxcpp/maddy/wiki/Tutorial:-Setting-up-a-mail-server-with-maddy" >&2
}

run
