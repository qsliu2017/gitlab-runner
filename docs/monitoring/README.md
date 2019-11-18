# GitLab Runner monitoring

GitLab Runner can be monitored using [Prometheus].

## Embedded Prometheus metrics

> The embedded HTTP Statistics Server with Prometheus metrics was
introduced in GitLab Runner 1.8.0.

The GitLab Runner is instrumented with native Prometheus
metrics, which can be exposed via an embedded HTTP server on the `/metrics`
path. The server - if enabled - can be scraped by the Prometheus monitoring
system or accessed with any other HTTP client.

The exposed information includes:

- Runner business logic metrics (e.g., the number of currently running jobs)
- Go-specific process metrics (garbage collection stats, goroutines, memstats, etc.)
- general process metrics (memory usage, CPU usage, file descriptor usage, etc.)
- build version information

The metrics format is documented in Prometheus'
[Exposition formats](https://prometheus.io/docs/instrumenting/exposition_formats/)
specification.

These metrics are meant as a way for operators to monitor and gain insight into
GitLab Runners. For example, you may be interested if the load average increase
on your runner's host is related to an increase of processed jobs or not. Or
you are running a cluster of machines to be used for the jobs and you want to
track build trends to plan changes in your infrastructure.

### Learning more about Prometheus

To learn how to set up a Prometheus server to scrape this HTTP endpoint and
make use of the collected metrics, see Prometheus's [Getting
started](https://prometheus.io/docs/prometheus/latest/getting_started/) guide. Also
see the [Configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)
section for more details on how to configure Prometheus, as well as the section
on [Alerting rules](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) and setting up
an [Alertmanager](https://prometheus.io/docs/alerting/alertmanager/) to
dispatch alert notifications.

## `pprof` HTTP endpoints

> `pprof` integration was introduced in GitLab Runner 1.9.0.

While having metrics about internal state of Runner process is useful
we've found that in some cases it would be good to check what is happening
inside of the Running process in real time. That's why we've introduced
the `pprof` HTTP endpoints.

`pprof` endpoints will be available via an embedded HTTP server on `/debug/pprof/`
path.

You can read more about using `pprof` in its [documentation][go-pprof].

## Configuration of the metrics HTTP server

> **Note:**
The metrics server exports data about the internal state of the
GitLab Runner process and should not be publicly available!

The metrics HTTP server can be configured in two ways:

- with a `listen_address` global configuration option in `config.toml` file,
- with a `--listen-address` command line option for the `run` command.

In both cases the option accepts a string with the format `[host]:<port>`,
where:

- `host` can be an IP address or a host name,
- `port` is a valid TCP port or symbolic service name (like `http`). We recommend to use port `9252` which is already [allocated in Prometheus](https://github.com/prometheus/prometheus/wiki/Default-port-allocations).

If the listen address does not contain a port, it will default to `9252`.

Examples of addresses:

- `:9252` - will listen on all IPs of all interfaces on port `9252`
- `localhost:9252` - will only listen on the loopback interface on port `9252`
- `[2001:db8::1]:http` - will listen on IPv6 address `[2001:db8::1]` on the HTTP port `80`

Remember that for listening on ports below `1024` - at least on Linux/Unix
systems - you need to have root/administrator rights.

Also please notice, that HTTP server is opened on selected `host:port`
**without any authorization**. If you plan to bind the metrics server
to a public interface then you should consider to use your firewall to
limit access to this server or add a HTTP proxy which will add the
authorization and access control layer.

[go-pprof]: https://golang.org/pkg/net/http/pprof/
[prometheus]: https://prometheus.io

## Using runner referees to ship extra job data to GitLab

> Runner referees were added in GitLab Runner 12.6.0.

Runner referees are special workers within the runner manager that query additional data related to a job and upload their results to GitLab as job artifacts.

### Using the metrics referee

External Prometheus instances that have been pre-configured to collect metrics from custom runner images that execute jobs can be automatically queried by GitLab Runner over the course of a job run. These queried metrics are shipped to GitLab as job artifacts at the end of each job for execution analysis.

#### Executors that support the metrics referee

- [`docker-machine`](../executors/docker_machine.md)

#### Configuration of the metrics referee for a runner

Define `[runner.referees]` and `[runner.referees.metrics]` in your `config.toml` file within a runner section and add the following fields.

| Setting              | Description                                                                                                                        |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `prometheus_address` | server that collects metrics from runner instances and must be accessible by the runnner manager when the job finishes             |
| `query_interval`     | how often the Prometheus instance associated with a job is queried for time series data                                            |
| `metric_queries`     | an array of [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics) queries that will be exectued for each interval |


Here is a complete configuration example for `node_exporter` metrics.

```
[[runners]]
  [runners.referees]
    [runners.referees.metrics]
      prometheus_address = "http://localhost:9090"
      query_interval = "10s"
      metric_queries = [
        "arp_entries:rate(node_arp_entries{{selector}}[{interval}])",
        "context_switches:rate(node_context_switches_total{{selector}}[{interval}])",
        "cpu_seconds:rate(node_cpu_seconds_total{{selector}}[{interval}])",
        "disk_read_bytes:rate(node_disk_read_bytes_total{{selector}}[{interval}])",
        "disk_written_bytes:rate(node_disk_written_bytes_total{{selector}}[{interval}])",
        "memory_bytes:rate(node_memory_MemTotal_bytes{{selector}}[{interval}])",
        "memory_swap_bytes:rate(node_memory_SwapTotal_bytes{{selector}}[{interval}])",
        "network_tcp_active_opens:rate(node_netstat_Tcp_ActiveOpens{{selector}}[{interval}])",
        "network_tcp_passive_opens:rate(node_netstat_Tcp_PassiveOpens{{selector}}[{interval}])",
        "network_receive_bytes:rate(node_network_receive_bytes_total{{selector}}[{interval}])",
        "network_receive_drops:rate(node_network_receive_drop_total{{selector}}[{interval}])",
        "network_receive_errors:rate(node_network_receive_errs_total{{selector}}[{interval}])",
        "network_receive_packets:rate(node_network_receive_packets_total{{selector}}[{interval}])",
        "network_transmit_bytes:rate(node_network_transmit_bytes_total{{selector}}[{interval}])",
        "network_transmit_drops:rate(node_network_transmit_drop_total{{selector}}[{interval}])",
        "network_transmit_errors:rate(node_network_transmit_errs_total{{selector}}[{interval}])",
        "network_transmit_packets:rate(node_network_transmit_packets_total{{selector}}[{interval}])"
      ]
```

Metric queries are in `cononical_name:query_string` format. The query string supports two variable spots that will be replaced during execution.

| Setting      | Description                                                                                                                           |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------- |
| `{selector}` | replaced with a `label_name=label_value` pair that uniquely selects metrics generated by a specific runner instance within Prometheus |
| `{interval}` | replaced with the `query_interval` parameter from the main configuration of this referee                                              |

For example, a shared runner environment using docker-machine `{selector}` would appear as `node=shared-runner-123`.
