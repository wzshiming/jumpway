# Jump Way WebUI

This directory contains the React-based configuration editor for Jump Way.

## Development

To start the development server:

```bash
npm install
npm start
```

The development server will run on http://localhost:3000 and proxy API requests to http://localhost:8080.

## Building

To build the production version:

```bash
npm run build
```

The build output will be in the `build/` directory.

## Deployment

After building, copy the build directory contents to `../statics/config/`:

```bash
cp -r build/* ../statics/config/
```

The Go application will automatically serve the static files from the embedded file system.

## Features

- View and edit Jump Way configuration
- Manage contexts and their routing configurations
- Configure proxy settings
- Manage no-proxy lists
- Real-time configuration updates via REST API
