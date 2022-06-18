# Filesystem

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

**Syntax:** root _path_ <br>
**Default:** not set

Path to the FS directory. Must be readable and writable by the server process.
If it does not exist - it will be created (parent directory should be writable
for this). Relative paths are interpreted relatively to server state directory.

