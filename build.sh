#!/bin/sh

destdir=/
builddir="$PWD/build"
prefix=/usr/local
version=
static=0
goflags="-trimpath" # set some flags to avoid passing "" to go

print_help() {
	cat >&2 <<EOF
Usage:
	./build.sh [options] {build,install}

Script to build, package or install Maddy Mail Server.

Options:
    -h, --help              guess!
    --builddir              directory to build in (default: $builddir)

Options for ./build.sh build:
    --static                build static self-contained executables (musl-libc recommended)
    --version <version>     version tag to embed into executables (default: auto-detect)

Options for ./build.sh install:
    --prefix <path>         installation prefix (default: $prefix)
    --destdir <path>        system root (default: $destdir)
EOF
}

while :; do
	case "$1" in
		-h|--help)
		   print_help
		   exit
		   ;;
		--builddir)
		   shift
		   builddir="$1"
		   ;;
		--prefix)
		   shift
		   prefix="$1"
		   ;;
		--destdir)
			shift
			destdir="$1"
			;;
		--version)
			shift
			version="$1"
			;;
		--static)
			static=1
			;;
		--)
			break
			shift
			;;
		-?*)
			echo "Unknown option: ${1}. See --help." >&2
			exit 2
			;;
		*)
			break
	esac
	shift
done


if [ "$version" = "" ]; then
	version=unknown
	if [ -e .version ]; then
		version="$(cat .version)"
	fi
	if [ -e .git ] && command -v git 2>/dev/null >/dev/null; then
		version="${version}+$(git rev-parse --short HEAD)"
	fi
fi

set -e

build_man_pages() {
	set +e
	if ! command -v scdoc >/dev/null 2>/dev/null; then
		echo '--- No scdoc utility found. Skipping man pages building.' >&2
		set -e
		return
	fi
	set -e

	echo '-- Building man pages...' >&2

	mkdir -p "${builddir}"/man/man1 "${builddir}"/man/man5

	for f in ./docs/man/*.1.scd; do
		scdoc < "$f" > "${builddir}/man/$(basename "$f" .scd)"
	done
	for f in ./docs/man/*.5.scd; do
		scdoc < "$f" > "${builddir}/man/$(basename "$f" .scd)"
	done
}

build() {
	mkdir -p "${builddir}"
	echo "-- Version: ${version}" >&2
	if [ "$(go env CC)" = "" ]; then
        echo '-- [!] WARNING: No C compiler available. maddy will be built without SQLite3 support and default configuration will be unusable.' >&2
    fi

	if [ "$static" -eq 1 ]; then
		echo "-- Building main server executable..." >&2
		# This is literally impossible to specify this line of arguments as part of ${goflags}
		# using only POSIX sh features (and even with Bash extensions I can't figure it out).
		go build ${goflags} -trimpath \
			-buildmode pie -tags 'osusergo netgo static_build' -ldflags '-extldflags="-fnoPIC -static"' \
			-ldflags="-X \"github.com/foxcpp/maddy.Version=${version}\"" -o "${builddir}/maddy" ./cmd/maddy
		echo "-- Building management utility (maddyctl)..." >&2
		go build ${goflags} -trimpath \
			-buildmode pie -tags 'osusergo netgo static_build' -ldflags '-extldflags="-fnoPIC -static"' \
			-ldflags="-X \"github.com/foxcpp/maddy.Version=${version}\"" -o "${builddir}/maddyctl" ./cmd/maddyctl
	else
		echo "-- Building maddy" >&2
		go build ${goflags} -trimpath -ldflags="-X \"github.com/foxcpp/maddy.Version=${version}\"" -o "${builddir}/maddy" ./cmd/maddy
		echo "-- Building maddyctl" >&2
		go build ${goflags} -trimpath -ldflags="-X \"github.com/foxcpp/maddy.Version=${version}\"" -o "${builddir}/maddyctl" ./cmd/maddyctl
	fi

	build_man_pages

	echo "-- Copying misc files..." >&2

	mkdir -p "${builddir}/systemd"
	command install -Dm 0644 -t "${builddir}/systemd" dist/systemd/*

	command install -Dm 0644 -t "${builddir}/" maddy.conf
}

install() {
	command install -Dm 0755 "${builddir}/maddy" "${destdir}/${prefix}/bin/maddy"
	command install -Dm 0755 "${builddir}/maddyctl" "${destdir}/${prefix}/bin/maddyctl"
	for f in "${builddir}"/man/*.1; do
		command install -Dm 0644 "$f" "${destdir}/${prefix}/share/man/man1/$(basename "$f")"
	done
	for f in "${builddir}"/man/*.5; do
		command install -Dm 0644 "$f" "${destdir}/${prefix}/share/man/man5/$(basename "$f")"
	done
	command install -Dm 0644 ./maddy.conf "${destdir}/etc/maddy/maddy.conf"
	command install -Dm 0644 -t "${destdir}/${prefix}/lib/systemd/system/" "${builddir}"/systemd/*.service
}

# Old build.sh compatibility
install_pkg() {
	echo '-- [!] Replace install_pkg with just install in build.sh invocation' >&2
	install
}
package() {
	echo '-- [!] Replace package with build in build.sh invocation' >&2
	build
}

if [ $# -eq 0 ]; then
	build
else
	for arg in "$@"; do
		eval "$arg"
	done
fi
