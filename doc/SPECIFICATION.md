# Specification
## Common Replies

### Server replies
- Operation succesfully performed: `OK`
- Error occurred: `ERR <error_code>`
- Inminent server shutdown: `SHTDWN <unix_stamp>`

> **NOTE**: All commands send by the client except `KEEP` will get a response from the server.

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

### Permission levels
By default, the protocol implements **2 levels** of permissions that start from `0`, which indicates the **lowest level** of permissions. The server can decide what actions can be or not performed with a certain level of permissions and can add **more permission levels**. They can be performed using the following command, indicating in the header information the action to be performed. The user must be logged in to perform administrative actions, assuming they have enough permissions.

`ADMIN <arg_1> <arg_2> ... <arg_n>` _Client_

The argument amount is not fixed and will depend on the action. The server may reply with an error.

## Hooks

Clients can request a **subscription** to an *event*, also called a **hook**. This means that whenever the event is triggered, a notification will be sent to the client application.

### Subscriptions
Any client can request a subscription to a hook by indicating the *hook number* in the header's information field. An error will be thrown if no such hook exists.

`SUB` _Client_

In the same way, the client application can unsubscribe from any event for which they are subscribed. An error will be thrown if the user is not subscribed to that event.

`UNSUB` _Client_

> **NOTE**: After a logout or a disconnection, all subscriptions of the client application will be automatically terminated.

### Events
Whenever an event is triggered, the server will send a packet using the _Null ID_ with the corresponding *hook number* in the header's information field.

`HOOK` _Server_