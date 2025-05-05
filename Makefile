# Environment
CC=go
BUILD=build

.PHONY: clean
all: $(BUILD)/gcserver $(BUILD)/gcclient
server: $(BUILD)/gcserver
client: $(BUILD)/gcclient

# Create build folder if it doesn't exist
$(BUILD):
	if ! [ -d "./$(BUILD)" ]; then mkdir $(BUILD); fi

# We check the OS environment varible for the .exe extension
$(BUILD)/gcserver: $(BUILD)
ifeq ($(OS),Windows_NT)
	$(CC) build -o $(BUILD)/gcserver.exe ./server
else 
	$(CC) build -o $(BUILD)/gcserver ./server
endif

$(BUILD)/gcclient: $(BUILD)
	echo TBD

# Clean build folder
clean: $(BUILD)
	rm -r $(BUILD)

