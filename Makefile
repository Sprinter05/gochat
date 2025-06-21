# Environment
CC=go
BUILD=build

.PHONY: clean
all: $(BUILD)/gochat-server $(BUILD)/gochat-client
server: $(BUILD)/gochat-server
client: $(BUILD)/gochat-client

# Create build folder if it doesn't exist
$(BUILD):
	if ! [ -d "./$(BUILD)" ]; then mkdir $(BUILD); fi

# We check the OS environment varible for the .exe extension
$(BUILD)/gochat-server: $(BUILD)
ifeq ($(OS),Windows_NT)
	$(CC) build -o $(BUILD)/gochat-server.exe ./server
else 
	$(CC) build -o $(BUILD)/gochat-server ./server
endif

$(BUILD)/gochat-client: $(BUILD)
ifeq ($(OS),Windows_NT)
	$(CC) build -o $(BUILD)/gochat-client.exe ./client
else 
	$(CC) build -o $(BUILD)/gochat-client ./client
endif

# Clean build folder
clean: $(BUILD)
	rm -r $(BUILD)

