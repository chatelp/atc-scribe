#!/bin/bash

# Build script for Co-ATC server
# This script builds a Linux binary for the detected CPU architecture.

echo "Creating bin directory, if it's not there. We build things, that's what we do."
mkdir -p bin

# Detect CPU architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)
        GOARCH="amd64"
        ;;
    aarch64|arm64)
        GOARCH="arm64"
        ;;
    armv7l)
        GOARCH="arm"
        ;;
    i686|i386)
        GOARCH="386"
        ;;
    *)
        echo "Unknown architecture: $ARCH. Defaulting to amd64."
        GOARCH="amd64"
        ;;
esac

# Build the server binary for Linux
echo "Building Co-ATC server for Linux $GOARCH... It's going to be a beautiful binary. Linux - the real deal!"
GOOS=linux GOARCH=$GOARCH go build -o bin/co-atc ./cmd/server

# Check if build was successful
if [ $? -eq 0 ]; then
    echo "Build successful! A tremendous success. The best Linux $GOARCH build, everyone agrees."
    
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