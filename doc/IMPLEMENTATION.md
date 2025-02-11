# Implementation
## Header Format
Both server and client headers share the following header format, which occupies **6 bytes**:

    0         4             12           20     22               32              48
    +---------+-------------+------------+------+----------------+---------------+
    | Version | Action Code | Reply Info | Args | Payload Length | Identificator |
    +---------+-------------+------------+------+----------------+---------------+

- **Protocol Version** | `4 bits`: Ensures that both client and server share the same protocol format. Communication between the client application and the server cannot happen if the versions differ.
- **Action Code** | `8 bits`: Instruction code that determines the action code that must be performed. Both client and server have their own, independent codes.
- **Reply Information** | `8 bits`: Additional information provided by specific instructions such as `ERR` codes.
- **Arguments** | `2 bits` : Amount of arguments to be read.
- **Length** | `10 bits`: Indicates the size of the payload in bytes.
- **Identificator** | `16 bits`: Indicates the packet identification the client has provided.

## Codes
`0x0` is reserved as an invalid value.

### Action Codes
- `OK` = 0x01
- `ERR` = 0x02
- `REG` = 0x03
- `VERIF` = 0x04
- `REQ` = 0x05
- `USRS` = 0x06
- `RECIV` = 0x07
- `CONN` = 0x08
- `MSG` = 0x09
- `DISCN` = 0x0A
- `DEREG` = 0x0B
- `SHTDWN` = 0x0C

> **NOTE**: Not all actions can be used by both client and server, check the specification for details.

## Reply Info

If the action to perform requires no additional information the "**Reply Info**" field should be `0xFF`. Otherwise it should be any of the following:

> **NOTE**: Behaviour when the information value is incorrect is undefined.

### Error Codes for ERR
- `ERR_UNDEFINED` (0x00): Undefined generic error.
- `ERR_INVALID` (0x01): Invalid operation performed.
- `ERR_NOTFOUND` (0x02): Requested content not found.
- `ERR_VERSION` (0x03): Versions do not match.
- `ERR_HANDSHAKE` (0x04): Handshake process has failed.
- `ERR_ARGS` (0x05): Invalid arguments provided.
- `ERR_MAXSIZ` (0x06): Payload is too big.
- `ERR_HEADER` (0x07): Invalid header provided.
- `ERR_NOSESS` (0x08): User is not in a session.
- `ERR_LOGIN` (0x09): User can not be logged in.
- `ERR_CONN` (0x0A): Connection problem occured.
- `ERR_EMPTY` (0x0B): Request yielded an empty result.
- `ERR_PACKET` (0x0C): Problem with packet answer.

### Argument for USRS
- `OFFLINE` (0x0): Show all users.
- `ONLINE` (0x1): Show online users.

## Body

### Payload
The payload will start being read after processing the header, both should be separated with **CRLF** (`\r\n`). A total of *n* reads will be performed, corresponding to the amount of arguments specified by the header. Each read will stop when **CRLF** is found. If at any point the payload goes over the maximum allowed size, an error will be produced.