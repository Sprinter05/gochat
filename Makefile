# Environment
CC=go
BUILD=build

.PHONY: clean
all: $(BUILD)/gochat_server $(BUILD)/gochat_client
server: $(BUILD)/gochat_server
client: $(BUILD)/gochat_client

# Create build folder if it doesn't exist
$(BUILD):
	if ! [ -d "./$(BUILD)" ]; then mkdir $(BUILD); fi

# We check the OS environment varible for the .exe extension
$(BUILD)/gochat_server: $(BUILD)
ifeq ($(OS),Windows_NT)
	$(CC) build -o $(BUILD)/gochat_server.exe ./server
else 
	$(CC) build -o $(BUILD)/gochat_server ./server
endif

$(BUILD)/gochat_client: $(BUILD)
ifeq ($(OS),Windows_NT)
	$(CC) build -o $(BUILD)/gochat_client.exe ./client
else 
	$(CC) build -o $(BUILD)/gochat_client ./client
endif

# Clean build folder
clean: $(BUILD)
	rm -r $(BUILD)

