# Odyssey
KentikLabs synthetic Kubernetes operator.  This operator can be used to create
[Kentik synthetic tests](https://www.kentik.com/product/synthetics/) on a Kubernetes cluster.
The data can then be viewed in the [Kentik Portal](https://www.kentik.com).

![Odyssey Example](/hack/odyssey-example-portal.png?raw=true)

# Build
To build the operator:

```bash
make
```

This will build the protos and binary.

## Run Locally
To build and run the Go binary from your host outside of the cluster (for development):

```bash
make KENTIK_EMAIL=<your-kentik-email> KENTIK_API_TOKEN=<your-api-token> install run
```

## Image
To build a Docker image of the controller:

```bash
docker build -t <name> .
```

# Deploy
In order to see the task results in the Kentik Portal you will need to create a secret
that contains your user account and [API Token](https://portal.kentik.com/v4/profile).

Create the Odyssey namespace:

```
kubectl create ns odyssey-system
```

Create the Secret:

```
kubectl -n odyssey-system create secret generic --from-literal=email=user@kentik.com --from-literal=token=1234567890 kentik
```

Replace `user@kentik.com` and `1234567890` with your own email and token.

## Option 1: Deploy using Kubernetes manifest
To deploy using the latest manifest from `main`:

```
kubectl apply -f https://raw.githubusercontent.com/kentik/odyssey/main/deploy/odyssey.yaml
```

## Option 2: Deploy from Repository
To deploy to a Kubernetes cluster using the repo:

```
make IMG=<your-image> deploy
```

# Usage
To create and deploy a synth server to run tests, use the following example (change `<your-site-name>` to your
own site name you have configured in https://portal.kentik.com/v4/core/quick-views/sites):

```yaml
apiVersion: synthetics.kentiklabs.com/v1
kind: SyntheticTask
metadata:
  name: myservice-synth
spec:
  kentik_site: "<your-site-name>"
  fetch:
    - service: myservice
      target: /
      port: 8080
      method: GET
      period: 60s
      expiry: 5s
  ping:
    - name: myservice
      kind: service
      protocol: tcp
      port: 8080
      count: 1
      period: 60s
      expiry: 5s
      timeout: 1000
  trace:
    - name: myservice
      kind: service
      port: 8080
      limit: 5
      period: 60s
      count: 1
      timeout: 1000
      expiry: 5s
```

*Note*: you can optionally specify `server_image` and `agent_image` to override the
default images for the components.  This is useful for development.

This will inform the operator to launch a synth server and agent in the desired namespace
and configure the server with three tasks (`fetch`, `ping`, and `trace`). These
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
| `period` | optional (default: `60s`) | Interval to perform check |
| `expiry` | optional (default: `5s`) | Timeout for the check to complete|
| `ignoreTLSErrors` | optional (default: `false`) | Ignore TLS errors on request |

## Ping
Ping performs a network ping and latency, etc. This check can be used
with a Kubernetes `Pod`, `Deployment`, `Service`, or `Ingress`. The connection info
will be automatically resolved by the operator.

|Name      |Required  | Description|
|----------|----------|----------|
| `kind` | yes | Kubernetes object to check (`Deployment`, `Pod`, `Service`, `Ingress`)|
| `name` | yes | Name of the object to check |
| `protocol` | optional (default: `icmp`) | Protocol to use in the check (`icmp`, `tcp`, `udp`)|
| `port` | optional (default: `0`) | Port to use for the check|
| `count` | optional (default: `1`) | Number of tries for the check|
| `timeout` | yes (default: `1000`) | Timeout (in ms) for the check|
| `period` | optional (default: `60s`) | Interval to perform check |
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
| `timeout` | yes (default: `1000`) | Timeout (in ms) for the check|
| `limit` | optional (default: `3` min: `1` max: `50`) | Maximum number of hops|
| `period` | optional (default: `60s`) | Interval to perform check |
| `delay` | optional (default: `0ms`) | Delay (in ms) before each check|
| `expiry` | optional (default: `5s`) | Timeout for the check to complete|
