# Configuration

See [`config.template.yaml`](./config.template.yaml) for a complete example configuration.

## Interpolation

The configuration supports basic interpolation:
- `${env.SOME_VAR_NAME}` references a variable from the OS environment variables.
- `${var.some_var_name}` references a variable from the `vars` section of each configured environment.

## Syntax Checking and Inline Documentation

If you're using VS Code with the popular YAML extension by Red Hat, you can put the following at the top of your config file to get schema information and syntax checking!

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/jaksonkallio/checkify/refs/heads/master/config.schema.json
```

## Environments

An _environment_ represents a plane where services/hosts are operating. You may optionally configure arbitrary variables for each environment that are usable in check configuration. You must set up at least one environment.

In the following example, there are three environments that we decided to call `prod`, `stage`, and `external`. We might put our production services in `prod`, our staging services in `stage`, and maybe put some external dependency checks in the `external` environment. A check can be configured for one or more environments.

```yaml
environments:
  - name: prod
    vars:
      base_url: https://api.example.com
      my_api_token: ${env.EXAMPLE_PROD_API_TOKEN}
  - name: stage
    vars:
      base_url: https://api-stage.example.com
      my_api_token: ${env.EXAMPLE_STAGE_API_TOKEN}
  - name: external
```

If you don't have a use case for multiple environments (common with homelabs) or just want to start out simply, you can  set up a minimal config with just one environment. Just make sure to then assign all of your checks to this one environment.

```yaml
environments:
  - name: homelab
```

## Checks

Now that the environments have been configured, it's time to create the actual checks. There are several different types of checks:

- **HTTP Checks** run against an HTTP service endpoint. Assert pass/fail based on status code, parsed response body, and headers. The most common type of check.
- **TCP Checks** attempts to open a TCP connection to a specified host and port.
- **Ping Checks** sends pings to hosts. Assert pass/fail based on ping statistics. Note that ping checks require special permissions, see the dedicated Ping Check section below for more information.

In the config file, checks are defined like so:

```yaml
checks:
  http: # Array of HTTP checks here
  tcp: # Array of TCP checks here
  ping: # Array of ping checks here
```

Common to all check types are these configuration properties:

`name` string should be in a format that you're comfortable working with in downstream monitoring platforms. I recommend standardizing around using kebab-case or snake_case.

`environments` string array must contain at least one environment name.

`tags` string array can be whatever values you want (or omitted entirely). The use case for tags is to be able to passthrough metadata about the check to downstream monitoring platforms.

`interval` is the frequency at which the check should be evaluated (in Go `time.Duration` string format).

### HTTP Checks

#### Configuring HTTP Checks

```yaml
checks:
  http:
    - name: some-service
      environments: [prod, stage]
      method: GET
      endpoint: "${var.base_url}/api/ping"
      headers:
        Authorization: "Bearer ${var.my_api_token}"
      interval: 30s
      timeout: 5s
      tags: [health, some-service]
      response_format: string
      assertion: code == 200 && body == "pong"
```

`method` must be some REST method: `GET`, `POST`, `PUT`, `DELETE`, etc.

`endpoint` is the URL to send the HTTP request to.

`headers` is a string map of header name to header value for the request.

`timeout` will cancel the request and fail the check evaluation if exceeded.

`response_format` should be `json`, `string`, `binary`, or `none`.

`assertion` is a CEL expression that resolves to either `true` / `false`, indicating whether the assertion is true. Within the CEL expression, you may use the following variables:

- `code` is the status code (int) of the request.
- `body` is a map if `response_format: json`, or a string if `response_format: string`, or bytes if `response_format: binary`, or null if `response_format: none`.
- `headers` is a map with string keys and string values.

### TCP Checks

TCP checks attempt to establish a TCP connection to a specified host and port. The check passes if the connection is successfully established and fails otherwise. This is useful for verifying that services are listening on their expected ports.

#### Configuring TCP Checks

```yaml
checks:
  tcp:
    - name: github-ssh
      environments: [prod]
      host: github.com
      port: 22
      timeout: 5s
      interval: 1m
      tags: [health, external-service]
```

`host` is the hostname or IP address to connect to.

`port` is the TCP port number to connect to.

`timeout` will cancel the connection attempt and fail the check if exceeded.

### Ping Checks

#### Docker and Privileges
Ping checks require the Linux capability `CAP_NET_RAW`. This capability is not available in rootless Docker, and therefore ICMP ping checks cannot function in that mode.

This limitation exists because access to raw sockets is required, which the Linux kernel intentionally restricts to privileged containers for security reasons.

If `CAP_NET_RAW` is not available, ping checks will fail with an error like `socket: permission denied`. In such environments, consider using alternative check types (HTTP, TCP) instead to test reachability.

#### Configuring Ping Checks

```yaml
checks:
  ping:
    - name: google-dns-ipv6
      environments: [external]
      host: "2001:4860:4860::8888"
      count: 4
      interval: 1m
      timeout: 5s
      ip_stack: ipv6
      tags: [health, google, dns]
      assertion: packets_received >= packets_sent && avg_rtt_ms < 2000
```

`host` is the host domain name or IP address to ping.

`count` is how many pings to send.

`timeout` is the maximum time to wait for reply.

`ip_stack` must be either `ipv6`, `ipv4`, or `any`.

`assertion` is a CEL expression that resolves to either `true` / `false`, indicating whether the assertion is true. Within the CEL expression, you may use the following variables:

- `avg_rtt_ms` is the average round-trip time in milliseconds (int).
- `max_rtt_ms` is the maximum round-trip time in milliseconds (int).
- `min_rtt_ms` is the minimum round-trip time in milliseconds (int).
- `packet_loss_pct` is the packet loss percentage (double).
- `packets_received` is the number of packets received (int).
- `packets_received_dup` is the number of duplicate packets received (int).
- `packets_sent` is the number of packets sent (int).

## Monitoring

### Prometheus

Configure to expose Prometheus metrics:

```yaml
prometheus:
  port: 2112
  path: /metrics
```
