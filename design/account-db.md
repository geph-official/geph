# Account database

This document describes how the account database is laid out.

## Access to the database

The database is a PostgreSQL database living on `eressea`, but it doesn't listen over the network. Instead, there's a UNIX user `psql-conn` that database users must use via SSH tunneling to access the database.

All security is performed over SSH; the PostgreSQL database itself is accessed with the master role of `postgres:postgres`. AutoSSH and similar things must be used to maintain an SSH connection to the database server from other servers, with authentication done by putting lines in the database server's `authorized_keys`.

## Account-related tables

These tables store the current state of accounts.

### AccInfo

This table stores static information about accounts.

````
CREATE TABLE AccInfo (
    Uid     TEXT PRIMARY KEY,
    Uname   TEXT NOT NULL UNIQUE,
    Ctime   TIMESTAMP
)
````

Once a new row is added to this table, it is generally not going to be altered or deleted in the future.

The `Uid` is `base32(blake2b(client_pubkey)[:10])`.

### AccBalances

This table stores the balances of every account.

````
CREATE TABLE AccBalances (
    Uid TEXT PRIMARY KEY REFERENCES AccInfo,
    Mbs INTEGER NOT NULL
)
````

### AccBillLog

This table logs all billing actions. **TBD**
