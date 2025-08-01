#!/bin/bash

# Exit on error
set -e

# Define the application name
APP_NAME="privoxy"

# Create release directory
RELEASE_DIR="release"
mkdir -p $RELEASE_DIR

# Define target platforms
PLATFORMS=("linux/amd64" "windows/amd64" "darwin/amd64" "darwin/arm64")

# Loop through platforms and build
for PLATFORM in "${PLATFORMS[@]}"
do
    # Split platform into OS and ARCH
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}

    # Set the output binary name
    OUTPUT_NAME="$APP_NAME"
    if [ $GOOS = "windows" ]; then
        OUTPUT_NAME="$APP_NAME.exe"
    fi

    echo "Building for $GOOS/$GOARCH..."

    # Build the binary
    env GOOS=$GOOS GOARCH=$GOARCH go build -o "$OUTPUT_NAME" .

    # Create a temporary directory for packaging
    PACKAGE_DIR="$RELEASE_DIR/${APP_NAME}_${GOOS}_${GOARCH}"
    mkdir -p "$PACKAGE_DIR"

    # Move binary and copy config files
    mv "$OUTPUT_NAME" "$PACKAGE_DIR/"
    cp "config.yaml" "$PACKAGE_DIR/"
    cp "domain.txt" "$PACKAGE_DIR/"

    # Create the zip archive
    (cd $RELEASE_DIR && zip -r "${APP_NAME}_${GOOS}_${GOARCH}.zip" "${APP_NAME}_${GOOS}_${GOARCH}")

    # Clean up the temporary package directory
    rm -r "$PACKAGE_DIR"

    echo "Successfully packaged for $GOOS/$GOARCH"
    echo "---"
done

echo "All builds completed successfully. Packages are in the '$RELEASE_DIR' directory."
