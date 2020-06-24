#!/bin/bash

DESTDIR=$DESTDIR
if [ -z "$PREFIX" ]; then
    PREFIX=/usr/local
fi
if [ -z "$FAIL2BANDIR" ]; then
    FAIL2BANDIR=/etc/fail2ban
fi
if [ -z "$CONFDIR" ]; then
    CONFDIR=/etc/maddy
fi

script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd $script_dir

install -Dm 0644 -t "$DESTDIR/$PREFIX/share/vim/vimfiles/ftdetect/" vim/ftdetect/maddy-conf.vim
install -Dm 0644 -t "$DESTDIR/$PREFIX/share/vim/vimfiles/ftplugin/" vim/ftplugin/maddy-conf.vim
install -Dm 0644 -t "$DESTDIR/$PREFIX/share/vim/vimfiles/syntax/" vim/syntax/maddy-conf.vim

install -Dm 0644 -t "$DESTDIR/$FAIL2BANDIR/jail.d/" fail2ban/jail.d/*
install -Dm 0644 -t "$DESTDIR/$FAIL2BANDIR/filter.d/" fail2ban/filter.d/*

install -Dm 0644 -t "$DESTDIR/$PREFIX/lib/systemd/system/" systemd/maddy.service systemd/maddy@.service
