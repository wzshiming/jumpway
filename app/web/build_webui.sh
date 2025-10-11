#!/bin/bash

# Build the React WebUI and copy to statics directory

set -e

cd "$(dirname "$0")"

echo "Building React WebUI..."
cd webui
npm run build

echo "Copying build output to statics/config..."
cd ..
rm -rf statics/config
cp -r webui/build statics/config

echo "WebUI built and deployed successfully!"
