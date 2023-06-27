# Upgrading from older maddy versions

It is generally possible to just install latest version (e.g. using build.sh
script) over the existing installation.

It is recommended to backup state directory (usually /var/lib/maddy for Linux)
before doing so. The new server version may automatically convert DB files in a
way that will make them unreadable by older versions.

Specific instructions for upgrading between versions with incompatible changes
are documented on this page below.

## Incompatible version migration

## 0.2 -> 0.3

0.3 includes a significant change to the authentication code that makes it
completely independent of IMAP index. This means 0.2 "unified" database cannot
be used in 0.3 and auto-migration is not possible. Additionally, the way
passwords are hashed is changed, meaning that after migration passwords will
need to be reset.

**Migration utility is SQLite-specific, if you need one that works for
Postgres - reach out at the IRC channel.**

1. Make sure the server is not running.

```
systemctl stop maddy
```

2. Take a backup of `imapsql.db*` files in state directory (/var/lib/maddy).

```
mkdir backup
cp /var/lib/maddy/imapsql.db* backup/
```

3. Compile migration utility:

```
git clone https://github.com/foxcpp/maddy.git
cd maddy/
git checkout v0.3.0
cd cmd/migrate-db-0.2
go build
```

4. Run compiled binary:

```
./migrate-db-0.2 /var/lib/maddy/imapsql.db
```

5. Open maddy.conf and make following changes:

Remove `local_authdb` name from imapsql configuration block:
```
imapsql local_mailboxes {
    driver sqlite3
    dsn imapsql.db
}
```

Add `local_authdb` configuration block using `pass_table` module:

```
pass_table local_authdb {
    table sql_table {
        driver sqlite3
        dsn credentials.db
        table_name passwords
    }
}
```

6. Use `maddy creds create ACCOUNT_NAME` to add credentials to `pass_table`
   store.

7. Start the server back.

```
systemctl start maddy
```

## 0.1 -> 0.2

0.2 requires several changes in configuration file.

Change
```
sql local_mailboxes local_authdb {
```
to
```
imapsql local_mailboxes local_authdb {
```

Replace
```
replace_rcpt postmaster postmaster@$(primary_domain)
```
with
```
replace_rcpt static {
    entry postmaster postmaster@$(primary_domain)
}
```
and

```
replace_rcpt "(.+)\+(.+)@(.+)" "$1@$3"
```
with
```
replace_rcpt regexp "(.+)\+(.+)@(.+)" "$1@$3"
```
