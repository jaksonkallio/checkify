Declarative API checks in YAML, across multiple environments. Check metrics can be scraped with Prometheus for visualization and alerting in Grafana. 

See [demo/README.md](demo/README.md) for a demo.

![demo/screenshot.png](screenshot)

## Features

- **Declarative YAML Configuration**: Define checks with endpoints, assertions, and intervals in YAML.
- **Multi-Environment Support**: Run the same checks across prod, stage, dev, etc.
- **Powerful Assertions**: Uses [CEL](https://github.com/google/cel-go) for asserting responses.
- **Monitoring Integration**: Check results are logged to stdout in JSON format and exposed as Prometheus metrics.

## Configuration

See [`config.template.yaml`](./config.template.yaml) for a complete example configuration.

The configuration supports basic interpolation:
- `${env.SOME_VAR_NAME}` references a variable from the OS environment variables.
- `${var.some_var_name}` references a variable from the `vars` section of each configured environment.

Set up at least one environment:
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
```

The `vars` in each environment are arbitrarily defined by you, the configurer. Any `vars` you add will be usable within the check definitions.


Add at least one check:
```yaml
checks:
  - name: api-gateway-ping
    environments: [prod, stage]
    endpoint: "${var.base_url}/ping"
    method: GET
    headers:
      Authorization: "Bearer ${var.my_api_token}"
    interval: 30s
    timeout: 5s
    tags: [health, api-gateway]
    response_format: string
    assertion: |
      code == 200 &&
      body == "pong"
```

`name` should be in a format that you're comfortable working with in downstream monitoring platforms, consider using kabob-case or snake_case.

`environments` must contain at least one environment, or the check won't be evaluated at all.

`tags` is an arbitrary string array, they can be whatever values you want (or omitted entirely). The use case for tags is to be able to passthrough metadata about the check to downstream monitoring platforms.

`response_format` should be `json`, `string`, `binary`, or `none`. If `none`, the CEL variable `body` will be `null`.

`assertion` is a CEL expression that resolves to either `true` / `false`, indicating whether the assertion is true. Within the CEL expression, you may use the following variables:
- `code` is the status code (int) of the request.
- `body` is a map if `response_format: json`, or a string if `response_format: string`, or bytes if `response_format: binary`, or null if `response_format: none`.
- `headers` is a map with string keys and string values.

If you want to expose evaluated checks as Prometheus metrics, add that somewhere too:
```yaml
prometheus:
  port: 2112
  path: /metrics
```

## Monitoring

After checks have been configured, and you set up Prometheus to scrape from the configured metrics endpoint, then you can utilize the check metrics in Grafana.

The following Prometheus metrics are exposed:
- `check_pass_latency_ms` histogram of passing check latencies with `name`, `environment`, `tags` labels.
- `check_evaluations_total` counter of total check evaluations with `name`, `environment`, `tags`, `pass` labels.
- `check_pass` gauge of check current pass/fail state with `name`, `environment`, `tags` labels.
- `check_last_evaluated_timestamp_seconds` gauge of last evaluated unix timestamp with `name`, `environment`, `tags` labels.

See the [demo](./demo/README.md) for examples of Grafana visualization panels using these metrics.

## Commands

- `evaluator` - Run checks repeatedly at their configured interval (default)
- `evaluate-once <check-name>` - Run a specific check once
- `validate` - Validate configuration file
