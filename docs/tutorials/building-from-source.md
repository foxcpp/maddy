# Building from source

## System dependencies

You need C toolchain, Go toolchain and Make:

On Debian-based system this should work:
```
apt-get install golang-1.18 gcc libc6-dev make
```

Additionally, if you want manual pages, you should also have scdoc installed.
Figuring out the appropriate way to get scdoc is left as an exercise for
reader (for Ubuntu 22.04 LTS it is in repositories).

## Recent Go toolchain

maddy depends on a rather recent Go toolchain version that may not be
available in some distributions (*cough* Debian *cough*).

It should not be hard to grab a recent built toolchain from golang.org:
```
wget "https://dl.google.com/go/go1.18.9.linux-amd64.tar.gz"
tar xf "go1.18.19.linux-amd64.tar.gz"
export GOROOT="$PWD/go"
export PATH="$PWD/go/bin:$PATH"
```

## Step-by-step

1. Clone repository
```
$ git clone https://github.com/foxcpp/maddy.git
$ cd maddy
```

3. Select the appropriate version to build:
```
$ git checkout v0.6.0      # a specific release
$ git checkout master      # next bugfix release
$ git checkout dev         # next feature release
```

2. Build & install it
```
$ ./build.sh
# ./build.sh install
```

3. Have fun!
