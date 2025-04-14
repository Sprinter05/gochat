# Implementation
## Header Format
Both server and client headers share the following header format, which occupies **8 bytes**:

    0     4             12           20     24               38              48         64
    +-----+-------------+------------+------+----------------+---------------+----------+
    | Ver | Action Code | Reply Info | Args | Payload Length | Identificator | Reserved |
    +-----+-------------+------------+------+----------------+---------------+----------+

- **Protocol Version** | `4 bits`: Ensures that both client and server share the same protocol format. Communication between the client application and the server cannot happen if the versions differ.
- **Action Code** | `8 bits`: Instruction code that determines the action code that must be performed. Both client and server have their own, independent codes.
- **Reply Information** | `8 bits`: Additional information provided by specific instructions such as `ERR` and `ADMIN` codes or `USRS` arguments.
- **Arguments** | `4 bits` : Amount of arguments to be read.
- **Length** | `14 bits`: Indicates the size of the payload in bytes including delimiters.
- **Identificator** | `10 bits`: Indicates the packet identification the client has provided.
- **Reserved** | `16 bits`: For future extension that might be needed. (`0xFFFF` by default)

## Codes
`0x0` is reserved as an invalid action code.

### Action Codes
- `OK` = 0x01
- `ERR` = 0x02
- `REG` = 0x03
- `VERIF` = 0x04
- `REQ` = 0x05
- `USRS` = 0x06
- `RECIV` = 0x07
- `LOGIN` = 0x08
- `MSG` = 0x09
- `LOGOUT` = 0x0A
- `DEREG` = 0x0B
- `SHTDWN` = 0x0C
- `ADMIN` = 0x0D
- `KEEP` = 0x0E
- `SUB` = 0x0F
- `UNSUB` = 0x10
- `HOOK` = 0x11

> **NOTE**: Not all actions can be used by both client and server, check the specification for details.

## Reply Info

If the action to perform requires no additional information the "**Reply Info**" field should be `0xFF`. Otherwise it should be any of the following:

> **NOTE**: If the operation does not accept info and it is non-empty an error will be returned

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
- `ERR_PERMS` (0x0D): Lacking permissions to run the action.
- `ERR_SERVER` (0x0E): Failed to perform a server-side operation.
- `ERR_IDLE` (0x0F): User has been idle for too long.
- `ERR_EXISTS` (0x10): Content already exists.
- `ERR_DEREG` (0x11): User is no longer registered.
- `ERR_DUPSESS` (0x12): Session already exists in another endpoint.

### Argument for USRS
- `OFFLINE` (0x0): Show all users.
- `ONLINE` (0x1): Show online users.

### Admin Operations for ADMIN
- `ADMIN_SHTDWN <stamp>` (0x00): Schedules a shutdown for the server.
- `ADMIN_DEREG <username>` (0x01): Deregistrates a specified user.
- `ADMIN_BRDCAST <msg>` (0x02): Broadcasts a message to all online users.
- `ADMIN_CHGPERMS <username> <level>` (0x03): Changes the permission level of a user.
- `ADMIN_KICK <username>` (0x04): Kicks a user, also disconnecting it.

### Hooks for SUB, UNSUB and HOOK
- `HOOK_ALL` (0x00): Subscribes/unsubscribes too all existing hooks.
- `HOOK_NEWLOGIN` (0x01): Triggers whenever a new user succesfully logs into the server. 
- `HOOK_NEWLOGOUT` (0x02): Triggers whenever a user either disconnects or logs out.
- `HOOK_DUPSESS` (0x03): Triggers whenever an attempt to log into your account from another endpoint happens.
- `HOOK_PERMSCHG` (0x04): Triggers whenever your account's permissions have changed

## Body

### Payload
The payload will start being read after processing the header, both should be separated with **CRLF** (`\r\n`). A single argument may not be bigger than `2047` bytes. The server is free to implement any method to read from the connection.