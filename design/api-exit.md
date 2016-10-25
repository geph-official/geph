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

Clients obtain info by doing `POST /get-nodes`. The payload is `sig-get-nodes`, with a signature in the `X-Geph-Signature` header, and the full public key in `X-Geph-Pubkey`. The response is:

    {
        "Expires": ISO 8601 24 hrs after,
        "Nodes":
        {
            "hostname": "cookie"
            ...
        }
    }

which must be signed, via the `X-Geph-Signature` header, with the key of the exit node.

**TODO**: actually do a proof-of-work

### Proxying traffic

Port 2378 runs an extremely simple proxy protocol over unobfuscated Niaucchi. The client sends this:

    1 bytes: 0x00 (higher numbers reserved for future extensions)
    n bytes: pascal-string of destination address

and is patched through (no ack to save an RTT).

The KiSS uses the exit node's public key to authenticate traffic, ensuring that no man-in-the-middle attack can happen.

In addition, the `/warpfront/` hosts an warpfront-based tunnel for those who have to do HTTP.
