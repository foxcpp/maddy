# Building from source

## System dependencies

You need C toolchain, Go toolchain and Make:

On Debian-based system this should work:
```
apt-get install golang-1.14 gcc libc6-dev make
```

Additionally, if you want manual pages, you should also have scdoc installed.
Figuring out the appropriate way to get scdoc is left as an exercise for
reader (for Ubuntu 19.10 it is in repositories).

## Recent Go toolchain

maddy depends on a rather recent Go toolchain version that may not be
available in some distributions (*cough* Debian *cough*).

It should not be hard to grab a recent built toolchain from golang.org:
```
wget "https://dl.google.com/go/go1.14.6.linux-amd64.tar.gz"
tar xf "go1.14.6.linux-amd64.tar.gz"
export GOROOT="$PWD/go"
export PATH="$PWD/go/bin:$PATH"
```

## Step-by-step

1. Clone repository
```
git clone https://github.com/foxcpp/maddy.git
cd maddy
```

3. Switch to the corresponding release.
e.g.
```
git checkout v0.4.0
```
or to in-development version:
```
git checkout dev
```

2. Build & install it
```
./build.sh
```

3. Have fun!
