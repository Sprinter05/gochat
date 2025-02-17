# Environment
CC = go
BUILD = build

.PHONY: clean
default: $(BUILD)/gcserver

# Create build folder if it doesn't exist
$(BUILD):
	if ! [ -d "./$(BUILD)" ]; then mkdir $(BUILD); fi

$(BUILD)/gcserver: $(BUILD)
	$(CC) build -o $(BUILD)/gcserver ./server 

# Clean build folder
clean: $(BUILD)
	rm -r $(BUILD)