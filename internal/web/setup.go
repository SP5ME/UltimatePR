package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/packet-radio/ultimatepr/internal/config"
)

type setupRequest struct {
	Mode        string `json:"mode"`
	Callsign    string `json:"callsign"`
	Locator     string `json:"locator"`
	QTH         string `json:"qth"`
	Language    string `json:"language"`
	StationSSID uint8  `json:"station_ssid"`
	NodeSSID    uint8  `json:"node_ssid"`
	BBSSSID     uint8  `json:"bbs_ssid"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Confirm     string `json:"confirm_password"`
}

// RunSetup starts a deliberately small, unauthenticated first-run service. It
// is used only while the configuration file does not exist.
func RunSetup(ctx context.Context, listen, configPath string, log *slog.Logger) error {
	mux := http.NewServeMux()
	done := make(chan struct{}, 1)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, setupHTML)
	})
	mux.HandleFunc("GET /setup.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = io.WriteString(w, setupCSS)
	})
	mux.HandleFunc("GET /setup.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = io.WriteString(w, setupJS)
	})
	mux.HandleFunc("POST /api/setup", func(w http.ResponseWriter, r *http.Request) {
		var q setupRequest
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&q) != nil {
			http.Error(w, "Nieprawidłowe dane", 400)
			return
		}
		if q.Password != q.Confirm || len(q.Password) < 4 || len(q.Password) > 128 {
			http.Error(w, "Hasła muszą być identyczne i mieć 4–128 znaków", 400)
			return
		}
		if strings.TrimSpace(q.Username) == "" {
			q.Username = "admin"
		}
		c := appconfig.New(q.Mode, q.Callsign, q.Locator, q.QTH, q.Language, q.StationSSID, q.NodeSSID, q.BBSSSID)
		c.Web.Username = strings.TrimSpace(q.Username)
		hash, err := hashPassword(q.Password)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		c.Web.PasswordHash = hash
		if err = saveSetup(configPath, c); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"saved":true}`)
		done <- struct{}{}
	})
	mux.HandleFunc("POST /api/setup/import", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(2 << 20); err != nil {
			http.Error(w, "Plik jest nieprawidłowy lub za duży", 400)
			return
		}
		f, _, err := r.FormFile("config")
		if err != nil {
			http.Error(w, "Wybierz plik konfiguracji", 400)
			return
		}
		defer f.Close()
		if err = importSetup(configPath, f); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"saved":true}`)
		done <- struct{}{}
	})
	srv := &http.Server{Addr: listen, Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		log.Info("first-run setup listening", "address", "http://"+listen)
		errCh <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
	case <-done:
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}

func saveSetup(path string, c appconfig.Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return appconfig.SaveModel(path, c)
}

func importSetup(path string, f multipart.File) error {
	b, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return err
	}
	if len(b) > 1<<20 {
		return fmt.Errorf("plik przekracza 1 MB")
	}
	c, err := appconfig.Parse(b)
	if err != nil {
		return fmt.Errorf("nieprawidłowa konfiguracja: %w", err)
	}
	return saveSetup(path, c)
}

const setupHTML = `<!doctype html><html lang="pl"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>UltimatePR — pierwsze uruchomienie</title><link rel="stylesheet" href="/setup.css"></head><body><main class="box"><h1>Konfigurator UltimatePR</h1><p class="muted">Utwórz nową konfigurację albo wczytaj wcześniej pobraną kopię konfiguracji.</p>
<form id="import"><label>Kopia konfiguracji YAML</label><input name="config" type="file" accept=".yaml,.yml" required><button>Wczytaj kopię i uruchom</button></form><hr>
<form id="setup"><label>Tryb pracy</label><select name="mode"><option value="station">Tylko stacja</option><option value="station-node-bbs">Stacja + NODE + BBS</option></select><div class="grid"><div><label>Znak wywoławczy</label><input name="callsign" required maxlength="6"></div><div><label>Lokator Maidenhead (opcjonalnie)</label><input name="locator" placeholder="KO02JD" title="Format: AA99AA, np. KO02JD"></div></div><label>QTH (opcjonalnie)</label><input name="qth"><label>Język</label><select name="language"><option value="pl">Polski</option><option value="en">English</option></select><div class="grid"><div><label>SSID stacji</label><input name="station_ssid" type="number" min="0" max="15" value="0"></div><div data-mode-group="advanced" hidden><label>SSID NODE</label><input name="node_ssid" type="number" min="0" max="15" value="7"></div><div data-mode-group="advanced" hidden><label>SSID BBS</label><input name="bbs_ssid" type="number" min="0" max="15" value="8"></div></div><label>Użytkownik panelu</label><input name="username" value="admin" required><div class="grid"><div><label>Hasło (minimum 4 znaki)</label><input name="password" type="password" minlength="4" required></div><div><label>Powtórz hasło</label><input name="confirm_password" type="password" minlength="4" required></div></div><button>Zapisz i uruchom UltimatePR</button></form><p id="msg" class="error"></p></main><script src="/setup.js"></script></body></html>`

const setupCSS = `body{font:16px system-ui;background:#eef3ee;color:#17251b;margin:0}.box{max-width:720px;margin:5vh auto;background:#fff;padding:28px;border:1px solid #bdc9be;border-radius:12px;box-shadow:0 12px 35px #243b2920}h1{margin-top:0}hr{border:0;border-top:1px solid #d6ded7;margin:26px 0}label{display:block;margin:13px 0 5px}input,select,button{font:inherit;padding:10px;border:1px solid #aab8ac;border-radius:7px}input,select{width:100%;box-sizing:border-box}button{background:#1e713b;color:white;margin-top:18px;cursor:pointer}.grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.muted{color:#59685d}.error{color:#a11;white-space:pre-wrap}.success{color:#176b36}.label-note{color:#1f5fbf;font-weight:600}@media(max-width:600px){.box{margin:0;border-radius:0;padding:20px}.grid{display:block}}`

const setupJS = `const msg=document.querySelector('#msg');const mode=document.querySelector('select[name="mode"]');const language=document.querySelector('select[name="language"]');const importForm=document.querySelector('#import');const setupForm=document.querySelector('#setup');const labels=[...setupForm.querySelectorAll('label')];const modeOptions=mode?mode.querySelectorAll('option'):[];const importLabel=importForm.querySelector('label');const importButton=importForm.querySelector('button');const setupButton=setupForm.querySelector('button');const title=document.querySelector('h1');const intro=document.querySelector('.muted');const locatorInput=setupForm.querySelector('input[name="locator"]');const advancedFields=[...document.querySelectorAll('[data-mode-group="advanced"]')];const text={pl:{title:'Konfigurator UltimatePR',intro:'Utwórz nową konfigurację albo wczytaj wcześniej pobraną kopię konfiguracji.',importLabel:'Kopia konfiguracji YAML',importButton:'Wczytaj kopię i uruchom',modeLabel:'Tryb pracy',modeStation:'Tylko stacja',modeFull:'Stacja + NODE + BBS',callsign:'Znak wywoławczy',locator:'Lokator Maidenhead',locatorNote:'(opcjonalny, ale zalecany)',qth:'QTH',qthNote:'(opcjonalny, ale zalecany)',language:'Język',ssidStation:'SSID stacji',ssidNode:'SSID NODE',ssidBbs:'SSID BBS',username:'Użytkownik panelu',password:'Hasło (minimum 4 znaki)',confirm:'Powtórz hasło',submit:'Zapisz i uruchom UltimatePR',langPl:'Polski',langEn:'English',locatorTitle:'Format: AA99AA, np. KO02JD'},en:{title:'UltimatePR setup',intro:'Create a new configuration or load a previously downloaded backup.',importLabel:'Configuration YAML backup',importButton:'Load backup and start',modeLabel:'Operating mode',modeStation:'Station only',modeFull:'Station + NODE + BBS',callsign:'Callsign',locator:'Maidenhead locator',locatorNote:'(optional, recommended)',qth:'QTH',qthNote:'(optional, recommended)',language:'Language',ssidStation:'Station SSID',ssidNode:'NODE SSID',ssidBbs:'BBS SSID',username:'Panel user',password:'Password (minimum 4 characters)',confirm:'Confirm password',submit:'Save and start UltimatePR',langPl:'Polish',langEn:'English',locatorTitle:'Format: AA99AA, e.g. KO02JD'}};function syncModeFields(){const advanced=mode&&mode.value==='station-node-bbs';for(const el of advancedFields)el.hidden=!advanced}function applyLanguage(){const lang=language&&language.value==='en'?'en':'pl';const t=text[lang];document.documentElement.lang=lang;if(title)title.textContent=t.title;if(intro)intro.textContent=t.intro;if(importLabel)importLabel.textContent=t.importLabel;if(importButton)importButton.textContent=t.importButton;if(setupButton)setupButton.textContent=t.submit;if(labels.length>=11){labels[0].textContent=t.modeLabel;labels[1].textContent=t.callsign;labels[2].innerHTML=t.locator+' <span class="label-note">'+t.locatorNote+'</span>';labels[3].innerHTML=t.qth+' <span class="label-note">'+t.qthNote+'</span>';labels[4].textContent=t.language;labels[5].textContent=t.ssidStation;labels[6].textContent=t.ssidNode;labels[7].textContent=t.ssidBbs;labels[8].textContent=t.username;labels[9].textContent=t.password;labels[10].textContent=t.confirm}if(modeOptions.length===2){modeOptions[0].textContent=t.modeStation;modeOptions[1].textContent=t.modeFull}if(language){const opts=language.querySelectorAll('option');if(opts.length===2){opts[0].textContent=t.langPl;opts[1].textContent=t.langEn}}if(locatorInput){locatorInput.placeholder='KO02JD';locatorInput.title=t.locatorTitle}syncModeFields()}async function send(url,body,headers){msg.className='error';msg.textContent='Zapisywanie…';const r=await fetch(url,{method:'POST',body,headers});if(!r.ok){msg.textContent=await r.text();return}msg.className='success';msg.textContent='Zapisano. UltimatePR uruchomi się ponownie.';setTimeout(()=>location.href='/',4000)}if(mode){mode.addEventListener('change',syncModeFields)}if(language){language.addEventListener('change',applyLanguage)}applyLanguage();document.querySelector('#setup').onsubmit=e=>{e.preventDefault();const f=new FormData(e.target),o=Object.fromEntries(f);['station_ssid','node_ssid','bbs_ssid'].forEach(k=>o[k]=Number(o[k]));send('/api/setup',JSON.stringify(o),{'Content-Type':'application/json'})};document.querySelector('#import').onsubmit=e=>{e.preventDefault();send('/api/setup/import',new FormData(e.target))};`

