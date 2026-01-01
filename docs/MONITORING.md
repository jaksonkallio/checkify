# Monitoring

After checks have been configured, and you set up Prometheus to scrape from the configured metrics endpoint, then you can utilize the check metrics in Grafana.

The following Prometheus metrics are exposed:
- `check_pass_latency_ms` histogram of passing check latencies with `name`, `environment`, `tags` labels.
- `check_evaluations_total` counter of total check evaluations with `name`, `environment`, `tags`, `pass` labels.
- `check_pass` gauge of check current pass/fail state with `name`, `environment`, `tags` labels.
- `check_last_evaluated_timestamp_seconds` gauge of last evaluated unix timestamp with `name`, `environment`, `tags` labels.

See the [demo](../demo/README.md) for examples of Grafana visualization panels using these metrics.
