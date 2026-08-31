package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	appconfig "github.com/packet-radio/ultimatepr/internal/config"
	"github.com/packet-radio/ultimatepr/internal/mheard"
	"github.com/packet-radio/ultimatepr/internal/monitor"
	"github.com/packet-radio/ultimatepr/internal/session"
	"github.com/packet-radio/ultimatepr/internal/terminalcodec"
)

func TestConfigRestartRequired(t *testing.T) {
	current := appconfig.Config{}
	languageOnly := current
	languageOnly.Application.Language = "en"
	if configRestartRequired(current, languageOnly) {
		t.Fatal("language-only change requires restart")
	}
	channelOnly := current
	channelOnly.Application.UpdateChannel = "dev"
	if configRestartRequired(current, channelOnly) {
		t.Fatal("update-channel-only change requires restart")
	}
	stationChange := current
	stationChange.Terminal.Callsign = "SP5ME"
	if !configRestartRequired(current, stationChange) {
		t.Fatal("station change did not require restart")
	}
}

func TestConfigModelPutPersistsGameHallToggle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	c := appconfig.New(appconfig.ModeStation, "SP5ME", "", "", "pl", 0, 2, 8)
	if err := appconfig.SaveModel(path, c); err != nil {
		t.Fatal(err)
	}
	c.GameHall.Enabled = true
	c.GameHall.Callsign = "SP5ME"
	c.GameHall.SSID = 14
	c.GameHall.Language = "pl"
	c.GameHall.InviteTimeoutSeconds = 120
	body, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	s := New(Config{ConfigPath: path}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	s.configModelPut(recorder, httptest.NewRequest(http.MethodPut, "/api/config/model", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	saved, err := appconfig.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.GameHall.Enabled || saved.GameHall.Callsign != "SP5ME" || saved.GameHall.SSID != 14 {
		t.Fatalf("saved Game Hall=%+v", saved.GameHall)
	}
}

func TestConfigModelPutPreservesAPITokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	c := appconfig.New(appconfig.ModeStation, "SP5ME", "", "", "pl", 0, 2, 8)
	c.API.Enabled = true
	c.API.Tokens = []appconfig.APIToken{{Name: "ha", Hash: strings.Repeat("a", 64), Scopes: []string{"status.read"}}}
	if err := appconfig.SaveModel(path, c); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	s := New(Config{ConfigPath: path}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	s.configModelPut(w, httptest.NewRequest(http.MethodPut, "/api/config/model", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	saved, err := appconfig.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.API.Tokens) != 1 || saved.API.Tokens[0].Hash != strings.Repeat("a", 64) {
		t.Fatalf("API tokens were not preserved: %+v", saved.API.Tokens)
	}
}

func TestAPITokenCreateStoresOnlyHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	c := appconfig.New(appconfig.ModeStation, "SP5ME", "", "", "pl", 0, 2, 8)
	if err := appconfig.SaveModel(path, c); err != nil {
		t.Fatal(err)
	}
	s := New(Config{ConfigPath: path}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	s.apiTokenCreate(w, httptest.NewRequest(http.MethodPost, "/api/application/api-tokens", strings.NewReader(`{"name":"home-assistant","scopes":["status.read","mheard.read"]}`)))
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var response struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Token == "" {
		t.Fatalf("response=%s", w.Body.String())
	}
	saved, err := appconfig.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.API.Enabled || len(saved.API.Tokens) != 1 {
		t.Fatalf("API config=%+v", saved.API)
	}
	h := sha256.Sum256([]byte(response.Token))
	if saved.API.Tokens[0].Hash != hex.EncodeToString(h[:]) {
		t.Fatal("stored hash does not match generated token")
	}
	if strings.Contains(string(mustRead(t, path)), response.Token) {
		t.Fatal("plaintext token was written to configuration")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestGameHallControlsParticipateInConfigWorkflow(t *testing.T) {
	workflow, err := assets.ReadFile("static/config-workflow.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"c.GameHall", "gameHallServiceEnabled", "gameHallCallsign", "gameHallSSID", "gameHallLanguage", "gameHallInviteTimeout"} {
		if !bytes.Contains(workflow, []byte(required)) {
			t.Fatalf("configuration workflow does not contain %q", required)
		}
	}
	styles, err := assets.ReadFile("static/discovery-fixes.css")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(styles, []byte("#experimentalFeaturesCard.config-section-card{display:block")) {
		t.Fatal("experimental features card does not override the legacy grid layout")
	}
}

func TestStatusReportsDirectAIService(t *testing.T) {
	s := New(Config{AICallsign: "SP5ME", AISSID: 12, AIEnabled: true}, nil)
	recorder := httptest.NewRecorder()
	s.status(recorder, httptest.NewRequest("GET", "/api/status", nil))
	var status map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status["ai"] != "SP5ME-12" || status["ai_enabled"] != true {
		t.Fatalf("AI status=%v", status)
	}
}

func TestStatusReportsGameHallService(t *testing.T) {
	s := New(Config{GameHallCallsign: "SP5ME", GameHallSSID: 14, GameHallEnabled: true}, nil)
	recorder := httptest.NewRecorder()
	s.status(recorder, httptest.NewRequest("GET", "/api/status", nil))
	var status map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status["game_hall"] != "SP5ME-14" || status["game_hall_enabled"] != true {
		t.Fatalf("Game Hall status=%v", status)
	}
}

func TestAIModelsConnectsAndReturnsOllamaTags(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"models":[{"name":"qwen3:8b"},{"name":"gemma3:4b"}]}`)
	}))
	defer ollama.Close()

	s := &Server{}
	w := httptest.NewRecorder()
	s.aiModels(w, httptest.NewRequest(http.MethodPost, "/api/ai/models", strings.NewReader(`{"url":"`+ollama.URL+`"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var response struct {
		Connected bool     `json:"connected"`
		Models    []string `json:"models"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if !response.Connected || len(response.Models) != 2 || response.Models[0] != "qwen3:8b" {
		t.Fatalf("response=%+v", response)
	}
}

func TestBeaconEndpointAlwaysSendsClassicBeacon(t *testing.T) {
	beaconCalls, uprdCalls := 0, 0
	s := &Server{cfg: Config{
		SendBeacon: func(context.Context) error { beaconCalls++; return nil },
		SendUPRD:   func(context.Context) error { uprdCalls++; return nil },
	}}
	w := httptest.NewRecorder()
	s.beacon(w, httptest.NewRequest("POST", "/api/beacon", nil))
	if w.Code != 204 || beaconCalls != 1 || uprdCalls != 0 {
		t.Fatalf("beacon endpoint calls = beacon:%d uprd:%d status:%d", beaconCalls, uprdCalls, w.Code)
	}
}

func TestUPRDEndpointSendsOnlyUPRDStatus(t *testing.T) {
	beaconCalls, uprdCalls := 0, 0
	s := &Server{cfg: Config{
		SendBeacon: func(context.Context) error { beaconCalls++; return nil },
		SendUPRD:   func(context.Context) error { uprdCalls++; return nil },
	}}
	w := httptest.NewRecorder()
	s.uprdSend(w, httptest.NewRequest(http.MethodPost, "/api/uprd", nil))
	if w.Code != http.StatusNoContent || beaconCalls != 0 || uprdCalls != 1 {
		t.Fatalf("UPRD endpoint calls = beacon:%d uprd:%d status:%d", beaconCalls, uprdCalls, w.Code)
	}
}

func TestMHeardDirectExcludesUPRDReports(t *testing.T) {
	store := mheard.New(10)
	store.Heard("SP1AAA", "radio")
	store.Reported([]string{"SP2BBB"}, "SP3CCC", "radio")
	s := &Server{cfg: Config{MHeard: store}}
	w := httptest.NewRecorder()
	s.mheard(w, httptest.NewRequest("GET", "/api/mheard?mode=direct", nil))
	var entries []mheard.Entry
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Callsign != "SP1AAA" {
		t.Fatalf("direct MHEARD = %+v", entries)
	}
}

func TestValidateWebListenerRejectsUnavailableAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := validateWebListener("127.0.0.1:8080", listener.Addr().String()); err == nil {
		t.Fatal("unavailable web listener address accepted")
	}
}

func TestValidateWebListenerAcceptsAvailableAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	if err := validateWebListener("127.0.0.1:8080", address); err != nil {
		t.Fatalf("available web listener address rejected: %v", err)
	}
}

func TestTerminalMessageFromEventUsesDataForPayload(t *testing.T) {
	msg := terminalMessageFromEvent(session.Event{Type: "data", Data: []byte("Hello\r\n")})
	if msg.Type != "data" {
		t.Fatalf("Type = %q, want %q", msg.Type, "data")
	}
	if msg.Data != "Hello\r\n" {
		t.Fatalf("Data = %q, want %q", msg.Data, "Hello\r\n")
	}
	if msg.Error != "" {
		t.Fatalf("Error = %q, want empty", msg.Error)
	}
}

func TestTerminalMessageFromEventFallsBackToMessage(t *testing.T) {
	msg := terminalMessageFromEvent(session.Event{Type: "state", State: session.Disconnected, Message: "Rozlaczono."})
	if msg.Type != "state" {
		t.Fatalf("Type = %q, want %q", msg.Type, "state")
	}
	if msg.State != string(session.Disconnected) {
		t.Fatalf("State = %q, want %q", msg.State, session.Disconnected)
	}
	if msg.Data != "Rozlaczono." {
		t.Fatalf("Data = %q, want %q", msg.Data, "Rozlaczono.")
	}
}

func TestTerminalTemplateExpansion(t *testing.T) {
	cfg := Config{TerminalCallsign: "SP5ME", TerminalSSID: 0, OperatorName: "Jan", ApplicationLocator: "KO02JD", ApplicationQTH: "Warszawa"}
	got := terminalReplyText("Call: {CALL}\r\nImię: {NAME}\r\nLOC: {LOC}\r\nQTH: {QTH}\r\n", terminalMacroContext(callsign(cfg.TerminalCallsign, cfg.TerminalSSID), "SQ9ABC", cfg))
	want := "Call: SP5ME\r\nImię: Jan\r\nLOC: KO02JD\r\nQTH: Warszawa\r\n"
	if got != want {
		t.Fatalf("expanded template = %q, want %q", got, want)
	}
	blank := Config{TerminalCallsign: "SP5ME", TerminalSSID: 0, ApplicationQTH: "Warszawa"}
	cleaned := terminalReplyText("Imię: {NAME}\r\nQTH: {QTH}\r\n", terminalMacroContext(callsign(blank.TerminalCallsign, blank.TerminalSSID), "SQ9ABC", blank))
	if strings.Contains(cleaned, "Imię:") || !strings.Contains(cleaned, "QTH: Warszawa") {
		t.Fatalf("blank macro handling failed: %q", cleaned)
	}
}

func TestAutomaticWelcomePayloadEndsWithCRLF(t *testing.T) {
	text := terminalReplyText("Hello, for help type: /h", nil)
	codec, err := terminalcodec.New(terminalcodec.Default)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := codec.Encode(text)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("Hello, for help type: /h\r\n")
	if !bytes.Equal(payload, want) {
		t.Fatalf("welcome payload = % X, want % X", payload, want)
	}
	if bytes.HasSuffix(payload, []byte("\r\n\r\n")) || bytes.HasSuffix(payload, []byte("\n\n")) {
		t.Fatalf("welcome payload has duplicated line ending: % X", payload)
	}
}

func TestExpandTerminalMessageAtSendTime(t *testing.T) {
	cfg := Config{TerminalCallsign: "SP5ME", TerminalSSID: 3, OperatorName: "Miki", ApplicationLocator: "KO02MD", ApplicationQTH: "Warszawa"}
	values := terminalMacroContext(callsign(cfg.TerminalCallsign, cfg.TerminalSSID), "SQ9ABC-7", cfg)

	got := expandTerminalMessage("Czesc {REMOTE}, tu {NAME} z {QTH}, {LOC}, de {CALL}.\r\n", values)
	want := "Czesc SQ9ABC-7, tu Miki z Warszawa, KO02MD, de SP5ME-3.\r\n"
	if got != want {
		t.Fatalf("send-time macro expansion = %q, want %q", got, want)
	}

	plain := "Tekst {NIEZNANE} pozostaje bez zmian.\r\n"
	if got := expandTerminalMessage(plain, values); got != plain {
		t.Fatalf("plain message changed: %q", got)
	}
}

func TestPrepareTerminalMessagePreservesUTF8(t *testing.T) {
	values := map[string]string{"NAME": "Mikołaj"}
	got := prepareTerminalMessage("Zażółć gęślą jaźń, {NAME}!\r\n", values)
	want := "Zażółć gęślą jaźń, Mikołaj!\r\n"
	if got != want {
		t.Fatalf("prepared terminal message = %q, want %q", got, want)
	}
}

func TestMonitorClearEndpoint(t *testing.T) {
	store := monitor.New(4)
	pid := byte(0xF0)
	store.Add("RX", "radio", ax25.Frame{Destination: ax25.Address{Callsign: "BEACON"}, Source: ax25.Address{Callsign: "SP5ME"}, Type: ax25.TypeUI, PID: &pid}, 16)
	s := &Server{cfg: Config{Monitor: store}}
	w := httptest.NewRecorder()
	s.monitorClear(w, httptest.NewRequest("DELETE", "/api/monitor", nil))
	if w.Code != 204 {
		t.Fatalf("status=%d", w.Code)
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("monitor entries=%d", len(got))
	}
}

func TestTerminalRemoteCommandsRequireSlashAndSeparateLine(t *testing.T) {
	tests := map[string]string{
		"/i":        "info",
		"/v":        "version",
		" /MH ":     "mheard",
		"/h":        "help",
		"/?":        "help",
		"I":         "",
		"INFO":      "",
		"MH":        "",
		"tekst /I":  "",
		"/MH teraz": "",
	}
	for input, want := range tests {
		if got := terminalRemoteCommand(input); got != want {
			t.Errorf("terminalRemoteCommand(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTerminalHelpListsRemoteCommands(t *testing.T) {
	help := terminalHelpResponse()
	for _, command := range []string{"/I", "/V", "/MH", "/H", "/?"} {
		if !strings.Contains(help, command) {
			t.Errorf("help does not list %s: %q", command, help)
		}
	}
}

func TestTerminalVersionResponse(t *testing.T) {
	if got := terminalVersionResponse("0.4.4-dev.0+local"); got != "UltimatePR 0.4.4-dev.0+local\r\n" {
		t.Fatalf("version response = %q", got)
	}
}

func TestInboundViaFormatsReturnPath(t *testing.T) {
	first, _ := ax25.ParseAddress("DIGI2-2")
	second, _ := ax25.ParseAddress("DIGI1")
	if got := inboundVia([]ax25.Address{first, second}); got != "DIGI2-2,DIGI1" {
		t.Fatalf("inbound via=%q", got)
	}
}

func TestHasActiveBrowserTracksNotificationConnections(t *testing.T) {
	s := New(Config{}, nil)
	if s.HasActiveBrowser() {
		t.Fatal("new server reports an active browser")
	}
	s.notify[1] = make(chan notification, 1)
	if !s.HasActiveBrowser() {
		t.Fatal("notification connection is not reported as an active browser")
	}
}

func TestBrowserKickChannelClosesPreviousConnection(t *testing.T) {
	s := New(Config{}, nil)
	previous := make(chan struct{})
	s.notify[1] = make(chan notification, 1)
	s.notifyKick[1] = previous

	s.notifyMu.Lock()
	for _, kick := range s.notifyKick {
		close(kick)
	}
	s.notifyMu.Unlock()

	select {
	case <-previous:
	case <-time.After(time.Second):
		t.Fatal("previous browser connection was not kicked")
	}
}

func TestOperatorStationRunsInBackgroundWithoutBrowser(t *testing.T) {
	s := New(Config{
		TerminalCallsign: "SP5ME",
		TerminalAway:     "Czesc {REMOTE}, operator nieobecny, tu {CALL}",
		TerminalInfo:     "Info {CALL}",
	}, nil)
	input := bytes.NewBufferString("/I\r")
	var output bytes.Buffer

	s.ServeOperatorAX25(session.InboundRoute{Remote: "SQ9ABC", Port: "radio"}, input, &output)

	got := output.String()
	for _, want := range []string{"Czesc SQ9ABC, operator nieobecny, tu SP5ME\r\n", "Info SP5ME\r\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("background station output %q does not contain %q", got, want)
		}
	}
	if len(s.incoming) != 0 {
		t.Fatalf("background connection was left waiting for a browser: %+v", s.incoming)
	}
}
