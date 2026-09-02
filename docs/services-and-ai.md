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

## Service Registry / lokalne uslugi

UltimatePR keeps a shared Service Registry for local services. The registry
matches internal dependencies by `service_id`, while callsign and SSID remain
the external AX.25 address.

- `node-main`, `bbs-main`, `chat-main`, and `game-main` are the default service IDs.
- An enabled local service registers itself after startup and unregisters on stop.
- If a local service is disabled or stopping, the resolver returns `service unavailable` instead of falling through to RF.
- The registry exposes service state and capabilities so NODE and BBS can ask
  what a service can do instead of checking whether it is a specific module.

## Ollama

UltimatePR does not run a model. It calls the remote Ollama `/api/chat` HTTP endpoint:

```text
AI Service -> HTTP -> Ollama API -> Model
```

Example configuration:

```yaml
experimental: {services: true, uprd: true, map: true}
server: {callsign: SP5ME, ssid: 2}
bbs:
  enabled: true
  callsign: SP5ME
  ssid: 8
  sysop_callsign: SP5ME
  max_sessions: 10
  hierarchical_address: BBS.SP5ME.POL.EURO
  new_user_message: "Witaj nowy uzytkowniku. Uzupelnij profil, aby korzystac z BBS."
  info_message: "{TITLE} [{CALL}]\r\nSYSOP: {SYSOP}\r\nAdres: {ADDRESS}"
  prompt: "{CALL}> "
  housekeeping:
    bulletin_retention_days: 90
    personal_retention_days: 180
    log_retention_days: 30
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

Outbound service sessions use the existing AX.25 Hub and Manager through a
common `Connect`, `Read`, `Write`, `Close`, and `Status` contract. The BBS
forwarder can use `transport: node` without a parallel AX.25 stack; incoming
frames return through the normal Dispatcher and internal return path.
