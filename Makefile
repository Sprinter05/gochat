CC=go

# Environment
BUILD = build
PKG = src

default: $(BUILD)/program

# Create build folder if it doesn't exist
$(BUILD):
	if ! [ -d "./$(BUILD)" ]; then mkdir $(BUILD); fi

# Clean build folder
clean: $(BUILD)
	rm -r $(BUILD)