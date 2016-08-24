## Geph's design, the big picture

### Participants

We currently ignore accounts, since that's complicated.

At the core, Geph is a semi-federated network. Every **exit node** autonomously manages its **entry nodes**. **Clients** can query an exit node to obtain a small list of eligible entry nodes. Finally, there is one global **binder** that distributes to everybody the list of exit nodes, and also provides a blocking-resistant conduit to the exits.

There are not too many exit nodes, and they are all highly trusted. In practice, we put (logically) one exit node per region, in a jurisdiction as privacy-friendly as possible (i.e. places like Switzerland or Japan, not the UK or Singapore).   

### Protocols

A surprising amount of communication in the network, other than the tunneling of the traffic itself, happens in plaintext. This is because most of the traffic is not truly confidential --- for example, nothing too bad happens if an eavesdropper learns about which entry is registered with which exit. Encryption can actually hurt since we may end up relying on it too much, causing security fails when the encryption ends up not as strong as it seemed. "Put your eggs in one basket and watch it carefully". Integrity protection in the form of signatures and timestamps, however, are used whenever needed.

To that end, there is a globally-known key for the binder, distributed with the program. The register uses its key to sign the list of exit nodes, which also contains public keys. The exit nodes' keys are used to sign partial entry node lists, and also to validate traffic going through them.

Every entry node has an obfuscation cookie, which is given to the exit node to which they belong.

### Joining the network as an entry node

Anybody can join their entry node. They contact an exit node periodically, uploading information. Once the exit node receives the information, it tests the entry node to make sure the information is correct, and then enters it into its database. Entry nodes with stale information are removed from the database.

### Joining the network as a client

The client first contacts the binder to get the list of exit nodes. The binder will give the client a signed list containing an expiration date, before which the client can cache the list.

Then, the client will select an exit node. It can either do this autonomously, or ask the binder to recommend an exit node based on their IP address etc. It queries for entry nodes by giving a proof-of-work, and receives a consistent subset of entry nodes, together with a signature, and an expiration date of a few days later.

Finally, the client picks the fastest entry node available and connects to it.

Note: all the caching prevents too many lookups to the binder. This is because it's the only centralized part of the system, and loading it too much will break stuff / cost me lots of monies.
