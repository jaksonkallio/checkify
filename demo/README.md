This demo showcases Checkify running with Prometheus for metrics collection and Grafana for visualization.

Start the demo:
```bash
cd checkify/demo
docker compose up
```

This will set up three places you can see:
- Grafana dashboard: http://localhost:3000
  - Username: `admin`
  - Password: `admin`
  - Go to "Dashboards" → "Checkify Overview"
- Prometheus metrics exposed by Checkify: http://localhost:2112/metrics
- Prometheus web UI: http://localhost:9090
