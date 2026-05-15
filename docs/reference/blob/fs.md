# Filesystem

This module stores message bodies in a file system directory.

Module does not escape path separators so "a/b" will be stored in "a" subdirectory
that must exist already. ".." is allowed as long as it does not escape the
configured root directory.

Module supports both `storage.blob` and `table` interfaces and can be used
as a metadata store this way.

## Configuration directives

```
storage.blob.fs <directory>
table.fs <directory>
```

```
storage.blob.fs {
    root <directory>
}

table.fs {
    root <directory>
}
```

### root _path_
Default: not set

Path to the FS directory. Must be readable and writable by the server process.
If it does not exist - it will be created (parent directory should be writable
for this). Relative paths are interpreted relatively to server state directory.

