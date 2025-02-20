# Specification
## Common Replies

### Server replies
- Operation succesfully performed: `OK`
- Error occurred: `ERR <error_code>`
- Inminent server shutdown: `SHTDWN <unix_stamp>`

## Connection

When connecting to the server it is important to know that **any malformed packet** will automatically close the connection. Moreover, the server has a **10 minutes** deadline for receiving packets, after which the connection will close if nothing is received. A `KEEP` packet may be used to allow the connection to persist.

### Registering a user
The client application must create an RSA key pair and then send **both** the public key and username to the server, which will only error if the username or key is already in use. The RSA key pair should be **4096 bits** and be sent in `PKIX, ASN.1 DER` format to the server.

`REG <username> <rsa_pub>` _Client_

### Verification handshake

The client must send its public key to connect to the server.

`LOGIN <username>` _Client_

The server will reply with a **cyphered random text** to verify that the user owns the private key, or an error if no user with that username exists.

`VERIF <cypher_text>` _Server_

The client must return the decyphered text to the server.

`VERIF <username> <decyphered_text>` _Client_
> **NOTE**: The verification of the decyphered text has a 2 minutes timeout.

If the text is incorrect an error is replied. Any future commands from that user **will be tied to the connection** until the user disconnects or the server shuts down. This prevents someone else from logging in with the same account from a different location.

### User disconnection
Informs the server that the user should be marked as **offline**. No parameters apart from the action code are needed as the IP is tied to the user. The server must then **release the IP from the user**.

`LOGOUT` _Client_

## Communication

### Requesting connection with a user
To start messaging a user, the client application must request the public key from that user to the server. Said key can be cached by the client so the request is only made once per user.

`REQ <username>` _Client_

If the user does not exist an error will be given.

`REQ <username> <rsa_pub>` _Server_

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
If the user was offline and got new messages while offline, it can request a "**catch up**" after a succesful verification. In a "**catch up**" all messages are transferred to the client from the server. From that point onwards, the server is **no longer responsible of saving old messages** once they have been transferred. The client application can implement any method they want for storing messages locally.

`RECIV` _Client_

The server will reply with **as many packets as messages** are pending.

`RECIV <username> <unix_stamp> <cypher_message>` _Server_

### Deregistering a user
The client application can request the server to have a user deleted, but if said user had sent messages prior to its deregistration, the "**catch up**" **will still happen** for all users who have received messages. No parameters apart from the action code are needed as the IP is tied to the user.

`DEREG` _Client_

Once the deregistration has happened the server must then **release the IP tied to the user**.

## Permissions

### Permission levels
By default, the protocol implements **2 levels** of permissions that start from `0`, which indicates the **lowest level** of permissions. The server can decide what actions can be or not performed with a certain level of permissions and can add **more permission levels**. They can be performed using the following command, indicating in the header information the action to be performed. The user must be logged in to perform administrative actions, assuming they have enough permissions.

`ADMIN <arg_1> <arg_2> ... <arg_n>`

The argument amount is not fixed and will depend on the action. The server may reply with an error.