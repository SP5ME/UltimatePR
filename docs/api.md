# UltimatePR API v1

UltimatePR API is an optional, versioned, read-only telemetry interface for Home Assistant, dashboards, monitoring tools, bots, mobile clients and external AI orchestration. It is an adapter over UltimatePR core services: it does not expose KISS, AXUDP, sockets, raw AX.25 transmission or a separate TX queue.

## Enabling and securing the API

The API uses the existing Web UI HTTP listener, so it does not create another server. It is disabled when the `api` section is absent and must be explicitly enabled. For a local-only installation bind the shared listener to loopback:

```yaml
web:
  listen: 127.0.0.1:8080
  username: admin
  allowed_addresses: [127.0.0.1, "::1"]
api:
  enabled: true
  tokens:
    - name: home-assistant
      hash: "SHA256_HEX_OF_THE_TOKEN"
      scopes: [status.read, ports.read, mheard.read]
    - name: telemetry
      hash: "SHA256_HEX_OF_ANOTHER_TOKEN"
      scopes: [status.read, ports.read, mheard.read, monitor.read, sessions.read, events.read, node.read, bbs.read, digipeater.read]
```

The recommended setup is **Configuration → API** in the Web UI: choose the Home Assistant profile and create a token. Creating the first token also enables the API and restarts UltimatePR. The plaintext token is displayed only once; copy it directly to the client. Network binding and allowed LAN addresses remain under **Configuration → Network**.

Manual YAML and PowerShell generation remain available for unattended installations. Generate a long random token, store the plaintext only in the client, and put its SHA-256 digest in YAML:

```powershell
$token = -join ((1..48) | ForEach-Object { 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'[(Get-Random -Maximum 62)] })
$bytes = [Text.Encoding]::UTF8.GetBytes($token)
$hash = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($bytes)).ToLowerInvariant()
"TOKEN=$token`nHASH=$hash"
```

Bearer tokens are compared in constant time and are never logged. Supported read scopes are `status.read`, `ports.read`, `mheard.read`, `monitor.read`, `sessions.read`, `events.read`, `node.read`, `bbs.read`, and `digipeater.read`. `admin` grants every scope and should be reserved for controlled clients. CORS is not enabled. Browser WebSocket origins are limited to the listener's own origin; non-browser clients may omit `Origin`.

To expose the shared listener to a trusted LAN, change `web.listen` and restrict `web.allowed_addresses`. Put a TLS reverse proxy in front of UltimatePR before crossing an untrusted network. API v1 provides no write operations.

## First requests

Health is deliberately public and cheap, suitable for Docker, systemd and load balancers:

```bash
curl http://127.0.0.1:8080/api/v1/health
curl -H "Authorization: Bearer TOKEN" http://127.0.0.1:8080/api/v1/status
curl -H "Authorization: Bearer TOKEN" http://127.0.0.1:8080/api/v1/mheard
curl -H "Authorization: Bearer TOKEN" http://127.0.0.1:8080/api/v1/mheard/summary
curl -H "Authorization: Bearer TOKEN" http://127.0.0.1:8080/api/v1/ports
```

Swagger-style human documentation is available at `http://127.0.0.1:8080/api/docs`; the complete OpenAPI document is at `/api/openapi.yaml` and in [openapi.yaml](openapi.yaml).

## Monitor and WebSocket events

The monitor is bounded to at most 500 entries per response:

```bash
curl -H "Authorization: Bearer TOKEN" "http://127.0.0.1:8080/api/v1/monitor?limit=100&direction=rx&port=vhf"
```

Connect to `ws://127.0.0.1:8080/api/v1/events` with the same Bearer header. For example with `websocat`:

```bash
websocat -H="Authorization: Bearer TOKEN" ws://127.0.0.1:8080/api/v1/events
```

Events use one envelope:

```json
{"type":"frame.rx","timestamp":"2026-08-31T10:15:00Z","data":{"port":"vhf","source":"SP5ME","destination":"APRS","bytes":42}}
```

The first implementation emits `frame.rx`, `frame.tx`, and `digipeater.activity`. Each client has a bounded 64-event queue. A client that fills its queue is disconnected, so radio RX and TX processing never waits for an API consumer.

## Home Assistant and external AI

Home Assistant REST sensors can query `/api/v1/status`, `/api/v1/mheard/summary`, and `/api/v1/ports` with an `Authorization` header. Keep the token in `secrets.yaml`; use `value_json.active_sessions`, `value_json.heard_1h`, or the returned port items as sensor attributes. No custom component is required for basic polling.

An external Ollama bridge can subscribe to WebSocket events, decide outside UltimatePR whether an action is appropriate, and read context from REST. API v1 intentionally cannot transmit. A future write API must call `session.Hub`/Session Manager and the existing scheduler rather than sending to a transport directly.

## Versioning and errors

The public contract is under `/api/v1/`. Future incompatible changes belong under `/api/v2/`. Timestamps use RFC 3339 JSON encoding. Errors use HTTP status codes and a stable envelope:

```json
{"error":{"code":"not_found","message":"Port not found"}}
```

API DTOs are separate from radio/session internals. BBS status exposes only aggregate counts, never message bodies, users, passwords, filesystem paths or configuration secrets.
