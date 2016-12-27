## Client REST-based RPC interface

The client daemon exposes an RPC interface on localhost for communication with GUIs and other purposes. It's based upon pushing around JSON over HTTP because basically every language (Racket for desktop, Java for Android, etc) has a JSON parser and HTTP library.
z
This document is far from complete and will be added to as the stuff gets implemented.

### Exit status

Normally, the daemon never stops running until somebody kills it. However, on certain fatal errors, the daemon will exit, in which case the exit status will give the reason for death:

    403: authentication failed (mnemonic from HTTP)
    502: gave up on network (mnemonic from HTTP again) (currently not implemented)
    anything else: unexpected runtime error (dump stderr somewhere to debug)

### Summary of internal state

This is usually enough to give a brief status report. The endpoint is `GET /summary` and the result is this big JSON structure:

    {
        "Status": one of "connected", "connecting"
        "Uptime": in seconds,
        "BytesTX": total bytes transmitted this session,
        "BytesRX": total bytes received this session
    }

### Detailed network information

This is used to display detailed network information, useful for nerds or for debugging. The endpoint is `GET /netinfo` and the thing looks like:

    {
        "Exit": IP address,
        "Entry": first 64 bits of hash of cookie, hex-encoded,
        "Protocol": currently only "cl-ni-mi" but in the future can identify different protocols,
        "ActiveTunns": ["host:port"...]
    }

ActiveTunns contains a list of active connections.

### Detailed account information

The endpoint is `GET /accinfo` and the thing looks like:

    {
        "Username": username,
        "AccID": account ID (pubkey fingerprint),
        "Balance": remaining account balance, in MiB
        (more fields in the future)
    }

Calling this endpoint causes the daemon to pull the latest data from the network, so it should not be called very frequently.
