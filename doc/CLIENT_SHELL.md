# Client Shell
The client shell allows the client to send packets to the server, with full authority over the actions and arguments. The shell can also simultaneously listen for incoming packets to the client.

## Client commands
The client can perform the following actions as commands, following the format specified:

`VER`: Prints out the gochat version the client has installed.

`HELP`: Prints out a manual for the use of this shell.

`REG <rsa_pub> <username>`: Provides the generated RSA public key and username to register to the server.

`CONN <rsa_pub>`: Connects to the server by providing the already generated RSA public key.

`VERIF <decyphered_text>`: Replies to the server's verification request, providing the decyphered_text.

`REQ <username>`: Used to request a connection with another client in order to begin messaging.

`USRS <online/all>`: Requests the server a list of either the users online or all of them, depending on the token specified on the argument.

`MSG <username> <unix_stamp> <cypher_payload>`: Sends a message to the specified user, providing the specified UNIX timestamp and the payload, which is the chyphered text message.

`DISCN`: Disconnects the client from the server.

`DEREG`: Deregisters the user from the server.

`EXIT`: Closes the shell.