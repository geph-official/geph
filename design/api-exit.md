## API for exit nodes

Unencrypted HTTP (with signing) is used for control data, while KiSS is used for proxied data.

### Uploading info about an entry node

An entry node can post info to `POST /update-node`. The payload is:

    {
        "Addr": host:port,
        "Cookie": 96-bit obfuscation cookie in Base64,
    }

The exit node always claims success, but will do its own testing.

### Obtaining info about entry nodes

Clients obtain info by doing `POST /get-nodes`. ~~The payload is `sig-get-nodes`, with a signature in the `X-Geph-Signature` header, and the full public key in `X-Geph-Pubkey`.~~ The response is:

    {
        "Expires": ISO 8601 24 hrs after,
        "Nodes":
        {
            "hostname": "cookie"
            ...
        }
    }

which must be signed, via the `X-Geph-Signature` header, with the key of the exit node.

**TODO**: currently the entry nodes are allocated to IP addresses. This can be bad for adversaries controlling large IP spaces (like China!), though currently China is not known to employ large IP spaces to run active probes from. There are other ways, such as assigning nodes to

    - Proof of work (vulnerable to adversaries with huge computational power; very annoying to mobile users)
    - Usernames (vulnerable to adversaries with many usernames, arguably easier than controlling many IPs)
