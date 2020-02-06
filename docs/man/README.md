maddy manual pages
-------------------

The reference documentation is maintained in the scdoc format and is compiled
into a set of Unix man pages viewable using the standard `man` utility.

See https://git.sr.ht/~sircmpwn/scdoc for information about the tool used to
build pages.
It can be used as follows:
```
scdoc < maddy-filters.5.scd > maddy-filters.5
man ./maddy-filters.5
```

build.sh script in the repo root compiles and installs man pages if the scdoc
utility is installed in the system.
