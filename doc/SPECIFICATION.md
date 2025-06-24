# The gochat Protocol

The **gochat protocol** is a chat protocol based on **TCP** designed for end-to-end (**E2E**) encrypted communication between users in the same server. This document specifies the **version 1** protocol in detail.

## Commands

The following diagram illustrates the format that a command travelling through the connection must have:

    +--------+------------+------+------------+
    | Header | Argument 1 | .... | Argument n |
    +--------+------------+------+------------+

The header msut be *8 bytes* long and both the separator between header and arguments (also called **payload**) and between each argument must be `\r\n`. The protocol also requires of a *trailing separator* after the last argument.

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
- **Identificator** | `10 bits`: Indicates the packet identification number the client has provided. The server's reply to a command must have the same identificator that was sent by the client in order to easily identify replies.
- **Reserved** | `16 bits`: For future extension that might be needed. (`0xFFFF` by default)

#### Special Codes

- **Null ID**: `0x0` is a reserved *Identificator* that may only be used by the server to send a command to the client that does not require a reply.
- **Null Action**: `0x0` is an invalid *Action Code* that cannot be used.
- **Empty Info**: `0xFF` is the value that should be used in the *Information* when the command does not have any additional information.

#### Action Codes

> **NOTE**: Not all actions can be used by both client and server.

- `OK`     | `0x01` (*Server only*)
- `ERR`    | `0x02` (*Server only*)
- `KEEP`   | `0x03` (*Client only*)
- `REG`    | `0x04` (*Client only*)
- `DEREG`  | `0x05` (*Client only*)
- `LOGIN`  | `0x06` (*Client only*)
- `LOGOUT` | `0x07` (*Client only*)
- `VERIF`  | `0x08`
- `REQ`    | `0x09`
- `USRS`   | `0x0A`
- `MSG`    | `0x0B` (*Client only*)
- `RECIV`  | `0x0C`
- `SHTDWN` | `0x0D` (*Server only*)
- `ADMIN`  | `0x0E` (*Client only*)
- `SUB`    | `0x0F` (*Client only*)
- `UNSUB`  | `0x10` (*Client only*)
- `HOOK`   | `0x11` (*Server only*)
- `HELLO`  | `0x12` (*Server only*)

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
- `ERR_CORRUPTED` (`0x14`): Data found is corrupted.
- `ERR_OPTION`    (`0x15`): Invalid option provided.
- `ERR_DISCN`     (`0x16`): Endpoint manually closed the connection.

##### Types of user lists

The following list of codes are used by `USRS`.

- `USRS_ALL`         (`0x0`): Show all usernames.
- `USRS_ONLINE`      (`0x1`): Show online usernames.
- `USRS_ALLPERMS`    (`0x2`): Show all usernames and permissions.
- `USRS_ONLINEPERMS` (`0x3`): Show online usernames and permissions.

##### Admin Operations

The following list of codes are used by `ADMIN`.

- `ADMIN_SHTDWN`   (`0x00`): Schedules a shutdown for the server.
- `ADMIN_DEREG`    (`0x01`): Deregistrates a specified user.
- `ADMIN_BRDCAST`  (`0x02`): Broadcasts a message to all online users.
- `ADMIN_CHGPERMS` (`0x03`): Changes the permission level of a user.
- `ADMIN_KICK`     (`0x04`): Kicks a user, also disconnecting it.
- `ADMIN_MOTD`     (`0x05`): Changes the MOTD of the server.

##### Hooks

The following list of codes are used by `SUB`, `UNSUB` and `HOOK`.

- `HOOK_ALL`       (`0x00`): Subscribes/unsubscribes too all existing hooks.
- `HOOK_NEWLOGIN`  (`0x01`): Triggers whenever a new user succesfully logs into the server. 
- `HOOK_NEWLOGOUT` (`0x02`): Triggers whenever a user either disconnects or logs out.
- `HOOK_DUPSESS`   (`0x03`): Triggers whenever an attempt to log into your account from another endpoint happens.
- `HOOK_PERMSCHG`  (`0x04`): Triggers whenever someone's permissions have changed.

### Payload

It is important that no single argument is bigger than **2047 bytes**. This document will use the following notation to indicate the payload and command format for each **Action**:

    ACTION <arg_1> <arg_2> ... [arg_n] (Source -> Destination)

If the argument is put between `<>` it means it is **obligatory**, if it is put between `[]` it means it is **optional**. All optional arguments *must go at the end*, after the obligatory ones.

### Replies

The following exhaustive list specifies all possible replies for each command prerequisites that can be sent by the client:

- `REG`    -> `OK` or `ERR`
- `LOGIN`  -> `VERIF`, `OK` or `ERR`
- `VERIF`  -> `OK` or `ERR`
- `REQ`    -> `REQ` or `ERR`
- `USRS`   -> `USRS` or `ERR`
- `LOGOUT` -> `OK` or `ERR`
- `DEREG`  -> `OK` or `ERR`
- `MSG`    -> `OK` or `ERR`
- `RECIV`  -> `OK` or `ERR`
- `SUB`    -> `OK` or `ERR`
- `UNSUB`  -> `OK` or `ERR`
- `ADMIN`  -> `OK` or `ERR`
- `KEEP`   -> *No reply*

## Connection

The connection to the server can be established using either **plain TCP** or **TLS** (implementation is optional), recommending the use of ports `9037` and `8037` respectively, although these can be changed.

When connecting to the server it is important to know that *any malformed packet* must automatically close the connection. It is recommended for the server to send a _Null ID_ `ERR` packet when a connection must be closed informing of the problem to the client, although it is not obligatory to do so. Moreover, the server should implement a **deadline** for receiving packets, after which the connection must close if nothing is received. A `KEEP` packet may be implemented to allow the connection to persist. 

The server can limit the amount of connected users, which means that when connection the server might be *unable to accept new clients* on the connection, in which case the connection should await until a spot is free. Once the client can be connected, an `HELLO` packet with a _Null ID_ must be sent to the client.

    HELLO <motd> (Server -> Client)

If a shutdown is scheduled, a `SHTDWN` packet with a _Null ID_ must be sent to all logged in users. Timestamps must be in byte integer format.

    SHTDWN <timestamp> (Server -> Client)

> **NOTE**: The server can implement whatever method it wants for choosing which awaiting client should be connected next.

## Permissions

By default, the protocol only requires a single level of permissions, `0`, which indicates the *lowest level* of permissions. The server is free to add more permission levels which *must be higher* than the lowest level. The server can decide what **administrative actions** can be or not performed with a certain level of permissions, this means that all operations that are not `ADMIN` must be able to be ran with the lowest level of permissions. Levels must also *be incremental*, meaning that higher levels of permissions must be able to run everything the lower levels can. Permission levels should be sent in arguments as a *single byte* meaning that levels can range from 0-255.

## Hooks

Clients can request a **subscription** to an **event**, also called a **hook**. This means that whenever the event is triggered, a notification must be sent to the client application.

## Operations

### User accounts

User accounts in the server must be identified by a **username** (can only contain lowercase letters and numbers, without spaces) and an **RSA Public Key**. Said key must be `4096` bits and whenever used as an argument, it must be in **PKIX, ASN.1 DER** format. Usernames cannot be changed but the server is free to decide how to handle usernames when an account is deleted (for example, allowing new users to register using that dangling username).

#### User registration

The client application must use an **RSA key pair** and then send both the public key and username to the server, only one *unique instance of a username or public key* must be allowed. Usernames must always be turned automatically into lowercase to allow client implementations to use uppercase for other purposes.

    REG <username> <rsa_pub> (Client -> Server)

#### Verification handshake

The client must send its **username** to log into the server, additionally it can provide a **reusable token** (explained below) to login, which must only be accepted if the connection has been secured with **TLS**.

    LOGIN <username> [token] (Client -> Server)

If not using a **reusable token**, the server must reply with a *random cyphered text* to verify that the user owns the private key.

    VERIF <cyphered_text> (Server -> Client)

The client must return the decyphered text to the server from the *same connection* from which the login process was initiated.

    VERIF <username> <decyphered_text> (Client -> Server)

> **NOTE**: The verification of the decyphered text should be implemented with a server-side timeout.

Any future commands from that user *must be tied to the connection* until the user logs out, disconnects or the server shuts down. This prevents someone else from logging in with the same account from a different location. If the connection is secure, the decyphered text must be stored in the server as a **reusable token**, which, in case of a disconnect, can be used when logging in again, effectively skipping the handshake process. This mechanism should only be activated *once the user has disconnected*. Said token should also have an **expiry date**, after which the token must be deleted. It is up to the server to allow for a token to be *used more than once*.

> **NOTE**: Reusable tokens must not be renewed after being used, meaning its expiry date cannot change.

#### User disconnection

Informs the server that the user must be marked as **offline**. The server must then *release the connection from the user*. This command may also be used to *cancel an ongoing verification*. The user must be logged in to perform this operation.

    LOGOUT (Client -> Server)

#### Deregistering a user

A user can ask for its account to be deleted, but if said user had sent messages prior to its deregistration, those messages *will still be delivered*. The user must be logged in to perform this operation.

    DEREG (Client -> Server)

Once the deregistration has happened the server must then *release the connection tied to the user*.

### Message communication

#### Requesting connection with a user

To start messaging a user, the client application must request the **public key** of that user to the server. Said key can be saved by the client so the request is only made once per user, although it is important to note that, as stated above, usernames can be reused by a new account once they become *dangling*. The user must be logged in to perform this operation.

    REQ <username> (Client -> Server)

The server must reply with the public key and the **permission level** (as an *integer*) of the requested user.

    REQ <username> <rsa_pub> <permission> (Server -> Client)

#### Listing all users

The client application can request a list of *all users* that are registered in that server. The argument should go in the header's **Information**, the list of available options is detailed above. The user must be logged in to perform this operation.

    USRS (Client -> Server)

The server must reply with a list of all users separated by the **newline character** (`\n`) (including the user that requested the list). If the requested type of listing *includes permissions* it must be in the format `<username> <permission>`.

    USRS <username_list> (Server -> Client)
    
> **NOTE**: There is no predefined way in which the user list should be sorted

#### Sending a message

Messages *should be cyphered* with the private key by the client application. The server is *not responsible* for verifying that the text is cyphered, nor that the public key for cyphering has been saved by the client. The **timestamp** must be in standard *UNIX second timestamp* (which means `4 bytes`). If the destination user is offline, the server is responsible for *caching the message* until it is requested by the destination. The user must be logged in to perform this operation.

    MSG <username> <unix_stamp> <cypher_message> (Client -> Server)

> **NOTE**: The `OK` reply does not imply that the other user has received the message, only that it has been sent.

#### Receiving messages

When a new message is sent to the user a `RECIV` with a _Null ID_ must be sent by the server.

    RECIV <username> <unix_stamp> <cyphered_message> (Server -> Client)

If the user was offline and got new messages while offline, it can request a "**catch up**" after a succesful verification. In a "**catch up**" all messages are transferred to the client from the server. From that point onwards, the server is *no longer responsible of saving those messages* once they have been transferred. The client application can implement any method they want for storing messages locally. It is advised that the client application performs this operation *right after* a successful login. The user must be logged in to perform this operation.

    RECIV (Client -> Server)

The server will reply with *as many packets as messages* are pending.

### Miscellaneous

#### Administrative operations

To perform an administative operation, the following command must be used, specifying the operation to perform in the header's **Information** field. The list of available options is detailed above. The user must be logged in to perform this operation.

    ADMIN <arg_1> <arg_2> ... <arg_n> (Client -> Server)

The argument amount is not fixed and will depend on the action. An exhaustive list of administrative operations and their arguments is detailed below:

- `ADMIN_SHTDWN <timestamp>`
- `ADMIN_DEREG <username>`
- `ADMIN_BRDCAST <message>`
- `ADMIN_CHGPERMS <username> <permission>`
- `ADMIN_KICK <username>`
- `ADMIN_MOTD <motd>`

> **NOTE**: Usage of `ADMIN_BRDCAST` requires TLS as the message must NOT be encrypted when being sent to the server.

#### Subscriptions to events

Any client can request a subscription to a hook by indicating the hook in the header's **Information**. The list of available hooks is detailed above. The user must be logged in to perform this operation.

    SUB (Client -> Server)

In the same way, the client application can unsubscribe from any event for which they are subscribed. The user must be logged in to perform this operation.

    UNSUB (Client -> Server)

> **NOTE**: After a logout or a disconnection, all subscriptions the client may have made must be removed.

#### Triggering events

Whenever an event is triggered, the server must send a `HOOK` packet using the _Null ID_ with the corresponding hook in the header's **Information** field. It will also include any relevant information for the hook.

    HOOK <arg_1> <arg_2> ... <arg_n> (Server -> Client)

The argument amount is not fixed and will depend on the action. An exhaustive list of administrative operations and their arguments is detailed below:

- `HOOK_NEWLOGIN <username> <permission>`
- `HOOK_NEWLOGOUT <username>`
- `HOOK_DUPSESS <ip>`
- `HOOK_PERMSCHG <username> <permission>`