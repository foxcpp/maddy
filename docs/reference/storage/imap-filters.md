# IMAP filters

Most storage backends support application of custom code late in delivery
process. As opposed to using SMTP pipeline modifiers or checks, it allows
modifying IMAP-specific message attributes. In particular, it allows
code to change target folder and add IMAP flags (keywords) to the message.

There is no way to reject message using IMAP filters, this should be done
earlier in SMTP pipeline logic. Quarantined messages are not processed
by IMAP filters and are unconditionally delivered to Junk folder (or other
folder with \Junk special-use attribute).

To use an IMAP filter, specify it in the 'imap\_filter' directive for the
used storage backend, like this:
```
storage.imapsql local_mailboxes {
   ...
   
   imap_filter {
       command /etc/maddy/sieve.sh {account_name}
   }
}
```

## System command filter (imap.filter.command)

This filter is similar to check.command module
and runs a system command to obtain necessary information.

Usage:
```
command executable_name args... { }
```

Same as check.command, following placeholders are supported for command
arguments: {source\_ip}, {source\_host}, {source\_rdns}, {msg\_id}, {auth\_user},
{sender}. Note: placeholders
in command name are not processed to avoid possible command injection attacks.

Additionally, for imap.filter.command, {account\_name} placeholder is replaced
with effective IMAP account name, {rcpt_to}, {original_rcpt_to} provide
access to the SMTP envelope recipient (before and after any rewrites),
{subject} is replaced with the Subject header, if it is present.

Note that if you use provided systemd units on Linux, maddy executable is
sandboxed - all commands will be executed with heavily restricted filesystem
access and other privileges. Notably, /tmp is isolated and all directories
except for /var/lib/maddy and /run/maddy are read-only. You will need to modify
systemd unit if your command needs more privileges.

Command output should consist of zero or more lines. First one, if non-empty, overrides
destination folder. All other lines contain additional IMAP flags to add
to the message. If command wants to add flags without changing folder - first
line should be empty.

It is valid for command to not write anything to stdout. In this case its
execution will have no effect on delivery.

Output example:
```
Junk
```
In this case, message will be placed in the Junk folder.

```

$Label1
```
In this case, message will be placed in inbox and will have
'$Label1' added.
