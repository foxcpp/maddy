imapsql-ctl utility
-------------------

Maddy fork of utility from go-imap-sql repo, extended with functionality to
parse maddy configuration files.

#### --unsafe option

Per RFC 3501, server must send notifications to clients about any mailboxes
change. Since imapsql-ctl is a low-level tool it doesn't implements any way to
tell server to send such notifications. Most popular SQL RDBMSs don't provide
any means to detect database change and we currently have no plans on
implementing anything for that on go-imap-sql level.

Therefore, you generally should avoid writting to mailboxes if client who owns
this mailbox is connected to the server. Failure to send required notifications
may result in data damage depending on client implementation.
