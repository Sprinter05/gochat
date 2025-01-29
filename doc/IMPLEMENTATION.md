# Implementation
## Header Format
Both server and client headers share the following header format, which occupies **2 bytes**:

    0         3             10           15
    +---------+-------------+------------+
    | Version | Action Code | Reply Info |
    +---------+-------------+------------+

- **Protocol Version** | `3 bits`: Ensures that both client and server share the same protocol format. Communication between the client application and the server cannot happen if the versions differ.
- **Action Code** | `7 bits`: Instruction code that determines the action code that must be performed. Both client and server have their own, independent codes.
- **Reply Information** | `6 bits`: Additional information provided by specific instructions such as `ERR` codes.

## Codes
`0x0` is reserved as an invalid value.

### Server Action Codes
- `OK` = 0x01
- `ERR` = 0x02
- `SHTDWN` = 0x03
- `VERIF` = 0x04
- `REQ` = 0x05
- `USRS` = 0x06
- `RECIV` = 0x07

### Client Action Codes
- `OK` = 0x01
- `ERR` = 0x02
- `REG` = 0x03
- `VERIF` = 0x04
- `REQ` = 0x05
- `USRS` = 0x06
- `CONN` = 0x07
- `MSG` = 0x08
- `DISCN` = 0x09
- `DEREG` = 0x0A

## Reply Info

If the action to perform requires no additional information the "**Reply Info**" field should be `111111`. Otherwise it should be any of the following:

> **NOTE**: Behaviour when the information value is incorrect is undefined.

### Error Codes
- `ERR_UNDEFINED` (0x00): Undefined generic error.
- `ERR_INVALID` (0x01): Invalid operation performed.
- `ERR_NOTFOUND` (0x02): Requested content not found.
- `ERR_VERSION` (0x03): Versions do not match.
- `ERR_HANDSHAKE` (0x04): Handshake process has failed.

## Body
### Length
After the header comes the **length**, which occupies another **2 bytes** that compose an unsigned integer of type `uint16`, which specifies the maximum length in bytes the payload can have.

### Payload
The payload will start being read right after the length and arguments will be separated by **CRLF** (`\r\n`). The server is not responsible for reading extra bytes, that should be handled by the client application by specifying an adequate length. Said arguments will be processed as an string.