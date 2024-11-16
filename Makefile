CC=go

# Environment
BUILD = build
PKG = src

default: $(BUILD)/program

# Create build folder if it doesn't exist
$(BUILD):
	if ! [ -d "./$(BUILD)" ]; then mkdir $(BUILD); fi

$(BUILD)/program:
	$(CC) build -o build/program $(PKG)/main.go

# Clean build folder
clean: $(BUILD)
	rm -r $(BUILD)