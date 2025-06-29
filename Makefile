# Environment
CC=go
BUILD=build

# Set the extension and compiler options
# We need to set CGO if we want to compile for windows from linux

# Running under windows
ifeq ($(OS),Windows_NT)
	OS=windows
	PREFIX=.exe
	CGO=CGO_ENABLED=1 CXX=x86_64-w64-mingw32-g++ CC=x86_64-w64-mingw32-gcc
endif

# Compiling for windows
ifeq ($(OS), windows)
	PREFIX=.exe
	CGO=CGO_ENABLED=1 CXX=x86_64-w64-mingw32-g++ CC=x86_64-w64-mingw32-gcc
endif

# If compiling for linux we remove
ifeq ($(OS), linux)
	undefine PREFIX
	undefine CGO
endif

# Executable names
SERVERNAME=gochat-server$(PREFIX)
CLIENTNAME=gochat-client$(PREFIX)

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
	GOOS=$(OS) GOARCH=$(ARCH) $(CGO) \
	$(CC) build -o $(BUILD)/$(SERVERNAME) \
	-ldflags "-X main.serverBuild=$(VERSION)" \
	./server

$(BUILD)/$(CLIENTNAME): $(BUILD)
	GOOS=$(OS) GOARCH=$(ARCH) $(CGO) \
	$(CC) build -o $(BUILD)/$(CLIENTNAME) \
	-ldflags "-X main.clientBuild=$(VERSION)" \
	./client

# Clean build folder
clean: $(BUILD)
	rm -r $(BUILD)

