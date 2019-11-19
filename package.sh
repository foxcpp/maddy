#!/bin/bash

# Build maddy using get.sh and copy all installation files into maddy-pkgdir-XXXXXXXXX directory.
# DO NOT RUN FROM THE SOURCE DIRECTORY. IT WILL BREAK.

script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

if [ "$script_dir" = "$PWD" ]; then
    echo 'Do not run package.sh from the source directory.' >&2
    exit 1
fi

if [ "$pkgdir" = "" ]; then
    pkgdir=$PWD/maddy-pkgdir-`date +%s`
    rm -rf $pkgdir
    mkdir $pkgdir
fi
export PREFIX=$pkgdir/usr FAIL2BANDIR=$pkgdir/etc/fail2ban CONFPATH=$pkgdir/etc/maddy/maddy.conf NO_RUN=1 SUDO=fakeroot
source $script_dir/get.sh

mkdir -p maddy-setup
cd maddy-setup/

function run() {
    ensure_go_toolchain
    download_and_compile
    install_executables
    install_dist
    install_man
    install_config </dev/null
}

run
