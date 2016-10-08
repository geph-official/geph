# API for the binder

## Overview

This document details the REST API exposed by the Geph binder located at `https://binder.geph.io`.

One important thing to mention is that the binder is also served over domain-fronting CDNs (currently Amazon CloudFront). This, of course, breaks end-to-end security. Thus, particularly sensitive methods MUST not be accessed over domain fronting, and the binder will try to enforce this. Domain fronting is used --- only by clients --- to distribute information crucial to bootstrapping a connection to the free internet.

## Methods allowed over domain-fronting

### Querying the list of exit nodes

The method is `GET /exit-info`. The response is:

    {
        "Expires": "ISO 8601",
        "Exits":
        {
            "server-hostname": "server-public-key"
            ...
        }
    }

where the server names are hostnames, and the server public key is encoded in Base64.

The hex-encoded signature of the entire response, signed with a hard-coded key kept offline, is put in the header `X-Geph-Signature`. The signature is required to prevent subversion of the list of exits even when using domain fronting, or even when the binder's TLS private keys are stolen.

**Note**: the expiry date MUST not be more than 1 month in the future; this mitigates replay attacks.

### Proxying through to an exit node

Clients often cannot directly connect to the exit nodes since they are blocked. Thus, the binder hosts a proxy service.

All URLs like `https://binder.geph.io/exits/noram.exits.geph.io/...` serve as reverse proxies to `http://noram.exits.geph.io:8081/...`.

Security is provided by signatures at the exits, as detailed in the document about them.

## Methods only allowed with end-to-end encryption

### General info about an account

Information about an account can be obtained at `POST /account-summary`. The request contains the following JSON body:

    {
        "PubKey": client's public key,
        "DateSig": signature of current date (YYYY-MM-DD) with client's public key
    }

The response contains a summary of information about an account:

    {
        "Username": username,
        "RegDate": date of registration (YYYY-MM-DD),
        "Balance": MiB remaining in account
    }

### Detailed info about usage

TBD

### Billing

TBD
