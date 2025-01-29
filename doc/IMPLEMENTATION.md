# Implementation
## Header Format
Both server and client headers share the following header format, which occupies **4 bytes**:

    0         4             12           20     22               32
    +---------+-------------+------------+------+----------------+
    | Version | Action Code | Reply Info | Args | Payload Length |
    +---------+-------------+------------+------+----------------+

- **Protocol Version** | `4 bits`: Ensures that both client and server share the same protocol format. Communication between the client application and the server cannot happen if the versions differ.
- **Action Code** | `8 bits`: Instruction code that determines the action code that must be performed. Both client and server have their own, independent codes.
- **Reply Information** | `8 bits`: Additional information provided by specific instructions such as `ERR` codes.
- **Arguments** | `2 bits` : Amount of arguments to be read.
- **Length** | `10 bits`: Indicates the size of the payload in bytes.

## Codes
`0x0` is reserved as an invalid value.

### Server Action Codes
- `OK` = 0x01
- `ERR` = 0x02
- `VERIF` = 0x04
- `REQ` = 0x05
- `USRS` = 0x06
- `RECIV` = 0x07
- `SHTDWN` = 0x0C

### Client Action Codes
- `OK` = 0x01
- `ERR` = 0x02
- `REG` = 0x03
- `VERIF` = 0x04
- `REQ` = 0x05
- `USRS` = 0x06
- `CONN` = 0x08
- `MSG` = 0x09
- `DISCN` = 0x0A
- `DEREG` = 0x0B

## Reply Info

If the action to perform requires no additional information the "**Reply Info**" field should be `0xFF`. Otherwise it should be any of the following:

> **NOTE**: Behaviour when the information value is incorrect is undefined.

### Error Codes
- `ERR_UNDEFINED` (0x00): Undefined generic error.
- `ERR_INVALID` (0x01): Invalid operation performed.
- `ERR_NOTFOUND` (0x02): Requested content not found.
- `ERR_VERSION` (0x03): Versions do not match.
- `ERR_HANDSHAKE` (0x04): Handshake process has failed.

## Body

### Payload
The payload will start being read right after the header until **CRLF** (`\r\n`) is read. The server may handle the read bytes according to the supplied length in the header. If the payload has several arguments as specified by the header it will be read again.