(() => {
  const byId = id => document.getElementById(id);
  const clone = value => JSON.parse(JSON.stringify(value));
  const workflowTranslations = {
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
	if (byId('servicesNodeCallsign')) { c.Server.Callsign = byId('servicesNodeCallsign').value.trim().toUpperCase(); c.Server.SSID = number('servicesNodeSSID', 2); }
	c.Node.Enabled = byId('nodeServiceEnabled')?.checked === true;
    c.Node.Alias = byId('nodeAlias')?.value.trim().toUpperCase() || '';
    c.Node.Listen = byId('nodeListen')?.value.trim() || '';
    c.Node.Language = byId('nodeLanguage')?.value || c.Node.Language;
    if (typeof collectRows === 'function') c.Node.Neighbors = collectRows('neighborRows', 'neighbor');
    c.BBS.Callsign = byId('bbsCallsign')?.value.trim().toUpperCase() || '';
    c.BBS.SSID = number('bbsSSID');
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
	c.AI = {...(c.AI||{}),Enabled:byId('aiServiceEnabled')?.checked===true,Callsign:byId('aiCallsign')?.value.trim().toUpperCase()||'',SSID:number('aiSSID',12),Provider:'ollama',URL:byId('aiURL')?.value.trim()||'',Model:byId('aiModel')?.value.trim()||'',TimeoutSeconds:number('aiTimeout',120),MaxResponseChars:number('aiMaxResponse',2000),MaxContext:number('aiMaxContext',20),SystemPrompt:byId('aiSystemPrompt')?.value||'',QueueSize:number('aiQueueSize',8),Concurrency:number('aiConcurrency',1)};
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
    const panels = ['stationConfigPanel','terminalConfigPanel','tncConfigPanel','networkConfigPanel','applicationConfigPanel','beaconConfigPanel','uprDirectConfigPanel','databaseConfigPanel'];
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
    const messageCard = byId('stationMessageText')?.closest('.settings-card');
    const terminalPanel = byId('terminalConfigPanel');
    if (messageCard && terminalPanel && messageCard.parentElement !== terminalPanel) terminalPanel.prepend(messageCard);

    const historyFields = byId('historyDatabase')?.parentElement;
    const databasePanel = byId('databaseConfigPanel');
    if (historyFields && databasePanel && !byId('historySettingsCard')) {
      const card = document.createElement('div');
      card.id = 'historySettingsCard';
      card.className = 'settings-card';
      card.innerHTML = '<h3>Ustawienia historii</h3><div class="field-grid"></div>';
      const grid = card.querySelector('.field-grid');
      const rows = [
        ['historyEnabled','Historia aktywna'],
        ['historyDatabase','Plik historii'],
        ['historyMaxStations','Limit stacji'],
        ['historyMaxSessions','Limit sesji na stację'],
        ['historyMaxLines','Limit fragmentów rozmowy'],
        ['historyMaxMB','Maksymalny rozmiar (MB)'],
        ['historyRetention','Czas przechowywania (dni)'],
      ];
      rows.forEach(([id,labelText]) => {
        const input = byId(id);
        if (!input) return;
        const label = document.createElement('label');
        if (input.type === 'checkbox') label.className = 'check';
        label.append(document.createTextNode(labelText), input);
        grid.appendChild(label);
      });
      historyFields.remove();
      databasePanel.prepend(card);
    }
    document.querySelectorAll('.config-panels > .settings-panel').forEach(panel => panel.classList.add('config-unified'));
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
