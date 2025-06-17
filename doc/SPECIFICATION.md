# The gochat Protocol

The **gochat protocol** is a chat protocol based on **TCP** designed for end-to-end (**E2E**) encrypted communication between users in the same server. This document specifies this protocol in detail.

## Commands

The following diagram illustrates the format that a command travelling through the connection must have:

    +--------+------------+------+------------+
    | Header | Argument 1 | .... | Argument n |
    +--------+------------+------+------------+

The header will be *8 bytes* long and both the separator between header and arguments (also called **payload**) and between each argument must be `\r\n`.

### Header

The following diagram indicates the different *bit fields* that the header must provide and the size of each one:

    0         4             12           20          24               38              48         64
    +---------+-------------+-------------+-----------+----------------+---------------+----------+
    | Version | Action Code | Information | Arguments | Payload Length | Identificator | Reserved |
    +---------+-------------+-------------+-----------+----------------+---------------+----------+

- **Version** | `4 bits`: Ensures that both client and server share the same protocol format. Communication between the client application and the server cannot happen if the versions differ.
- **Action Code** | `8 bits`: Instruction code that determines the action to be performed. Both client and server have their own, independent codes.
- **Information** | `8 bits`: Additional information provided by specific instructions.
- **Arguments** | `4 bits` : Amount of arguments to be read.
- **Length** | `14 bits`: Indicates the size of the payload (which includes all arguments) in bytes,  including delimiters.
- **Identificator** | `10 bits`: Indicates the packet identification number the client has provided. The server's reply to a command will have the same identificator that was sent by the client in order to easily identify replies.
- **Reserved** | `16 bits`: For future extension that might be needed. (`0xFFFF` by default)

#### Special Codes

- **Null ID**: `0x0` is a reserved *Identificator* that may only be used by the server to send a command to the client that does not require a reply.
- **Null Action**: `0x0` is an invalid *Action Code* that cannot be used.
- **Empty Info**: `0xFF` is the value that should be used in the *Information* when the command does not have any additional information.

#### Action Codes

> **NOTE**: Not all actions can be used by both client and server.

- `OK`     | `0x01` (*Server only*)
- `ERR`    | `0x02` (*Server only*)
- `REG`    | `0x03` (*Client only*)
- `VERIF`  | `0x04`
- `REQ`    | `0x05`
- `USRS`   | `0x06`
- `RECIV`  | `0x07`
- `LOGIN`  | `0x08` (*Client only*)
- `MSG`    | `0x09` (*Client only*)
- `LOGOUT` | `0x0A` (*Client only*)
- `DEREG`  | `0x0B` (*Client only*)
- `SHTDWN` | `0x0C` (*Server only*)
- `ADMIN`  | `0x0D` (*Client only*)
- `KEEP`   | `0x0E` (*Client only*)
- `SUB`    | `0x0F` (*Client only*)
- `UNSUB`  | `0x10` (*Client only*)
- `HOOK`   | `0x11` (*Server only*)

> **NOTE**: All commands sent by the client except `KEEP` must get a response from the server.

#### Information Field

> **NOTE**: If the operation does not accept info and it is non-empty an error must be returned.

##### Error Codes

The following list of codes are used by `ERR`.

- `ERR_UNDEFINED` (`0x00`): Undefined generic error.
- `ERR_INVALID`   (`0x01`): Invalid operation performed.
- `ERR_NOTFOUND`  (`0x02`): Requested content not found.
- `ERR_VERSION`   (`0x03`): Versions do not match.
- `ERR_HANDSHAKE` (`0x04`): Handshake process has failed.
- `ERR_ARGS`      (`0x05`): Invalid arguments provided.
- `ERR_MAXSIZE`   (`0x06`): Payload is too big.
- `ERR_HEADER`    (`0x07`): Invalid header provided.
- `ERR_NOSESS`    (`0x08`): User is not in a session.
- `ERR_LOGIN`     (`0x09`): User can not be logged in.
- `ERR_CONN`      (`0x0A`): Connection problem occured.
- `ERR_EMPTY`     (`0x0B`): Request yielded an empty result.
- `ERR_PACKET`    (`0x0C`): Problem with packet answer.
- `ERR_PERMS`     (`0x0D`): Lacking permissions to run the action.
- `ERR_SERVER`    (`0x0E`): Failed to perform a server-side operation.
- `ERR_IDLE`      (`0x0F`): User has been idle for too long.
- `ERR_EXISTS`    (`0x10`): Content already exists.
- `ERR_DEREG`     (`0x11`): User is no longer registered.
- `ERR_DUPSESS`   (`0x12`): Session already exists in another endpoint.
- `ERR_NOSECURE`  (`0x13`): Operation requires a secure connection.

##### Types of user lists

The following list of codes are used by `USRS`.

- `USRS_ALL`    (`0x0`): Show all users.
- `USRS_ONLINE` (`0x1`): Show online users.

##### Admin Operations

The following list of codes are used by `ADMIN`.

- `ADMIN_SHTDWN`   (`0x00`): Schedules a shutdown for the server.
- `ADMIN_DEREG`    (`0x01`): Deregistrates a specified user.
- `ADMIN_BRDCAST`  (`0x02`): Broadcasts a message to all online users.
- `ADMIN_CHGPERMS` (`0x03`): Changes the permission level of a user.
- `ADMIN_KICK`     (`0x04`): Kicks a user, also disconnecting it.

##### Hooks

The following list of codes are used by `SUB`, `UNSUB` and `HOOK`.

- `HOOK_ALL`       (`0x00`): Subscribes/unsubscribes too all existing hooks.
- `HOOK_NEWLOGIN`  (`0x01`): Triggers whenever a new user succesfully logs into the server. 
- `HOOK_NEWLOGOUT` (`0x02`): Triggers whenever a user either disconnects or logs out.
- `HOOK_DUPSESS`   (`0x03`): Triggers whenever an attempt to log into your account from another endpoint happens.
- `HOOK_PERMSCHG`  (`0x04`): Triggers whenever your account's permissions have changed

### Payload

It is important that no single argument is bigger than **2047 bytes**. This document will use the following notation to indicate the payload and command format for each **Action**:

    ACTION <arg_1> ... [arg_n] (Source -> Destination)

If the argument is put between `<>` it means it is **obligatory**, if it is put between `[]` it means it is **optional**. All optional arguments *must go at the end*, after the obligatory ones.

### Replies

The following exhaustive list specifies all possible replies for each command prerequisites that can be sent by the client:

- `REG`    -> `OK` or `ERR`
- `LOGIN`  -> `VERIF`, `OK` or `ERR`
- `VERIF`  -> `OK` or `ERR`
- `REQ`    -> `REQ` or `ERR`
- `USRS`   -> `USRS` or `ERR`
- `LOGOUT` -> `OK` or `ERR`
- `DISCN`  -> *No reply*
- `DEREG`  -> `OK` or `ERR`
- `MSG`    -> `OK` or `ERR`
- `RECIV`  -> `OK` or `ERR`
- `SUB`    -> `OK` or `ERR`
- `UNSUB`  -> `OK` or `ERR`
- `ADMIN`  -> `OK` or `ERR`
- `KEEP`   -> *No reply*

## Connection

When connecting to the server it is important to know that **any malformed packet** will automatically close the connection. Moreover, the server has a **10 minutes** deadline for receiving packets, after which the connection will close if nothing is received. A `KEEP` packet may be used to allow the connection to persist. Also, the server might be **unable to accept new clients** on the connection, in which case the connection **will await** until a spot is free. Once the client can be connected, an `OK` packet with a _Null ID_ will be sent to the client. It is important to note that the server can implement whatever method it wants for choosing which awaiting client should be connected next.

### Registering a user
The client application must create an RSA key pair and then send **both** the public key and username to the server, which will only error if the username or key is already in use. The RSA key pair should be **4096 bits** and be sent in `PKIX, ASN.1 DER` format to the server. Usernames should always be lowercase, this is to allow client implementations to use uppercase for other purposes.

`REG <username> <rsa_pub>` _Client_

### Verification handshake

The client must send its **username** to log into the server, additionally it can provide a **reusable token** to login, which will only be accepted if the connection is secure. If the token is not correct, an error will be replied.

`LOGIN <username> [token]` _Client_

If not using a **reusable token**, the server will reply with a **cyphered random text** to verify that the user owns the private key, or an error if no user with that username exists.

`VERIF <cypher_text>` _Server_

The client must return the decyphered text to the server.

`VERIF <username> <decyphered_text>` _Client_
> **NOTE**: The verification of the decyphered text has a 2 minutes timeout.

If the text is incorrect an error is replied. Any future commands from that user **will be tied to the connection** until the user disconnects or the server shuts down. This prevents someone else from logging in with the same account from a different location. If the connection is secure, the decyphered text will be stored in the server as a **reusable token**, which, in case of a disconnect, can be used when logging in again, effectively skipping the handshake process. Said token will also have an expiry date, after which the token will be deleted.

### User disconnection
Informs the server that the user should be marked as **offline**. No parameters apart from the action code are needed as the IP is tied to the user. The server must then **release the IP from the user**.

`LOGOUT` _Client_

## Communication

### Requesting connection with a user
To start messaging a user, the client application must request the public key from that user to the server. Said key can be cached by the client so the request is only made once per user. The server will reply with the public key and the permission level that user has.

`REQ <username>` _Client_

If the user does not exist an error will be given.

`REQ <username> <rsa_pub> <permission>` _Server_

### Listing all users

The client application can request a list of all users that are registered in that server. The argument should go in the header's information field, which will be checked with `1` (online users) or `0` (all users).

`USRS <online/all>` _Client_

The server will reply with a list of all users separated by the **newline character** (`\n`) (including the user that requested the list) or an error if no users exist.

`USRS <username_list>` _Server_

### Sending a message

Messages **should be cyphered** with the private key by the client application. The server is **not responsible** for verifying that the text is cyphered, nor that the public key for cyphering has been cached by the client.

`MSG <username> <unix_stamp> <cypher_message>` _Client_

The Server will reply with an error if the user is not found.
> **NOTE**: The operation correct reply does not imply that the other user has received the message, only that it has been sent.

### Receiving messages
If the user was offline and got new messages while offline, it can request a "**catch up**" after a succesful verification. In a "**catch up**" all messages are transferred to the client from the server. From that point onwards, the server is **no longer responsible of saving old messages** once they have been transferred. The client application can implement any method they want for storing messages locally. It is advised that the client application performs this operation right after a successful login.

`RECIV` _Client_

The server will reply with **as many packets as messages** are pending, sending an OK once all "**catch up**" messages have been sent.
> **NOTE**: `RECIV` may also be sent to receive a message while online, but in this case, said `RECIV` will come with a _Null ID_.

`RECIV <username> <unix_stamp> <cypher_message>` _Server_

### Deregistering a user
The client application can request the server to have a user deleted, but if said user had sent messages prior to its deregistration, the "**catch up**" **will still happen** for all users who have received messages. No parameters apart from the action code are needed as the IP is tied to the user.

`DEREG` _Client_

Once the deregistration has happened the server must then **release the IP tied to the user**.

## Permissions

By default, the protocol only requires a single level of permissions, `0`, which indicates the **lowest level** of permissions. The server is free to add more permission levels which *must be higher* than the lowest level. The server can decide what actions can be or not performed with a certain level of permissions.

## Hooks

Clients can request a **subscription** to an **event**, also called a **hook**. This means that whenever the event is triggered, a notification will be sent to the client application.

## Operations

### User accounts

User accounts in the server will be identified by a **lowercase username** and an **RSA Public Key**. Said key must be `4096` bits and whenever used as an argument, it must be in **PKIX, ASN.1 DER** format.

#### User registration

The client application must use an **RSA key pair** and then send both the public key and username to the server, only one *unique instance of a username or public key* must be allowed. Usernames must always be turned automatically into lowercase to allow client implementations to use uppercase for other purposes.

    REG <username> <rsa_pub> (Client -> Server)

#### Verification handshake

The client must send its **username** to log into the server, additionally it can provide a **reusable token** (explained below) to login, which must only be accepted if the connection has been secured with **TLS**.

    LOGIN <username> [token] (Client -> Server)

If not using a **reusable token**, the server will reply with a *random cyphered text* to verify that the user owns the private key.

    VERIF <cyphered_text> (Server -> Client)

The client must return the decyphered text to the server from the *same connection* from which the login process was initiated.

    VERIF <username> <decyphered_text> (Client -> Server)

> **NOTE**: The verification of the decyphered text should be implemented with a server-side timeout.

Any future commands from that user *must be tied to the connection* until the user logs out, disconnects or the server shuts down. This prevents someone else from logging in with the same account from a different location. If the connection is secure, the decyphered text will be stored in the server as a **reusable token**, which, in case of a disconnect, can be used when logging in again, effectively skipping the handshake process. Said token should also have an expiry date, after which the token must be deleted.

#### User disconnection

Informs the server that the user must be marked as **offline**. The server must then *release the connection from the user*. This command may also be used to *cancel an ongoing verification*. The user must be logged in to perform this operation.

    LOGOUT (Client -> Server)

#### Deregistering a user

A user can ask for its account to be deleted, but if said user had sent messages prior to its deregistration, those messages *will still be delivered*. The user must be logged in to perform this operation.

    DEREG (Client -> Server)

Once the deregistration has happened the server must then *release the connection tied to the user*.

### Message communication

#### Requesting connection with a user

To start messaging a user, the client application must request the **public key** of that user to the server. Said key can be saved by the client so the request is only made once per user. The user must be logged in to perform this operation.

    REQ <username> (Client -> Server)

The server will reply with the public key and the **permission level** (as an *integer*) of the requested user.

    REQ <username> <rsa_pub> <permission> (Server -> Client)

#### Listing all users

The client application can request a list of *all users* that are registered in that server. The argument should go in the header's **Information**, the list of available options is detailed above. The user must be logged in to perform this operation.

    USRS (Client -> Server)

The server will reply with a list of all users separated by the **newline character** (`\n`) (including the user that requested the list).

    USRS <username_list> (Server -> Client)

#### Sending a message

Messages *should be cyphered* with the private key by the client application. The server is *not responsible* for verifying that the text is cyphered, nor that the public key for cyphering has been saved by the client. The **timestamp** must be in standard *UNIX second timestamp* (which means `4 bytes`). The user must be logged in to perform this operation.

    MSG <username> <unix_stamp> <cypher_message> (Client -> Server)

> **NOTE**: The `OK` reply does not imply that the other user has received the message, only that it has been sent.

#### Receiving messages

When a new message is sent to the user a `RECIV` with a _Null ID_ will be sent by the server.

    RECIV <username> <unix_stamp> <cyphered_message> (Server -> Client)

If the user was offline and got new messages while offline, it can request a "**catch up**" after a succesful verification. In a "**catch up**" all messages are transferred to the client from the server. From that point onwards, the server is *no longer responsible of saving those messages* once they have been transferred. The client application can implement any method they want for storing messages locally. It is advised that the client application performs this operation *right after* a successful login. The user must be logged in to perform this operation.

    RECIV (Client -> Server)

The server will reply with *as many packets as messages* are pending.

### Miscellaneous

#### Administrative operations

To perform an administative operation, the following command must be used, specifying the operation to perform in the header's **Information** field. The list of available options is detailed above. The user must be logged in to perform this operation.

    ADMIN <arg_1> <arg_2> ... <arg_n> (Client -> Server)

The argument amount is not fixed and will depend on the action.

#### Subscriptions to events

Any client can request a subscription to a hook by indicating the hook in the header's **Information**. The list of available hooks is detailed above. The user must be logged in to perform this operation.

    SUB (Client -> Server)

In the same way, the client application can unsubscribe from any event for which they are subscribed. The user must be logged in to perform this operation.

    UNSUB (Client -> Server)

> **NOTE**: After a logout or a disconnection, all subscriptions the client may have made must be removed.

#### Triggering events

Whenever an event is triggered, the server will send a `HOOK` packet using the _Null ID_ with the corresponding hook in the header's **Information** field.

    HOOK (Server -> Client)