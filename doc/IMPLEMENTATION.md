# The gochat Server

This document is aimed to give the specifics of the behaviour of the server that is provided in this repository.

## Support

This server supports both **plain TCP** and **TLS** for the **v1 Protocol** on th standard default ports (`9037` and `8037` respectively).

This server implementes all **Actions**, including the optional `KEEP` for persistent connections. It also implements all **administrative operations** and all **hooks**.

**Dangling usernames** cannot be used by new accounts, meaning that once registered, that username can never be reused.

## Permissions

This server implements *3 levels* of permissions. The following, exhaustive list, indicates all levels and allowed administrative operations for each level.

- **USER**  = `0` 
- **ADMIN** = `1` 
    - `ADMIN_SHTDWN`
    - `ADMIN_BRDCAST`
    - `ADMIN_DEREG`
    - `ADMIN_KICK`
- **OWNER** = `2`
    - `ADMIN_CHGPERMS`
    - `ADMIN_MOTD`

## Limits

- **TLS handshakes** have a timeout of *20 seconds*
- **Inactivity** timeouts are of *25 minutes*
- **Verification handshakes** have a deadline of *2 minutes*
- **Usernames** cannot be bigger than *32 characters*
- **Reusable tokens** expire after *30 minutes* and can be used more than once
