# gochat
gochat is a light, **TCP**-based, client-server, **E2E** (end-to-end) chat protocol. Said protocol is detailed in the [Specification](doc/SPECIFICATION.md).
![Banner](images/gochat_banner_transparent.png)

## Content
This repository includes:
- The protocol [documentation](doc/SPECIFICATION.md)
- A client application with:
    - A **TUI** (terminal user interface)
    - A **shell** interface
- A **server** implementation with:
    - The **source code**
    - The [implementation](doc/IMPLEMENTATION.md) details.

## Stack
### Client stack
The client's local database is a [SQLite](https://www.sqlite.org/) database handled by [GORM](https://gorm.io/index.html). The client **TUI** is made possible thanks to [tview](https://github.com/rivo/tview).
### Server stack
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

For more information on the protocol, be sure to read the [Specification](doc/SPECIFICATION.md) and [Implementation](doc/IMPLEMENTATION.md).

## Build
### Compiling
In order to compile the client or server you may use the `Makefile`. To build the any of them, in the root of the repository, run:

```bash
$ make <server/client/all>
```

> Note: You can delete the compiled applications by running `make clean`

Executing any of these commands will generate a `build/` directory if it wasn't created already. In it, the compiled binary executables will be generated.

### Running
Both client and server provide a `--help` argument to see the available parameters for the program. The configuration file will be loaded automatically if it exists in the current directory and is named `config.json`.
 
## Running a client instance
The client application can be ran by just using the executable. Necessary files will be created automatically by the application. We recommend reading the **Quickstart Guides** for the [TUI](doc/TUI.md) and [Shell](doc/SHELL.md).

### Where can I connect?
We have chosen to create our own server instance that you can connect to, hosted at `gochat.sprintervps.party` on port `8037` (TLS) and port `9037` (No TLS).

## Running the server
### Running under Docker
This repository provides a full **Compose** stack to run the server. You just need to manually compile the *Docker image* through the provided **Dockerfile** first using the following command (ran in the root directory):

```bash
docker build -t Sprinter05/gochat:latest .
```

After that you will need to run the compose stack using the file found in `docker/compose.yml` and running the following command:

```bash
docker compose up -d
```

Once the stack has been initialised you will find *3 new folders* created in the same directory used to run the stack. The `config` folder contains the `server.json` configuration file which can be used to modify the behaviour of the server (restart is required after changes to the configuration file), the `logs` folder contains all the relevant server logs, and the `certs` folder is an empty folder that must be used if you want the **TLS** functionality (you must provide both the private key and certificate in said folder, making sure the names are correct in the configuration file).

### Running manually
To run manually you must use a **MariaDB** database and modify the server's configuration file accordingly to be able to connect to it.

### Creating TLS certificates
You may run the server with a **TLS** port by creating the required **TLS** certificates. It is recommended to use `certbot` for creating your certificates, following the instructions of your domain provider. The required files generated using `certbot` will be `fullchain.pem` (`cert_file`) and `privkey.pem` (`key_file`). Other methods should also work, but are not documented here. Please note that *self-signed* certificates are also supported, but not recommended.

> Note: When connecting to the server through the TLS port you must use the domain and port combination instead of using ip and port or the TLS certificate will fail to load.
