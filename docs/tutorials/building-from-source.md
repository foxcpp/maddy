# Building from source

## System dependencies

You need C toolchain, Go toolchain and Make:

On Debian-based system this should work:
```
apt-get install golang-1.23 gcc libc6-dev make
```

Additionally, if you want manual pages, you should also have scdoc installed.
Figuring out the appropriate way to get scdoc is left as an exercise for
reader (for Ubuntu 22.04 LTS it is in repositories).

## Recent Go toolchain

maddy depends on a rather recent Go toolchain version that may not be
available in some distributions (*cough* Debian *cough*).

`go` command in Go 1.21 or newer will automatically download up-to-date
toolchain to build maddy. It is necessary to run commands below only
if you have `go` command version older than 1.21.

```
wget "https://go.dev/dl/go1.23.5.linux-amd64.tar.gz"
tar xf "go1.23.5.linux-amd64.tar.gz"
export GOROOT="$PWD/go"
export PATH="$PWD/go/bin:$PATH"
```

## Step-by-step

1. Clone repository
```
$ git clone https://github.com/foxcpp/maddy.git
$ cd maddy
```

2. Select the appropriate version to build:
```
$ git checkout v0.8.0      # a specific release
$ git checkout master      # next bugfix release
$ git checkout dev         # next feature release
```

3. Build & install it
```
$ ./build.sh
$ sudo ./build.sh install
```

4. Finish setup as described in [Setting up](../setting-up) (starting from System configuration).


