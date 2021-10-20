# Odyssey
KentikLabs synthetic Kubernetes operator.

# Build
To build the operator:

```bash
make
```

This will build the protos and binary.

# Run Locally
To build and run the Go binary from your host outside of the cluster:

```bash
make install run
```

# Image
To build a Docker image of the controller:

```bash
docker build -t <name> .
```

# Deploy
To deploy to a Kubernetes cluster:

```
make IMG=<your-image> deploy
```
