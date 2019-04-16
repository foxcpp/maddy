## maddy-shadow-helper

External helper binary for interaction with shadow passwords database.
Unlike maddy-pam-helper it supports only local shadow database but it does
not have any C dependencies.

### Installation

maddy-shadow-helper is kinda dangerous binary and should not be allowed to be
executed by everybody but maddy's user. At the same moment it needs to have
access to read-protected files. For this reason installation should be done
very carefully to make sure to not introduce any security "holes".

#### First method

```shell
chown maddy: /usr/bin/maddy-shadow-helper
chmod u+x,g-x,o-x /usr/bin/maddy-shadow-helper
```

Also maddy-shadow-helper needs access to /etc/shadow, one of the ways to provide
it is to set file capability CAP_DAC_READ_SEARCH:
```
setcap cap_dac_read_search+ep /usr/bin/maddy-shadow-helper
```

#### Second method

Another, less restrictive is to make it setuid-root (assuming you have both maddy user and group):
```
chown root:maddy /usr/bin/maddy-shadow-helper
chmod u+xs,g+x,o-x /usr/bin/maddy-shadow-helper
```

#### Third method

The best way actually is to create `shadow` group and grant access to
/etc/shadow to it and then make maddy-shadow-helper setgid-shadow:
```
groupadd shadow
chown :shadow /etc/shadow
chmod g+r /etc/shadow
chown maddy:shadow /usr/bin/maddy-shadow-helper
chmod u+x,g+xs /usr/bin/maddy-shadow-helper
```

Pick what works best for you.
