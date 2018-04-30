# Account database

This document describes how the account database is laid out.

## Access to the database

The database is a PostgreSQL database living on `eressea`, but it doesn't listen over the network. Instead, there's a UNIX user `psql-conn` that database users must use via SSH tunneling to access the database.

All security is performed over SSH; the PostgreSQL database itself is accessed with the master role of `postgres:postgres`. AutoSSH and similar things must be used to maintain an SSH connection to the database server from other servers, with authentication done by putting lines in the database server's `authorized_keys`.

## Account-related tables

These tables store the current state of users.

### Users

This table stores *mandatory* information about users. Every user has an entry here.

````
CREATE TABLE Users (
    ID SERIAL PRIMARY KEY,
    Username TEXT NOT NULL,
    PwdHash TEXT NOT NULL,
    FreeBalance INTEGER NOT NULL,
    CreateTime TIMESTAMP NOT NULL
)
````

### PremiumPlans

This table stores all the different premium plans available. The description is a big blob for multilingual flavor text on the website.

````
CREATE TABLE PremiumPlans (
    Plan TEXT PRIMARY KEY,
    Description JSONB NOT NULL,
    MaxSpeed INTEGER NOT NULL,
    MonthlyBalance INTEGER NOT NULL,
    MaxConns INTEGER NOT NULL,
    Unlimited BOOL NOT NULL
)
````

### Subscriptions

This table stores subscriptions for paying customers.

````
CREATE TABLE Subscriptions (
    ID SERIAL PRIMARY KEY REFERENCES Users,
    Plan TEXT REFERENCES PremiumPlans(Plan),
    Expires TIMESTAMP NOT NULL
)
````
