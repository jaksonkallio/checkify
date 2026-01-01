# Contributing

## Scope

In scope:
- Support for multiple different check evaluation protocols (HTTP, ICMP ping, DNS, TCP, UDP).
- Evaluations test reachability and a pass/fail assertion using Common Expression Language (CEL).
- Storing in-memory pass/fail history of a particular check.
- Making check metrics available to monitoring platforms.
- Integration to alerting platforms for check pass/fail (without becoming the full alerting solution).
- Making current status of a particular check queryable by simple HTTP API.

Out of scope:
- Admin frontend web UI for monitoring, dashboard, visualization. Why: I believe that there are plenty of awesome monitoring, visualization, alerting, and dashboarding solutions out there already (Grafana, for example). It would be more efficient to simply focus on creating a system for evaluating checks, and make the results available to these monitoring platforms instead.
- Public frontend web UI for status page. Why: public status pages would introduce concerns around inbound public reachability, auth, performance/load, etc. I don't think this is the job of the same service that is evaluating checks. If you want to build a status page for these checks, perhaps consider building a small publicly accessible web app that renders HTML with metrics queried from Prometheus.
- Storing persistent database of check pass/fail history. Why: Checkify is currently a declarative config-based check evaluator, with very little state other than what is in memory. It would be great to keep the tool simple and stateless for as long as possible.
- Full alerting solution. Why: there's a bit of a blurry line here. I think its fine if we call webhooks for alerting platforms like Alertmanager, ntfy, gotify, or Apprise, but we shouldn't act as the actual alerting solution that tracks firing/resolved status (which requires persisted data) or manages delivery to contact points.
