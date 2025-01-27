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
### Server Action Codes
- `OK` = 0x00
- `ERR` = 0x01
- `SHTDWN` = 0x02
- `VERIF` = 0x03
- `REQ` = 0x04
- `USRS` = 0x05
- `RECIV` = 0x06

### Client Action Codes
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

## Reply Info

If the action to perform requires no additional information the "**Reply Info**" field should be `111111`. Otherwise it should be any of the following:

> **NOTE**: Behaviour when the information value is incorrect is undefined.

### Server Error Reply Codes
- `ERR_UNDEFINED` (0x00): Undefined generic error.
- `ERR_NOCONN` (0x01): Client cannot be reached.
- `ERR_NOTFOUND` (0x02): Requested content not found.
- `ERR_HANDSHAKE` (0x03): Handshake process has failed.

### Client Error Reply Codes
- `ERR_UNDEFINED` (0x00): Undefined generic error.
- `ERR_NOCONN` (0x01): Requested server cannot be reached.
- `ERR_BROKENMSG` (0x02): Message has been incorrectly received.

## Body
### Length
After the header comes the **length**, which occupies another **2 bytes** that compose an unsigned integer of type `uint16`, which specifies the maximum length in bytes the payload can have.

### Payload
The payload will start being read right after the length and arguments will be separated by **CRLF** (`\r\n`). The server is not responsible for reading extra bytes, that should be handled by the client application by specifying an adequate length. Said arguments will be processed as an string.