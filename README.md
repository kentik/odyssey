# Odyssey
KentikLabs synthetic Kubernetes operator.

# Build
To build the operator:

```bash
make
```

This will build the protos and binary.

## Run Locally
To build and run the Go binary from your host outside of the cluster (for development):

```bash
make install run
```

## Image
To build a Docker image of the controller:

```bash
docker build -t <name> .
```

# Deploy
To deploy to a Kubernetes cluster:

```
make IMG=<your-image> deploy
```

# Usage
To create and deploy a synth server to run tests, use the following example:

```yaml
apiVersion: synthetics.kentiklabs.com/v1
kind: SyntheticTask
metadata:
  name: demo
spec:
  fetch:
    - service: demo
      target: /
      port: 8080
      method: GET
      period: 30s
      expiry: 5s
  tls_handshake:
    - ingress: demo
      period: 30s
      expiry: 5s
  trace:
    - kind: deployment
      name: demo
      period: 30s
      expiry: 5s
```

*Note*: you can optionally specify `server_image` and `agent_image` to override the
default images for the components.  This is useful for development.

This will inform the operator to launch a synth server and agent in the desired namespace
and configure the server with three tasks (`fetch`, `tls_handshake`, and `trace`). These
will then be sent to the agent to perform at the specified intervals.

# Tasks
The following tasks are supported.

## Fetch
Fetch performs an HTTP operation on the specified Kubernetes service. The connection
info (IP, etc) will be automatically resolved by the operator.

|Name      |Required  | Description|
|----------|----------|----------|
| `service` | yes | Name of the Kubernetes service|
| `port` | yes | Port to check|
| `tls` | optional | Enable TLS for the check|
| `target` | yes | Path (i.e. `/`) for the check|
| `method` | optional (default: `GET`) | HTTP method to use for check |
| `period` | optional (default: `10s`) | Interval to perform check |
| `expiry` | optional (default: `5s`) | Timeout for the check to complete|

## TLS Handshake
The `tls_handshake` test performs a TLS handshake and reports info. This check requires
a Kubernetes Ingress object. The connection info will be automatically resolved by the operator.

|Name      |Required  | Description|
|----------|----------|----------|
| `ingress` | yes | Name of the Kubernetes ingress|
| `port` | optional (default: `443`) | Port to check|
| `period` | optional (default: `10s`) | Interval to perform check |
| `expiry` | optional (default: `5s`) | Timeout for the check to complete|

## Ping
Ping performs a network ping and latency, etc. This check can be used
with a Kubernetes `Pod`, `Deployment`, `Service`, or `Ingress`. The connection info
will be automatically resolved by the operator.

|Name      |Required  | Description|
|----------|----------|----------|
| `kind` | yes | Kubernetes object to check (`Deployment`, `Pod`, `Service`, `Ingress`)|
| `name` | yes | Name of the object to check |
| `count` | optional (default: `3`) | Number of tries for the check|
| `period` | optional (default: `10s`) | Interval to perform check |
| `delay` | optional (default: `0ms`) | Delay (in ms) before each check|
| `expiry` | optional (default: `5s`) | Timeout for the check to complete|

## Trace
Trace performs a network trace and reports hops, latency, etc. This check can be used
with a Kubernetes `Pod`, `Deployment`, `Service`, or `Ingress`. The connection info
will be automatically resolved by the operator.

|Name      |Required  | Description|
|----------|----------|----------|
| `kind` | yes | Kubernetes object to check (`Deployment`, `Pod`, `Service`, `Ingress`)|
| `name` | yes | Name of the object to check |
| `protocol` | optional (default: `udp`) | Protocol to use for the trace |
| `port` | optional (default: `0`) | Port to check|
| `count` | optional (default: `3`) | Number of tries for the check|
| `limit` | optional (default: `3` min: `1` max: `50`) | Maximum number of hops|
| `period` | optional (default: `10s`) | Interval to perform check |
| `delay` | optional (default: `0ms`) | Delay (in ms) before each check|
| `expiry` | optional (default: `5s`) | Timeout for the check to complete|

# InfluxDB
By default the agent will report the results to `stdout` of the container. To have them
sent to InfluxDB, use the following:

```yaml
apiVersion: synthetics.kentiklabs.com/v1
kind: SyntheticTask
metadata:
  name: demo
spec:
  influxdb:
    endpoint: "http://influx:8086/api/v2/write?bucket=default&org=system"
    username: "admin"
    password: "influxdb"
    token: "secret-token"
    organization: "system"
    bucket: "default"
```

|Name      |Required  | Description|
|----------|----------|----------|
| Endpoint | yes | InfluxDB endpoint. Must include the `/api/<version>/` to determine version.|
| Username | yes if no token | InfluxDB username|
| Password | yes if no token | InfluxDB password|
| Token | yes if no username/password | InfluxDB authentication token |
| Organization | yes | InfluxDB organization to send data|
| Bucket | yes | InfluxDB bucket to send data|
