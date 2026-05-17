# Deploying Packrune

Three ready-made paths.

## Docker

```bash
docker build -t packrune:dev .
docker run --rm -p 8080:8080 -v packrune-data:/data packrune:dev
```

The image is a multi-stage build: Node compiles the React frontend, Go
compiles the binary with the dist directory embedded, and the runtime stage
is `gcr.io/distroless/static-debian12:nonroot` (≈ 15 MB total).

A `docker-compose.yml` at the repo root brings up Packrune with sensible
defaults; uncomment the Postgres / MinIO blocks to switch to a
horizontally-scalable topology.

## Kubernetes (Helm)

```bash
helm install packrune deploy/helm/packrune \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=packrune.example.com
```

Pin a Postgres + S3-style backend to scale beyond a single replica. See
`values.yaml` for every knob.

## systemd

For VM / bare-metal deployments:

```bash
sudo cp deploy/systemd/packrune.service /etc/systemd/system/
sudo useradd --system --home /var/lib/packrune packrune
sudo install -d -o packrune -g packrune /var/lib/packrune /etc/packrune
sudo install -m 0755 ./bin/packrune /usr/local/bin/packrune
sudo systemctl daemon-reload
sudo systemctl enable --now packrune
```

A minimal `/etc/packrune/packrune.yaml`:

```yaml
server:
  addr: ":8080"
database:
  driver: sqlite
  dsn: /var/lib/packrune/packrune.db
storage:
  backend: fs
  fs:
    root: /var/lib/packrune/storage
log:
  level: info
  format: json
```

## Dogfooding

The Helm chart in `deploy/helm/packrune/` is itself a `helm install`-able
chart. Once Packrune is running, push the chart to your own Packrune Helm
repo with `helm cm-push deploy/helm/packrune http://your-host/helm/` — yes,
Packrune ships from Packrune.
