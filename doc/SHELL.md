# gochat Shell Quickstart Guide
*(Last revision: 27-06-2025)*

The gochat client offers a simplified Command Line Interface in the form of a shell that offers a command-driven way of communicating with gochat servers, ideal for command testing.

## Run

In order to run gochat in its CLI mode, run:

```
./client -shell
```

This will open a shell:

```
(not connected) gochat() >
```

## In-shell Documentation

You can type "HELP" (commands are not case-sensitive) to get a list of every command and its information and usage. `HELP <command>` will only print information about the specified command.

## Connecting to the server

Ideally, the shell will automatically connect to the specified server in the configuration file. If that is not the case, you must connect to a server before performing any other operation:

```
(not connected) gochat() > CONN <address> <port>
[OK] succesfully connected to the server
[INFO] server MOTD (message of the day):
Welcome to the server!
[INFO] listening for incoming packets...
gochat() > 
```

## Registration and login

To execute most of the gochat protocol commands, you must be logged in to a user. You may create one with `REG`:

```
gochat() > REG
username: alice
password: 
repeat password: 
[...] generating RSA key pair...
[...] hashing password...
[...] performing registration...
[...] encrypting private key...
[OK] local user alice successfully added to the database
```

Log in to the user with `LOGIN`:

```
gochat() > LOGIN alice
alice's password: 
[...] checking password...
[...] decrypting private key...
[...] performing login...
[...] awaiting response...
[...] performing verification...
[...] awaiting response...
[...] verification successful
[OK] login successful!
Welcome, alice
[...] querying permissions...

[OK] logged in with permission level 0
gochat(alice) >
```

## Starting communication

You may want to know what external users are registered in order to communicate with them. You can do that with `USRS`

```
gochat(alice) > USRS all
[...] awaiting response...
all users:
alice
bob
```

In order to communicate with a user, you have to request it first to get their public key:

```
gochat(alice) > REQ bob
[...] awaiting response...
[OK] external user bob successfully added to the database
```

Now you're free to message Bob:

```
gochat(alice) > MSG bob hello!
[...] awaiting response...
[OK] message sent correctly
```

## Receiving messages

If someone messages you while you are logged in, the message will be printed automatically:

```
[2025-02-30 00:00:00 +0000 CEST] alice: hello!
gochat(bob) >
```

However, if Bob was not logged in when Alice sent the message, he will have to run the `RECIV` command in order to receive unread messages:

```
gochat(bob) > RECIV
[2025-02-30 00:00:00 +0000 CEST] alice: hello!
```

Be sure to read the repository documentation or use the `HELP` command to learn about what else you can do with gochat.

