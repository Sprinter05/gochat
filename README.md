# gochat
gochat is a light, **TCP**-based, client-server, **E2E*** (end-to-end) chat protocol. Said protocol is detailed in the [Specification](https://github.com/Sprinter05/gochat/blob/main/doc/SPECIFICATION.md).

## Content
This repository includes:
- The protocol [documentation](https://github.com/Sprinter05/gochat/blob/main/doc/SPECIFICATION.md)
- A client application with:
    - A **TUI** (terminal user interface)
    - A **shell** interface
- A **server** implementation with:
    - The **source code**
    - The [implementation](https://github.com/Sprinter05/gochat/blob/main/doc/IMPLEMENTATION.md)

## Stack
### Client stack:
The client's local database is a [SQLite](https://www.sqlite.org/) database handled by [GORM](https://gorm.io/index.html). The client terminal UI is made possible thanks to [tview](https://github.com/rivo/tview).
### Server stack:
The server database runs under [MariaDB](https://mariadb.org/), also handled by with [GORM](https://gorm.io/index.html).

## Packages
gochat is made up of the following packets:
- **internal:**
    - **log:** Implements a global **logging** system.
    - **models:** Implements **concurrently-safe data types**.
    - **spec:** Implements the common code between client and server that makes the **gochat protocol** and **implementation** work.
- **client:**
    - **commands:** Implements all the functionality required for **every client command** specified in the gochat protocol.
    - **db:** Implements the **database** connection management.
    - **ui**: Implements the **TUI** for the application.
    - **cli**: Implements the **shell** for the application.
- **server:**
    - **db:** Implements the **database** connection management.
    - **hubs**: Implements the functionalities required to **fulfill client requests**.

## Protocol
The gochat protocol is made to run underneath a **TCP** connection. The protocol also supports **TLS**, **RSA-encrypted** communication between clients, **administrative commands** and subscription-based **hooks** (also called events).

For more information on the protocol, be sure to read the [Specification](https://github.com/Sprinter05/gochat/blob/main/doc/SPECIFICATION.md) and [Implementation](https://github.com/Sprinter05/gochat/blob/main/doc/IMPLEMENTATION.md).

## Build
### Compiling
In order to compile the client or server you may use the `Makefile`. To build the server, in the root of the repository, run:

```bash
$ make server
```

Or alternatively, to build the client:

```bash
$ make client
```

> Note: You can compile both the client and server with `make all`, and delete the applications with `make clean`

Executing any of these commands will generate a `build/` directory if it wasn't created already. In it, the compiled binary executables will be generated.

## Running the server
#### Setting up the server database
The server requires an open MariaDB database. You may create a **MariaDB** service 
on your own, but this repository also includes a **Docker** stack with both the database and server.

You may compile the *server image* using the provided **Dockerfile** in `docker/Dockerfile`.

With `docker/compose.yml` you can run the stack. Install `docker-compose` if you need to (or **Docker** as a whole), `cd` into `docker/` and run:

```bash
$ docker compose up
```

or

```bash
$ docker compose up -d
```

to run it in detached mode.

#### (Optional) Creating TLS certificates
You may run the server in TLS mode creating the required TLS certificates. Use any prefered method you may have.

#### Configuring the server

TODO

## Running a client instance
To run a client instance you need to set up a configuration file first. There is an example configuration file in the repository: `client_config.json`.

Once the file is set up, you can open the terminal UI by executing:

```bash
$ build/gcclient
```

You can open the shell mode by executing:

```bash
$ build/gcclient --shell
```