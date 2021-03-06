# API for the binder

## Overview

This document details the REST API exposed by the Geph binder located at `https://binder.geph.io`.

## Querying the list of exit nodes

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

The hex-encoded signature of the entire response, signed with a hard-coded key ~~kept offline~~, is put in the header `X-Geph-Signature`. The signature is required to prevent subversion of the list of exits even when using domain fronting, or even when the binder's TLS private keys are stolen.

**Note**: the expiry date MUST not be more than 1 month in the future; this mitigates replay attacks.

## Proxying through to an exit node

Clients often cannot directly connect to the exit nodes since they are blocked. Thus, the binder hosts a proxy service.

All URLs like `https://binder.geph.io/exits/noram.exits.geph.io/...` serve as reverse proxies to `http://noram.exits.geph.io:8081/...`.

Security is provided by signatures at the exits, as detailed in the document about them.

## Obtaining a captcha

The method is `POST /fresh-captcha`. (POST prevents middleboxes like the CDN from caching). The response is:

    {
        "CaptchaID": some base64 string,
        "CaptchaImg": base64-encoded PNG image
    }

## Registering an account (LEGACY)

The method is `POST /register-account`. The request looks like this:

    {
        "Username": desired username,
        "PubKey": public key,
        "CaptchaID": id of solved captcha,
        "CaptchaSoln": solution for captcha
    }

In the response, `200` means a successful registration, `400` means malformed request (badly formatted JSON, badly formatted username, etc), `409` means the username already exists, and `403` in case the captcha is wrong.

## Registering an account (NEW)

The method is `POST /users/`

## Account status

Account status can be read at `POST /user-status`. The request:

    {
        "Username": ...
    }

The response:

    {
        "FreeBalance": ...,
        "PremiumInfo": {
            "Plan": ...,
            "Desc": ...,
            "Unlimited": ...,
            "ExpUnix": ...
        }
    }

## General info about an account (LEGACY)

Information about an account can be obtained at `POST /account-summary`. The request contains the following JSON body:

    {
        "PrivKey": client's private key
    }

Yes, the private key is handed over to the binder. This is not an issue, since the private key is not used to ensure end-to-end encryption; even if the private key were to be intercepted, users still cannot be eavesdropped upon.

We give over the private key instead of the username and password to make the expensive hashing happen on the client side.

The response contains a summary of information about an account:

    {
        "Username": username,
        "RegDate": date of registration (RFC3339),
        "Balance": MiB remaining in account
    }

### Detailed info about usage

TBD

### Billing

TBD
