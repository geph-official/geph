# Overview

Geph currently operates as a SOCKS proxy, but the next generation of Geph aims to be a network-layer VPN.

# Pontikos: a cost-aware, adaptive obfuscated transport

## General overview

In Pontikos, there are four roles: **clients**, **relays**, **exits**, and the **binder**. Clients authenticate to and obtain *routes* from the binder, consisting of several relays selected from a geographically-determined pool.  
