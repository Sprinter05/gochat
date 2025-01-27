# Specification
## Common Replies

### Server replies
- Server error reply: `ERR <error_code>`
- Server acknowledgment: `OK`
- Inminent server shutdown: `SHTDWN <unix_stamp>`

### Client replies
- Client error reply: `ERR <error_code>`
- Client acknowledgment: `OK`

## Connection
### Registering a user
The client application must create an RSA key pair and then send **both** the public key and username to the server, which must give an acknowledgment reply.

`REG <rsa_pub>\r\n<username>` _Client_

### Verification handshake

The client must send its public key to connect to the server:

`CONN <rsa_pub>` _Client_

The server will reply with a connection acknowledgment and a **cyphered random text** to verify that the user owns the private key, or an error if no user with that key exists:

`VERIF <cypher_text>` _Server_

The IP from the client application will now be cached into the server and **tied to the user** to avoid foreign IPs from authenticating with that user. The client must return the decyphered text to the server:

`VERIF <decyphered_text>` _Client_
> **NOTE**: The verification of the decyphered text has a 2 minutes timeout.

If the text is incorrect an error is replied, otherwise an acknowledgment will be sent. Any future commands from that user **will be tied to the IP** until the user disconnects or the server shuts down.

### User disconnection
Informs the server that the user should be marked as **offline**. No parameters apart from the action code are needed as the IP is tied to the user. The server can reply with an acknowledgment and must then **release the IP from the user**.

`DISCN` _Client_
> **NOTE**: The behaviour for marking a user as offline by default is not defined in this protocol specification.

## Communication

### Requesting connection with a user
To start messaging a user, the client application must request the public key from that user to the server. Said key can be cached by the client so the request is only made once per user:

`REQ <username>` _Client_

If the user exists, an acknowledgment will be replied, followed by the requested public key, otherwise an error will be given.

`REQ <rsa_pub>` _Server_

### Listing all users

The client application can request a list of all users that are registered in that server. The parameter is a **single byte** in which the *Least Significant Bit* will be checked with `1` (online users) or `0` (all users).

`USRS <online/all>` _Client_

The server will reply with an acknowledgment and then a list of all users separated by the **newline character** (`\n`) (not including the user that requested the list) or an error if no users exist.

`USRS <username_list>` _Server_

### Sending a message

Messages **should be cyphered** with the private key by the client application. The server is **not responsible** for verifying that the text is cyphered, nor that the public key for cyphering has been cached by the client.

`MSG <username> <unix_stamp>\r\n<cypher_payload>` _Client_

The Server will reply with an error if user not found, otherwise it will acknowledge that the message has been correctly sent.
> **NOTE**: The acknowledgment does not imply that the other user has received the message.

### Receiving messages
If the user was offline and got new messages while offline, the server will perform a "**catch up**" after a succesful verification from said user. In a "**catch up**" all messages are transferred to the client from the server. From that point onwards, the server is **no longer responsible of saving old messages** once they have been transferred. The client application can implement any method they want for storing messages locally.

`RECIV <username> <unix_stamp>\r\n<cypher_text>` _Server_

The client application should respond with an acknowledgment or an error in the case the message has not been correctly obtained, in which case the server will send the message again until the client application has acknowledged its receival.

> **NOTE**: The server can automatically delete the messages after a 5 minutes timeout once the message has been sent to the client application if no response is received.

### Deregistering a user
The client application can request the server to have a user deleted, but if said user had sent messages prior to its deregistration, the "**catch up**" **will still happen** for all users who have received messages. No parameters apart from the action code are needed as the IP is tied to the user.

`DEREG` _Client_

Once the deregistration has happened the server can reply with an acknowledgment and must then **release the IP tied to the user**.