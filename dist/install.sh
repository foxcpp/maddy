#!/bin/bash

if [ -z "$PREFIX" ]; then
    PREFIX=/usr/local
fi
if [ -z "$FAIL2BANDIR" ]; then
    FAIL2BANDIR=/etc/fail2ban
fi

install -Dm 0644 -t "$PREFIX/share/vim/vimfiles/ftdetect/" vim/ftdetect/maddy-conf.vim
install -Dm 0644 -t "$PREFIX/share/vim/vimfiles/ftplugin/" vim/ftplugin/maddy-conf.vim
install -Dm 0644 -t "$PREFIX/share/vim/vimfiles/syntax/" vim/syntax/maddy-conf.vim

install -Dm 0644 -t "$FAIL2BANDIR/jail.d/" fail2ban/jail.d/*
install -Dm 0644 -t "$FAIL2BANDIR/filter.d/" fail2ban/filter.d/*

install -Dm 0644 -t "$PREFIX/lib/systemd/system/" systemd/maddy.service systemd/maddy@.service
