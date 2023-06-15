#!/bin/sh

destdir=/
builddir="$PWD/build"
prefix=/usr/local
version=
static=0
if [ "${GOFLAGS}" = "" ]; then
	GOFLAGS="-trimpath" # set some flags to avoid passing "" to go
fi

output_suffix=
if [[ -n "${GOOS}" ]] && [[ -n "${GOARCH}" ]]; then
  output_suffix="_${GOOS}_${GOARCH}"
fi

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
    --tags <tags>           build tags to use
    --version <version>     version tag to embed into executables (default: auto-detect)

Additional flags for "go build" can be provided using GOFLAGS environment variable.

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
		--tags)
			shift
			tags="$1"
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
		echo '-- [!] No scdoc utility found. Skipping man pages building.' >&2
		set -e
		return
	fi
	set -e

	echo '-- Building man pages...' >&2

	mkdir -p "${builddir}/man"
	for f in ./docs/man/*.1.scd; do
		scdoc < "$f" > "${builddir}/man/$(basename "$f" .scd)"
	done
}

build() {
	mkdir -p "${builddir}"
	echo "-- Version: ${version}" >&2
	if [ "$(go env CC)" = "" ]; then
        echo '-- [!] No C compiler available. maddy will be built without SQLite3 support and default configuration will be unusable.' >&2
    fi

	if [ "$static" -eq 1 ]; then
		echo "-- Building main server executable..." >&2
		# This is literally impossible to specify this line of arguments as part of ${GOFLAGS}
		# using only POSIX sh features (and even with Bash extensions I can't figure it out).
		go build -trimpath -buildmode pie -tags "$tags osusergo netgo static_build" \
			-ldflags "-extldflags '-fno-PIC -static' -X \"github.com/foxcpp/maddy.Version=${version}\"" \
			-o "${builddir}/maddy${output_suffix}" ${GOFLAGS} ./cmd/maddy
	else
		echo "-- Building main server executable..." >&2
		go build -tags "$tags" -trimpath -ldflags="-X \"github.com/foxcpp/maddy.Version=${version}\"" -o "${builddir}/maddy${output_suffix}" ${GOFLAGS} ./cmd/maddy
	fi

	build_man_pages

	echo "-- Copying misc files..." >&2

	mkdir -p "${builddir}/systemd"
	cp dist/systemd/*.service "${builddir}/systemd/"
	cp maddy.conf "${builddir}/maddy.conf"
}

install() {
	echo "-- Installing built files..." >&2

	command install -m 0755 -d "${destdir}/${prefix}/bin/"
	command install -m 0755 "${builddir}/maddy" "${destdir}/${prefix}/bin/"
	command ln -s maddy "${destdir}/${prefix}/bin/maddyctl"
	command install -m 0755 -d "${destdir}/etc/maddy/"
	command install -m 0644 ./maddy.conf "${destdir}/etc/maddy/maddy.conf"

	# Attempt to install systemd units only for Linux.
	# Check is done using GOOS instead of uname -s to account for possible
	# package cross-compilation.
	if [ "$(go env GOOS)" = "linux" ]; then
		command install -m 0755 -d "${destdir}/${prefix}/lib/systemd/system/"
		command install -m 0644 "${builddir}"/systemd/*.service "${destdir}/${prefix}/lib/systemd/system/"
	fi

	if [ -e "${builddir}"/man ]; then
		command install -m 0755 -d "${destdir}/${prefix}/share/man/man1/"
		for f in "${builddir}"/man/*.1; do
			command install -m 0644 "$f" "${destdir}/${prefix}/share/man/man1/"
		done
	fi
}

# Old build.sh compatibility
install_pkg() {
	echo "-- [!] Replace 'install_pkg' with 'install' in build.sh invocation" >&2
	install
}
package() {
	echo "-- [!] Replace 'package' with 'build' in build.sh invocation" >&2
	build
}

if [ $# -eq 0 ]; then
	build
else
	for arg in "$@"; do
		eval "$arg"
	done
fi
