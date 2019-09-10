## maddy-pam-helper

External setuid binary for interaction with shadow passwords database or other
privileged objects necessary to run PAM authentication.

### Building

It is really easy to build it using any GCC:
```
gcc pam.c main.c -lpam -o maddy-pam-helper
```

Yes, it is not a Go binary.


### Installation

maddy-pam-helper is kinda dangerous binary and should not be allowed to be
executed by everybody but maddy's user. At the same moment it needs to have
access to read-protected files. For this reason installation should be done
very carefully to make sure to not introduce any security "holes".

#### First method

```shell
chown maddy: /usr/bin/maddy-pam-helper
chmod u+x,g-x,o-x /usr/bin/maddy-pam-helper
```

Also maddy-pam-helper needs access to /etc/shadow, one of the ways to provide
it is to set file capability CAP_DAC_READ_SEARCH:
```
setcap cap_dac_read_search+ep /usr/bin/maddy-pam-helper
```

#### Second method

Another, less restrictive is to make it setuid-root (assuming you have both maddy user and group):
```
chown root:maddy /usr/bin/maddy-pam-helper
chmod u+xs,g+x,o-x /usr/bin/maddy-pam-helper
```

#### Third method

The best way actually is to create `shadow` group and grant access to
/etc/shadow to it and then make maddy-pam-helper setgid-shadow:
```
groupadd shadow
chown :shadow /etc/shadow
chmod g+r /etc/shadow
chown maddy:shadow /usr/bin/maddy-pam-helper
chmod u+x,g+xs /usr/bin/maddy-pam-helper
```

Pick what works best for you.

### PAM service

maddy-pam-helper uses custom service instead of pretending to be su or sudo.
Because of this you should configure PAM to accept it.

Minimal example using local passwd/shadow database for authentication can be
found in [maddy.conf][maddy.conf] file.
It should be put into /etc/pam.d/maddy.
