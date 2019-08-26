caddyfile lexer copied from [caddy](https://github.com/caddyserver/caddy) project.

Taken from the following commit:
```
commit ed4c2775e46b924d4851e04cc281633b1b2c15af
Author: Alexander Danilov <SmilingNavern@users.noreply.github.com>
Date:   Wed Aug 21 20:13:34 2019 +0300

    main: log caddy version on start (#2717)

```

License of the original code is included in LICENSE.APACHE file in this
directory.

No signficant changes was made to the code (e.g. it is safe to update it from
caddy repo).

The code is copied because caddy brings quite a lot of dependencies we don't
use and this slows down many tools.
