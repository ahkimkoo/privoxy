[中文](./README_zh.md)

# Smart HTTP Proxy

This is a lightweight, intelligent HTTP proxy server written in Go. It automatically routes traffic based on the Gfwlist, sending requests for specified domains through a SOCKS5 proxy while allowing other traffic to connect directly.

## Features

- **Intelligent Routing**: Automatically proxies traffic for domains listed in the Gfwlist.
- **Custom Domains**: Allows users to specify their own domains to be proxied via a `domain.txt` file.
- **Configurable**: All settings can be managed through a `config.yaml` file.
- **Automatic Gfwlist Updates**: Keeps the Gfwlist rules up-to-date automatically based on a configurable schedule.

## How It Works

The proxy listens for incoming HTTP/HTTPS requests. When a request is received:
1.  It checks if the requested domain is in the custom `domain.txt` list.
2.  If not, it checks if the domain matches any rule in the Gfwlist.
3.  If the domain is found in either list, the request is forwarded through a pre-configured SOCKS5 proxy.
4.  Otherwise, the request connects directly to the destination server.

## Configuration

The server is configured using a layered approach. Settings are applied in the following order of priority:

1.  **Command-line Flags** (Highest priority)
2.  **`config.yaml` File**
3.  **Default Values** (Lowest priority)

This means you can use the `config.yaml` file for your base settings and override specific ones with command-line flags when you run the application.

### Default Values
- **Listen Address**: `:8118`
- **SOCKS5 Address**: `127.0.0.1:1080`
- **Update Frequency**: `24` hours

### 1. `config.yaml` (Optional)

You can create this file to override the default settings.

```yaml
# The address and port for the proxy to listen on
listen_addr: ":8080"

# The address and port of your SOCKS5 proxy server
socks5_addr: "127.0.0.1:7890"

# How often (in hours) to update the Gfwlist
update_frequency_hours: 24
```

### 2. `domain.txt`

This file allows you to add your own domains that you want to be routed through the SOCKS5 proxy. Add one domain per line. Comments can be added using `#`.

```
# Add custom domains here, one per line.
# Example:
my-custom-domain.com
another-one.net
```

## How to Run

### Prerequisites

- Go (version 1.18 or higher)
- A running SOCKS5 proxy server (if you intend to access proxied sites)

### Steps

1.  **Install dependencies**:
    ```bash
    go mod tidy
    ```

2.  **(Optional) Configure the proxy**:
    - Create a `config.yaml` file to set your base configuration.
    - Add any custom domains to `domain.txt`.

3.  **Run the server**:

    - **With default settings:**
      ```bash
      go run .
      ```

    - **Using settings from `config.yaml`:**
      (Simply create the file and run the command above)

    - **Overriding settings with command-line flags:**
      ```bash
      go run . -listen=":9000" -socks5="127.0.0.1:1086"
      ```

    - **Show help for all flags:**
      ```bash
      go run . -h
      ```

The proxy server will start, load the configuration, load custom domains, update the Gfwlist if necessary, and begin listening for connections.
