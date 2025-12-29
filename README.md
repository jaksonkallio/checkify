Checkify enables declaratively define API checks in YAML across multiple environments. Check metrics can be scraped with Prometheus for visualization and alerting in Grafana.

See [demo/README.md](demo/README.md) for a demo.

## Features

- **Declarative YAML Configuration**: Define checks with endpoints, assertions, and intervals in YAML.
- **Multi-Environment Support**: Run the same checks across prod, stage, dev, etc.
- **Powerful Assertions**: Powerful expression language for asserting responses.
- **Monitoring Integration**: Check results are logged to stdout in JSON format and exposed as Prometheus metrics.

## Commands

- `evaluator` - Run checks repeatedly at their configured interval (default)
- `evaluate-once <check-name>` - Run a specific check once
- `validate` - Validate configuration file

## Configuration

See `config.template.yaml` for an complete example configuration.
