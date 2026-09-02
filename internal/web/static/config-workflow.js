(() => {
  const byId = id => document.getElementById(id);
  const clone = value => JSON.parse(JSON.stringify(value));
  const workflowTranslations = {
    'NODE':'NODE',
    'Tożsamość noda':'Node identity',
    'NODE aktywny':'NODE enabled',
    'Alias':'Alias',
    'SSID noda':'Node SSID',
    'Język noda':'Node language',
    'NET/ROM':'NET/ROM',
    'NET/ROM aktywny':'NET/ROM enabled',
    'Mnemonic noda':'Node mnemonic',
    'Interwał NODES (s)':'NODES interval (s)',
    'Obsolescence':'Obsolescence',
    'Minimalna jakość':'Minimum quality',
    'Maks. wpisów routingu':'Maximum routing entries',
    'Wiadomości NODE':'NODE messages',
    'Powitanie NODE':'NODE welcome message',
    'Pożegnanie NODE':'NODE goodbye message',
    'Trasy noda':'Node routes',
    'Znane destinations są osiągane przez wskazanego sąsiada.':'Known destinations are reached through the selected neighbor.',
    'Dodaj trasę':'Add route',
    'Via sąsiad':'Via neighbor',
    'Aktywna':'Enabled',
    'Panel WWW':'Web panel',
    'Adres, pod którym działa panel administracyjny UltimatePR.':'The address where the UltimatePR administration panel is available.',
    'Adres i port panelu':'Panel address and port',
    'Dostęp sieciowy':'Network access',
    'Użytkownik panelu':'Panel user',
    'Dane logowania są zapisywane oddzielnie i nie wymagają restartu UltimatePR. Po zapisaniu nastąpi wylogowanie.':'Login details are saved separately and do not require an UltimatePR restart. You will be logged out after saving.',
    'Nazwa użytkownika':'Username',
    'Pozostaw puste, jeśli nie chcesz zmieniać hasła.':'Leave empty if you do not want to change the password.',
    'Zapisz dane logowania':'Save login details',
    'Ustawienia historii':'History settings',
    'Historia aktywna':'History enabled',
    'Plik historii':'History file',
    'Limit stacji':'Station limit',
    'Limit sesji na stację':'Sessions per station',
    'Limit fragmentów rozmowy':'Conversation fragment limit',
    'Maksymalny rozmiar (MB)':'Maximum size (MB)',
    'Czas przechowywania (dni)':'Retention period (days)',
  };
  if (typeof uiTranslations !== 'undefined' && typeof uiReverseTranslations !== 'undefined') {
    Object.assign(uiTranslations, workflowTranslations);
    Object.assign(uiReverseTranslations, Object.fromEntries(Object.entries(workflowTranslations).map(([pl,en]) => [en,pl])));
  }
  let baseline = null;
  let dirty = false;
  let saving = false;

  function number(id, fallback = 0) {
    const value = Number(byId(id)?.value);
    return Number.isFinite(value) && value !== 0 ? value : fallback;
  }

  function csv(value) {
    return String(value || '').split(',').map(part => part.trim()).filter(Boolean);
  }

  function collectSafePorts() {
    const originals = baseline?.Ports || [];
    return [...document.querySelectorAll('#tncPortRows .tnc-port-row')].map((row, index) => {
      const get = key => row.querySelector(`[data-k="${key}"]`);
      const id = get('ID')?.value.trim() || '';
      const original = originals.find(port => port.ID === id) || originals[index];
      if (original?.Type === 'axudp') return clone(original);
      const optional = key => get(key)?.value === '' ? null : Number(get(key)?.value);
      const fullDuplex = get('KISSFullDuplex')?.value || '';
      return {
        ...(original || {}),
        ID: id,
        enabled: get('Enabled')?.checked === true,
        Type: 'kiss-tcp',
        Host: get('Host')?.value.trim() || '',
        Port: Number(get('Port')?.value) || 0,
        MaxFrameBytes: Number(original?.MaxFrameBytes) || 4096,
        ReconnectSeconds: Number(original?.ReconnectSeconds) || 5,
        KISSPort: Number(get('KISSPort')?.value) || 0,
        KISSTXDelay: optional('KISSTXDelay'),
        KISSPersistence: optional('KISSPersistence'),
        KISSSlotTime: optional('KISSSlotTime'),
        KISSTXTail: optional('KISSTXTail'),
        KISSFullDuplex: fullDuplex === '' ? null : fullDuplex === 'true',
        tncproxy_enabled: get('TNCProxyEnabled')?.checked === true,
        tncproxy_port: Number(get('TNCProxyPort')?.value) || 0,
      };
    });
  }

  function candidate() {
    if (!baseline) return null;
    const c = clone(baseline);
    c.Application.OperatorName = byId('applicationOperatorName')?.value.trim() || '';
    c.Application.Locator = byId('applicationLocator')?.value.trim() || '';
    c.Application.QTH = byId('applicationQTH')?.value.trim() || '';
    c.Application.WelcomeMessage = byId('stationMessageWelcomeTab')?.classList.contains('active') ? byId('stationMessageText')?.value || '' : configModel?.Application?.WelcomeMessage || c.Application.WelcomeMessage;
    c.Application.AwayMessage = configModel?.Application?.AwayMessage || c.Application.AwayMessage;
    c.Application.GoodbyeMessage = configModel?.Application?.GoodbyeMessage || c.Application.GoodbyeMessage;
    c.Application.InfoMessage = configModel?.Application?.InfoMessage || c.Application.InfoMessage;
    if (typeof stationMessageKey !== 'undefined' && byId('stationMessageText')) {
      const fields = {welcome:'WelcomeMessage', away:'AwayMessage', goodbye:'GoodbyeMessage', info:'InfoMessage'};
      c.Application[fields[stationMessageKey] || 'WelcomeMessage'] = byId('stationMessageText').value;
    }
    c.Application.TerminalEOL = byId('terminalEOL')?.value || c.Application.TerminalEOL;
    c.Application.AX25T1Seconds = number('ax25T1', 10);
    c.Application.AX25T3Seconds = number('ax25T3', 300);
    c.Application.AX25N2 = number('ax25N2', 10);
    c.Application.AX25N1 = number('ax25N1', 256);
    c.Application.Language = byId('applicationLanguage')?.value || c.Application.Language;
    c.Application.Mode = byId('applicationMode')?.value || c.Application.Mode;
    c.Application.UpdateChannel = byId('updateChannel')?.value || c.Application.UpdateChannel;
    c.Terminal.Callsign = byId('terminalCallsign')?.value.trim().toUpperCase() || '';
    c.Terminal.SSID = number('terminalSSID');
    c.Server.Callsign = byId('nodeCallsign')?.value.trim().toUpperCase() || '';
    c.Server.SSID = number('nodeSSID');
    c.Node.ServiceID = byId('nodeServiceID')?.value.trim() || c.Node.ServiceID;
	c.Node.Enabled = byId('nodeEnabled')?.checked === true;
    c.Node.Alias = byId('nodeAlias')?.value.trim().toUpperCase() || '';
    c.Node.Listen = byId('nodeListen')?.value.trim() || '';
    c.Node.Language = byId('nodeLanguage')?.value || c.Node.Language;
    c.Node.WelcomeMessage = byId('nodeWelcomeMessage')?.value || '';
    c.Node.GoodbyeMessage = byId('nodeGoodbyeMessage')?.value || '';
    c.Node.NetROMEnabled = byId('nodeNetROMEnabled')?.checked === true;
    c.Node.NetROMMnemonic = byId('nodeNetROMMnemonic')?.value.trim().toUpperCase() || '';
    c.Node.NetROMInterval = number('nodeNetROMInterval', 3600);
    c.Node.NetROMObsolescence = number('nodeNetROMObsolescence', 6);
    c.Node.NetROMMinQuality = number('nodeNetROMMinQuality', 1);
    c.Node.NetROMMaxDestinations = number('nodeNetROMMaxDestinations', 50);
    if (typeof collectRows === 'function') c.Node.Neighbors = collectRows('neighborRows', 'neighbor');
    if (typeof collectRows === 'function') c.Node.Routes = collectRows('nodeRouteRows', 'route');
    c.BBS.Callsign = byId('bbsCallsign')?.value.trim().toUpperCase() || '';
    c.BBS.SSID = number('bbsSSID');
    c.BBS.ServiceID = byId('bbsServiceID')?.value.trim() || c.BBS.ServiceID;
	if (byId('servicesBBSCallsign')) { c.BBS.Callsign = byId('servicesBBSCallsign').value.trim().toUpperCase(); c.BBS.SSID = number('servicesBSSID', 8); }
	c.BBS.Enabled = byId('bbsServiceEnabled')?.checked === true;
    c.BBS.Title = byId('bbsTitle')?.value.trim() || '';
    c.BBS.Address = byId('bbsAddress')?.value.trim().toUpperCase() || '';
    c.BBS.Listen = byId('bbsListen')?.value.trim() || '';
    c.BBS.ForwardListen = byId('bbsForwardListen')?.value.trim() || '';
    c.BBS.Database = byId('bbsDatabase')?.value.trim() || '';
    c.BBS.Language = byId('bbsLanguage')?.value || c.BBS.Language;
    c.BBS.BeaconVia = byId('bbsBeaconVia')?.value.trim().toUpperCase() || '';
    c.BBS.Forwarding.Enabled = byId('fwdEnabled')?.checked === true;
    c.BBS.Forwarding.IntervalMinutes = number('fwdInterval', 5);
    c.BBS.Forwarding.MaxMessages = number('fwdMaxMessages');
    c.BBS.Forwarding.MaxBodyBytes = number('fwdMaxBody');
    if (typeof collectRows === 'function') c.BBS.Forwarding.Peers = collectRows('peerRows', 'peer');
    c.Web.Listen = byId('webListen')?.value.trim() || '';
    c.Web.Username = baseline.Web.Username;
    c.Web.AllowedAddresses = csv(byId('webAllowedAddresses')?.value);
    c.API = {...(c.API || {}), Enabled: byId('apiEnabled')?.checked === true};
    c.Ports = collectSafePorts();
    c.Beacon = {
      ...c.Beacon,
      Enabled: byId('beaconEnabled')?.checked === true,
      Port: byId('beaconPort')?.value || c.Beacon.Port,
      Destination: byId('beaconDestination')?.value.trim().toUpperCase() || '',
      Via: byId('beaconVia')?.value.trim().toUpperCase() || '',
      Text: byId('beaconText')?.value.trim() || '',
      IntervalMinutes: number('beaconInterval', 10),
    };
    c.History = {
      ...c.History,
      Enabled: byId('historyEnabled')?.checked === true,
      Database: byId('historyDatabase')?.value.trim() || '',
      MaxStations: number('historyMaxStations', 100),
      MaxSessionsPerStation: number('historyMaxSessions', 10),
      MaxLinesPerStation: number('historyMaxLines', 50),
      MaxBytes: number('historyMaxMB', 5) * 1048576,
      RetentionDays: number('historyRetention', 90),
    };
    if (byId('uprdEnabled')) {
      c.UPRD = {
        ...c.UPRD,
        Enabled: byId('uprdEnabled').checked,
        IntervalSeconds: number('uprdInterval', 10) * 60,
        MHeardLimit: Math.min(10, Math.max(1, number('uprdLimit', 5))),
      };
    }
	c.Experimental = {UPRD:byId('uprdirectExperimental')?.checked===true,Map:byId('mapExperimental')?.checked===true,Services:byId('servicesExperimental')?.checked===true,Node:false,BBS:false,AI:false};
	c.AI = {...(c.AI||{}),ServiceID:byId('aiServiceID')?.value.trim()||c.AI?.ServiceID||'',Enabled:byId('aiServiceEnabled')?.checked===true,Callsign:byId('aiCallsign')?.value.trim().toUpperCase()||'',SSID:number('aiSSID',12),Provider:'ollama',URL:byId('aiURL')?.value.trim()||'',Model:byId('aiModel')?.value.trim()||'',TimeoutSeconds:number('aiTimeout',120),MaxResponseChars:number('aiMaxResponse',2000),MaxContext:number('aiMaxContext',20),SystemPrompt:byId('aiSystemPrompt')?.value||'',QueueSize:number('aiQueueSize',8),Concurrency:number('aiConcurrency',1)};
	c.GameHall = {...(c.GameHall||{}),ServiceID:byId('gameHallServiceID')?.value.trim()||c.GameHall?.ServiceID||'',Enabled:byId('gameHallServiceEnabled')?.checked===true,Callsign:byId('gameHallCallsign')?.value.trim().toUpperCase()||'',SSID:Number(byId('gameHallSSID')?.value)||0,Language:byId('gameHallLanguage')?.value||'pl',InviteTimeoutSeconds:number('gameHallInviteTimeout',120)};
    return c;
  }

  function restartProjection(value) {
    const projected = clone(value);
    if (projected.Application) {
      delete projected.Application.Language;
      delete projected.Application.UpdateChannel;
    }
    return projected;
  }

  function requiresRestart(next = candidate()) {
    return !!next && JSON.stringify(restartProjection(next)) !== JSON.stringify(restartProjection(baseline));
  }

  function updateState() {
    if (!baseline || saving) return;
    const next = candidate();
    dirty = !!next && JSON.stringify(next) !== JSON.stringify(baseline);
    const restart = dirty && requiresRestart(next);
    const save = byId('saveConfig');
    const discard = byId('reloadConfig');
    const notice = byId('configNotice');
    if (save) {
      save.disabled = !dirty;
      save.textContent = !dirty ? 'Brak zmian do zapisania' : restart ? 'Zapisz i uruchom ponownie' : 'Zapisz zmiany';
    }
    if (discard) {
      discard.disabled = !dirty;
      discard.textContent = 'Odrzuć zmiany';
    }
    if (notice) {
      notice.className = 'config-notice' + (restart ? ' restart-warning' : '');
      notice.setAttribute('aria-live', 'polite');
      notice.textContent = !dirty ? 'Konfiguracja jest aktualna — nie ma zmian do zapisania.' : restart ? 'Zmiany nie są jeszcze zapisane. Zapis spowoduje ponowne uruchomienie UltimatePR.' : 'Zmiany nie są jeszcze zapisane. Kliknij „Zapisz zmiany”.';
    }
    document.querySelectorAll('.config-restart-note').forEach(note => note.classList.toggle('changed', restart));
  }

  async function saveChanges(confirmRestart = true) {
    const next = candidate();
    if (!next || JSON.stringify(next) === JSON.stringify(baseline)) {
      updateState();
      return {saved: true, restart_required: false};
    }
    const restart = requiresRestart(next);
    if (restart && confirmRestart) {
      const active = typeof sessions === 'undefined' ? 0 : [...sessions.values()].filter(session => ['connected','sending','timer_recovery','awaiting_connection'].includes(session.state)).length;
      const count = active ? ` Zostanie rozłączonych aktywnych sesji: ${active}.` : '';
      if (!await showAppConfirm(`Niektóre zmiany nie mogą zostać zastosowane bez ponownego uruchomienia UltimatePR.${count} Wszystkie aktywne sesje zostaną rozłączone. Lista MHEARD zostanie zachowana.`)) return null;
    }
    saving = true;
    const save = byId('saveConfig');
    const notice = byId('configNotice');
    if (save) save.disabled = true;
    if (notice) { notice.className = 'config-notice'; notice.textContent = 'Sprawdzanie ustawień…'; }
    try {
      const response = await fetch('/api/config/model', {method:'PUT', headers:{'Content-Type':'application/json'}, body:JSON.stringify(next)});
      const body = await response.text();
      if (!response.ok) throw new Error(body);
      const result = JSON.parse(body || '{}');
      baseline = clone(next);
      configModel = clone(next);
      dirty = false;
      saving = false;
      if (result.restart_required) {
        if (notice) notice.textContent = 'Ustawienia zapisane. UltimatePR uruchamia się ponownie, a MHEARD zostanie odtworzony…';
        waitForRestart();
      } else {
        updateState();
      }
      return result;
    } catch (error) {
      saving = false;
      if (notice) { notice.className = 'config-notice error'; notice.textContent = 'Nie zapisano: ' + friendlyConfigError(error.message); }
      updateState();
      return null;
    }
  }

  function installRestartNotes() {
    const panels = ['stationConfigPanel','terminalConfigPanel','tncConfigPanel','networkConfigPanel','apiConfigPanel','applicationConfigPanel','beaconConfigPanel','uprDirectConfigPanel','databaseConfigPanel','nodeConfigPanel'];
    panels.forEach(id => {
      const panel = byId(id);
      if (!panel || panel.querySelector('.config-restart-note')) return;
      const note = document.createElement('p');
      note.className = 'config-notice config-restart-note';
      note.textContent = 'Zmiana niektórych ustawień w tej zakładce może wymagać ponownego uruchomienia UltimatePR. Aktywne sesje zostaną wtedy rozłączone, a lista MHEARD pozostanie zachowana.';
      panel.append(note);
    });
  }

  function installUnifiedLayout() {
    document.querySelectorAll('.config-panels > .settings-panel').forEach(panel => panel.classList.add('config-unified'));
    document.querySelectorAll('.config-panels > .settings-panel > .settings-card:not(.service-config-card):not(.config-section-card)').forEach((card, index) => {
      const title = card.querySelector(':scope > h3, :scope > .card-head h3, :scope > .settings-card-heading h3');
      if (!title) return;
      const sectionId = card.id || `${card.parentElement.id}Section${index + 1}`;
      if (!card.id) card.id = sectionId;
      const bodyId = `${sectionId}Body`;
      const header = document.createElement('div');
      header.className = 'config-section-header';
      header.dataset.controls = bodyId;
      const heading = document.createElement('strong');
      heading.textContent = title.textContent;
      const actions = document.createElement('div');
      actions.className = 'config-section-actions';
      const headingContainer = title.closest('.card-head, .settings-card-heading');
      headingContainer?.querySelectorAll(':scope > button').forEach(button => actions.appendChild(button));
      const toggle = document.createElement('button');
      toggle.className = 'config-section-toggle';
      toggle.type = 'button';
      toggle.setAttribute('aria-expanded', 'false');
      toggle.setAttribute('aria-controls', bodyId);
      toggle.title = `Rozwiń sekcję ${heading.textContent}`;
      toggle.innerHTML = '<span aria-hidden="true"></span>';
      header.append(heading);
      if (actions.childElementCount) header.append(actions);
      header.append(toggle);
      title.remove();
      const body = document.createElement('div');
      body.id = bodyId;
      body.className = 'config-section-body';
      body.hidden = true;
      while (card.firstChild) body.appendChild(card.firstChild);
      card.classList.add('config-section-card');
      card.append(header, body);
      header.addEventListener('click', event => {
        if (event.target.closest('button:not(.config-section-toggle), input, select, textarea, a')) return;
        const expanded = toggle.getAttribute('aria-expanded') === 'true';
        toggle.setAttribute('aria-expanded', String(!expanded));
        toggle.title = `${expanded ? 'Rozwiń' : 'Zwiń'} sekcję ${heading.textContent}`;
        body.hidden = expanded;
      });
    });
  }

  function leaveDialog(action) {
    if (!dirty) { action(); return; }
    const modal = document.createElement('div');
    modal.className = 'app-confirm-backdrop';
    const restart = requiresRestart();
    modal.innerHTML = `<div class="app-confirm-modal" role="dialog" aria-modal="true"><h3>Niezapisane zmiany</h3><p>${restart ? 'Niektóre zmiany wymagają restartu i rozłączenia wszystkich sesji. MHEARD zostanie zachowany.' : 'Masz niezapisane zmiany konfiguracji.'}</p><div class="app-confirm-actions config-leave-actions"><button class="secondary stay">Pozostań</button><button class="secondary discard">Odrzuć zmiany i przejdź</button><button class="primary save">${restart ? 'Zapisz i uruchom ponownie' : 'Zapisz zmiany'}</button></div></div>`;
    document.body.appendChild(modal);
    const close = () => modal.remove();
    modal.querySelector('.stay').onclick = close;
    modal.querySelector('.discard').onclick = () => { dirty = false; close(); action(); };
    modal.querySelector('.save').onclick = async () => { const result = await saveChanges(false); if (result && !result.restart_required) { close(); action(); } };
  }

  const originalFillConfig = window.fillConfig;
  window.fillConfig = c => {
    baseline = clone(c);
    originalFillConfig(c);
    if (byId('apiEnabled')) byId('apiEnabled').checked = c.API?.Enabled === true;
    if (byId('apiPublicAddress')) byId('apiPublicAddress').value = `${location.protocol}//${location.host}/api/v1/`;
    loadAPITokens();
    setTimeout(() => {
      installRestartNotes();
      installUnifiedLayout();
      if (typeof applyUILanguage === 'function') applyUILanguage(uiLanguage);
      // Some controls display normalized defaults instead of the raw JSON value.
      // Compare later edits with the fully rendered form, not with its pre-render payload.
      const rendered = candidate();
      if (rendered) baseline = clone(rendered);
      updateState();
    }, 0);
  };

  const config = byId('configView');
  config?.addEventListener('input', () => setTimeout(updateState, 0));
  config?.addEventListener('change', () => setTimeout(updateState, 0));
  config?.addEventListener('click', event => {
    if (event.target.closest('.remove-row, #addNeighbor, #addPeer, #addTNCPort, #beaconTextDefault, #resetTerminalDefaults')) setTimeout(updateState, 0);
  });
  byId('saveConfig').onclick = () => saveChanges(true);
  byId('reloadConfig').onclick = () => loadConfig();
  const originalShowConfigPart = window.showConfigPart;
  window.showConfigPart = part => {
    originalShowConfigPart(part);
    if (byId('apiConfigPanel')) byId('apiConfigPanel').hidden = part !== 'api';
    byId('configAPITab')?.classList.toggle('active', part === 'api');
  };
  byId('configAPITab').onclick = () => window.showConfigPart('api');
  byId('apiNetworkSettings').onclick = () => window.showConfigPart('network');
  const scopeInputs = () => [...document.querySelectorAll('#apiScopeList input[type="checkbox"]')];
  const setScopes = wanted => scopeInputs().forEach(input => { input.checked = wanted.includes(input.value); });
  byId('apiScopesHomeAssistant').onclick = () => setScopes(['status.read','ports.read','mheard.read','node.read','bbs.read','digipeater.read']);
  byId('apiScopesAllRead').onclick = () => setScopes(scopeInputs().map(input => input.value));

  async function loadAPITokens() {
    const rows = byId('apiTokenRows');
    if (!rows) return;
    try {
      const response = await fetch('/api/application/api-tokens');
      if (!response.ok) throw new Error(await response.text());
      const data = await response.json();
      rows.replaceChildren();
      for (const token of data.items || []) {
        const row = document.createElement('div'); row.className = 'api-token-row';
        const info = document.createElement('div'), name = document.createElement('strong'), scopes = document.createElement('small');
        name.textContent = token.name; scopes.textContent = (token.scopes || []).join(', '); info.append(name, scopes);
        const remove = document.createElement('button'); remove.type = 'button'; remove.className = 'danger-button'; remove.textContent = 'Usuń';
        remove.onclick = async () => {
          if (!await showAppConfirm(`Usunąć token API „${token.name}”? Klient utraci dostęp po restarcie UltimatePR.`)) return;
          const result = await fetch('/api/application/api-tokens/' + encodeURIComponent(token.name), {method:'DELETE'});
          if (!result.ok) { byId('apiTokenNotice').className='config-notice error'; byId('apiTokenNotice').textContent=await result.text(); return; }
          byId('apiTokenNotice').className='config-notice ok'; byId('apiTokenNotice').textContent='Token usunięty. UltimatePR uruchamia się ponownie…'; row.remove(); waitForRestart();
        };
        row.append(info, remove); rows.append(row);
      }
      if (!(data.items || []).length) rows.textContent = 'Nie utworzono jeszcze żadnych tokenów.';
    } catch (error) { rows.textContent = 'Nie udało się pobrać tokenów: ' + error.message; }
  }

  byId('apiTokenCreate').onclick = async () => {
    const notice=byId('apiTokenNotice'), name=byId('apiTokenName').value.trim(), scopes=scopeInputs().filter(input=>input.checked).map(input=>input.value);
    notice.className='config-notice'; notice.textContent='Generowanie bezpiecznego tokenu…';
    try {
      const response=await fetch('/api/application/api-tokens',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name,scopes})});
      const body=await response.text(); if(!response.ok) throw new Error(body);
      const result=JSON.parse(body); byId('apiNewTokenValue').textContent=result.token; byId('apiNewToken').hidden=false;
      notice.className='config-notice ok'; notice.textContent='Token utworzony. Skopiuj go teraz; UltimatePR uruchomi się ponownie.';
      await navigator.clipboard?.writeText(result.token).catch(()=>{}); setTimeout(()=>waitForRestart(),500);
    } catch(error) { notice.className='config-notice error'; notice.textContent='Nie utworzono tokenu: '+error.message; }
  };
  byId('apiTokenCopy').onclick = async () => { await navigator.clipboard.writeText(byId('apiNewTokenValue').textContent); byId('apiTokenCopy').textContent='Skopiowano'; };
  const updateChannelButton = byId('saveUpdateChannel');
  if (updateChannelButton) {
    const saveUpdateChannel = updateChannelButton.onclick;
    updateChannelButton.onclick = async event => {
      await saveUpdateChannel?.call(updateChannelButton, event);
      if (byId('updateState')?.textContent === 'Kanał zapisany.' && baseline) {
        baseline.Application.UpdateChannel = byId('updateChannel').value;
        if (configModel?.Application) configModel.Application.UpdateChannel = byId('updateChannel').value;
        updateState();
      }
    };
  }

  ['navTerminal','navInfo','logout'].forEach(id => {
    const element = byId(id);
    if (!element) return;
    const action = element.onclick;
    element.addEventListener('click', event => {
      if (byId('configView')?.hidden || !dirty) return;
      event.preventDefault();
      event.stopImmediatePropagation();
      leaveDialog(() => action?.call(element, event));
    }, true);
  });

  window.addEventListener('beforeunload', event => {
    if (!dirty || saving) return;
    event.preventDefault();
    event.returnValue = '';
  });
})();
