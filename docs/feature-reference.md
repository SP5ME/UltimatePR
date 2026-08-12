# Feature reference

| Function | LinBPQ | URONode | pyBBS | This project |
|---|---|---|---|---|
| KISS TCP | Integrated port driver feeding the central switch | Relies on Linux AX.25 plumbing | Not present | First transport; streaming codec and reconnecting client |
| AX.25 | Full link/session implementation | Kernel AX.25 through libax25 | Not present | Independent modulo-8 connected-mode client and incoming NODE/BBS sessions; one-frame window |
| Node | Rich command shell and application switch | Small, readable per-user node shell | BBS-like Telnet command UI | Minimal HELP/INFO/PORTS/MHEARD/USERS/BYE after sessions |
| MHEARD | Updated from observed port traffic | Reads Linux heard/proc facilities | Tracks Telnet users | Bounded in-memory store behind a persistence-ready interface |
| Monitor | Decodes traffic crossing ports | Uses system/kernel information | Application logging only | Bounded normalized RX/TX event stream |
| Terminal | Host/application terminal sharing node ports | External client/session process | Telnet BBS terminal | Browser packet terminal using the common session manager |
| Routing | Port, session and network routing | AX.25/NET/ROM/ROSE gateways | Mail topology BFS | Initially explicit port/session routing; mail routing later |
| BBS | Mature BPQMail with forwarding | External mailbox integration | Working SQLite mail prototype | Persistent private/bulletin mail, Telnet and incoming AX.25 access, forwarding queue |
| Forwarding | Text, FBB and compressed variants | Delegated to mailbox | Private FWD1 over TCP | B2F (`FC/FS/F>/FF/FQ`) and LZHUF implemented; classical fallback, secure login and LinBPQ cross-tests remain |
| AXIP/UDP | Multiple IP encapsulation modes | Not its primary transport | Plain TCP neighbours | AXUDP transport with optional FCS and peer filtering; raw AXIP protocol 93 remains separate |
