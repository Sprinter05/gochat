# Protocol Implementation
## Header
Both server and client headers share the following header format, which occupies **2 bytes**:

    0         3             10           15
    +---------+-------------+------------+
    | Version | Action Code | Reply Info |
    +---------+-------------+------------+

- **Protocol Version** | `3 bits`: Ensures that both client and server share the same protocol format. Communication between the client application and the server cannot happen if the versions differ.
- **Action Code** | `7 bits`: Instruction code that determines the action code that must be performed. Both client and server have their own, independent codes.
- **Reply Information** | `6 bits`: Additional information provided by specific instructions such as `OK` or `ERR` codes.

## Server Action Codes
- `OK` = 0x00
- `ERR` = 0x01
- `SHTDWN` = 0x02
- `VERIF` = 0x03
- `REQ` = 0x04
- `USRS` = 0x05
- `RECIV` = 0x06

## Client Action Codes
- `OK` = 0x00
- `ERR` = 0x01
- `REG` = 0x02
- `VERIF` = 0x03
- `REQ` = 0x04
- `USRS` = 0x05
- `CONN` = 0x06
- `MSG` = 0x07
- `DISCN` = 0x08
- `DEREG` = 0x09

## Error Reply Codes
TBD

## Action Reply Codes
TBD