## API for the binder

All parties talk to the register over HTTP.

### Querying the list of exit nodes

The API endpoint is `GET /exit-info`. The response is:

    {
        "Expires": "ISO 8601",
        "Exits":
        {
            "server-hostname": "server-public-key"
            ...
        }
    }

where the server names are hostnames, and the server public key is encoded in Base64.

The hex-encoded signature of the entire response is put in the header `X-Geph-Signature`.

### Proxying through to an exit node

Clients often cannot directly connect to the exit nodes since they are blocked. Thus, the binder hosts a proxy service.

All URLs like `http://binder.geph.ch/exits/noram.exits.geph.ch/...` serve as reverse proxies to `http://noram.exits.geph.ch/...`.
