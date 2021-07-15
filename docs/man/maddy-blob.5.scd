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

# Amazon S3 storage (storage.blob.s3)

This modules stores messages bodies in a bucket on S3-compatible storage.

```
storage.blob.s3 {
    endpoint play.min.io
    secure yes
    access_key "Q3AM3UQ867SPQQA43P2F"
    secret_key "zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG"
    bucket maddy-test

    # optional
    region eu-central-1
    object_prefix maddy/
}
```

Example:
```
storage.imapsql local_mailboxes {
    ...
    msg_store s3 {
        endpoint s3.amazonaws.com
        access_key "..."
        secret_key "..."
        bucket maddy-messages
        region us-west-2
    }
}
```

## Configuration directives

*Syntax:* endpoint _address:port_

REQUIRED.

Root S3 endpoint. e.g. s3.amazonaws.com

*Syntax:* secure _boolean_ ++
*Default:* yes

Whether TLS should be used.

*Syntax:* access_key _string_ ++
*Syntax:* secret_key _string_

REQUIRED.

Static S3 credentials.

*Syntax:* bucket _name_

REQUIRED.

S3 bucket name. The bucket must exist and
be read-writable.

*Syntax:* region _string_ ++
*Default:* not set

S3 bucket location. May be called "endpoint"
in some manuals.

*Syntax:* object_prefix _string_ ++
*Default:* empty string

String to add to all keys stored by maddy.

Can be useful when S3 is used as a file system.