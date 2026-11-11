'use strict';
const $ = id => document.getElementById(id);
const api = () => window.go.main.App;
const names = {factory:'Install a factory image',capture:'Capture a golden image',golden:'Restore a golden image'};
const boardNames = {unoq:'UNO Q',ventunoq:'VENTUNO Q'};
let mode = '', board = '', step = 0, furthest = 0, devices = [], status = {}, plan = null, request = null, running = false;
const selected = new Set(), cards = new Map();
let logs = [];
let latestRelease = '';
let portPoll = false, runGeneration = 0;
const runDevices = new Map();
let selectionEdited = false;
let excludedSource = '';
let imageLookup = 0;
let batchStarted = 0, batchFinished = 0;
const panels = ['boardStep','imageStep','connectStep','reviewStep','runPanel'];

function error(e) {
  $('error').textContent = String(e);
  $('error').classList.remove('hidden');
  $('error').focus();
  $('error').scrollIntoView({block:'nearest'});
}
function clearError() { $('error').classList.add('hidden'); }
function showHome() {
  if (running) return;
  $('home').classList.remove('hidden');
  $('wizard').classList.add('hidden');
  clearError();
  $('homeTitle').focus();
  window.scrollTo(0,0);
}
function setStep(next) {
  step = next;
  furthest = Math.max(furthest,next);
  panels.forEach((id,i) => $(id).classList.toggle('hidden',i !== step));
  document.querySelectorAll('[data-step]').forEach(button => {
    const i = Number(button.dataset.step);
    button.disabled = running || i > furthest || i === step;
    button.classList.toggle('complete',i < step);
    if (i === step) button.setAttribute('aria-current','step');
    else button.removeAttribute('aria-current');
  });
  $('wizardFooter').classList.toggle('hidden',step === 4);
  $('confirmLabel').classList.toggle('hidden',step !== 3 || mode === 'capture');
  $('nextHint').classList.toggle('hidden',step === 3 && mode !== 'capture');
  $('changeTask').disabled = running;
  $('taskName').textContent = names[mode]+(board ? ' · '+boardNames[board] : '');
  updateActions();
  const heading = $(panels[step]).querySelector('h1');
  heading.focus();
  window.scrollTo(0,0);
  if (step === 2) refresh();
}
function begin(next) {
  if (running) return;
  runGeneration++; $('sorting').classList.add('hidden');
  mode = next; board = ''; plan = null; furthest = 0; excludedSource = ''; imageLookup++; $('imageSize').classList.add('hidden');
  selected.clear(); selectionEdited = false;
  $('ready').checked = false; $('confirmed').checked = false;
  $('image').value = ''; $('programmer').value = ''; $('version').value = ''; $('os').value = '';
  $('advanced').open = false; $('strategy').value = 'auto'; $('targetContents').value = 'factory';
  document.querySelectorAll('input[name=board]').forEach(input => input.checked = false);
  document.querySelector('input[name=factorySource][value=latest]').checked = true;
  $('home').classList.add('hidden'); $('wizard').classList.remove('hidden');
  configureImage(); renderDevices(); clearError(); setStep(0);
}
function factoryLocal() { return document.querySelector('input[name=factorySource]:checked').value === 'local'; }
function configureImage() {
  $('imageTitle').textContent = {factory:'Choose your factory image',capture:'Where should we save your image?',golden:'Choose your saved golden image'}[mode];
  $('imageIntro').textContent = {factory:'Use an official image for '+(boardNames[board] || 'your board')+'. We prepare it once for the entire batch.',capture:'Choose a new folder for the configured board’s complete Linux storage.',golden:'Choose the image folder captured from a configured board of the same model.'}[mode];
  $('factorySource').classList.toggle('hidden',mode !== 'factory');
  $('imageInput').classList.toggle('hidden',mode === 'factory' && !factoryLocal());
  $('captureNote').classList.toggle('hidden',mode !== 'capture');
  $('targetContentsPanel').classList.toggle('hidden',mode !== 'golden');
  $('imageLabel').textContent = {factory:'Factory archive or extracted folder',capture:'New golden image folder',golden:'Golden image folder'}[mode];
  $('image').placeholder = mode === 'capture' ? 'Choose a new folder to create' : 'Choose an existing image';
  $('browse').textContent = mode === 'factory' ? 'Choose archive…' : mode === 'capture' ? 'Choose save location…' : 'Choose image folder…';
  $('folder').classList.toggle('hidden',mode !== 'factory');
  $('imageHelp').textContent = {factory:'Supports factory archives and extracted image folders.',capture:'The folder must not already exist. Your source board will not be written.',golden:'The folder must contain manifest.json. Its captured source is excluded from targets.'}[mode];
  $('jobsLabel').classList.toggle('hidden',mode === 'capture');
  $('strategyLabel').classList.toggle('hidden',mode !== 'golden');
  $('programmerLabel').classList.toggle('hidden',mode !== 'capture');
  $('versionLabel').classList.toggle('hidden',mode === 'golden' || (mode === 'factory' && factoryLocal()));
  $('osLabel').classList.toggle('hidden',mode === 'golden' || (mode === 'factory' && factoryLocal()));
  $('settingsHint').textContent = mode === 'capture' ? 'One source board · automatic programmer' : $('jobs').value+' parallel boards · '+(mode === 'golden' ? 'automatic strategy' : 'shared image preparation');
  updateActions();
}
function guidance() {
  const vent = board === 'ventunoq';
  $('boardPicture').src = vent ? 'assets/ventunoq-jctl.png' : 'assets/unoq-jctl.png';
  $('boardPicture').alt = 'Official '+boardNames[board]+' EDL bridge pin illustration';
  const instructions = vent ? ['Disconnect USB and external power.','Bridge JCTL pin 1 (GND) and pin 2 (FORCED_USB_BOOT_N), as shown.','Connect external power first, then the USB-C data cable.'] : ['Disconnect board power and USB.','Fit the EDL bridge shown in the official illustration.','Reconnect using a USB data cable and stable power.'];
  $('instructions').replaceChildren(...instructions.map(text => {const li = document.createElement('li'); li.textContent = text; return li;}));
  $('connectTitle').textContent = mode === 'capture' ? 'Connect your source board' : 'Connect your target boards';
  $('connectIntro').textContent = mode === 'capture' ? 'Connect the configured '+boardNames[board]+' you want to copy, then select its serial number.' : 'Put your '+boardNames[board]+' boards in EDL mode, then choose the boards to '+(mode === 'factory' ? 'install.' : 'restore.');
  $('deviceTitle').textContent = mode === 'capture' ? 'Choose one source board' : 'Select your targets';
  $('selectAll').classList.toggle('hidden',mode === 'capture');
}
function updateActions() {
  const next = $('next');
  let hint = '', ready = true;
  if (step === 0) {ready = !!board; hint = ready ? boardNames[board]+' selected' : 'Choose a board to continue.'; next.textContent = 'Continue to image →';}
  if (step === 1) {ready = mode === 'factory' && !factoryLocal() || !!$('image').value.trim(); hint = ready ? 'Next: USB setup and board selection' : 'Choose an image '+(mode === 'capture' ? 'save location' : 'folder or archive')+' to continue.'; next.textContent = 'Continue to connection →';}
  if (step === 2) {ready = selected.size > 0 && (mode !== 'capture' || selected.size === 1) && $('ready').checked; hint = !selected.size ? 'Select '+(mode === 'capture' ? 'one source board.' : 'at least one target board.') : !$('ready').checked ? 'Confirm the hardware checklist to continue.' : selected.size+' board(s) selected'; next.textContent = 'Review '+(mode === 'capture' ? 'capture' : selected.size+' target'+(selected.size === 1 ? '' : 's'))+' →';}
  if (step === 3) {ready = mode === 'capture' || $('confirmed').checked; hint = ready ? (mode === 'capture' ? 'Your source board will not be written.' : 'Only the listed targets will be written.') : 'Confirm replacing data before starting.'; next.textContent = mode === 'capture' ? 'Start capture' : mode === 'factory' ? 'Install on '+plan.targets.length+' board(s)' : 'Restore '+plan.targets.length+' board(s)';}
  next.disabled = !ready || running; $('nextHint').textContent = hint;
  $('previous').textContent = step === 0 ? '← All tasks' : 'Back';
}
function targetDevices() {return devices;}
function renderDevices() {
  const root = $('devices'); root.replaceChildren();
  for (const device of devices) {
    const label = document.createElement('label'); label.className = 'device';
    const input = document.createElement('input'); input.type = mode === 'capture' ? 'radio' : 'checkbox'; input.name = 'source'; input.checked = selected.has(device.serial); input.disabled = running;
    input.onchange = () => {selectionEdited = true; if (mode === 'capture') selected.clear(); input.checked ? selected.add(device.serial) : selected.delete(device.serial); renderDevices();};
    const div = document.createElement('div'), name = document.createElement('b'), detail = document.createElement('small');
    name.textContent = (device.slot ? device.slot+' · ' : '')+device.serial;
    if (device.serial === excludedSource) {const tag = document.createElement('span'); tag.className = 'golden-tag'; tag.textContent = 'Golden device'; name.append(' ',tag);}
    detail.textContent = (device.path ? 'USB port: '+device.path+' · ' : 'USB port unavailable · ')+(device.serial === excludedSource ? 'Unchecked by default. Select to restore this board too.' : device.product); div.append(name,detail); label.append(input,div); root.append(label);
  }
  if (!devices.length) {
    const div = document.createElement('div'); div.className = 'empty-state';
    const title = document.createElement('strong'), text = document.createElement('p');
    title.textContent = 'No boards detected yet'; text.textContent = 'Follow the EDL setup above, connect a USB data cable, then refresh devices.'; div.append(title,text); root.append(div);
  }
  $('selectedCount').textContent = devices.length+' connected · '+selected.size+' of '+targetDevices().length+' targets selected';
  $('selectAll').textContent = selected.size && selected.size === targetDevices().length ? 'Clear selection' : 'Select all targets';
  updateActions();
}
async function refresh() {
  if (running) return;
  const button = $('refresh'); button.disabled = true;
  try {
    devices = await api().FixtureDevices($('fixture').value.trim());
    if (mode !== 'capture' && step === 2 && !selectionEdited) devices.filter(device => device.serial !== excludedSource).forEach(device => selected.add(device.serial));
    for (const serial of selected) if (!devices.some(device => device.serial === serial)) selected.delete(serial);
    renderDevices();
    // A refreshed list invalidates a prepared review if a target disconnected.
    if (step === 3 && plan && plan.targets.some(serial => !selected.has(serial))) {plan = null; furthest = 2; setStep(2); error('A selected board disconnected. Check the target list and review again.');}
  } catch (e) {error(e);} finally {button.disabled = false;}
}
async function browse(kind,field='image') {
  try {const path = await api().Browse(kind); if (path) $(field).value = path; updateActions(); if (field === 'image') await inspectSelectedImage();} catch (e) {error(e);}
}
async function inspectSelectedImage() {
  const token = ++imageLookup, path = $('image').value.trim();
  $('imageSize').classList.add('hidden');
  if (mode !== 'golden' || !path || status.demo) return;
  try {
    const image = await api().Image(path);
    if (token !== imageLookup || mode !== 'golden') return;
    $('imageSize').textContent = formatBytes(image.bytes)+' image · '+boardNames[image.board];
    $('imageSize').classList.remove('hidden');
  } catch (_) { /* Path validation is shown when continuing. */ }
}
function readRequest() {
  return {fixture:$('fixture').value.trim(),restoreSource:mode === 'golden' && selected.has(excludedSource),mode,board,image:mode === 'factory' && !factoryLocal() ? '' : $('image').value.trim(),source:[...selected][0] || '',targets:[...selected],programmer:mode === 'capture' ? $('programmer').value.trim() : '',version:mode === 'golden' || (mode === 'factory' && factoryLocal()) ? '' : $('version').value.trim(),os:mode === 'golden' || (mode === 'factory' && factoryLocal()) ? '' : $('os').value.trim(),strategy:$('strategy').value,targetContents:$('targetContents').value,jobs:Number($('jobs').value),timeoutMinutes:Number($('timeout').value),ready:$('ready').checked,confirmed:$('confirmed').checked};
}
function summaryItem(label,value) {const div = document.createElement('div'), term = document.createElement('dt'), detail = document.createElement('dd'); term.textContent = label; detail.textContent = value; div.append(term,detail); return div;}
async function review() {
  clearError(); $('next').disabled = true;
  try {
    request = readRequest(); plan = await api().Plan(request);
    $('planTitle').textContent = mode === 'capture' ? 'Ready to capture your board?' : 'Ready to '+(mode === 'factory' ? 'install' : 'restore')+' '+plan.targets.length+' board'+(plan.targets.length === 1 ? '?' : 's?');
    $('planExplanation').textContent = plan.explanation;
    $('strategyTitle').textContent = (plan.automatic ? 'Automatically selected: ' : 'Manual strategy: ')+plan.strategyLabel;
    $('strategyReason').textContent = plan.strategyReason;
    $('planStats').replaceChildren(summaryItem('Board model',boardNames[board]),summaryItem(mode === 'capture' ? 'Source boards' : 'Targets & concurrency',plan.targets.length+' selected · '+plan.jobs+' parallel'),summaryItem(mode === 'capture' ? 'Save location' : 'Image',request.image || 'Latest official factory image'),summaryItem('Verification',plan.verification),summaryItem(mode === 'capture' ? 'Capture size' : 'Data to write',plan.dataScale),summaryItem(mode === 'capture' ? 'Estimated time' : 'Estimated total time',plan.timing));
    $('estimateNote').textContent = mode === 'golden' ? plan.targets.length > 1 ? 'Boards run concurrently and share USB bandwidth. Watch individual step ETAs below.' : 'Based on previous runs; matching boards can finish sooner.' : '';
    $('targetHeading').textContent = mode === 'capture' ? 'Source board (read only)' : 'Selected targets';
    $('planTargets').replaceChildren(...plan.targets.map(serial => {const span = document.createElement('span'); const device = devices.find(d => d.serial === serial); span.textContent = (device?.slot ? device.slot+' · ' : '')+serial+(device?.path ? ' · USB port '+device.path : ''); return span;}));
    $('warnings').textContent = plan.warnings.join('\n'); $('warnings').classList.toggle('hidden',!plan.warnings.length);
    $('confirmed').checked = false; $('confirmLabel').classList.toggle('hidden',mode === 'capture');
    setStep(3);
  } catch (e) {error(e); updateActions();}
}
async function advance() {
  clearError();
  if (step === 0) {configureImage(); guidance(); setStep(1);}
  else if (step === 1) {
    if (mode === 'golden' && !status.demo) {
      try {const image = await api().Image($('image').value.trim()); excludedSource = image.source || ''; selected.delete(excludedSource); if (image.board !== board) {error('This golden image is for '+boardNames[image.board]+'. Go back and choose that board model.'); return;}} catch (e) {error(e); return;}
    }
    guidance(); setStep(2);
  } else if (step === 2) await review();
  else if (step === 3) await start();
}
function newCard(serial) {
  const el = document.createElement('div'); el.className = 'progress-card'; el.dataset.status = 'queued';
  const title = document.createElement('b'); title.textContent = (runDevices.get(serial)?.slot ? runDevices.get(serial).slot+' · ' : '')+serial;
  const state = document.createElement('span'); state.className = 'status'; state.textContent = 'Queued';
  const phase = document.createElement('p'); phase.textContent = 'Waiting to start';
  const bar = document.createElement('progress'); bar.max = 100; bar.setAttribute('aria-label','Progress for '+serial);
  const data = document.createElement('div'); data.className = 'transfer-data'; data.textContent = plan && plan.bytes ? formatBytes(plan.bytes)+' image' : 'Waiting for transfer';
  const metrics = document.createElement('div'); metrics.className = 'metrics';
  const port = document.createElement('small'); port.className = 'port-assignment'; port.textContent = 'USB port: '+(runDevices.get(serial)?.path || devices.find(d => d.serial === serial)?.path || 'unavailable');
  const connection = document.createElement('p'); connection.className = 'port-presence';
  el.append(title,state,port,connection,phase,bar,data,metrics); $('progressCards').append(el);
  const card = {el,title,port,state,phase,bar,data,metrics,connection,present:null,missing:0,activePhase:''}; cards.set(serial,card); return card;
}
function updateRunSummary() {
  const values = [...cards.values()].map(card => card.el.dataset.status);
  const finished = values.filter(s => ['verified','flashed','captured'].includes(s)).length;
  const failed = values.filter(s => ['failed','cancelled'].includes(s)).length;
  const seconds = batchStarted ? Math.floor(((batchFinished || Date.now())-batchStarted)/1000) : 0;
  const elapsed = Math.floor(seconds/60)+'m'+String(seconds%60).padStart(2,'0')+'s';
  $('runSummary').textContent = (batchStarted ? 'Batch elapsed '+elapsed+' · ' : '')+finished+' of '+values.length+' complete'+(failed ? ' · '+failed+' need attention' : '')+(running ? ' · '+(values.length-finished-failed)+' remaining' : '');
}
async function start() {
  clearError();
  try {
    request.confirmed = $('confirmed').checked; $('next').disabled = true;
    runGeneration++; runDevices.clear();
    const snapshot = await api().FixtureDevices(request.fixture || '');
    for (const serial of plan.targets) { const device = snapshot.find(d => d.serial === serial); if (!device) throw new Error(serial+' disconnected. Refresh and review again.'); runDevices.set(serial,{...device}); }
    $('sorting').classList.add('hidden'); $('unplugHistory').replaceChildren();
    cards.clear(); $('progressCards').replaceChildren(); logs = []; $('liveLog').textContent = '';
    for (const serial of plan.targets) newCard(serial);
    $('captureHandoff').classList.add('hidden'); $('runScale').textContent = plan.dataScale+' · '+plan.timing; $('newRun').classList.add('hidden'); $('restoreCaptured').classList.add('hidden'); $('cancel').classList.remove('hidden'); $('cancel').disabled = false; $('cancel').textContent = 'Stop operation';
    $('runTitle').textContent = plan.title; $('runMessage').textContent = 'Preparing the image and selected boards…';
    batchStarted = Date.now(); batchFinished = 0; running = true; setStep(4); updateRunSummary();
    await api().Start(request);
  } catch (e) {running = false; setStep(3); error(e);}
}
function event(e) {
  if (!running) return;
  const text = [e.serial,e.phase,e.message].filter(Boolean).join(' · ');
  if (text) {logs.push(text); if (logs.length > 400) logs.shift(); $('liveLog').textContent = logs.join('\n');}
  if (e.serial && e.serial !== 'image') {
    const card = cards.get(e.serial) || newCard(e.serial);
    if (e.kind === 'assignment') { runDevices.set(e.serial,{serial:e.serial,path:e.path,slot:e.slot}); card.title.textContent = (e.slot ? e.slot+' · ' : '')+e.serial; card.port.textContent = 'USB port: '+(e.path || 'unavailable'); }
    if (e.phase && e.kind !== 'log') card.phase.textContent = e.phase;
    if (e.kind === 'progress') {
      card.bar.value = e.percent;
      if (card.activePhase !== e.phase) {card.activePhase = e.phase; card.data.textContent = 'Waiting for progress';}
      if (e.data) card.data.textContent = e.data+' '+dataVerb(e.phase);
      card.metrics.textContent = e.percent.toFixed(1)+'%'+(e.rate ? ' · '+e.rate : '')+(e.elapsed ? ' · '+e.elapsed+' elapsed' : '')+(e.eta ? ' · ~'+e.eta+' left in this step' : '');
      if (card.el.dataset.status === 'queued') {card.el.dataset.status = 'working'; card.state.textContent = 'Working';}
    }
    if (e.status) {card.el.dataset.status = e.status; card.state.textContent = e.status; if (['verified','flashed','captured'].includes(e.status)) {card.bar.value = 100; card.phase.textContent = e.status === 'captured' ? 'Image saved' : e.status === 'verified' ? 'Storage verified' : 'Factory installation complete';} if (e.message) card.metrics.textContent = e.message; else if (e.kind === 'result') {card.metrics.textContent = e.elapsed ? e.elapsed+' elapsed' : ''; if (e.bytesWritten || e.bytesSkipped) card.data.textContent = formatBytes(e.bytesWritten)+' written · '+formatBytes(e.bytesSkipped)+' skipped';}}
  } else if (e.message || e.phase) $('runMessage').textContent = e.message || e.phase;
  if (e.kind === 'data' && e.data) $('runScale').textContent = e.data+' '+plan.timing;
  if (e.kind === 'done') {
    batchFinished = Date.now(); running = false; $('changeTask').disabled = false;
    $('runTitle').textContent = e.status === 'finished' ? 'Operation complete' : e.status === 'cancelled' ? 'Operation stopped' : 'Check the board results';
    $('runMessage').textContent = e.message; $('cancel').classList.add('hidden'); $('newRun').classList.remove('hidden');
    if (mode === 'capture' && e.status === 'finished') {
      $('restoreCaptured').classList.remove('hidden'); $('captureHandoff').classList.remove('hidden');
      $('handoffMessage').textContent = 'Unplug source '+request.source+'. Put the new '+boardNames[board]+' boards in EDL mode and connect them, then continue below. The source will be excluded from the target list.';
      if (!status.demo) api().Image(request.image).then(image => {if (mode === 'capture' && !running && $('image').value.trim() === request.image) $('runScale').textContent = formatBytes(image.bytes)+' image saved';}).catch(() => {});
    }
    $('sorting').classList.remove('hidden');
    $('sortingStatus').textContent = 'Checking USB ports…';
    pollPorts();
    if (status.smoke) {
      const results = [...cards.values()].map(card => card.el.dataset.status);
      api().ReportUISmoke(JSON.stringify({stage:'finished',success:e.status === 'finished' && results.length === 3 && results.every(s => s === 'flashed'),results,title:document.title,devices:devices.length,uxChecks:window.uxChecks}));
    }
  }
  updateRunSummary();
}
function updatePortPresence(paths) {
  const present = new Set(paths), changes = [];
  for (const [serial, card] of cards) {
    const path = runDevices.get(serial)?.path;
    if (!path) { card.connection.textContent = 'USB port unavailable'; continue; }
    const connected = present.has(path);
    // Require two missing snapshots to avoid highlighting brief USB resets.
    if (!connected && card.present === true && ++card.missing < 2) continue;
    if (connected) card.missing = 0;
    const previous = card.present;
    card.present = connected;
    card.connection.textContent = connected ? 'USB connected' : previous === true ? 'USB removed' : card.connection.textContent || 'USB not detected after flashing';
    card.el.classList.toggle('usb-removed', !connected);
    if (previous !== null && previous !== connected) {
      const result = card.state.textContent;
      const message = (connected ? 'Reconnected: ' : 'Removed: ')+(runDevices.get(serial)?.slot ? runDevices.get(serial).slot+' · ' : '')+serial+' · USB port '+path+' · '+result;
      const item = document.createElement('li'); item.textContent = new Date().toLocaleTimeString()+' · '+message;
      $('unplugHistory').prepend(item);
      while ($('unplugHistory').children.length > 32) $('unplugHistory').lastChild.remove();
      changes.push(message);
    }
  }
  if (changes.length || !$('unplugHistory').children.length) $('sortingStatus').textContent = changes.length ? changes.join(' / ') : 'Watching USB ports. Flash results remain on each card.';
}
async function pollPorts() {
  if (portPoll || running) return;
  portPoll = true;
  const generation = runGeneration;
  try {
    const paths = await api().USBPaths();
    if (generation === runGeneration && !running && step === 4) updatePortPresence(paths);
  } catch (e) {
    if (generation === runGeneration) $('sortingStatus').textContent = 'USB monitoring unavailable: '+String(e)+'. Last known connections retained.';
  } finally { portPoll = false; }
}
function renderUpdate(info) {
  latestRelease = info.latest;
  $('updateNotice').classList.toggle('hidden', !info.available && info.comparable);
  $('updateAvailable').textContent = info.available ? 'New '+(info.preview ? 'preview ' : '')+'version available: '+info.latest : !info.comparable ? 'Latest release: '+info.latest : '';
  $('updateStatus').textContent = info.available ? 'Update available' : !info.comparable ? 'Latest release: '+info.latest+' (development build)' : 'Up to date';
}
async function checkUpdates() {
  const button = $('checkUpdates');
  if (button.disabled) return;
  button.disabled = true; $('updateStatus').textContent = 'Checking GitHub…';
  try {renderUpdate(await api().CheckUpdates()); $('updateStatus').title = '';}
  catch (e) {$('updateStatus').textContent = 'Could not check updates. Try again later.'; $('updateStatus').title = String(e);}
  finally {button.disabled = false;}
}
async function init() {
  try {
    status = await api().Status();
    $('appVersion').textContent = 'App '+await api().AppVersion();
    checkUpdates();
    $('connection').textContent = status.demo ? 'Demo devices' : status.capabilities.cli ? 'Tools ready' : 'Tools need building';
    $('demo').classList.toggle('hidden',!status.demo);
    if (!status.demo && !status.capabilities.cli) error(status.capabilities.problem);
    document.querySelectorAll('[data-mode]').forEach(button => {button.disabled = !status.demo && (!status.capabilities.cli || (button.dataset.mode !== 'factory' && !status.capabilities.golden));});
    window.runtime.EventsOn('provision:event',event);
    for(const option of $('strategy').options){if(option.value==='hierarchical')option.disabled=!status.demo&&!status.capabilities.digest;if(option.value==='blocks')option.disabled=!status.demo&&(!status.capabilities.digest||!status.capabilities.blocks);}
    setInterval(() => {if (!running && mode && (step === 2 || step === 3)) refresh();},10000);
    setInterval(() => {if (running) updateRunSummary(); else if (mode && step === 4 && batchFinished) pollPorts();},1000);
    if (status.smoke) await smokeTest();
  } catch (e) {error(e);}
}
document.querySelectorAll('[data-mode]').forEach(button => button.onclick = () => begin(button.dataset.mode));
document.querySelectorAll('input[name=board]').forEach(input => input.onchange = () => {board = input.value; selected.clear(); selectionEdited = false; $('ready').checked = false; furthest = 0; configureImage(); guidance(); updateActions();});
document.querySelectorAll('input[name=factorySource]').forEach(input => input.onchange = configureImage);
document.querySelectorAll('[data-step]').forEach(button => button.onclick = () => {const target = Number(button.dataset.step); if (target < step && !running) {furthest = target; plan = null; setStep(target);}});
$('checkUpdates').onclick = checkUpdates;
$('viewRelease').onclick = () => api().OpenRelease(latestRelease).catch(error);
$('changeTask').onclick = showHome; $('previous').onclick = () => {if (step === 0) showHome(); else {plan = null; furthest = step-1; setStep(step-1);}};
$('next').onclick = advance; $('image').oninput = () => {updateActions(); inspectSelectedImage();}; $('ready').onchange = updateActions; $('confirmed').onchange = updateActions;
$('jobs').oninput = configureImage;
$('browse').onclick = () => browse(mode === 'capture' ? 'save' : mode === 'golden' ? 'folder' : 'archive');
$('folder').onclick = () => browse('folder'); $('chooseProgrammer').onclick = () => browse('programmer','programmer');
$('guide').onclick = () => api().OpenGuide(board); $('logs').onclick = () => api().RevealLogs(); $('refresh').onclick = refresh;
$('chooseFixture').onclick = async () => {await browse('fixture','fixture'); selectionEdited = false; selected.clear(); await refresh();};
$('fixture').onchange = () => {selectionEdited = false; selected.clear(); refresh();};
$('selectAll').onclick = () => {selectionEdited = true; if (selected.size && selected.size === targetDevices().length) selected.clear(); else targetDevices().forEach(device => selected.add(device.serial)); renderDevices();};
$('cancel').onclick = () => {if (confirm('Stop all active transfers? Interrupted boards may need to be flashed again.')) {$('cancel').disabled = true; $('cancel').textContent = 'Stopping…'; api().Cancel();}};
$('newRun').onclick = () => {$('cancel').disabled = false; $('cancel').textContent = 'Stop operation'; showHome();};
function restoreCapturedImage() {
  const image = request.image, model = board, source = request.source;
  begin('golden'); document.querySelector('input[name=board][value='+model+']').click();
  excludedSource = source; $('image').value = image; configureImage(); guidance(); inspectSelectedImage();
  $('connectIntro').textContent = 'Unplug golden source '+source+' and connect the new '+boardNames[board]+' boards to restore. Your captured image is already selected.';
  setStep(2);
}
function dataVerb(phase) {return /write/i.test(phase) ? 'written' : /digest|checksum|compar/i.test(phase) ? 'checked' : /read/i.test(phase) ? 'read' : 'processed';}
function formatBytes(bytes) {return bytes >= 1073741824 ? (bytes/1073741824).toFixed(2)+' GiB' : bytes >= 1048576 ? (bytes/1048576).toFixed(1)+' MiB' : (bytes || 0)+' B';}
$('restoreCaptured').onclick = restoreCapturedImage;
window.addEventListener('DOMContentLoaded',init);

async function smokeTest() {
  try {
    renderUpdate({latest:'v99.0.0-preview.1',available:true,comparable:true,preview:true});
    const updateNotification = !$('updateNotice').classList.contains('hidden') && $('updateAvailable').textContent.includes('v99.0.0-preview.1');
    renderUpdate({latest:'v0.1.2-preview.1',available:false,comparable:true});
    const noUpdateNotification = $('updateNotice').classList.contains('hidden') && $('updateStatus').textContent === 'Up to date';
    const homeVisible = !$('home').classList.contains('hidden') && $('wizard').classList.contains('hidden');
    const demoDevices = await api().Devices(), goldenSource = demoDevices[0].serial;
    mode = 'capture'; board = 'unoq'; request = {image:'/demo/captured',source:goldenSource};
    restoreCapturedImage(); await refresh();
    const captureHandoff = mode === 'golden' && board === 'unoq' && $('image').value === '/demo/captured' && step === 2 && excludedSource === goldenSource && devices.some(device => device.serial === goldenSource) && !$('devices').querySelector('input').disabled && $('devices').textContent.includes('Golden device') && !selected.has(goldenSource) && !$('ready').checked && selected.size === devices.length - 1 && $('connectIntro').textContent.includes('Unplug');
    $('devices').querySelector('input').click(); await refresh();
    const goldenSelectable = selected.has(goldenSource) && selected.size === devices.length && readRequest().restoreSource;
    begin('factory'); const boardRequired = $('next').disabled;
    document.querySelector('input[name=board][value=ventunoq]').click();
    const card = document.querySelector('input[name=board]:checked').closest('.board-card').getBoundingClientRect();
    const largeSelector = card.width > 250 && card.height >= 140;
    await advance();
    document.querySelector('input[name=factorySource][value=local]').click();
    const localNeedsImage = $('next').disabled;
    document.querySelector('input[name=factorySource][value=latest]').click();
    await advance(); await refresh();
    const allTargetsSelected = devices.length > 0 && selected.size === targetDevices().length;
    $('devices').querySelector('input').click();
    const deselected = devices[0].serial; await refresh();
    const selectionSurvivesRefresh = !selected.has(deselected);
    const targetsRequired = $('next').disabled;
    // Capture must limit selection to one source even after selecting a batch.
    mode = 'capture'; selected.clear(); renderDevices();
    $('devices').querySelector('input').click();
    $('devices').querySelectorAll('input')[1].click(); const oneSource = selected.size === 1;
    mode = 'factory'; selected.clear(); renderDevices();
    $('selectAll').click(); const checklistRequired = $('next').disabled;
    $('ready').checked = true; updateActions(); await advance();
    const confirmationRequired = $('next').disabled;
    const progressSerial = devices[0].serial;
    running = true;
    event({kind:'progress',serial:progressSerial,phase:'write',percent:30,data:'4.2 GiB / 14.60 GiB',elapsed:'2m',eta:'4m'});
    const progressCard = cards.get(progressSerial);
    const transferCounter = progressCard.data.textContent === '4.2 GiB / 14.60 GiB written' && progressCard.metrics.textContent.includes('2m elapsed');
    event({kind:'progress',serial:progressSerial,phase:'verify digests',percent:0});
    const phaseResetsData = progressCard.data.textContent === 'Waiting for progress';
    runDevices.clear(); devices.forEach(d => runDevices.set(d.serial,{...d}));
    cards.clear(); $('progressCards').replaceChildren();
    devices.forEach(d => newCard(d.serial));
    const first = devices[0], second = devices[1];
    cards.get(first.serial).state.textContent = 'failed'; cards.get(first.serial).el.dataset.status = 'failed';
    cards.get(second.serial).state.textContent = 'flashed'; cards.get(second.serial).el.dataset.status = 'flashed';
    updatePortPresence(devices.map(d => d.path));
    const remaining = devices.slice(1).map(d => d.path);
    updatePortPresence(remaining);
    const resetIgnored = cards.get(first.serial).present === true;
    updatePortPresence(remaining);
    const failedBoardIdentified = cards.get(first.serial).connection.textContent === 'USB removed' && $('unplugHistory').textContent.includes(first.serial) && $('unplugHistory').textContent.includes('failed') && cards.get(second.serial).present === true;
    updatePortPresence(devices.map(d => d.path));
    const reconnectIdentified = cards.get(first.serial).present === true && cards.get(first.serial).el.dataset.status === 'failed' && $('unplugHistory').textContent.includes('Reconnected');
    const portAssignments = cards.get(first.serial).el.textContent.includes(first.path) && $('planTargets').textContent.includes(first.path);
    $('unplugHistory').replaceChildren();
    running = false; cards.clear(); $('progressCards').replaceChildren();
    window.uxChecks = {updateNotification,noUpdateNotification,resetIgnored,failedBoardIdentified,reconnectIdentified,portAssignments,goldenSelectable,transferCounter,phaseResetsData,captureHandoff,homeVisible,boardRequired,largeSelector,localNeedsImage,allTargetsSelected,selectionSurvivesRefresh,targetsRequired,oneSource,checklistRequired,confirmationRequired,noHorizontalOverflow:document.documentElement.scrollWidth <= window.innerWidth};
    if (!Object.values(window.uxChecks).every(Boolean)) throw new Error('UX checks failed: '+JSON.stringify(window.uxChecks));
    $('confirmed').checked = true; updateActions(); await advance();
  } catch (e) {await api().ReportUISmoke(JSON.stringify({stage:'failed',success:false,error:String(e),uxChecks:window.uxChecks}));}
}
