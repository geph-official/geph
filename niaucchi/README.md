## niaucchi: the traffic smuggler

niaucchi is a drop-in replacement for TCP that incorporates KiSS/LLObfs obfuscation. It's specifically designed to *quickly* traverse the Great Firewall of China, so it also splits traffic over multiple TCP connections to get good throughput across the lossy, fat, and long links that typify international connections out of mainland China.

However, a niaucchi connection is quite expensive, so it should be used to tunnel protocols like SSH or WFSP where a single connection is reused for many application streams. Directly tunneling HTTP, especially HTTP 1.0, would cause terrible performance.
