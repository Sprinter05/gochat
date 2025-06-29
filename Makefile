# Environment
CC=go
BUILD=build

# Server name
SERVERNAME=gochat-server
ifeq ($(OS),Windows_NT)
	SERVERNAME=gochat-server.exe
endif

# Client name
CLIENTNAME=gochat-client
ifeq ($(OS),Windows_NT)
	CLIENTNAME=gochat-client.exe
endif

# Versioning
VERSION:=$(shell date +%s)

.PHONY: clean
all: $(BUILD)/$(SERVERNAME) $(BUILD)/$(CLIENTNAME)
server: $(BUILD)/$(SERVERNAME)
client: $(BUILD)/$(CLIENTNAME)

# Create build folder if it doesn't exist
$(BUILD):
	if ! [ -d "./$(BUILD)" ]; then mkdir $(BUILD); fi

# We check the OS environment varible for the .exe extension
$(BUILD)/$(SERVERNAME): $(BUILD)
	$(CC) build -o $(BUILD)/$(SERVERNAME) \
	-ldflags "-X main.serverBuild=$(VERSION)" \
	./server

$(BUILD)/$(CLIENTNAME): $(BUILD)
	$(CC) build -o $(BUILD)/$(CLIENTNAME) \
	-ldflags "-X main.clientBuild=$(VERSION)" \
	./client

# Clean build folder
clean: $(BUILD)
	rm -r $(BUILD)

