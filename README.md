# Booru MCP

An MCP server, written in Go, for booru images.

The tools it will expose are not built yet. What exists so far is the server skeleton: configuration, bearer-token
authentication, the Streamable HTTP MCP endpoint, and the build and deployment plumbing.

## Usage

Deploy as a Docker image:

```sh
docker run -d \
  -p 8080:8080 \
  -e API_KEY=change-me \
  ghcr.io/wishmatic/booru-mcp:latest
```

The MCP endpoint is served at `/mcp`.

All other configuration is optional; see [.env.example](.env.example).

### Authentication

`API_KEY` is required on every request, sent as `Authorization: Bearer <API_KEY>`.

## License

Booru MCP is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).
