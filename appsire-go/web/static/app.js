document.addEventListener('DOMContentLoaded', () => {
  const $ = (id) => document.getElementById(id);
  const loginScreen = $('loginScreen');
  const appShell = $('appShell');
  const loginForm = $('loginForm');
  const companySelect = $('companySelect');
  const authForm = $('authForm');
  const dropzone = $('dropzone');
  const fileInput = $('fileInput');
  const btnCancelDownload = $('btnCancelDownload');
  const progressSection = $('progressSection');
  const bulkButtons = [$('btnDownloadXml')];

  let loadedComprobantes = [];
  let companies = [];
  let isLicenseActive = false;
  let isDownloading = false;
  let pollInterval = null;
  let sseSource = null;
  const downloadedFiles = new Map();
  let currentProposalView = null;

  localStorage.removeItem('sire_creds');
  restoreRememberedLogin();
  setDefaultProposalPeriod();
  bindEvents();
  checkSession();
  setInterval(checkLicenseHeartbeat, 5 * 60 * 1000);

  function bindEvents() {
    loginForm.addEventListener('submit', login);
    $('btnLogout').addEventListener('click', logout);
    $('btnManageLicense').addEventListener('click', () => showLogin('Ingresa para cambiar de licencia'));
    authForm.addEventListener('submit', saveCompany);
    $('btnSelectCompany').addEventListener('click', () => connectSelectedCompany(false));
    $('btnDeleteCompany').addEventListener('click', deleteSelectedCompany);
    $('btnDownloadProposal').addEventListener('click', downloadProposal);

    $('btnBrowse').addEventListener('click', () => fileInput.click());
    dropzone.addEventListener('click', (event) => {
      if (event.target !== $('btnBrowse')) fileInput.click();
    });
    ['dragenter', 'dragover'].forEach((eventName) => {
      dropzone.addEventListener(eventName, (event) => {
        event.preventDefault();
        dropzone.classList.add('dragover');
      });
    });
    ['dragleave', 'drop'].forEach((eventName) => {
      dropzone.addEventListener(eventName, (event) => {
        event.preventDefault();
        dropzone.classList.remove('dragover');
      });
    });
    dropzone.addEventListener('drop', (event) => {
      if (event.dataTransfer.files.length) uploadExcel(event.dataTransfer.files[0]);
    });
    fileInput.addEventListener('change', (event) => {
      if (event.target.files.length) uploadExcel(event.target.files[0]);
    });

    $('concurrencyRange').addEventListener('input', (event) => {
      $('concurrencyVal').textContent = event.target.value;
    });
    $('btnDownloadXml').addEventListener('click', () => startDownload('XML'));
    btnCancelDownload.addEventListener('click', cancelDownload);
    $('uploadedRecordsBody').addEventListener('click', handleRecordAction);
    $('proposalPreviewBody').addEventListener('click', handleRecordAction);
    $('resultsTbody').addEventListener('click', handleRecordAction);
    $('fileViewerClose').addEventListener('click', closeFileViewer);
    $('fileViewer').addEventListener('click', (event) => {
      if (event.target === $('fileViewer')) closeFileViewer();
    });
    $('sidebarToggle').addEventListener('click', toggleSidebar);
    document.querySelectorAll('.sidebar-link').forEach((link) => {
      link.addEventListener('click', () => {
        document.querySelectorAll('.sidebar-link').forEach((item) => item.classList.remove('active'));
        link.classList.add('active');
        closeSidebar();
      });
    });
    $('btnOpenFolder').addEventListener('click', openFolder);
    $('btnDownloadZip').addEventListener('click', () => {
      window.location.href = '/api/files/download-zip';
    });

    document.querySelectorAll('.tab-btn').forEach((button) => {
      button.addEventListener('click', () => {
        document.querySelectorAll('.tab-btn').forEach((item) => item.classList.remove('active'));
        document.querySelectorAll('.tab-content').forEach((item) => item.classList.remove('active'));
        button.classList.add('active');
        $(button.dataset.tab).classList.add('active');
      });
    });
  }

  function restoreRememberedLogin() {
    $('loginUsername').value = localStorage.getItem('autosire_username') || '';
    $('loginServer').value = localStorage.getItem('autosire_server') || 'http://localhost:8090';
  }

  function setDefaultProposalPeriod() {
    const today = new Date();
    $('proposalPeriod').value = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}`;
  }

  async function checkSession() {
    try {
      const response = await fetch('/api/session');
      const data = await response.json();
      $('displayMachineId').textContent = data.machine_id || 'No disponible';
      if (!data.authenticated) {
        showLogin(data.message || 'Inicia sesión para continuar');
        return;
      }
      showApplication(data);
      await loadCompanies(true);
      await checkSunatStatus();
    } catch (_) {
      showLogin('No se pudo comprobar la licencia AutoSire');
    }
  }

  async function checkLicenseHeartbeat() {
    if (!isLicenseActive) return;
    try {
      const response = await fetch('/api/session');
      const data = await response.json();
      if (!data.authenticated) {
        showLogin(data.message || 'La licencia dejó de estar activa');
        return;
      }
      showApplication(data);
    } catch (_) {
      showLogin('No se pudo validar la licencia AutoSire');
    }
  }

  async function login(event) {
    event.preventDefault();
    const username = $('loginUsername').value.trim();
    const password = $('loginPassword').value;
    const key = $('loginKey').value.trim();
    if (!key && (!username || !password)) {
      $('loginError').textContent = 'Ingresa usuario y contraseña, o una clave de licencia.';
      return;
    }

    $('loginSpinner').style.display = 'inline-block';
    $('btnLogin').disabled = true;
    $('loginError').textContent = '';
    try {
      const response = await fetch('/api/session/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username,
          password,
          key,
          server_url: $('loginServer').value.trim(),
          remember: $('loginRemember').checked
        })
      });
      const data = await response.json();
      if (!response.ok || !data.success) throw new Error(data.error || 'Acceso rechazado');

      if ($('loginRemember').checked) {
        localStorage.setItem('autosire_username', username || data.username || '');
        localStorage.setItem('autosire_server', $('loginServer').value.trim());
      } else {
        localStorage.removeItem('autosire_username');
        localStorage.removeItem('autosire_server');
      }
      $('loginPassword').value = '';
      $('loginKey').value = '';
      await checkSession();
    } catch (error) {
      $('loginError').textContent = error.message;
    } finally {
      $('loginSpinner').style.display = 'none';
      $('btnLogin').disabled = false;
    }
  }

  async function logout() {
    await fetch('/api/session/logout', { method: 'POST' });
    showLogin('Sesión cerrada');
  }

  function showLogin(message = '') {
    isLicenseActive = false;
    appShell.style.display = 'none';
    loginScreen.style.display = 'flex';
    $('loginError').textContent = message;
  }

  function showApplication(license) {
    isLicenseActive = true;
    loginScreen.style.display = 'none';
    appShell.style.display = '';
    $('licenseDot').classList.add('active');
    $('licenseLabel').textContent = `Licencia: ${license.username || 'Activa'}`;
    $('licenseDetailsText').textContent = `${license.plan || ''} · ${license.days_left ?? 0} días`;
    updateDownloadStartButton();
  }

  async function apiFetch(url, options = {}) {
    const response = await fetch(url, options);
    if (response.status === 401) {
      showLogin('La sesión o la licencia ya no es válida');
      throw new Error('Sesión no válida');
    }
    return response;
  }

  async function loadCompanies(autoConnect = false) {
    const response = await apiFetch('/api/companies');
    const data = await response.json();
    if (!response.ok || !data.success) throw new Error(data.error || 'No se pudieron cargar las empresas');
    companies = data.empresas || [];
    companySelect.innerHTML = companies.length
      ? companies.map((item) => `<option value="${item.id}">${escapeHtml(item.ruc)} · ${escapeHtml(item.razon_social)}</option>`).join('')
      : '<option value="">Aún no hay empresas</option>';

    const selected = companies.find((item) => item.seleccionada);
    if (selected) companySelect.value = String(selected.id);
    if (autoConnect && selected) await connectSelectedCompany(true);
  }

  async function saveCompany(event) {
    event.preventDefault();
    $('btnAuth').disabled = true;
    $('authSpinner').style.display = 'inline-block';
    try {
      const response = await apiFetch('/api/companies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ruc: $('ruc').value.trim(),
          razon_social: $('businessName').value.trim(),
          usuario_sol: $('usuarioSol').value.trim(),
          clave_sol: $('claveSol').value,
          client_id: $('clientId').value.trim(),
          client_secret: $('clientSecret').value
        })
      });
      const data = await response.json();
      if (!response.ok || !data.success) throw new Error(data.error || 'No se pudo guardar la empresa');
      authForm.reset();
      await loadCompanies(false);
      companySelect.value = String(data.id);
      await connectSelectedCompany(false);
    } catch (error) {
      alert(`Empresa no guardada: ${error.message}`);
    } finally {
      $('btnAuth').disabled = false;
      $('authSpinner').style.display = 'none';
    }
  }

  async function connectSelectedCompany(silent) {
    const id = Number(companySelect.value);
    if (!id) {
      if (!silent) alert('Guarda o selecciona una empresa.');
      return;
    }
    $('statusDetails').textContent = 'Conectando con SUNAT…';
    $('btnSelectCompany').disabled = true;
    try {
      const response = await apiFetch('/api/companies/select', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id })
      });
      const data = await response.json();
      if (!response.ok || !data.success) throw new Error(data.error || 'SUNAT rechazó las credenciales');
      setConnectedUI(data.ruc, data.razon_social);
      await loadCompanies(false);
    } catch (error) {
      setDisconnectedUI(error.message);
      if (!silent) alert(`No se pudo conectar con SUNAT: ${error.message}`);
    } finally {
      $('btnSelectCompany').disabled = false;
    }
  }

  async function deleteSelectedCompany() {
    const id = Number(companySelect.value);
    const item = companies.find((company) => company.id === id);
    if (!item || !confirm(`¿Eliminar ${item.razon_social} de este equipo?`)) return;
    const response = await apiFetch('/api/companies/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id })
    });
    const data = await response.json();
    if (!response.ok || !data.success) {
      alert(data.error || 'No se pudo eliminar la empresa');
      return;
    }
    setDisconnectedUI();
    await loadCompanies(false);
  }

  async function checkSunatStatus() {
    try {
      const response = await apiFetch('/api/auth/status');
      const data = await response.json();
      if (data.active) setConnectedUI(data.ruc, 'Token vigente');
      else setDisconnectedUI();
    } catch (_) {
      setDisconnectedUI();
    }
  }

  function setConnectedUI(ruc, name) {
    $('statusDot').classList.add('active');
    $('statusLabel').textContent = `SUNAT: ${ruc}`;
    $('statusDetails').textContent = name || 'Conectado';
  }

  function setDisconnectedUI(message = 'Selecciona una empresa para conectar') {
    $('statusDot').classList.remove('active');
    $('statusLabel').textContent = 'SUNAT: Desconectado';
    $('statusDetails').textContent = message;
  }

  async function downloadProposal() {
    if (!$('proposalPeriod').value) {
      alert('Selecciona el periodo tributario.');
      return;
    }
    const button = $('btnDownloadProposal');
    const resultBox = $('proposalResult');
    button.disabled = true;
    $('proposalSpinner').style.display = 'inline-block';
    resultBox.style.display = 'block';
    resultBox.textContent = 'SUNAT está generando la propuesta…';
    $('proposalPreview').style.display = 'none';
    $('proposalDownloads').style.display = 'none';
    loadedComprobantes = [];
    downloadedFiles.clear();
    currentProposalView = null;
    updateDownloadStartButton();
    try {
      const response = await apiFetch('/api/sire/proposal/download', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          periodo: $('proposalPeriod').value.replace('-', ''),
          libro: $('proposalBook').value,
          reutilizar_existente: $('proposalReuse').checked
        })
      });
      const data = await response.json();
      if (!response.ok || !data.success) throw new Error(data.error || 'No se pudo descargar la propuesta');
      const href = `/api/files/view?download=1&path=${encodeURIComponent(data.path)}`;
      const reused = data.reutilizado ? ' · propuesta anterior reutilizada' : '';
      resultBox.textContent = `${data.libro} lista · ticket ${data.ticket}${reused}`;
      loadedComprobantes = data.preview?.comprobantes || [];
      currentProposalView = { preview: data.preview, href, book: data.libro, period: data.periodo };
      $('proposalDownloads').style.display = 'block';
      renderProposalPreview(data.preview, href, data.libro, data.periodo, true);
      updateDownloadStartButton();
    } catch (error) {
      resultBox.textContent = `Error: ${error.message}`;
    } finally {
      button.disabled = false;
      $('proposalSpinner').style.display = 'none';
    }
  }

  async function uploadExcel(file) {
    const formData = new FormData();
    formData.append('excel', file);
    dropzone.style.opacity = '0.55';
    $('excelSummary').style.display = 'none';
    try {
      const response = await apiFetch('/api/excel/upload', { method: 'POST', body: formData });
      const data = await response.json();
      if (!response.ok || !data.success) throw new Error(data.error || 'No se pudo leer el Excel');
      loadedComprobantes = data.comprobantes || [];
      const meta = data.metadata || {};
      $('metaSheet').textContent = meta.hoja || 'Detectada';
      $('metaPeriodo').textContent = meta.periodo || 'No especificado';
      $('metaEmpresa').textContent = `${meta.ruc || ''} ${meta.razon_social || ''}`.trim() || 'General';
      $('metaTotal').textContent = data.total || 0;
      $('excelSummary').style.display = 'block';
      renderLoadedRecords();
      updateDownloadStartButton();
    } catch (error) {
      alert(`Error al procesar Excel: ${error.message}`);
    } finally {
      dropzone.style.opacity = '1';
    }
  }

  function updateDownloadStartButton() {
    bulkButtons.forEach((button) => {
      button.disabled = !isLicenseActive || loadedComprobantes.length === 0 || isDownloading;
    });
  }

  async function startDownload(tipo) {
    if (!loadedComprobantes.length) {
      alert('Genera y visualiza una propuesta SIRE antes de iniciar la descarga.');
      return;
    }

    const comprobantes = loadedComprobantes.filter((comp) =>
      comp?.ruc && comp?.tipo && comp?.serie && comp?.numero &&
      !(tipo === 'CDR' && String(comp.serie).toUpperCase().startsWith('E'))
    );
    if (!comprobantes.length) {
      alert(`La propuesta no contiene comprobantes que admitan ${tipo}.`);
      return;
    }

    const payload = {
      comprobantes,
      tipos: [tipo],
      concurrency: 6
    };
    resetProgress(comprobantes.length);
    try {
      const response = await apiFetch('/api/download/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      const data = await response.json();
      if (!response.ok || !data.success) throw new Error(data.error || 'No se pudo iniciar');
      isDownloading = true;
      startLiveProgress();
    } catch (error) {
      alert(`Error al iniciar descarga: ${error.message}`);
      finishProgress();
    }
  }

  function resetProgress(total) {
    bulkButtons.forEach((button) => { button.disabled = true; });
    btnCancelDownload.style.display = 'inline-flex';
    progressSection.style.display = 'block';
    progressSection.scrollIntoView({ behavior: 'smooth' });
    $('progressBar').style.width = '0%';
    $('progressPercent').textContent = '0%';
    $('progressMessage').textContent = 'Preparando descarga…';
    $('statTotal').textContent = total;
    $('statProcessed').textContent = '0';
    $('statSuccess').textContent = '0';
    $('statError').textContent = '0';
    $('resultsTbody').innerHTML = '';
    $('terminalLogs').innerHTML = '';
  }

  async function cancelDownload() {
    if (!confirm('¿Detener la descarga?')) return;
    await apiFetch('/api/download/cancel', { method: 'POST' });
  }

  function startLiveProgress() {
    if (sseSource) sseSource.close();
    if (pollInterval) clearInterval(pollInterval);
    sseSource = new EventSource('/api/download/events');
    sseSource.onmessage = (event) => {
      const data = JSON.parse(event.data);
      updateProgress(data.status, data.logs);
      if (isTerminalStatus(data.status)) finishProgress();
    };
    sseSource.onerror = () => {
      if (sseSource) sseSource.close();
      sseSource = null;
      if (isDownloading) startPolling();
    };
  }

  function startPolling() {
    pollInterval = setInterval(async () => {
      const response = await apiFetch('/api/download/status');
      const data = await response.json();
      updateProgress(data.status, data.logs);
      if (isTerminalStatus(data.status)) finishProgress();
    }, 900);
  }

  function isTerminalStatus(status) {
    return status && ['completado', 'detenido', 'error'].includes(status.estado);
  }

  function finishProgress() {
    isDownloading = false;
    if (sseSource) sseSource.close();
    if (pollInterval) clearInterval(pollInterval);
    sseSource = null;
    pollInterval = null;
    btnCancelDownload.style.display = 'none';
    updateDownloadStartButton();
  }

  function updateProgress(status, logs) {
    if (!status) return;
    const percent = Math.min(100, status.porcentaje || 0);
    $('progressBar').style.width = `${percent}%`;
    $('progressPercent').textContent = `${percent.toFixed(1)}%`;
    $('progressMessage').textContent = status.mensaje || '';
    $('statTotal').textContent = status.total_items || 0;
    $('statProcessed').textContent = status.procesados || 0;
    $('statSuccess').textContent = status.exitosos || 0;
    $('statError').textContent = status.errores || 0;
    if ($('statSpeed')) $('statSpeed').textContent = `${(status.velocidad_items_seg || 0).toFixed(1)} docs/s`;
    if ($('statEta')) $('statEta').textContent = status.tiempo_restante || '--';
    if (status.resultados) renderResults(status.resultados);
    if (logs) renderLogs(logs);
  }

  function renderResults(items) {
    items.forEach((item) => {
      if (item.exito && item.ruta_local) downloadedFiles.set(fileKey(item.comprobante, item.tipo), item.ruta_local);
    });
    if (currentProposalView) {
      renderProposalPreview(
        currentProposalView.preview,
        currentProposalView.href,
        currentProposalView.book,
        currentProposalView.period
      );
    }
    $('resultsCount').textContent = items.length;
    $('resultsTbody').innerHTML = items.slice(-300).reverse().map((item) => {
      const comp = item.comprobante || {};
      const file = item.exito && item.ruta_local
        ? `<button type="button" class="btn-link viewer-link" data-view-path="${encodeURIComponent(item.ruta_local)}" data-view-type="${escapeHtml(item.tipo || '')}">Ver</button>`
        : '—';
      return `<tr>
        <td><span class="badge-status ${item.exito ? 'badge-ok' : 'badge-error'}">${item.exito ? 'OK' : 'ERROR'}</span></td>
        <td>${escapeHtml(item.tipo || '')}</td><td>${escapeHtml(comp.ruc || '')}</td>
        <td>${escapeHtml(comp.tipo || '')}</td><td><strong>${escapeHtml(comp.serie || '')}-${escapeHtml(comp.numero || '')}</strong></td>
        <td>${escapeHtml(item.estado_cdr || '—')}</td><td>${escapeHtml((item.digest_value || '').slice(0, 12) || '—')}</td>
        <td>${file}</td><td>${escapeHtml(item.error || item.origen || 'Descargado')}</td>
      </tr>`;
    }).join('');
  }

  function renderProposalPreview(preview, zipHref, book, period, shouldScroll = false) {
    const panel = $('proposalPreview');
    const headers = preview?.headers || [];
    const rows = preview?.rows || [];
    const comprobantes = preview?.comprobantes || [];
    $('proposalPreviewTitle').textContent = `${book} · ${period}`;
    $('proposalPreviewMeta').textContent = `${preview?.total_rows || 0} registros · ${preview?.file_name || 'archivo SUNAT'}${preview?.truncated ? ' · mostrando los primeros 1000' : ''}`;
    $('proposalZipLink').href = zipHref;
    $('proposalPreviewHead').innerHTML = `<tr>${headers.map((header) => `<th>${escapeHtml(header)}</th>`).join('')}<th>XML</th><th>PDF</th><th>CDR</th></tr>`;
    $('proposalPreviewBody').innerHTML = rows.map((row, rowIndex) => {
      const comp = comprobantes[rowIndex] || {};
      return `<tr>${headers.map((_, index) => `<td>${escapeHtml(row[index] || '')}</td>`).join('')}${['XML', 'PDF', 'CDR'].map((tipo) => renderArtifactActions(comp, rowIndex, tipo)).join('')}</tr>`;
    }).join('');
    panel.style.display = 'block';
    if (shouldScroll) panel.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  function renderLoadedRecords() {
    const section = $('uploadedRecordsSection');
    if (!loadedComprobantes.length) {
      section.style.display = 'none';
      return;
    }
    section.style.display = 'block';
    $('uploadedRecordsCount').textContent = `${loadedComprobantes.length} registros`;
    $('uploadedRecordsBody').innerHTML = loadedComprobantes.map((comp, index) => `<tr>
      <td><strong>${escapeHtml(comp.tipo || '')} · ${escapeHtml(comp.serie || '')}-${escapeHtml(comp.numero || '')}</strong></td>
      <td>${escapeHtml(comp.ruc || '')}</td>
      <td>${escapeHtml(comp.fecha_emision || '—')}</td>
      ${['PDF', 'XML', 'CDR'].map((tipo) => renderArtifactActions(comp, index, tipo)).join('')}
    </tr>`).join('');
  }

  function renderArtifactActions(comp, index, tipo) {
    const path = downloadedFiles.get(fileKey(comp, tipo));
    if (path) {
      const encoded = encodeURIComponent(path);
      return `<td><button type="button" class="mini-action" data-view-path="${encoded}" data-view-type="${tipo}">Ver</button></td>`;
    }
    return '<td></td>';
  }

  function handleRecordAction(event) {
    const button = event.target.closest('[data-view-path]');
    if (!button) return;
    $('fileViewerTitle').textContent = `Visor ${button.dataset.viewType || 'de archivo'}`;
    $('fileViewerFrame').src = `/api/files/view?path=${button.dataset.viewPath}`;
    $('fileViewer').showModal();
  }

  function closeFileViewer() {
    $('fileViewer').close();
    $('fileViewerFrame').src = 'about:blank';
  }

  function fileKey(comp, tipo) {
    return `${comp?.id || `${comp?.ruc}-${comp?.tipo}-${comp?.serie}-${comp?.numero}`}|${tipo}`;
  }

  function toggleSidebar() {
    const open = $('sidebar').classList.toggle('open');
    $('sidebarToggle').setAttribute('aria-expanded', String(open));
  }

  function closeSidebar() {
    $('sidebar').classList.remove('open');
    $('sidebarToggle').setAttribute('aria-expanded', 'false');
  }

  function renderLogs(logs) {
    $('terminalLogs').innerHTML = logs.map((line) => `<div class="terminal-line">${escapeHtml(line)}</div>`).join('');
    $('terminalLogs').scrollTop = $('terminalLogs').scrollHeight;
  }

  async function openFolder() {
    const response = await apiFetch('/api/files/open-folder', { method: 'POST' });
    if (!response.ok) alert('No se pudo abrir la carpeta de descargas.');
  }

  function escapeHtml(value) {
    return String(value ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  // Monitoreo de actividad de la ventana (Heartbeat y Shutdown)
  function initWindowLifecycle() {
    // Enviar latido cada 3 segundos para mantener vivo el proceso local
    setInterval(() => {
      fetch('/api/heartbeat', { method: 'POST' }).catch(() => {});
    }, 3000);

    // Cuando el usuario cierra la ventana (con la X o Alt+F4)
    window.addEventListener('beforeunload', () => {
      if (navigator.sendBeacon) {
        navigator.sendBeacon('/api/shutdown');
      } else {
        fetch('/api/shutdown', { method: 'POST', keepalive: true }).catch(() => {});
      }
    });
  }

  initWindowLifecycle();
});
