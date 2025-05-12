# gochat
Light chat application with its own TCP protocol

gochat is a light, TCP-based Client-Server chat application that runs its own application protocol.

## Content
This repository includes:
- A client **terminal interface**
- A client **shell interface**
- The **server** source code

## Stack
### Client stack:
The client's local database is a [SQLite](https://www.sqlite.org/) database connected with [GORM](https://gorm.io/index.html). The client terminal UI is made possible thanks to [tview](https://github.com/rivo/tview).
### Server stack:
The server database is a [MariaDB](https://mariadb.org/) database, also connected with [GORM](https://gorm.io/index.html).

## Packages
gochat is made up of the following packets:
- **internal:**
    - **log:** Implements a global **log**.
    - **models:** Implements **concurrently-safe data types**.
    - **spec:** Implements the functionality that makes the **gochat protocol** work.
- **client:**
    - **commands:** Implements all the functionality required for **every client command** specified in the gochat protocol.
    - **db:** Implements the **client database** connection management.
    - **ui**: Implements the **Terminal UI** for the application.
    - **shell**: Implements the **client shell** for the application.
- **server:**
    - **db:** Implements the **server database** connection management.
    - **hubs**: Implements the functionalities required to **fulfill client requests**.

## Protocol
The gochat protocol is meant to run underneath a **TCP** connection. The protocol is able to handle **sessions** (with optional reusable tokens and **TLS** connections), client-to-client **RSA-encrypted communication**, **admin-exclusive commands** and subscription-based **hooks**.

For more information on the protocol, be sure to read the **protocol specification** in `doc/`.

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
The server requires an open MariaDB database. You may create a MariaDB service with `docker/docker-compose.yml`. Install `docker-compose` if you need to (or Docker as a whole if you're on Windows), `cd` into `docker/` and run:

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

#### Setting up the environment variables
Create a `.env` file with the following variables:

```
SRV_ADDR="127.0.0.1"
SRV_PORT="9037"
TLS_PORT="8037"
TLS_CERT="certs/gochat.pem"
TLS_KEYF="certs/gochat.key"
LOG_LEVL="ALL"
DB_LOGF="logs/db.log"
DB_USER="gochatuser"
DB_PSWD="gochatpass"
DB_ADDR="0.0.0.0"
DB_PORT="3306"
DB_NAME="gochat"
```

These values will fit the defined database user data values in `docker/docker-compose.yml`. Nevertheless, feel free to change them according to your needs. Do make sure the `TLS_CER`  and `TLS_KEYF` variables contain the appropiate path values according to your TLS certificates.

#### Execution
Once the database is up and running and the `.env` file created, you may run the server:

``` bash
$ build/gcserver <path-to-env-file>
```

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

## Protocol case examples
> Note: The case examples will be performed in a shell instance for visibility purposes.