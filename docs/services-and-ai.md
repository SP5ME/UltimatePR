# Services, SSIDs and AI

The service layer is exposed by the single **Configuration → Application → Experimental features → Services** gate. NODE, BBS and AI are then enabled independently under **Configuration → Services**. Disabling a service prevents its listener, RF endpoint and NODE registration from being created. Re-enabling it uses the preserved service configuration after restart.

Default addresses are `CALL` (terminal, SSID 0), `CALL-2` (NODE), `CALL-7` (reserved CHAT/Convers), `CALL-8` (BBS), and `CALL-12` (AI). These are UltimatePR defaults, not a claimed AX.25 standard, and every implemented service SSID is configurable.

```text
RF / TCP
   |
Terminal / NODE
   |
Service Registry
   +-- BBS
   +-- AI
   +-- future services
```

`SERVICES` lists only registered, enabled services. From NODE use `BBS`, `AI`, or `C BBS` / `C AI`. Direct RF connections to the configured BBS or AI callsign reach the same backend objects. DIGI remains infrastructure and has no dedicated SSID.

## Ollama

UltimatePR does not run a model. It calls the remote Ollama `/api/chat` HTTP endpoint:

```text
AI Service -> HTTP -> Ollama API -> Model
```

Example configuration:

```yaml
experimental: {services: true, uprd: true, map: true}
server: {callsign: SP5ME, ssid: 2}
bbs: {enabled: true, callsign: SP5ME, ssid: 8}
ai:
  enabled: true
  callsign: SP5ME
  ssid: 12
  provider: ollama
  url: http://192.168.1.50:11434
  model: qwen3:8b
  timeout_seconds: 120
  max_context: 20
  max_response_chars: 2000
  concurrency: 1
  queue_size: 8
  system_prompt: "Answer concisely for packet radio."
```

Each RF/NODE connection owns its conversation context. Context is discarded on disconnect. Calls are bounded by timeout and concurrency; responses have Markdown reduced, a character limit, and RF pages controlled with `M` or `Q`.
