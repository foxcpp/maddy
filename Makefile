SHELL := bash
#.SHELLFLAGS := -eu -o pipefail
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

# Install configuration
DESTDIR ?= /
PREFIX ?= /usr/local
SYSTEMDUNITS ?= $(PREFIX)/lib/systemd
FAIL2BANDIR ?= /etc/fail2ban
HAVE_SCDOC = $(shell command -v scdoc | grep -c "scdoc" || true)

# Build configuration
TAGS ?= ""
LDFLAGS ?= "-Wl,-z,relro,-z,now"
CGO_LDFLAGS ?= $(LDFLAGS)

# Compiled into maddy{,ctl} executables.
MADDY_VER ?= $(shell ./git-version.sh)
CONFIGDIR ?= /etc/maddy
STATEDIR ?= /var/lib/maddy
RUNTIMEDIR ?= /run/maddy

.PHONY: all test check install man-pages clean

all: cmd/maddy/maddy cmd/maddyctl/maddyctl man-pages

man-pages:
	@$(MAKE) -s $(shell find docs/man -name '*.scd' | sed 's/.scd//')

# Manual pages
%.1: %.1.scd
	@echo '-- Generating $<'
	@scdoc < $< > $@

%.5: %.5.scd
	@echo '-- Generating $<'
	@scdoc < $< > $@

cmd/maddy/maddy: $(shell find -name '*.go' -or -name '*.c' -or -name '*.h')
	@echo '-- Building maddy...'
	@go build -trimpath -buildmode=pie -tags "$(TAGS)" \
		-ldflags "-extldflags \"$(LDFLAGS)\" \
		    -X \"github.com/foxcpp/maddy.DefaultLibexecDirectory=$(PREFIX)/lib/maddy\" \
		    -X \"github.com/foxcpp/maddy.DefaultStateDirectory=$(STATEDIR)\" \
		    -X \"github.com/foxcpp/maddy.DefaultRuntimeDirectory=$(RUNTIMEDIR)\" \
		    -X \"github.com/foxcpp/maddy.ConfigDirectory=$(CONFIGDIR)\" \
		    -X \"github.com/foxcpp/maddy.Version=$(MADDY_VER)\"" \
		-o $@ ./cmd/maddy

cmd/maddyctl/maddyctl: $(shell find -name '*.go' -or -name '*.c' -or -name '*.h')
	@echo '-- Building maddyctl...'
	@go build -trimpath -buildmode=pie -tags "$(TAGS)" \
		-ldflags "-extldflags \"$(LDFLAGS)\" \
		    -X \"github.com/foxcpp/maddy.DefaultLibexecDirectory=$(PREFIX)/lib/maddy\" \
		    -X \"github.com/foxcpp/maddy.DefaultStateDirectory=$(STATEDIR)\" \
		    -X \"github.com/foxcpp/maddy.DefaultRuntimeDirectory=$(RUNTIMEDIR)\" \
		    -X \"github.com/foxcpp/maddy.ConfigDirectory=$(CONFIGDIR)\" \
		    -X \"github.com/foxcpp/maddy.Version=$(MADDY_VER)\"" \
		-o $@ ./cmd/maddyctl

lint:
	@golangci-lint run

check:
	@echo '-- Running unit tests...'
	@go test -count 3 -race ./...
	@echo '-- Running integration tests...'
	@cd tests && ./run.sh

install: cmd/maddy/maddy cmd/maddyctl/maddyctl man-pages
	install -Dm 0755 -t "$(DESTDIR)/$(PREFIX)/bin" cmd/maddy/maddy cmd/maddyctl/maddyctl
ifeq ($(strip $(HAVE_SCDOC)),1)
	install -Dm 0644 -t "$(DESTDIR)/$(PREFIX)/share/man/man1" docs/man/*.1
	install -Dm 0644 -t "$(DESTDIR)/$(PREFIX)/share/man/man5" docs/man/*.5
endif
	install -Dm 0644 -t "$(DESTDIR)/$(PREFIX)/share/vim/vimfiles/ftdetect/" dist/vim/ftdetect/*.vim
	install -Dm 0644 -t "$(DESTDIR)/$(PREFIX)/share/vim/vimfiles/ftplugin/" dist/vim/ftplugin/*.vim
	install -Dm 0644 -t "$(DESTDIR)/$(PREFIX)/share/vim/vimfiles/syntax/" dist/vim/syntax/*.vim
	install -Dm 0644 -t "$(DESTDIR)/$(FAIL2BANDIR)/jail.d/" dist/fail2ban/jail.d/*
	install -Dm 0644 -t "$(DESTDIR)/$(FAIL2BANDIR)/filter.d/" dist/fail2ban/filter.d/*
	install -Dm 0644 -t "$(DESTDIR)/$(SYSTEMDUNITS)/system/" dist/systemd/*
	@sed -Ei "s!/usr/bin!$(PREFIX)/bin!g;\
    	s!/usr/lib/maddy!$(PREFIX)/lib/maddy!g;\
    	s!/etc/maddy!$(CONFIGDIR)!g" "$(DESTDIR)/$(SYSTEMDUNITS)/system/"*.service
	install -Dm 0644 -t "$(DESTDIR)/$(CONFIGDIR)/" maddy.conf

clean:
	@rm -f cmd/maddy/maddy cmd/maddyctl/maddyctl
	@rm -f docs/man/*.1 docs/man/*.5
