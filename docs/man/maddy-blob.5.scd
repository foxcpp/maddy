maddy-blob(5) "maddy mail server" "maddy reference documentation"

; TITLE Message blob storage

Some IMAP storage backends support pluggable message storage that allows
message contents to be stored separately from IMAP index.

Modules described in this page are what can be used with such storage backends.
In most cases they have to be specified using the 'msg_store' directive, like
this:
```
storage.imapsql local_mailboxes {
    msg_store fs /var/lib/email
}
```

Unless explicitly configured, storage backends with pluggable storage will
store messages in state_dir/messages (e.g. /var/lib/maddy/messages) FS
directory.

# FS directory storage (storage.blob.fs)

This module stores message bodies in a file system directory.

```
storage.blob.fs {
    root <directory>
}
```
```
storage.blob.fs <directory>
```

## Configuration directives

*Syntax:* root _path_ ++
*Default:* not set

Path to the FS directory. Must be readable and writable by the server process.
If it does not exist - it will be created (parent directory should be writable
for this). Relative paths are interpreted relatively to server state directory.