#!/bin/sh
set -euo pipefail
IFS=$'\n'

GOVERSION=1.13.1
MADDYVERSION=master
PREFIX=/usr/local
SYSTEMDUNITS=/etc/systemd
CONFPATH=/etc/maddy/maddy.conf

mkdir -p maddy-setup/
cd maddy-setup/

if ! which go >/dev/null; then
    download=1
else
    if [ "`go version | grep -o "go$GOVERSION"`" = "go$GOVERSION" ]; then
        echo "Using system Go toolchain." >&2
        download=0
        GO=`which go`
    else
        download=1
    fi
fi

if [ $download -eq 1 ]; then
    echo "Downloading Go $GOVERSION toolchain..." >&2
    if ! [ -e go$GOVERSION ]; then
        if ! [ -e go$GOVERSION.linux-amd64.tar.gz ]; then
            wget -q 'https://dl.google.com/go/go1.13.3.linux-amd64.tar.gz'
        fi
        tar xf go$GOVERSION.linux-amd64.tar.gz
        mv go go$GOVERSION
    fi
    GO=go$GOVERSION/bin/go
fi

export GOPATH="$PWD/gopath"
export GOBIN="$GOPATH/bin"

echo 'Downloading and compiling maddy...' >&2

export GO111MODULE=on
$GO get github.com/foxcpp/maddy/cmd/{maddy,maddyctl}@$MADDYVERSION

echo 'Installing maddy...' >&2

sudo mkdir -p "$PREFIX/bin"
sudo cp "$GOPATH/bin/maddy" "$GOPATH/bin/maddyctl" "$PREFIX/bin/"

echo 'Downloading and installing systemd unit files...' >&2

wget -q "https://raw.githubusercontent.com/foxcpp/maddy/$MADDYVERSION/dist/systemd/maddy.service" -O maddy.service
wget -q "https://raw.githubusercontent.com/foxcpp/maddy/$MADDYVERSION/dist/systemd/maddy@.service" -O maddy@.service

sed -Ei "s!/usr/bin!$PREFIX/bin!g" maddy.service maddy@.service

sudo cp maddy.service maddy@.service "$SYSTEMDUNITS/system/"
sudo systemctl daemon-reload

echo 'Creating maddy user and group...' >&2

sudo useradd -UMr -s /sbin/nologin maddy || true

if ! [ -e "$CONFPATH" ]; then
    echo 'Downloading and installing default configuration...' >&2

    wget -q "https://raw.githubusercontent.com/foxcpp/maddy/$MADDYVERSION/maddy.conf" -O maddy.conf
    sudo mkdir -p /etc/maddy/

    host=`hostname`
    read -p "What's your domain, btw? [$host] > " DOMAIN
    if [ "$DOMAIN" = "" ]; then
        DOMAIN=$host
    fi
    echo 'Good, I will put that into configuration for you.' >&2

    sed -Ei "s/^\\$\\(primary_domain\) = .+$/$\(primary_domain\) = $DOMAIN/" maddy.conf
    sed -Ei "s/^\\$\\(hostname\) = .+$/$\(hostname\) = $DOMAIN/" maddy.conf

    sudo cp maddy.conf /etc/maddy/
else
    echo "Configuration already exists in /etc/maddy/maddy.conf, skipping defaults installation." >&2
fi

echo "Okay, almost ready." >&2
echo "It's up to you to figure out TLS certificates and DNS stuff, though." >&2
echo "Here is the tutorial to help you:" >&2
echo "https://github.com/foxcpp/maddy/wiki/Tutorial:-Setting-up-a-mail-server-with-maddy" >&2
