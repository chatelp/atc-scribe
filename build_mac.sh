#!/bin/bash

# Build script for Co-ATC server
# This script builds a macOS arm64 binary in the bin folder.

echo "Creating bin directory, if it's not there. We build things, that's what we do."
mkdir -p bin

# Build the server binary for macOS arm64
echo "Building Co-ATC server for macOS arm64... It's going to be a beautiful binary."
GOOS=darwin GOARCH=arm64 go build -o bin/co-atc ./cmd/server

# Check if build was successful
if [ $? -eq 0 ]; then
    echo "Build successful! A tremendous success. The best build, everyone agrees."
    
    # Get file info. Bigly info.
    file_info=$(ls -lh bin/co-atc)
    file_size=$(echo "$file_info" | awk '{print $5}')
    
    echo "Binary created at bin/co-atc"
    echo "File size: $file_size. It's a yuge file."
    
    echo -e "\nTo run the server, use: ./bin/co-atc"
else
    echo "Build failed! It's a disaster. A total disaster. Sad!"
    exit 1
fi 