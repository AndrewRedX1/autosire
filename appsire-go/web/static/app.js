document.addEventListener('DOMContentLoaded', () => {
  const $ = (id) => document.getElementById(id);

  function showToast(message, type = 'info') {
    let container = document.getElementById('notionToastContainer');
    if (!container) {
      container = document.createElement('div');
      container.id = 'notionToastContainer';
      container.className = 'notion-toast-container';
      document.body.appendChild(container);
    }

    const toast = document.createElement('div');
    toast.className = `notion-toast notion-toast-${type}`;

    let icon = 'ℹ️';
    if (type === 'success') icon = '✔';
    else if (type === 'error') icon = '✕';
    else if (type === 'warning') icon = '⚠';

    toast.innerHTML = `
      <span class="notion-toast-icon">${icon}</span>
      <span class="notion-toast-msg">${escapeHtml(message)}</span>
    `;

    container.appendChild(toast);

    requestAnimationFrame(() => {
      toast.classList.add('visible');
    });

    setTimeout(() => {
      toast.classList.remove('visible');
      setTimeout(() => toast.remove(), 250);
    }, 4500);
  }

  // =========================================================================
  // NOTION SWEETALERT MODAL (Diálogos Centrados Modernos y No-Bloqueantes)
  // =========================================================================

  function showSweetAlert({
    title = '',
    text = '',
    type = 'info', // 'success' | 'error' | 'warning' | 'info' | 'question'
    confirmButtonText = 'Aceptar',
    showCancelButton = false,
    cancelButtonText = 'Cancelar',
    isDanger = false
  }) {
    return new Promise((resolve) => {
      const oldOverlay = document.getElementById('notionSwalOverlay');
      if (oldOverlay) oldOverlay.remove();

      const overlay = document.createElement('div');
      overlay.id = 'notionSwalOverlay';
      overlay.className = 'notion-swal-overlay';

      let iconSvg = '';
      if (type === 'success') {
        iconSvg = `
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.8" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="20 6 9 17 4 12"></polyline>
          </svg>`;
      } else if (type === 'error') {
        iconSvg = `
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.8" stroke-linecap="round" stroke-linejoin="round">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>`;
      } else if (type === 'warning') {
        iconSvg = `
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.8" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"></circle>
            <line x1="12" y1="8" x2="12" y2="12"></line>
            <line x1="12" y1="16" x2="12.01" y2="16"></line>
          </svg>`;
      } else {
        iconSvg = `
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.8" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"></circle>
            <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"></path>
            <line x1="12" y1="17" x2="12.01" y2="17"></line>
          </svg>`;
      }

      overlay.innerHTML = `
        <div class="notion-swal-card" role="dialog" aria-modal="true">
          <div class="notion-swal-icon ${type}">
            ${iconSvg}
          </div>
          <h2 class="notion-swal-title">${escapeHtml(title)}</h2>
          <div class="notion-swal-text">${escapeHtml(text)}</div>
          <div class="notion-swal-actions">
            ${showCancelButton ? `<button type="button" class="notion-swal-btn notion-swal-btn-cancel" id="notionSwalBtnCancel">${escapeHtml(cancelButtonText)}</button>` : ''}
            <button type="button" class="notion-swal-btn notion-swal-btn-confirm ${isDanger ? 'danger' : ''}" id="notionSwalBtnConfirm">${escapeHtml(confirmButtonText)}</button>
          </div>
        </div>
      `;

      document.body.appendChild(overlay);

      requestAnimationFrame(() => {
        overlay.classList.add('active');
        const confirmBtn = document.getElementById('notionSwalBtnConfirm');
        if (confirmBtn) confirmBtn.focus();
      });

      function cleanup(result) {
        overlay.classList.remove('active');
        window.removeEventListener('keydown', handleKeyDown);
        setTimeout(() => overlay.remove(), 220);
        resolve(result);
      }

      function handleKeyDown(e) {
        if (e.key === 'Escape') {
          cleanup(false);
        } else if (e.key === 'Enter') {
          cleanup(true);
        }
      }

      window.addEventListener('keydown', handleKeyDown);

      document.getElementById('notionSwalBtnConfirm')?.addEventListener('click', () => cleanup(true));
      document.getElementById('notionSwalBtnCancel')?.addEventListener('click', () => cleanup(false));
      overlay.addEventListener('click', (e) => {
        if (e.target === overlay) cleanup(false);
      });
    });
  }

  const notifySuccess = (title, text = '') => showSweetAlert({ title, text, type: 'success', confirmButtonText: 'Aceptar' });
  const notifyError = (title, text = '') => showSweetAlert({ title, text, type: 'error', confirmButtonText: 'Entendido' });
  const notifyWarning = (title, text = '') => showSweetAlert({ title, text, type: 'warning', confirmButtonText: 'Entendido' });
  const notifyInfo = (title, text = '') => showSweetAlert({ title, text, type: 'info', confirmButtonText: 'Entendido' });
  const notifyConfirm = (title, text = '', confirmButtonText = 'Sí, continuar', isDanger = false) =>
    showSweetAlert({
      title,
      text,
      type: isDanger ? 'warning' : 'question',
      confirmButtonText,
      showCancelButton: true,
      cancelButtonText: 'Cancelar',
      isDanger
    });

  // Vistas y Navegación
  const viewEmpresas = $('viewEmpresas');
  const viewRce = $('viewRce');
  const viewRvie = $('viewRvie');
  const navLinkEmpresas = $('navLinkEmpresas');
  const navGroupSire = $('navGroupSire');
  const sidebarGroupSire = $('sidebarGroupSire');
  const navLinkRce = $('navLinkRce');
  const navLinkRvie = $('navLinkRvie');
  const btnCollapseSidebar = $('btnCollapseSidebar');
  const btnExpandSidebar = $('btnExpandSidebar');
  const btnHeaderSwitchCompany = $('btnHeaderSwitchCompany');
  const sireSharedDownloads = $('sireSharedDownloads');
  const downloadsSlotRce = $('downloadsSlotRce');
  const downloadsSlotRvie = $('downloadsSlotRvie');

  // Pantallas de acceso
  const loginScreen = $('loginScreen');
  const appShell = $('appShell');
  const loginForm = $('loginForm');

  // Módulo de Empresas
  const txtBuscarEmpresa = $('txtBuscarEmpresa');
  const cboPeriodoVcto = $('cboPeriodoVcto');
  const cboOrdenEmpresas = $('cboOrdenEmpresas');
  const tblEmpresas = $('tblEmpresas');
  const tbodyEmpresas = $('tbodyEmpresas');
  const chkSelectAllEmpresas = $('chkSelectAllEmpresas');
  const btnSelectEmpresa = $('btnSelectEmpresa');
  const btnNuevaEmpresa = $('btnNuevaEmpresa');
  const btnEditarEmpresa = $('btnEditarEmpresa');
  const btnGenerarToken = $('btnGenerarToken');
  const btnEliminarEmpresa = $('btnEliminarEmpresa');
  const lblVencInfo = $('lblVencInfo');

  // Modal Empresa
  const modalEmpresa = $('modalEmpresa');
  const modalEmpresaTitle = $('modalEmpresaTitle');
  const btnCerrarModalEmpresa = $('btnCerrarModalEmpresa');
  const btnCancelarModalEmpresa = $('btnCancelarModalEmpresa');
  const formModalEmpresa = $('formModalEmpresa');
  const chkVerClaveSol = $('chkVerClaveSol');
  const chkVerClientSecret = $('chkVerClientSecret');

  // SIRE & Descargas
  const dropzone = $('dropzone');
  const excelFile = $('excelFile');
  const btnDownloadTemplate = $('btnDownloadTemplate');
  const legacyDownloadPdf = $('legacyDownloadPdf');
  const legacyDownloadXml = $('legacyDownloadXml');
  const legacyDownloadCdr = $('legacyDownloadCdr');
  const legacyCancelDownload = $('legacyCancelDownload');
  const progressSection = $('progressSection');

  // Estado global
  let companies = [];
  let selectedCompanyId = null;
  let activeCompany = null;
  let isLicenseActive = false;
  let isDownloading = false;
  let loadedComprobantes = [];
  let rceComprobantes = [];
  let rvieComprobantes = [];
  let pollInterval = null;
  let sseSource = null;
  const downloadedFiles = new Map();
  let currentProposalView = null;
  let currentViewerPath = '';
  let currentViewerRawXML = '';

  initTheme();
  restoreRememberedLogin();
  populatePeriodsCombo();
  initSidebarState();
  bindEvents();
  loadBuildInfo();
  checkSession();
  setInterval(checkLicenseHeartbeat, 5 * 60 * 1000);

  async function loadBuildInfo() {
    const label = $('appBuildInfo');
    if (!label) return;
    try {
      const response = await fetch('/api/app/info', { cache: 'no-store' });
      const info = await response.json();
      const version = info?.version || 'dev';
      const builtAt = info?.built_at || 'sin fecha';
      label.textContent = `v${version} · ${builtAt}`;
      label.title = `Ejecutable ${version}, compilado ${builtAt}`;
    } catch (_) {
      label.textContent = 'Versión local no identificada';
    }
  }

  // =========================================================================
  // VISTAS Y NAVEGACIÓN
  // =========================================================================

  function switchView(targetViewId) {
    const views = {
      viewEmpresas: { el: viewEmpresas, link: navLinkEmpresas },
      viewRce: { el: viewRce, link: navLinkRce },
      viewRvie: { el: viewRvie, link: navLinkRvie }
    };

    // Ocultar todas las vistas y remover clases activas
    Object.keys(views).forEach((key) => {
      const v = views[key];
      if (v.el) {
        v.el.classList.remove('active');
        v.el.style.display = 'none';
      }
      if (v.link) {
        v.link.classList.remove('active');
      }
    });

    const target = views[targetViewId];
    if (target) {
      if (target.el) {
        target.el.classList.add('active');
        target.el.style.display = 'block';
      }
      if (target.link) {
        target.link.classList.add('active');
      }
    }

    // Si navegamos a una opción de SIRE, asegurar que el grupo esté expandido
    if (targetViewId === 'viewRce' || targetViewId === 'viewRvie') {
      if (sidebarGroupSire) {
        sidebarGroupSire.classList.remove('collapsed');
      }
      // Mantener oculto el contenedor masivo de descargas (se trasladará a un módulo aparte)
      if (sireSharedDownloads) {
        sireSharedDownloads.style.display = 'none';
      }
      syncCompanyTitlesInViews();
    } else {
      if (sireSharedDownloads) {
        sireSharedDownloads.style.display = 'none';
      }
    }
  }

  function syncCompanyTitlesInViews() {
    const text = activeCompany ? `${activeCompany.ruc} — ${activeCompany.razon_social}` : 'Ninguna empresa seleccionada';
    const rceTitle = $('currentWorkCompanyTitleRce');
    const rvieTitle = $('currentWorkCompanyTitleRvie');
    if (rceTitle) rceTitle.textContent = text;
    if (rvieTitle) rvieTitle.textContent = text;

    // Sincronizar período por defecto si está vacío
    const defaultPeriod = cboPeriodoVcto?.value || new Date().toISOString().slice(0, 7);
    const periodRce = $('proposalPeriodRce');
    const periodRvie = $('proposalPeriodRvie');
    if (periodRce && !periodRce.value) periodRce.value = defaultPeriod;
    if (periodRvie && !periodRvie.value) periodRvie.value = defaultPeriod;
  }

  // =========================================================================
  // SIDEBAR PLEGABLE NOTION
  // =========================================================================

  function toggleSidebar(forceCollapse) {
    const isCurrentlyCollapsed = appShell.classList.contains('sidebar-collapsed');
    const willCollapse = typeof forceCollapse === 'boolean' ? forceCollapse : !isCurrentlyCollapsed;

    if (willCollapse) {
      appShell.classList.add('sidebar-collapsed');
      localStorage.setItem('autosire_sidebar_collapsed', 'true');
    } else {
      appShell.classList.remove('sidebar-collapsed');
      localStorage.setItem('autosire_sidebar_collapsed', 'false');
    }
  }

  function initSidebarState() {
    if (localStorage.getItem('autosire_sidebar_collapsed') === 'true') {
      toggleSidebar(true);
    }
  }

  function bindEvents() {
    // Navegación entre vistas
    navLinkEmpresas.addEventListener('click', () => switchView('viewEmpresas'));
    navGroupSire.addEventListener('click', () => {
      sidebarGroupSire.classList.toggle('collapsed');
    });
    navLinkRce.addEventListener('click', () => switchView('viewRce'));
    navLinkRvie.addEventListener('click', () => switchView('viewRvie'));

    // Plegado y desplegado de sidebar
    btnCollapseSidebar?.addEventListener('click', (e) => {
      e.stopPropagation();
      toggleSidebar(true);
    });
    btnExpandSidebar?.addEventListener('click', (e) => {
      e.stopPropagation();
      toggleSidebar(false);
    });
    $('sidebarToggle')?.addEventListener('click', (e) => {
      e.stopPropagation();
      toggleMobileDrawer();
    });

    // Atajo de teclado Notion (Ctrl + \)
    window.addEventListener('keydown', (e) => {
      if (e.ctrlKey && e.key === '\\') {
        e.preventDefault();
        toggleSidebar();
      }
    });

    if (btnHeaderSwitchCompany) {
      btnHeaderSwitchCompany.addEventListener('click', () => switchView('viewEmpresas'));
    }
    document.querySelectorAll('.btn-change-company').forEach((btn) => {
      btn.addEventListener('click', () => switchView('viewEmpresas'));
    });

    // Auth & Licencia
    loginForm.addEventListener('submit', login);
    $('btnLogout').addEventListener('click', logout);
    $('btnManageLicense').addEventListener('click', () => showLogin('Ingresa para cambiar de licencia'));

    // Módulo de Empresas
    txtBuscarEmpresa.addEventListener('input', renderEmpresasTable);
    cboPeriodoVcto.addEventListener('change', () => {
      updateNoticeCallout();
      renderEmpresasTable();
    });
    cboOrdenEmpresas.addEventListener('change', renderEmpresasTable);

    chkSelectAllEmpresas.addEventListener('change', (e) => {
      const checked = e.target.checked;
      tbodyEmpresas.querySelectorAll('.chk-empresa').forEach((chk) => {
        chk.checked = checked;
      });
    });

    // Clic en encabezados de tabla para ordenar
    tblEmpresas.querySelectorAll('th[data-sort]').forEach((th) => {
      th.addEventListener('click', () => {
        const sortType = th.dataset.sort;
        if (sortType === 'nombre') {
          cboOrdenEmpresas.value = cboOrdenEmpresas.value === 'Nombre_asc' ? 'Nombre_desc' : 'Nombre_asc';
        } else if (sortType === 'ruc') {
          cboOrdenEmpresas.value = 'Ruc_asc';
        } else if (sortType === 'token') {
          cboOrdenEmpresas.value = 'Token_desc';
        } else if (sortType === 'ult') {
          cboOrdenEmpresas.value = 'UltDigito_asc';
        } else if (sortType === 'vencesire') {
          cboOrdenEmpresas.value = 'VenceSire_asc';
        } else if (sortType === 'vencepdt') {
          cboOrdenEmpresas.value = 'VencePdt_asc';
        }
        renderEmpresasTable();
      });
    });

    // Botones de acción del Macro
    btnSelectEmpresa.addEventListener('click', () => {
      if (!selectedCompanyId) {
        notifyWarning('Seleccione una Empresa', 'Por favor, selecciona una empresa de la lista para continuar.');
        return;
      }
      selectCompany(selectedCompanyId);
    });

    btnNuevaEmpresa.addEventListener('click', () => openModalEmpresa(false));
    btnEditarEmpresa.addEventListener('click', () => {
      if (!selectedCompanyId) {
        notifyWarning('Seleccione una Empresa', 'Por favor, selecciona la empresa que deseas editar.');
        return;
      }
      openModalEmpresa(true, selectedCompanyId);
    });

    btnGenerarToken.addEventListener('click', generateCompanyToken);
    btnEliminarEmpresa.addEventListener('click', deleteCompany);

    // Modal Empresa
    btnCerrarModalEmpresa.addEventListener('click', () => modalEmpresa.close());
    btnCancelarModalEmpresa.addEventListener('click', () => modalEmpresa.close());
    formModalEmpresa.addEventListener('submit', handleSaveModalEmpresa);

    chkVerClaveSol.addEventListener('change', (e) => {
      $('txtEmpresaClaveSol').type = e.target.checked ? 'text' : 'password';
    });
    chkVerClientSecret.addEventListener('change', (e) => {
      $('txtEmpresaClientSecret').type = e.target.checked ? 'text' : 'password';
    });

    // SIRE & Descargas RCE / RVIE
    $('btnDownloadProposalRce')?.addEventListener('click', () => downloadProposalForBook('RCE'));
    $('btnDownloadProposalRvie')?.addEventListener('click', () => downloadProposalForBook('RVIE'));
    btnDownloadTemplate.addEventListener('click', () => {
      window.location.href = '/api/excel/template';
    });

    $('btnBrowse').addEventListener('click', () => excelFile.click());
    dropzone.addEventListener('click', (event) => {
      if (event.target !== $('btnBrowse')) excelFile.click();
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
    excelFile.addEventListener('change', (event) => {
      if (event.target.files.length) uploadExcel(event.target.files[0]);
    });

    $('concurrencyRange').addEventListener('input', (event) => {
      $('concurrencyVal').textContent = event.target.value;
    });

    legacyDownloadXml.addEventListener('click', () => startDownload('XML'));
    legacyDownloadPdf.addEventListener('click', () => startDownload('PDF'));
    legacyDownloadCdr.addEventListener('click', () => startDownload('CDR'));
    legacyCancelDownload.addEventListener('click', cancelDownload);

    for (const book of ['RCE', 'RVIE']) {
      const suffix = book === 'RCE' ? 'Rce' : 'Rvie';
      for (const [buttonName, type] of [['Xml', 'XML'], ['Cdr', 'CDR'], ['Pdf', 'PDF'], ['Desc', 'DESC']]) {
        $(`btnDescarga${buttonName}${suffix}`)?.addEventListener('click', () => startProposalDownload(book, type));
      }
    }

    $('uploadedRecordsBody').addEventListener('click', handleRecordAction);
    $('proposalPreviewBodyRce')?.addEventListener('click', handleRecordAction);
    $('proposalPreviewBodyRvie')?.addEventListener('click', handleRecordAction);
    $('resultsTbody').addEventListener('click', handleRecordAction);
    $('fileViewerClose').addEventListener('click', closeFileViewer);
    $('fileViewer').addEventListener('click', (event) => {
      if (event.target === $('fileViewer')) closeFileViewer();
    });
    $('fileViewerSummaryTab').addEventListener('click', () => showXMLViewerTab('summary'));
    $('fileViewerRawTab').addEventListener('click', () => showXMLViewerTab('raw'));

    $('btnOpenFolder').addEventListener('click', openFolder);
    $('btnDownloadZip').addEventListener('click', () => {
      window.location.href = '/api/files/download-zip';
    });

    const themeToggle = $('themeToggleBtn');
    if (themeToggle) {
      themeToggle.addEventListener('click', toggleTheme);
    }

    document.querySelectorAll('.tab-btn').forEach((button) => {
      button.addEventListener('click', () => {
        document.querySelectorAll('.tab-btn').forEach((item) => item.classList.remove('active'));
        document.querySelectorAll('.tab-content').forEach((item) => item.classList.remove('active'));
        button.classList.add('active');
        $(button.dataset.tab).classList.add('active');
      });
    });
  }

  // =========================================================================
  // PERIODOS Y CALENDARIO
  // =========================================================================

  function populatePeriodsCombo() {
    const meses = ['', 'Ene', 'Feb', 'Mar', 'Abr', 'May', 'Jun', 'Jul', 'Ago', 'Set', 'Oct', 'Nov', 'Dic'];
    const now = new Date();
    cboPeriodoVcto.innerHTML = '';

    // 1 mes en el futuro hasta 12 meses atrás
    for (let offset = 1; offset >= -12; offset--) {
      const d = new Date(now.getFullYear(), now.getMonth() + offset, 1);
      const text = `${meses[d.getMonth() + 1]}-${d.getFullYear()}`;
      const opt = document.createElement('option');
      opt.value = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`;
      opt.textContent = text;
      // Por defecto el mes anterior (o actual)
      if (offset === -1) {
        opt.selected = true;
      }
      cboPeriodoVcto.appendChild(opt);
    }
    updateNoticeCallout();
  }

  function updateNoticeCallout() {
    const selectedText = cboPeriodoVcto.options[cboPeriodoVcto.selectedIndex]?.textContent || 'este período';
    const content = $('lblVencInfoText');
    if (content) {
      content.innerHTML = `Período tributario activo: <strong>${escapeHtml(selectedText)}</strong>. Selecciona un contribuyente para gestionar sus comprobantes electrónicos y propuesta SIRE.`;
    }
  }

  // =========================================================================
  // GESTIÓN DE EMPRESAS
  // =========================================================================

  async function loadCompanies(autoConnect = false) {
    let retries = 2;
    let lastErr = null;
    while (retries >= 0) {
      try {
        const response = await apiFetch('/api/companies');
        const data = await response.json();
        if (!response.ok || !data.success) throw new Error(data.error || 'No se pudieron cargar las empresas');
        companies = data.empresas || [];

        const selected = companies.find((item) => item.seleccionada);
        if (selected) {
          selectedCompanyId = selected.id;
          updateActiveCompanyHeader(selected);
          if (autoConnect) {
            connectCompanyQuietly(selected.id);
          }
        } else if (companies.length > 0 && !selectedCompanyId) {
          selectedCompanyId = companies[0].id;
        }

        renderEmpresasTable();
        return;
      } catch (err) {
        lastErr = err;
        retries--;
        if (retries >= 0) {
          await new Promise((r) => setTimeout(r, 600));
        }
      }
    }
    tbodyEmpresas.innerHTML = `<tr><td colspan="9" style="text-align:center; color:var(--danger); padding:24px;">Error al cargar empresas: ${escapeHtml(lastErr.message)}</td></tr>`;
  }

  function renderEmpresasTable() {
    const filter = (txtBuscarEmpresa.value || '').trim().toUpperCase();
    let list = [...companies];

    if (filter) {
      list = list.filter((c) =>
        (c.razon_social || '').toUpperCase().includes(filter) ||
        (c.ruc || '').toUpperCase().includes(filter)
      );
    }

    // Ordenamiento
    const sortVal = cboOrdenEmpresas.value;
    list.sort((a, b) => {
      switch (sortVal) {
        case 'Nombre_desc':
          return (b.razon_social || '').localeCompare(a.razon_social || '');
        case 'Ruc_asc':
          return (a.ruc || '').localeCompare(b.ruc || '');
        case 'UltDigito_asc':
          return (a.ult_digito || '').localeCompare(b.ult_digito || '');
        case 'Token_desc':
          return (a.estado_token || 0) - (b.estado_token || 0);
        case 'Nombre_asc':
        default:
          return (a.razon_social || '').localeCompare(b.razon_social || '');
      }
    });

    const lblCount = $('lblEmpresasCount');
    if (lblCount) {
      lblCount.textContent = `${list.length} de ${companies.length} empresa(s) en SQLite local`;
    }

    if (list.length === 0) {
      tbodyEmpresas.innerHTML = `<tr><td colspan="9" style="text-align:center; padding:36px; color:var(--notion-text-subtle);">No se encontraron empresas registradas. Haz clic en <strong>+ Nueva empresa</strong> para agregar un contribuyente.</td></tr>`;
      return;
    }

    tbodyEmpresas.innerHTML = list.map((item, index) => {
      const isSelected = item.id === selectedCompanyId;
      let tokenBadge = '<span class="notion-tag notion-tag-gray">○ Sin Token</span>';
      if (item.estado_token === 2) {
        tokenBadge = '<span class="notion-tag notion-tag-green">● Token Activo</span>';
      } else if (item.estado_token === 1) {
        tokenBadge = '<span class="notion-tag notion-tag-yellow">◑ Parcial</span>';
      }

      return `
        <tr class="${isSelected ? 'selected' : ''}" data-id="${item.id}">
          <td class="col-sel"><input type="checkbox" class="chk-empresa" data-id="${item.id}" ${isSelected ? 'checked' : ''}></td>
          <td class="col-nro">${index + 1}</td>
          <td class="col-empresa"><strong>${escapeHtml(item.razon_social)}</strong></td>
          <td class="col-ruc"><code>${escapeHtml(item.ruc)}</code></td>
          <td class="col-token">${tokenBadge}</td>
          <td class="col-ult">${escapeHtml(item.ult_digito || '—')}</td>
          <td class="col-vencesire">${escapeHtml(item.vence_sire || '—')}</td>
          <td class="col-vencepdt">${escapeHtml(item.vence_pdt || '—')}</td>
          <td class="col-estado">${escapeHtml(item.estado || 'Activo')}</td>
        </tr>
      `;
    }).join('');

    // Eventos de selección de filas
    tbodyEmpresas.querySelectorAll('tr[data-id]').forEach((row) => {
      row.addEventListener('click', (e) => {
        if (e.target.tagName === 'INPUT') return;
        const id = Number(row.dataset.id);
        selectedCompanyId = id;
        tbodyEmpresas.querySelectorAll('tr').forEach((r) => r.classList.remove('selected'));
        row.classList.add('selected');
      });

      row.addEventListener('dblclick', () => {
        const id = Number(row.dataset.id);
        selectCompany(id);
      });
    });
  }

  function updateActiveCompanyHeader(company) {
    const companyChanged = activeCompany && (!company || activeCompany.id !== company.id);
    if (companyChanged) resetCompanyWorkspace();
    if (!company) {
      activeCompany = null;
      $('activeCompanyPill').style.display = 'none';
      syncCompanyTitlesInViews();
      return;
    }
    activeCompany = company;
    $('activeCompanyName').textContent = `${company.ruc} · ${company.razon_social}`;
    $('activeCompanyPill').style.display = 'inline-flex';
    syncCompanyTitlesInViews();
  }

  async function selectCompany(id) {
    btnSelectEmpresa.disabled = true;
    $('statusDetails').textContent = 'Conectando con SUNAT…';
    try {
      const response = await apiFetch('/api/companies/select', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id })
      });
      const data = await response.json();
      if (!response.ok || !data.success) throw new Error(data.error || 'Error al conectar con SUNAT');

      setConnectedUI(data.ruc, data.razon_social);
      await loadCompanies(false);

      const comp = companies.find((c) => c.id === id);
      if (comp) {
        updateActiveCompanyHeader(comp);
      }

      // Transición automática al módulo de compras RCE
      switchView('viewRce');
    } catch (err) {
      setDisconnectedUI(err.message);
      notifyError('Error de Conexión SUNAT', `No se pudo conectar con SUNAT: ${err.message}`);
    } finally {
      btnSelectEmpresa.disabled = false;
    }
  }

  async function connectCompanyQuietly(id) {
    try {
      const response = await apiFetch('/api/companies/select', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id })
      });
      const data = await response.json();
      if (response.ok && data.success) {
        setConnectedUI(data.ruc, data.razon_social);
      }
    } catch (_) {}
  }

  async function generateCompanyToken() {
    if (!selectedCompanyId) {
      notifyWarning('Seleccione una Empresa', 'Debe seleccionar una empresa de la tabla para generar su token de acceso.');
      return;
    }
    btnGenerarToken.disabled = true;
    try {
      const response = await apiFetch('/api/companies/token', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: selectedCompanyId })
      });
      const data = await response.json();
      if (!response.ok || !data.success) throw new Error(data.error || 'Error generando token en SUNAT');

      // 1. Recargar empresas inmediatamente en segundo plano
      await loadCompanies(false);

      // 2. Modal SweetAlert de éxito llamativo y centrado
      notifySuccess('Token Generado con Éxito', data.message || `Token generado para ${data.razon_social}. Válido por 1 hora.`);
    } catch (err) {
      notifyError('Error al Generar Token', err.message);
    } finally {
      btnGenerarToken.disabled = false;
    }
  }

  async function deleteCompany() {
    // Buscar todas las empresas marcadas con checkbox
    const checked = Array.from(tbodyEmpresas.querySelectorAll('.chk-empresa:checked'))
      .map((chk) => Number(chk.dataset.id))
      .filter(Boolean);

    let idsToDelete = checked;
    if (idsToDelete.length === 0 && selectedCompanyId) {
      idsToDelete = [selectedCompanyId];
    }

    if (idsToDelete.length === 0) {
      notifyWarning('Selección Requerida', 'Seleccione la empresa o empresas que desea eliminar.');
      return;
    }

    const confirmed = await notifyConfirm(
      '¿Eliminar empresa(s)?',
      `¿Seguro que desea eliminar ${idsToDelete.length} empresa(s) registrada(s) en este equipo? Esta acción no se puede deshacer.`,
      'Sí, eliminar',
      true
    );
    if (!confirmed) return;

    try {
      const response = await apiFetch('/api/companies/delete-multiple', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids: idsToDelete })
      });
      const data = await response.json();
      if (!response.ok || !data.success) throw new Error(data.error || 'No se pudo eliminar');

      selectedCompanyId = null;
      await loadCompanies(false);
      setDisconnectedUI();
      notifySuccess('Empresas Eliminadas', 'Las empresas seleccionadas han sido eliminadas correctamente.');
    } catch (err) {
      notifyError('Error al Eliminar', err.message);
    }
  }

  // Modal Nueva / Editar Empresa
  async function openModalEmpresa(isEdit, companyId = null) {
    formModalEmpresa.reset();
    $('chkVerClaveSol').checked = false;
    $('chkVerClientSecret').checked = false;
    $('txtEmpresaClaveSol').type = 'password';
    $('txtEmpresaClientSecret').type = 'password';

    if (isEdit && companyId) {
      modalEmpresaTitle.textContent = 'EDITAR EMPRESA';
      $('empresaId').value = String(companyId);
      try {
        const response = await apiFetch(`/api/companies?id=${companyId}`);
        const data = await response.json();
        if (!response.ok || !data.success) throw new Error(data.error || 'No se pudieron leer los datos');
        const c = data.empresa;
        $('txtEmpresaRuc').value = c.ruc || '';
        $('txtEmpresaNombre').value = c.razon_social || '';
        $('txtEmpresaUsuarioSol').value = c.usuario_sol || '';
        $('txtEmpresaClientId').value = c.client_id || '';
        $('cboEmpresaRegimen').value = c.regimen || '';
        $('txtEmpresaWhatsapp').value = c.whatsapp || '';
        $('txtEmpresaClaveSol').placeholder = '••••••••••••';
        $('txtEmpresaClientSecret').placeholder = '••••••••••••••••';
      } catch (err) {
        notifyError('Error al Cargar Datos', err.message);
        return;
      }
    } else {
      modalEmpresaTitle.textContent = 'NUEVA EMPRESA';
      $('empresaId').value = '0';
      $('txtEmpresaClaveSol').placeholder = '••••••••••••';
      $('txtEmpresaClientSecret').placeholder = '••••••••••••••••';
    }

    modalEmpresa.showModal();
    $('txtEmpresaNombre').focus();
  }

  async function handleSaveModalEmpresa(e) {
    e.preventDefault();
    const saveBtn = $('btnGuardarModalEmpresa');
    const spinner = $('modalEmpresaSpinner');
    saveBtn.disabled = true;
    spinner.style.display = 'inline-block';

    const id = Number($('empresaId').value) || 0;
    const payload = {
      id,
      ruc: $('txtEmpresaRuc').value.trim(),
      razon_social: $('txtEmpresaNombre').value.trim(),
      usuario_sol: $('txtEmpresaUsuarioSol').value.trim(),
      clave_sol: $('txtEmpresaClaveSol').value,
      client_id: $('txtEmpresaClientId').value.trim(),
      client_secret: $('txtEmpresaClientSecret').value,
      regimen: $('cboEmpresaRegimen').value,
      whatsapp: $('txtEmpresaWhatsapp').value.trim()
    };

    try {
      const response = await apiFetch('/api/companies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      const data = await response.json();
      if (!response.ok || !data.success) throw new Error(data.error || 'No se pudo guardar la empresa');

      modalEmpresa.close();
      await loadCompanies(false);
      if (data.id) {
        selectedCompanyId = data.id;
        renderEmpresasTable();
      }
      notifySuccess('Empresa Guardada', 'La empresa y sus credenciales se guardaron exitosamente en SQLite local.');
    } catch (err) {
      notifyError('Error al Guardar', err.message);
    } finally {
      saveBtn.disabled = false;
      spinner.style.display = 'none';
    }
  }

  // =========================================================================
  // SIRE & PROPUESTAS SUNAT
  // =========================================================================

  function setDefaultProposalPeriod() {
    const now = new Date();
    const prev = new Date(now.getFullYear(), now.getMonth() - 1, 1);
    const y = prev.getFullYear();
    const m = String(prev.getMonth() + 1).padStart(2, '0');
    const val = `${y}-${m}`;

    const rce = $('proposalPeriodRce');
    if (rce && !rce.value) rce.value = val;
    const rvie = $('proposalPeriodRvie');
    if (rvie && !rvie.value) rvie.value = val;
  }

  async function downloadProposalForBook(book) {
    const isRce = book === 'RCE';
    const periodInput = $(isRce ? 'proposalPeriodRce' : 'proposalPeriodRvie');
    const periodVal = periodInput ? periodInput.value : '';

    if (!periodVal) {
      notifyWarning('Período Requerido', `Selecciona el período tributario para consultar la propuesta ${book}.`);
      return;
    }

    const button = $(isRce ? 'btnDownloadProposalRce' : 'btnDownloadProposalRvie');
    const spinner = $(isRce ? 'proposalSpinnerRce' : 'proposalSpinnerRvie');
    const reuseCheck = $(isRce ? 'proposalReuseRce' : 'proposalReuseRvie');
    const wrap = $(isRce ? 'proposalPreviewWrapRce' : 'proposalPreviewWrapRvie');

    if (button) button.disabled = true;
    if (spinner) spinner.style.display = 'inline-block';
    if (wrap) wrap.style.display = 'none';

    try {
      const response = await apiFetch('/api/sire/proposal/download', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          periodo: periodVal.replace('-', ''),
          libro: book,
          reutilizar_existente: reuseCheck ? reuseCheck.checked : true
        })
      });
      const data = await response.json();
      if (!response.ok || !data.success) throw new Error(data.error || `No se pudo descargar la propuesta ${book}`);

      const comprobantes = data.preview?.comprobantes || [];
      if (isRce) {
        rceComprobantes = comprobantes;
        currentProposalContextRce = {
          companyId: activeCompany?.id ?? selectedCompanyId,
          ruc: activeCompany?.ruc || '',
          period: data.periodo,
          ticket: data.ticket,
          book
        };
      } else {
        rvieComprobantes = comprobantes;
        currentProposalContextRvie = {
          companyId: activeCompany?.id ?? selectedCompanyId,
          ruc: activeCompany?.ruc || '',
          period: data.periodo,
          ticket: data.ticket,
          book
        };
      }

      currentProposalView = { preview: data.preview, href: `/api/files/view?download=1&path=${encodeURIComponent(data.path)}`, book, period: data.periodo };

      const fileBtn = $(isRce ? 'btnDownloadOfficialFileRce' : 'btnDownloadOfficialFileRvie');
      if (fileBtn) {
        if (data.path) {
          fileBtn.href = `/api/files/view?download=1&path=${encodeURIComponent(data.path)}`;
          fileBtn.style.display = 'inline-flex';
        } else {
          fileBtn.style.display = 'none';
        }
      }

      renderProposalPreviewForBook(book, data.preview, data.libro, data.periodo);
      notifySuccess(`Propuesta ${book} Obtenida`, `Se procesaron exitosamente ${comprobantes.length} comprobantes de ${isRce ? 'compras' : 'ventas'}.`);
    } catch (error) {
      notifyError(`Error en Propuesta ${book}`, error.message);
    } finally {
      if (button) button.disabled = false;
      if (spinner) spinner.style.display = 'none';
    }
  }

  let currentProposalItemsRce = [];
  let currentProposalItemsRvie = [];
  let currentProposalContextRce = null;
  let currentProposalContextRvie = null;
  const proposalStatusMapsRce = createProposalStatusMaps();
  const proposalStatusMapsRvie = createProposalStatusMaps();

  function createProposalStatusMaps() {
    return {
      XML: new Map(),
      CDR: new Map(),
      PDF: new Map(),
      DESC: new Map()
    };
  }

  function proposalStatusMap(isRce, type) {
    const maps = isRce ? proposalStatusMapsRce : proposalStatusMapsRvie;
    return maps[type];
  }

  function clearProposalState() {
    rceComprobantes = [];
    rvieComprobantes = [];
    currentProposalItemsRce = [];
    currentProposalItemsRvie = [];
    currentProposalContextRce = null;
    currentProposalContextRvie = null;
    Object.values(proposalStatusMapsRce).forEach((statusMap) => statusMap.clear());
    Object.values(proposalStatusMapsRvie).forEach((statusMap) => statusMap.clear());
    for (const id of ['proposalPreviewWrapRce', 'proposalPreviewWrapRvie']) {
      const element = $(id);
      if (element) element.style.display = 'none';
    }
  }

  function resetCompanyWorkspace() {
    clearProposalState();
    currentProposalView = null;
    loadedComprobantes = [];
    downloadedFiles.clear();
    isDownloading = false;

    const excelSummary = $('excelSummary');
    if (excelSummary) excelSummary.style.display = 'none';
    const uploadedRecords = $('uploadedRecordsSection');
    if (uploadedRecords) uploadedRecords.style.display = 'none';
    const uploadedBody = $('uploadedRecordsBody');
    if (uploadedBody) uploadedBody.innerHTML = '';
    const resultsBody = $('resultsTbody');
    if (resultsBody) resultsBody.innerHTML = '';
    const resultsCount = $('resultsCount');
    if (resultsCount) resultsCount.textContent = '0';
    const terminal = $('terminalLogs');
    if (terminal) terminal.innerHTML = '';
    if (progressSection) progressSection.style.display = 'none';
    updateDownloadStartButton();
  }

  function normalizeNumero(num) {
    if (!num) return '';
    const clean = String(num).replace(/[\s,]/g, '');
    const parsed = parseInt(clean, 10);
    return isNaN(parsed) ? clean : String(parsed);
  }

  function getCompKey(comp) {
    if (!comp) return '';
    let tipo = comp.tipo || '';
    let serie = comp.serie || '';
    let numero = comp.numero || '';

    if (!serie && comp.comp_pago) {
      const parts = comp.comp_pago.trim().split(' ');
      if (parts.length > 1) {
        tipo = parts[0];
        const sn = parts[1].split('-');
        serie = sn[0];
        numero = sn[1];
      } else if (parts[0].includes('-')) {
        const sn = parts[0].split('-');
        serie = sn[0];
        numero = sn[1];
      }
    }

    tipo = String(tipo).padStart(2, '0').trim();
    serie = String(serie).toUpperCase().trim();
    numero = normalizeNumero(numero);

    return `${tipo}-${serie}-${numero}`;
  }

  function renderProposalPreviewForBook(book, preview, bookName, period) {
    const isRce = book === 'RCE';
    const wrap = $(isRce ? 'proposalPreviewWrapRce' : 'proposalPreviewWrapRvie');
    const metaBook = $(isRce ? 'proposalMetaBookRce' : 'proposalMetaBookRvie');
    const metaPeriod = $(isRce ? 'proposalMetaPeriodRce' : 'proposalMetaPeriodRvie');

    if (metaBook) metaBook.textContent = isRce ? 'COMPRAS (RCE)' : 'VENTAS (RVIE)';
    if (metaPeriod) metaPeriod.textContent = period || 'PERIODO';

    // Obtener los items parseados con las 24 columnas
    let items = preview?.items || [];

    // Fallback si no vinieran items estructurados
    if (!items.length && preview?.comprobantes) {
      items = (preview.comprobantes || []).map((c) => ({
        comp_pago: `${c.serie || ''}-${c.numero || ''}`.trim(),
        tipo_doc_ident: '6',
        ruc: c.ruc || '',
        razon_social: c.razon_social || '',
        fecha: c.fecha_emision || '',
        tipo_doc_ref: '',
        serie_doc_ref: '',
        nro_doc_ref: '',
        fecha_ref: '',
        bi_gravada: c.monto || '0.00',
        bi_gravada_10: '0.00',
        bi_grav_y_no_grav: '0.00',
        bi_no_gravada: '0.00',
        adq_no_gravada: '0.00',
        icbper: '0.00',
        igv: '0.00',
        grav_y_no_grav_igv: '0.00',
        igv_10: '0.00',
        importe_total: c.monto || '0.00',
        isc: '0.00',
        no_grav_igv: '0.00',
        otros_conceptos: '0.00',
        otros_tributos: '0.00',
        valor_adquisiciones: '0.00',
        moneda: c.moneda || 'PEN'
      }));
    }

    if (isRce) {
      currentProposalItemsRce = items;
    } else {
      currentProposalItemsRvie = items;
    }

    renderProposalTableGrid(isRce, items);

    // Configurar buscador en vivo
    const searchInput = $(isRce ? 'txtSearchProposalRce' : 'txtSearchProposalRvie');
    if (searchInput) {
      searchInput.value = '';
      searchInput.oninput = () => {
        const query = searchInput.value.toLowerCase().trim();
        const filtered = items.filter((it) => {
          if (!query) return true;
          return (it.comp_pago || '').toLowerCase().includes(query) ||
                 (it.ruc || '').toLowerCase().includes(query) ||
                 (it.razon_social || '').toLowerCase().includes(query) ||
                 (it.fecha || '').toLowerCase().includes(query);
        });
        renderProposalTableGrid(isRce, filtered);
      };
    }

    if (wrap) wrap.style.display = 'block';
  }

  function renderProposalTableGrid(isRce, items) {
    const body = $(isRce ? 'proposalPreviewBodyRce' : 'proposalPreviewBodyRvie');
    const foot = $(isRce ? 'proposalPreviewFootRce' : 'proposalPreviewFootRvie');
    const metaCount = $(isRce ? 'proposalMetaCountRce' : 'proposalMetaCountRvie');
    const metaTotal = $(isRce ? 'proposalMetaTotalRce' : 'proposalMetaTotalRvie');
    const xmlStatusMap = proposalStatusMap(isRce, 'XML');
    const cdrStatusMap = proposalStatusMap(isRce, 'CDR');
    const pdfStatusMap = proposalStatusMap(isRce, 'PDF');
    const descriptionStatusMap = proposalStatusMap(isRce, 'DESC');

    if (metaCount) metaCount.textContent = `${items.length} comprobantes`;

    if (!items.length) {
      if (body) {
        body.innerHTML = '<tr><td colspan="31" style="text-align:center; padding: 24px; color: var(--notion-text-subtle);">No se encontraron comprobantes en la propuesta oficial.</td></tr>';
      }
      if (foot) foot.innerHTML = '';
      if (metaTotal) metaTotal.textContent = 'Total: S/ 0.00';
      return;
    }

    // Acumuladores de totales
    let sumBIGravada = 0;
    let sumBIGravada10 = 0;
    let sumBIGravYNoGrav = 0;
    let sumBINoGravada = 0;
    let sumAdqNoGravada = 0;
    let sumICBPER = 0;
    let sumIGV = 0;
    let sumGravYNoGravIGV = 0;
    let sumIGV10 = 0;
    let sumImporteTotal = 0;
    let sumISC = 0;
    let sumNoGravIGV = 0;
    let sumOtrosConceptos = 0;
    let sumOtrosTributos = 0;
    let sumValorAdquisiciones = 0;

    const parseNum = (val) => {
      const n = parseFloat(String(val || '0').replace(/,/g, ''));
      return isNaN(n) ? 0 : n;
    };

    const fmtMoney = (num) => {
      return Number(num).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    };

    if (body) {
      body.innerHTML = items.map((it, idx) => {
        sumBIGravada += parseNum(it.bi_gravada);
        sumBIGravada10 += parseNum(it.bi_gravada_10);
        sumBIGravYNoGrav += parseNum(it.bi_grav_y_no_grav);
        sumBINoGravada += parseNum(it.bi_no_gravada);
        sumAdqNoGravada += parseNum(it.adq_no_gravada);
        sumICBPER += parseNum(it.icbper);
        sumIGV += parseNum(it.igv);
        sumGravYNoGravIGV += parseNum(it.grav_y_no_grav_igv);
        sumIGV10 += parseNum(it.igv_10);
        sumImporteTotal += parseNum(it.importe_total);
        sumISC += parseNum(it.isc);
        sumNoGravIGV += parseNum(it.no_grav_igv);
        sumOtrosConceptos += parseNum(it.otros_conceptos);
        sumOtrosTributos += parseNum(it.otros_tributos);
        sumValorAdquisiciones += parseNum(it.valor_adquisiciones);

        const compKey = getCompKey(it);
        const xmlCell = renderProposalArtifactStatus(xmlStatusMap.get(compKey), 'XML');
        const cdrCell = renderProposalArtifactStatus(
          cdrStatusMap.get(compKey),
          'CDR',
          String(it.serie || '').toUpperCase().startsWith('E')
        );
        const pdfCell = renderProposalArtifactStatus(pdfStatusMap.get(compKey), 'PDF');
        const descriptionCell = renderProposalDescription(descriptionStatusMap.get(compKey));

        return `
          <tr>
            <td style="text-align:center; color: var(--notion-text-subtle); font-weight: 500;">${idx + 1}</td>
            <td class="col-artifact">${xmlCell}</td>
            <td class="col-artifact">${cdrCell}</td>
            <td class="col-artifact">${pdfCell}</td>
            <td><strong>${escapeHtml(it.comp_pago || '—')}</strong></td>
            <td class="col-description">${descriptionCell}</td>
            <td style="text-align:center;">${escapeHtml(it.tipo_doc_ident || '—')}</td>
            <td><code>${escapeHtml(it.ruc || '—')}</code></td>
            <td style="max-width:260px; overflow:hidden; text-overflow:ellipsis;" title="${escapeHtml(it.razon_social || '')}">${escapeHtml(it.razon_social || '—')}</td>
            <td style="text-align:center;">${escapeHtml(it.fecha || '—')}</td>
            <td style="text-align:center;">${escapeHtml(it.moneda || 'PEN')}</td>
            <td class="num">${escapeHtml(it.tipo_cambio || '—')}</td>
            <td style="text-align:center;">${escapeHtml(it.tipo_doc_ref || '—')}</td>
            <td style="text-align:center;">${escapeHtml(it.serie_doc_ref || '—')}</td>
            <td style="text-align:center;">${escapeHtml(it.nro_doc_ref || '—')}</td>
            <td style="text-align:center;">${escapeHtml(it.fecha_ref || '—')}</td>
            <td class="num">${fmtMoney(it.bi_gravada)}</td>
            <td class="num">${fmtMoney(it.bi_gravada_10)}</td>
            <td class="num">${fmtMoney(it.bi_grav_y_no_grav)}</td>
            <td class="num">${fmtMoney(it.bi_no_gravada)}</td>
            <td class="num">${fmtMoney(it.adq_no_gravada)}</td>
            <td class="num">${fmtMoney(it.icbper)}</td>
            <td class="num">${fmtMoney(it.igv)}</td>
            <td class="num">${fmtMoney(it.grav_y_no_grav_igv)}</td>
            <td class="num">${fmtMoney(it.igv_10)}</td>
            <td class="num font-bold">${fmtMoney(it.importe_total)}</td>
            <td class="num">${fmtMoney(it.isc)}</td>
            <td class="num">${fmtMoney(it.no_grav_igv)}</td>
            <td class="num">${fmtMoney(it.otros_conceptos)}</td>
            <td class="num">${fmtMoney(it.otros_tributos)}</td>
            <td class="num">${fmtMoney(it.valor_adquisiciones)}</td>
          </tr>
        `;
      }).join('');
    }

    if (metaTotal) {
      metaTotal.textContent = `Total: S/ ${fmtMoney(sumImporteTotal)}`;
    }

    if (foot) {
      foot.innerHTML = `
        <tr>
          <td colspan="16" style="text-align: right; font-weight: 700; text-transform: uppercase;">TOTALES GENERALES EN SOLES (${items.length}):</td>
          <td class="num">${fmtMoney(sumBIGravada)}</td>
          <td class="num">${fmtMoney(sumBIGravada10)}</td>
          <td class="num">${fmtMoney(sumBIGravYNoGrav)}</td>
          <td class="num">${fmtMoney(sumBINoGravada)}</td>
          <td class="num">${fmtMoney(sumAdqNoGravada)}</td>
          <td class="num">${fmtMoney(sumICBPER)}</td>
          <td class="num">${fmtMoney(sumIGV)}</td>
          <td class="num">${fmtMoney(sumGravYNoGravIGV)}</td>
          <td class="num">${fmtMoney(sumIGV10)}</td>
          <td class="num font-bold" style="color: var(--tag-green-text); font-size: 0.9rem;">${fmtMoney(sumImporteTotal)}</td>
          <td class="num">${fmtMoney(sumISC)}</td>
          <td class="num">${fmtMoney(sumNoGravIGV)}</td>
          <td class="num">${fmtMoney(sumOtrosConceptos)}</td>
          <td class="num">${fmtMoney(sumOtrosTributos)}</td>
          <td class="num">${fmtMoney(sumValorAdquisiciones)}</td>
        </tr>
      `;
    }
  }

  function renderProposalArtifactStatus(status, type, notApplicable = false) {
    if (notApplicable) {
      return '<span class="artifact-badge-na" title="Los comprobantes de serie E no tienen CDR">N/A</span>';
    }
    if (!status) {
      return '<span class="artifact-badge-none">—</span>';
    }
    if (!status.exito) {
      const error = status.error || `No se pudo obtener ${type} de SUNAT`;
      return `<span class="artifact-badge-err" title="${escapeHtml(error)}">—</span>`;
    }
    const path = status.ruta_local ? encodeURIComponent(status.ruta_local) : '';
    const title = status.nom_archivo ? `${type}: ${status.nom_archivo}` : `${type} obtenido con éxito`;
    if (!path) {
      return `<span class="artifact-badge-ok" title="${escapeHtml(title)}">✓</span>`;
    }
    return `<button type="button" class="artifact-badge-ok" data-view-path="${path}" data-view-type="${type}" aria-label="Visualizar ${type}" title="${escapeHtml(title)}">✓</button>`;
  }

  function renderProposalDescription(status) {
    if (!status) {
      return '<span class="description-empty">—</span>';
    }
    if (!status.exito) {
      const error = status.error || 'SUNAT no devolvió una descripción';
      return `<span class="description-error" title="${escapeHtml(error)}">Sin información</span>`;
    }
    const description = String(status.descripcion || '').trim();
    const plate = String(status.placa || '').trim();
    const plateText = plate ? `<span class="description-plate">Placa: ${escapeHtml(plate)}</span>` : '';
    return `<span class="description-text">${escapeHtml(description || 'Sin descripción')}</span>${plateText}`;
  }

  // =========================================================================
  // DESCARGAS MASIVAS DESDE PROPUESTAS SIRE
  // =========================================================================

  function proposalDownloadLabel(type) {
    return type === 'DESC' ? 'descripciones' : type;
  }

  async function confirmSunatUnavailable() {
    return showSweetAlert({
      title: 'SUNAT no está respondiendo',
      text: 'SUNAT no está respondiendo en este momento.\n\nSe intentó conectar con el servicio de SUNAT para descargar comprobantes y está fuera de servicio.\n\nNo es una falla del aplicativo ni de su computadora: son intermitencias del servicio de SUNAT. Lo recomendable es esperar unos minutos y volver a intentarlo.\n\nLos comprobantes que ya descargó no se pierden; al reintentar solo se bajan los que faltan.\n\n¿Desea intentar la descarga de todas formas?',
      type: 'warning',
      confirmButtonText: 'Sí',
      showCancelButton: true,
      cancelButtonText: 'No'
    });
  }

  function showProposalProgressModal(type, onCancel) {
    const existing = document.getElementById('xmlProgressOverlay');
    if (existing) existing.remove();

    const overlay = document.createElement('div');
    overlay.id = 'xmlProgressOverlay';
    overlay.className = 'notion-swal-overlay';

    overlay.innerHTML = `
      <div class="notion-swal-card" style="max-width: 440px; padding: 26px 24px; text-align: center;">
        <div class="notion-swal-icon info" style="margin-bottom: 12px;">
          <span class="spinner" style="width: 26px; height: 26px; border-width: 3px; display: inline-block;"></span>
        </div>
        <h2 class="notion-swal-title" style="margin-bottom: 4px;">Obteniendo ${escapeHtml(proposalDownloadLabel(type))}</h2>
        <div class="notion-swal-text" id="xmlProgressMsg" style="margin-bottom: 16px; font-size: 0.82rem; color: var(--notion-text-muted);">
          Consultando servidores SUNAT en paralelo...
        </div>

        <div style="width: 100%; margin-bottom: 14px;">
          <div style="display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 6px;">
            <span style="font-size: 0.78rem; font-weight: 600; color: var(--notion-text-muted);" id="xmlProgressCount">0 / 0 procesados</span>
            <span style="font-size: 1.2rem; font-weight: 700; font-family: var(--font-mono); color: var(--primary);" id="xmlProgressPercent">0%</span>
          </div>
          <div style="height: 8px; width: 100%; background: var(--notion-border-strong); border-radius: 9999px; overflow: hidden;">
            <div id="xmlProgressBar" style="height: 100%; width: 0%; background: var(--primary); border-radius: 9999px; transition: width 0.2s ease;"></div>
          </div>
        </div>

        <div style="display: flex; flex-wrap: wrap; justify-content: center; gap: 10px 18px; margin-bottom: 20px; font-size: 0.82rem;">
          <span style="display: inline-flex; align-items: center; gap: 5px; color: var(--tag-green-text); font-weight: 600;">
            <span style="font-size: 1rem;">✓</span> <span id="xmlProgressSuccess">0</span> obtenidos
          </span>
          <span style="display: inline-flex; align-items: center; gap: 5px; color: var(--tag-yellow-text); font-weight: 600;">
            <span id="xmlProgressPending">0</span> pendientes
          </span>
          <span style="display: inline-flex; align-items: center; gap: 5px; color: var(--tag-red-text); font-weight: 600;">
            <span style="font-size: 1rem;">✕</span> <span id="xmlProgressError">0</span> fallidos definitivos
          </span>
        </div>

        <div style="display: flex; justify-content: center;">
          <button type="button" class="notion-swal-btn notion-swal-btn-cancel" id="btnCancelXmlDownload" style="font-size: 0.82rem; padding: 7px 18px;">
            Detener descarga
          </button>
        </div>
      </div>
    `;

    document.body.appendChild(overlay);
    requestAnimationFrame(() => overlay.classList.add('active'));

    const btnCancel = overlay.querySelector('#btnCancelXmlDownload');
    btnCancel?.addEventListener('click', async () => {
      btnCancel.disabled = true;
      btnCancel.textContent = 'Deteniendo descarga...';
      if (onCancel) {
        try {
          await onCancel();
        } catch (_) {}
      }
    });

    return {
      update(status) {
        const progressBar = document.getElementById('xmlProgressBar');
        const progressPercent = document.getElementById('xmlProgressPercent');
        const progressCount = document.getElementById('xmlProgressCount');
        const progressSuccess = document.getElementById('xmlProgressSuccess');
        const progressPending = document.getElementById('xmlProgressPending');
        const progressError = document.getElementById('xmlProgressError');
        const progressMsg = document.getElementById('xmlProgressMsg');

        const percent = Math.min(100, Math.max(0, Math.round(status.porcentaje || 0)));
        if (progressBar) progressBar.style.width = `${percent}%`;
        if (progressPercent) progressPercent.textContent = `${percent}%`;
        if (progressCount) progressCount.textContent = `${status.procesados || 0} / ${status.total_items || 0} procesados`;
        if (progressSuccess) progressSuccess.textContent = status.exitosos || 0;
        if (progressPending) progressPending.textContent = status.pendientes_reintento || 0;
        if (progressError) progressError.textContent = status.fallidos_definitivos || 0;
        if (progressMsg && status.mensaje) progressMsg.textContent = status.mensaje;
      },
      close() {
        overlay.classList.remove('active');
        setTimeout(() => overlay.remove(), 220);
      }
    };
  }

  async function startProposalDownload(book, type, continueOnSunatOutage = false) {
    const isRce = book === 'RCE';
    const allItems = isRce ? currentProposalItemsRce : currentProposalItemsRvie;
    const proposalContext = isRce ? currentProposalContextRce : currentProposalContextRvie;
    const proposalComprobantes = isRce ? rceComprobantes : rvieComprobantes;

    if (!allItems || !allItems.length) {
      notifyWarning('Sin Comprobantes', `Primero debes generar y visualizar la propuesta oficial de ${isRce ? 'Compras (RCE)' : 'Ventas (RVIE)'}.`);
      return;
    }

    const selectedPeriod = (isRce ? $('proposalPeriodRce')?.value : $('proposalPeriodRvie')?.value)?.replace('-', '') || '';
    if (!proposalContext || !activeCompany ||
        proposalContext.companyId !== activeCompany.id ||
        proposalContext.ruc !== activeCompany.ruc ||
        proposalContext.period !== selectedPeriod) {
      notifyWarning('Propuesta Desactualizada', 'La empresa o el período cambió. Vuelve a obtener la propuesta SIRE antes de continuar.');
      return;
    }

    // Filtrar si el usuario tiene una búsqueda activa en la tabla
    const searchInput = $(isRce ? 'txtSearchProposalRce' : 'txtSearchProposalRvie');
    const query = (searchInput?.value || '').toLowerCase().trim();
    const selectedEntries = allItems
      .map((item, index) => ({ item, index }))
      .filter(({ item }) => !query ||
        (item.comp_pago || '').toLowerCase().includes(query) ||
        (item.ruc || '').toLowerCase().includes(query) ||
        (item.razon_social || '').toLowerCase().includes(query) ||
        (item.fecha || '').toLowerCase().includes(query));
    const items = selectedEntries.map(({ item }) => item);

    if (!items.length) {
      notifyWarning('Sin Comprobantes', 'No hay comprobantes que coincidan con el filtro actual.');
      return;
    }

    const selectedComprobantes = selectedEntries
      .map(({ index }) => proposalComprobantes[index])
      .filter(Boolean);
    const invalidCount = selectedComprobantes.filter((comp) => !comp.ruc || !comp.tipo || !comp.serie || !comp.numero).length;
    const comprobantes = selectedComprobantes
      .filter((comp) => comp.ruc && comp.tipo && comp.serie && comp.numero)
      .map((comp, idx) => ({
        ...comp,
        id: comp.id || `${comp.ruc}-${comp.tipo}-${comp.serie}-${comp.numero}-${idx}`,
        periodo: proposalContext.period,
        empresa_ruc: proposalContext.ruc,
        empresa_razon_social: activeCompany.razon_social || '',
        row_index: comp.row_index ?? idx
      }));

    if (invalidCount > 0) {
      notifyWarning('Filas Omitidas', `${invalidCount} comprobante(s) no tienen RUC, tipo, serie o número completos y no serán consultados.`);
    }

    if (!comprobantes.length) {
      notifyWarning('Comprobantes no válidos', 'No se encontraron comprobantes con serie y número válidos para consultar.');
      return;
    }

    let progressModalRef = null;
    const progressModal = showProposalProgressModal(type, async () => {
      try {
        await apiFetch('/api/download/cancel', { method: 'POST' });
        setTimeout(async () => {
          try {
            const res = await apiFetch('/api/download/status');
            const data = await res.json();
            if (data?.status && progressModalRef?.handleUpdate) {
              progressModalRef.handleUpdate(data.status);
            }
          } catch (_) {}
        }, 300);
      } catch (_) {}
    });

    try {
      const response = await apiFetch('/api/download/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          comprobantes,
          tipos: [type],
          concurrency: type === 'DESC' ? 8 : 6,
          proposal_ticket: proposalContext.ticket,
          proposal_book: proposalContext.book,
          proposal_period: proposalContext.period,
          owner_ruc: proposalContext.ruc,
          document_timeout_seconds: 20,
          continue_on_sunat_outage: continueOnSunatOutage
        })
      });

      const data = await response.json();
      if (response.status === 503 && data?.code === 'SUNAT_UNAVAILABLE' && !continueOnSunatOutage) {
        progressModal.close();
        const proceed = await confirmSunatUnavailable();
        if (proceed) {
          return startProposalDownload(book, type, true);
        }
        return;
      }
      if (!response.ok || !data.success) {
        throw new Error(data.error || 'No se pudo iniciar el motor de descarga');
      }

      progressModalRef = progressModal;
      await trackDownloadProgress(progressModal, isRce, items, type);
    } catch (err) {
      progressModal.close();
      notifyError(`Error al obtener ${proposalDownloadLabel(type)}`, err.message);
    }
  }

  function trackDownloadProgress(progressModal, isRce, items, type) {
    return new Promise((resolve) => {
      let finished = false;
      let sse = null;
      let pollInterval = null;
      const statusMap = proposalStatusMap(isRce, type);

      const recordResults = (resultados) => {
        if (!resultados || !resultados.length) return;
        for (const res of resultados) {
          if (!res?.comprobante) continue;
          const key = getCompKey(res.comprobante);
          const val = {
            exito: res.exito,
            error: res.error || (res.exito ? '' : `No se pudo obtener ${proposalDownloadLabel(type)} de SUNAT`),
            nom_archivo: res.nom_archivo,
            ruta_local: res.ruta_local,
            descripcion: res.descripcion,
            placa: res.placa,
            estado_cdr: res.estado_cdr,
            codigo_cdr: res.codigo_cdr,
            mensaje_cdr: res.mensaje_cdr
          };
          statusMap.set(key, val);
          const tipo = String(res.comprobante.tipo || '').padStart(2, '0').trim();
          const serie = String(res.comprobante.serie || '').toUpperCase().trim();
          const numero = normalizeNumero(res.comprobante.numero);
          const fallbackKey = `${tipo}-${serie}-${numero}`;
          statusMap.set(fallbackKey, val);
          if (type === 'DESC' && res.xml_ruta) {
            const xmlValue = {
              exito: true,
              error: '',
              nom_archivo: res.xml_nombre || 'XML',
              ruta_local: res.xml_ruta
            };
            const xmlMap = proposalStatusMap(isRce, 'XML');
            xmlMap.set(key, xmlValue);
            xmlMap.set(fallbackKey, xmlValue);
          }
        }
      };

      const cleanup = (finalStatus) => {
        if (finished) return;
        finished = true;
        if (sse) { sse.close(); sse = null; }
        if (pollInterval) { clearInterval(pollInterval); pollInterval = null; }
        progressModal.close();

        if (finalStatus?.resultados) {
          recordResults(finalStatus.resultados);
        }

        // Refrescar la tabla para mostrar checks, errores y descripciones.
        renderProposalTableGrid(isRce, items);

	        const exitosos = finalStatus?.exitosos ?? 0;
	        const errores = finalStatus?.errores ?? 0;
	        const resumen = normalizedDownloadSummary(finalStatus);
	        const descargados = resumen.descargado || 0;
	        const existentes = resumen.ya_existente || 0;
	        const noDisponibles = resumen.no_disponible_sunat || 0;
	        const noAdmitidos = resumen.consulta_no_admitida || 0;
	        const recuperables = resumen.fallo_tecnico_recuperable || 0;
	        const detalleResumen = [
	          `• Descargados ahora: ${descargados}`,
	          `• Ya existentes: ${existentes}`,
	          `• No disponibles en SUNAT: ${noDisponibles}`,
	          `• Consultas no admitidas: ${noAdmitidos}`,
	          `• Fallos técnicos recuperables: ${recuperables}`
	        ].join('\n');
	        const closedByBudget = /presupuesto|deadline|límite de tiempo/i.test(finalStatus?.mensaje || '');

        if (finalStatus?.estado === 'detenido') {
          notifyWarning(
	            'Descarga Detenida',
	            `La descarga fue detenida por el usuario.\n\n${detalleResumen}\n\nProcesados: ${exitosos + errores}`
          );
	        } else if (errores > 0) {
	          notifyWarning(
            closedByBudget ? 'Consulta parcial por tiempo' : (exitosos > 0 ? 'Proceso parcial' : `No se pudo obtener ${proposalDownloadLabel(type)}`),
            `Se procesó el lote oficial de SUNAT.\n\n${detalleResumen}\n\nTotal: ${exitosos + errores}${closedByBudget ? '\n\nSUNAT no respondió dentro del presupuesto del lote. Puedes repetir la operación; los archivos existentes se reutilizarán.' : ''}`
	          );
	        } else {
	          notifySuccess(
            `${type === 'DESC' ? 'Consulta' : 'Descarga'} de ${proposalDownloadLabel(type)} finalizada`,
	            `Se procesó el lote oficial de SUNAT.\n\n${detalleResumen}\n\nTotal: ${exitosos + errores}`
          );
        }

        resolve(finalStatus);
      };

      const handleStatusUpdate = (status) => {
        if (!status) return;
        progressModal.update(status);
        if (status.resultados) {
          recordResults(status.resultados);
        }
        if (['completado', 'detenido', 'error'].includes(status.estado)) {
          cleanup(status);
        }
      };

      progressModal.handleUpdate = handleStatusUpdate;

      // Polling activo en paralelo cada 1.2s para evitar bloqueos si SSE se retrasa
      pollInterval = setInterval(async () => {
        if (finished) return;
        try {
          const res = await apiFetch('/api/download/status');
          const data = await res.json();
          if (data?.status) handleStatusUpdate(data.status);
        } catch (_) {}
      }, 1200);

      try {
        sse = new EventSource('/api/download/events');
        sse.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);
            if (data?.status) handleStatusUpdate(data.status);
          } catch (_) {}
        };
        sse.onerror = () => {
          if (sse) { sse.close(); sse = null; }
        };
      } catch (_) {}
    });
  }

  function normalizedDownloadSummary(status) {
    const keys = [
      'descargado',
      'ya_existente',
      'no_disponible_sunat',
      'consulta_no_admitida',
      'fallo_tecnico_recuperable'
    ];
    const backend = status?.resumen_categorias || {};
    const backendTotal = keys.reduce((total, key) => total + Number(backend[key] || 0), 0);
    if (backendTotal > 0 || !(status?.resultados || []).length) {
      return backend;
    }

    const summary = Object.fromEntries(keys.map((key) => [key, 0]));
    for (const result of status.resultados) {
      let category = result?.categoria;
      if (!keys.includes(category)) {
        if (result?.exito) {
          category = String(result.origen || '').toLowerCase() === 'archivo existente'
            ? 'ya_existente'
            : 'descargado';
        } else {
          const error = String(result?.error || '').toLowerCase();
          if (/"coderror":"301"|no se encontr[oó] el xml|http 404/.test(error)) {
            category = 'no_disponible_sunat';
          } else if (/"coderror":"302"|consulta inv[aá]lida|http 400|http 403/.test(error)) {
            category = 'consulta_no_admitida';
          } else {
            category = 'fallo_tecnico_recuperable';
          }
        }
      }
      summary[category] += 1;
    }
    return summary;
  }

  // =========================================================================
  // EXCEL & DESCARGAS MASIVAS
  // =========================================================================

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
      notifyError('Error al Procesar Excel', error.message);
    } finally {
      dropzone.style.opacity = '1';
    }
  }

  function updateDownloadStartButton() {
    const canDownload = isLicenseActive && loadedComprobantes.length > 0 && !isDownloading;
    legacyDownloadXml.disabled = !canDownload;
    legacyDownloadPdf.disabled = !canDownload;
    legacyDownloadCdr.disabled = !canDownload;
  }

  async function startDownload(tipo, continueOnSunatOutage = false) {
    if (!loadedComprobantes.length) {
      notifyWarning('Sin Comprobantes', 'Carga un Excel o genera una propuesta SIRE antes de descargar.');
      return;
    }

    const comprobantes = loadedComprobantes.filter((comp) =>
      comp?.ruc && comp?.tipo && comp?.serie && comp?.numero &&
      !(tipo === 'CDR' && String(comp.serie).toUpperCase().startsWith('E'))
    );
    if (!comprobantes.length) {
      notifyWarning('Comprobantes no Admitidos', `La lista no contiene comprobantes que admitan descarga de ${tipo}.`);
      return;
    }

    const concurrency = Number($('concurrencyRange').value) || 6;
    resetProgress(comprobantes.length);
    try {
      const response = await apiFetch('/api/download/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          comprobantes,
          tipos: [tipo],
          concurrency,
          document_timeout_seconds: 20,
          continue_on_sunat_outage: continueOnSunatOutage
        })
      });
      const data = await response.json();
      if (response.status === 503 && data?.code === 'SUNAT_UNAVAILABLE' && !continueOnSunatOutage) {
        finishProgress();
        progressSection.style.display = 'none';
        const proceed = await confirmSunatUnavailable();
        if (proceed) {
          return startDownload(tipo, true);
        }
        return;
      }
      if (!response.ok || !data.success) throw new Error(data.error || 'No se pudo iniciar');
      isDownloading = true;
      startLiveProgress();
    } catch (error) {
      notifyError('Error al Iniciar Descarga', error.message);
      finishProgress();
    }
  }

  function resetProgress(total) {
    legacyDownloadXml.disabled = true;
    legacyDownloadPdf.disabled = true;
    legacyDownloadCdr.disabled = true;
    legacyCancelDownload.style.display = 'inline-flex';
    progressSection.style.display = 'block';
    progressSection.scrollIntoView({ behavior: 'smooth' });
    $('progressBar').style.width = '0%';
    $('progressPercent').textContent = '0%';
    $('progressMessage').textContent = 'Iniciando descarga…';
    $('statTotal').textContent = total;
    $('statProcessed').textContent = '0';
    $('statSuccess').textContent = '0';
    if ($('statPending')) $('statPending').textContent = '0';
    $('statError').textContent = '0';
    $('resultsTbody').innerHTML = '';
    $('terminalLogs').innerHTML = '';
  }

  async function cancelDownload() {
    const ok = await notifyConfirm('¿Cancelar Descarga?', '¿Desea detener el procesamiento de comprobantes en curso?', 'Sí, cancelar', true);
    if (!ok) return;
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
    legacyCancelDownload.style.display = 'none';
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
    if ($('statPending')) $('statPending').textContent = status.pendientes_reintento || 0;
    $('statError').textContent = status.fallidos_definitivos || 0;
    if ($('statSpeed')) $('statSpeed').textContent = `${(status.velocidad_items_seg || 0).toFixed(1)} docs/s`;
    if ($('statEta')) $('statEta').textContent = status.tiempo_restante || '--';
    if (status.resultados) renderResults(status.resultados);
    if (logs) renderLogs(logs);
  }

  function renderResults(items) {
    items.forEach((item) => {
      if (item.exito && item.ruta_local) downloadedFiles.set(fileKey(item.comprobante, item.tipo), item.ruta_local);
    });
    $('resultsCount').textContent = items.length;
    $('resultsTbody').innerHTML = items.slice(-300).reverse().map((item) => {
      const comp = item.comprobante || {};
      const file = item.exito && item.ruta_local
        ? `<button type="button" class="mini-action" data-view-path="${encodeURIComponent(item.ruta_local)}" data-view-type="${escapeHtml(item.tipo || '')}">Ver</button>`
        : '<span style="color:var(--notion-text-subtle);">—</span>';
      return `<tr>
        <td><span class="notion-pill ${item.exito ? 'badge-ok' : 'badge-error'}">${item.exito ? '✓ OK' : '✕ ERROR'}</span></td>
        <td>${renderFormatBadge(item.tipo)}</td>
        <td><code>${escapeHtml(comp.ruc || '')}</code></td>
        <td>${renderDocTypeBadge(comp.tipo)}</td>
        <td><strong>${escapeHtml(comp.serie || '')}-${escapeHtml(comp.numero || '')}</strong></td>
        <td>${escapeHtml(item.estado_cdr || '—')}</td>
        <td><code style="font-size:0.75rem;">${escapeHtml((item.digest_value || '').slice(0, 10) || '—')}</code></td>
        <td>${file}</td>
        <td style="max-width:200px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">${escapeHtml(item.error || item.origen || 'Descargado')}</td>
      </tr>`;
    }).join('');
  }

  function renderFormatBadge(format) {
    const f = String(format || '').toUpperCase();
    if (f === 'XML') return '<span class="notion-pill" style="background:var(--tag-blue-bg); color:var(--tag-blue-text);">XML</span>';
    if (f === 'PDF') return '<span class="notion-pill" style="background:var(--tag-red-bg); color:var(--tag-red-text);">PDF</span>';
    if (f === 'CDR') return '<span class="notion-pill" style="background:var(--tag-green-bg); color:var(--tag-green-text);">CDR</span>';
    return `<span class="notion-pill">${escapeHtml(format || '—')}</span>`;
  }

  function renderDocTypeBadge(tipo) {
    const t = String(tipo || '').trim();
    if (t === '01' || t.toLowerCase().includes('factura')) {
      return '<span class="notion-pill" style="background:var(--tag-green-bg); color:var(--tag-green-text);">01 Factura</span>';
    }
    if (t === '03' || t.toLowerCase().includes('boleta')) {
      return '<span class="notion-pill" style="background:var(--tag-purple-bg); color:var(--tag-purple-text);">03 Boleta</span>';
    }
    if (t === '07' || t.toLowerCase().includes('crédito') || t.toLowerCase().includes('credito')) {
      return '<span class="notion-pill" style="background:var(--tag-red-bg); color:var(--tag-red-text);">07 NC</span>';
    }
    if (t === '08' || t.toLowerCase().includes('débito') || t.toLowerCase().includes('debito')) {
      return '<span class="notion-pill" style="background:var(--tag-orange-bg); color:var(--tag-orange-text);">08 ND</span>';
    }
    return `<span class="notion-pill" style="background:var(--tag-gray-bg); color:var(--tag-gray-text);">${escapeHtml(tipo || 'CPE')}</span>`;
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
      ${['PDF', 'XML', 'CDR'].map((tipo) => renderArtifactActions(comp, tipo)).join('')}
    </tr>`).join('');
  }

  function renderArtifactActions(comp, tipo) {
    const path = downloadedFiles.get(fileKey(comp, tipo));
    if (path) {
      const encoded = encodeURIComponent(path);
      return `<button type="button" class="mini-action" data-view-path="${encoded}" data-view-type="${tipo}">Ver ${tipo}</button>`;
    }
    return '<span style="color:#aaa;">—</span>';
  }

  function handleRecordAction(event) {
    const button = event.target.closest('[data-view-path]');
    if (!button) return;
    const type = String(button.dataset.viewType || '').toUpperCase();
    if (type === 'XML' || type === 'CDR') {
      openXMLViewer(button.dataset.viewPath, type);
      return;
    }
    resetFileViewer();
    $('fileViewerTitle').textContent = `Visor ${type || 'de archivo'}`;
    $('fileViewerFrame').src = `/api/files/view?path=${button.dataset.viewPath}`;
    $('fileViewerFrame').hidden = false;
    $('fileViewer').showModal();
  }

  function closeFileViewer() {
    $('fileViewer').close();
    resetFileViewer();
  }

  function resetFileViewer() {
    currentViewerPath = '';
    currentViewerRawXML = '';
    $('fileViewerFrame').src = 'about:blank';
    $('fileViewerFrame').hidden = true;
    $('invoiceViewer').hidden = true;
    $('invoiceViewer').innerHTML = '';
    $('xmlSourceViewer').hidden = true;
    $('xmlSourceViewer').textContent = '';
    $('fileViewerLoading').hidden = true;
    $('fileViewerTabs').hidden = true;
    $('fileViewerSummaryTab').classList.add('active');
    $('fileViewerRawTab').classList.remove('active');
    $('fileViewerSummaryTab').textContent = 'Comprobante';
  }

  async function openXMLViewer(encodedPath, type = 'XML') {
    resetFileViewer();
    currentViewerPath = encodedPath;
    const isCDR = type === 'CDR';
    $('fileViewerTitle').textContent = isCDR ? 'Visor CDR' : 'Comprobante electrónico';
    $('fileViewerSummaryTab').textContent = isCDR ? 'Constancia' : 'Comprobante';
    $('fileViewerTabs').hidden = false;
    $('fileViewerLoading').hidden = false;
    $('fileViewer').showModal();

    try {
      const response = await apiFetch(`/api/files/xml-preview?path=${encodedPath}`);
      const data = await response.json();
      if (!response.ok || !data.success) {
        throw new Error(data.error || 'No se pudo interpretar el XML');
      }
      if (currentViewerPath !== encodedPath || !$('fileViewer').open) return;
      const preview = data.preview || {};
      $('fileViewerTitle').textContent = preview.kind === 'cdr'
        ? `Visor CDR · ${preview.reference || preview.number || ''}`
        : `${preview.document_type || 'Comprobante electrónico'} · ${preview.number || ''}`;
      currentViewerRawXML = formatXMLSource(data.raw_xml || '');
      $('xmlSourceViewer').textContent = currentViewerRawXML;
      $('invoiceViewer').innerHTML = preview.kind === 'cdr'
        ? renderCDRPreview(preview)
        : (preview.kind === 'withholding' ? renderWithholdingPreview(preview) : renderInvoicePreview(preview));
      $('invoiceViewer').hidden = false;
    } catch (error) {
      if (currentViewerPath !== encodedPath || !$('fileViewer').open) return;
      $('invoiceViewer').innerHTML = `
        <div class="invoice-viewer-error">
          <strong>No se pudo representar el comprobante</strong>
          <span>${escapeHtml(error.message)}</span>
        </div>`;
      $('invoiceViewer').hidden = false;
    } finally {
      if (currentViewerPath === encodedPath) $('fileViewerLoading').hidden = true;
    }
  }

  function renderCDRPreview(cdr) {
    const sender = cdr.supplier || {};
    const receiver = cdr.customer || {};
    const status = String(cdr.response_status || 'SIN ESTADO').toUpperCase();
    const statusClass = status.startsWith('ACEPTADO')
      ? 'accepted'
      : (status === 'RECHAZADO' ? 'rejected' : (status === 'OBSERVADO' ? 'observed' : 'processed'));
    const notes = (cdr.notes || []).filter(Boolean);

    return `
      <article class="commercial-invoice cdr-preview">
        <header class="invoice-header">
          <div class="invoice-company">
            <span class="invoice-eyebrow">CONSTANCIA DE RECEPCIÓN</span>
            <h2>SUNAT</h2>
            <p>Resultado oficial del procesamiento del comprobante electrónico</p>
          </div>
          <div class="invoice-document-box">
            <span>DOCUMENTO RELACIONADO</span>
            <strong>${escapeHtml(cdr.reference || '—')}</strong>
            <small>Tipo ${escapeHtml(cdr.document_type_code || '—')}</small>
          </div>
        </header>

        <section class="cdr-status ${statusClass}">
          <div>
            <span>Estado SUNAT</span>
            <strong>${escapeHtml(status)}</strong>
          </div>
          <div>
            <span>Código de respuesta</span>
            <strong>${escapeHtml(cdr.response_code || '—')}</strong>
          </div>
          <p>${escapeHtml(cdr.response_message || cdr.reason || 'SUNAT no incluyó un mensaje descriptivo.')}</p>
        </section>

        <section class="invoice-meta-grid">
          ${invoiceMeta('Fecha de recepción', formatInvoiceDate(cdr.issue_date) || '—')}
          ${invoiceMeta('Hora de recepción', cdr.issue_time || '—')}
          ${invoiceMeta('Identificador CDR', cdr.number || '—')}
          ${invoiceMeta('Tipo de comprobante', cdr.document_type_code || '—')}
        </section>

        <section class="cdr-parties">
          ${cdrParty('Remitente de la respuesta', sender)}
          ${cdrParty('Destinatario', receiver)}
        </section>

        <section class="cdr-notes">
          <span class="invoice-eyebrow">INFORMACIÓN ADICIONAL</span>
          ${notes.length ? notes.map((note) => `<p>${escapeHtml(note)}</p>`).join('') : '<p>El CDR no contiene observaciones adicionales.</p>'}
        </section>
      </article>`;
  }

  function cdrParty(label, party) {
    return `<div class="invoice-party-card">
      <span class="invoice-eyebrow">${escapeHtml(label.toUpperCase())}</span>
      <strong>${escapeHtml(party.name || 'No informado')}</strong>
      <span>RUC ${escapeHtml(party.ruc || '—')}</span>
      ${party.address ? `<small>${escapeHtml(party.address)}</small>` : ''}
    </div>`;
  }

  async function showXMLViewerTab(tab) {
    const raw = tab === 'raw';
    $('fileViewerSummaryTab').classList.toggle('active', !raw);
    $('fileViewerRawTab').classList.toggle('active', raw);
    $('invoiceViewer').hidden = raw;
    $('xmlSourceViewer').hidden = !raw;
    if (!raw || currentViewerRawXML || !currentViewerPath) return;

    $('fileViewerLoading').hidden = false;
    try {
      const response = await apiFetch(`/api/files/view?path=${currentViewerPath}`);
      if (!response.ok) throw new Error('No se pudo leer el XML original');
      currentViewerRawXML = formatXMLSource(await response.text());
      $('xmlSourceViewer').textContent = currentViewerRawXML;
    } catch (error) {
      $('xmlSourceViewer').textContent = error.message;
    } finally {
      $('fileViewerLoading').hidden = true;
    }
  }

  function renderInvoicePreview(invoice) {
    const supplier = invoice.supplier || {};
    const customer = invoice.customer || {};
    const totals = invoice.totals || {};
    const financial = invoice.financial || null;
    const lines = invoice.lines || [];
    const taxes = invoice.taxes || [];
    const warnings = invoice.warnings || [];
    const currency = invoice.currency || 'PEN';
    const notes = (invoice.notes || []).filter(Boolean);
    const referenceDetails = [
      invoice.reference_type ? `Tipo ${escapeHtml(invoice.reference_type)}` : '',
      invoice.reference_date ? formatInvoiceDate(invoice.reference_date) : '',
      invoice.reason_code ? `Motivo ${escapeHtml(invoice.reason_code)}` : '',
      invoice.reason ? escapeHtml(invoice.reason) : ''
    ].filter(Boolean).join(' · ');
    const reference = invoice.reference
      ? `<div class="invoice-reference"><strong>Documento relacionado:</strong> ${escapeHtml(invoice.reference)}${referenceDetails ? ` · ${referenceDetails}` : ''}</div>`
      : '';
    const reconciliation = financial && financial.reconciles
      ? `<div class="invoice-reference"><strong>Conciliación:</strong> ${formatInvoiceAmount(financial.gross_settlement, currency)} − ${formatInvoiceAmount(financial.registry_total, currency)} = ${formatInvoiceAmount(financial.net_settlement, currency)}</div>`
      : '';

    return `
      <article class="commercial-invoice">
        <header class="invoice-header">
          <div class="invoice-company">
            <span class="invoice-eyebrow">EMISOR</span>
            <h2>${escapeHtml(supplier.name || 'Emisor no informado')}</h2>
            <p>RUC ${escapeHtml(supplier.ruc || '—')}</p>
            ${supplier.address ? `<small>${escapeHtml(supplier.address)}</small>` : ''}
          </div>
          <div class="invoice-document-box">
            <span>${escapeHtml((invoice.document_type || 'Comprobante electrónico').toUpperCase())}</span>
            <strong>${escapeHtml(invoice.number || '—')}</strong>
            <small>Código SUNAT ${escapeHtml(invoice.document_type_code || '—')}</small>
          </div>
        </header>

        <section class="invoice-meta-grid">
          ${invoiceMeta('Fecha de emisión', formatInvoiceDate(invoice.issue_date))}
          ${invoiceMeta('Fecha de vencimiento', formatInvoiceDate(invoice.due_date) || '—')}
          ${invoiceMeta('Moneda', currencyLabel(currency))}
          ${invoiceMeta(invoice.family === 'note' ? 'Efecto del documento' : 'Tipo de operación', invoice.family === 'note' ? (invoice.document_type_code === '07' ? 'Disminuye el documento relacionado' : 'Aumenta el documento relacionado') : (invoice.operation_type || '—'))}
        </section>

        <section class="invoice-party-card">
          <span class="invoice-eyebrow">CLIENTE / ADQUIRIENTE</span>
          <strong>${escapeHtml(customer.name || 'No informado')}</strong>
          <span>RUC ${escapeHtml(customer.ruc || '—')}</span>
          ${customer.address ? `<small>${escapeHtml(customer.address)}</small>` : ''}
        </section>

        ${reference}
        ${reconciliation}
        ${renderPaymentSummary(invoice.payment, currency)}
        ${warnings.length ? `<section class="cdr-notes"><span class="invoice-eyebrow">ADVERTENCIAS DEL XML</span>${warnings.map((warning) => `<p>${escapeHtml(warning)}</p>`).join('')}</section>` : ''}

        <section class="invoice-lines-wrap">
          <table class="invoice-lines">
            <thead>${financial
              ? '<tr><th>#</th><th>Concepto</th><th>Valor inicial</th><th>Cargos / ajustes</th><th>IGV</th><th>Total componente</th></tr>'
              : '<tr><th>#</th><th>Descripción</th><th>Cantidad</th><th>Precio unit.</th><th>IGV</th><th>Importe</th></tr>'}</thead>
            <tbody>
              ${lines.length ? lines.map((line, index) => `
                <tr>
                  <td>${escapeHtml(line.number || String(index + 1))}</td>
                  <td><strong>${escapeHtml(line.description || 'Sin descripción')}</strong>${line.code ? `<small>Código: ${escapeHtml(line.code)}</small>` : ''}${renderItemProperties(line.properties)}</td>
                  <td class="invoice-number">${financial ? formatInvoiceAmount(line.amount, currency) : `${escapeHtml(line.quantity || '—')} ${escapeHtml(line.unit_code || '')}`}</td>
                  <td class="invoice-number">${formatInvoiceAmount(financial ? line.adjustment_amount : line.unit_price, currency)}</td>
                  <td class="invoice-number">${formatInvoiceAmount(line.tax_amount, currency)}</td>
                  <td class="invoice-number"><strong>${formatInvoiceAmount(financial ? line.unit_price : line.amount, currency)}</strong></td>
                </tr>`).join('') : '<tr><td colspan="6" class="invoice-empty">El XML no contiene líneas de detalle.</td></tr>'}
            </tbody>
          </table>
        </section>

        <footer class="invoice-footer">
          <div class="invoice-notes">
            <span class="invoice-eyebrow">OBSERVACIONES</span>
            ${notes.length ? notes.map((note) => `<p>${escapeHtml(note)}</p>`).join('') : '<p>Sin observaciones.</p>'}
          </div>
          <dl class="invoice-totals">
            ${financial ? `
              ${invoiceTotal('Base registrable', financial.registry_base, currency)}
              ${invoiceTotal('IGV', financial.tax, currency)}
              ${invoiceTotal('Comisiones, cargos e impuestos', financial.registry_total, currency)}
              ${invoiceTotal('Liquidación bruta', financial.gross_settlement, currency)}
              <div class="invoice-total-payable"><dt>Neto liquidado</dt><dd>${formatInvoiceAmount(financial.net_settlement, currency)}</dd></div>
            ` : `
              ${invoiceTotal('Valor de venta', totals.tax_exclusive || totals.line_extension, currency)}
              ${renderTaxTotals(taxes, totals.tax, currency)}
              ${invoiceTotal('Precio de venta con impuestos', totals.tax_inclusive, currency, true)}
              ${invoiceTotal('Descuentos', totals.allowance, currency, true)}
              ${invoiceTotal('Otros cargos', totals.charge, currency, true)}
              ${invoiceTotal('Anticipos', totals.prepaid, currency, true)}
              ${invoiceTotal('Redondeo', totals.rounding, currency, true)}
              <div class="invoice-total-payable"><dt>Importe total</dt><dd>${formatInvoiceAmount(totals.payable || totals.tax_inclusive, currency)}</dd></div>
            `}
          </dl>
        </footer>
      </article>`;
  }

  function renderTaxTotals(taxes, fallbackTax, currency) {
    if (!taxes || !taxes.length) return invoiceTotal('IGV / tributos', fallbackTax, currency);
    return taxes.map((tax) => {
      const taxAmount = Number(tax.tax_amount || 0);
      const taxableAmount = Number(tax.taxable_amount || 0);
      if (Math.abs(taxAmount) < 0.000001 && Math.abs(taxableAmount) < 0.000001) return '';
      const base = Math.abs(taxableAmount) > 0.000001
        ? invoiceTotal(`${tax.name} · base`, tax.taxable_amount, currency)
        : '';
      const amount = Math.abs(taxAmount) > 0.000001
        ? invoiceTotal(tax.name, tax.tax_amount, currency)
        : '';
      return `${base}${amount}`;
    }).join('');
  }

  function renderItemProperties(properties) {
    return (properties || []).map((property) => {
      const label = property.name || property.code || 'Dato adicional';
      return `<small>${escapeHtml(label)}: ${escapeHtml(property.value || '—')}</small>`;
    }).join('');
  }

  function renderPaymentSummary(payment, currency) {
    if (!payment) return '';
    const installments = payment.installments || [];
    return `<section class="invoice-reference">
      <strong>Condición de pago:</strong> ${escapeHtml(payment.mode || 'No indicada')}
      ${payment.outstanding_amount ? ` · Saldo: ${formatInvoiceAmount(payment.outstanding_amount, currency)}` : ''}
      ${installments.map((installment) => ` · ${escapeHtml(installment.number || 'Cuota')}: ${formatInvoiceAmount(installment.amount, currency)}${installment.due_date ? ` (${formatInvoiceDate(installment.due_date)})` : ''}`).join('')}
      ${payment.detraction_code ? `<br><strong>Detracción:</strong> código ${escapeHtml(payment.detraction_code)}${payment.detraction_percent ? ` · ${escapeHtml(payment.detraction_percent)}%` : ''}${payment.detraction_amount ? ` · ${formatInvoiceAmount(payment.detraction_amount, currency)}` : ''}${payment.detraction_account ? ` · Cuenta ${escapeHtml(payment.detraction_account)}` : ''}` : ''}
    </section>`;
  }

  function renderWithholdingPreview(document) {
    const agent = document.supplier || {};
    const receiver = document.customer || {};
    const related = document.related_documents || [];
    const currency = document.currency || 'PEN';
    const adjustmentLabel = document.document_type_code === '20' ? 'Retención' : 'Percepción';
    const netLabel = document.document_type_code === '20' ? 'Neto pagado' : 'Neto cobrado';
    return `<article class="commercial-invoice">
      <header class="invoice-header">
        <div class="invoice-company">
          <span class="invoice-eyebrow">AGENTE</span>
          <h2>${escapeHtml(agent.name || 'Agente no informado')}</h2>
          <p>RUC ${escapeHtml(agent.ruc || '—')}</p>
        </div>
        <div class="invoice-document-box">
          <span>${escapeHtml((document.document_type || '').toUpperCase())}</span>
          <strong>${escapeHtml(document.number || '—')}</strong>
          <small>Código SUNAT ${escapeHtml(document.document_type_code || '—')}</small>
        </div>
      </header>
      <section class="invoice-meta-grid">
        ${invoiceMeta('Fecha de emisión', formatInvoiceDate(document.issue_date))}
        ${invoiceMeta('Moneda', currencyLabel(currency))}
        ${invoiceMeta('Documentos relacionados', String(related.length))}
        ${invoiceMeta('Tipo', adjustmentLabel)}
      </section>
      <section class="invoice-party-card">
        <span class="invoice-eyebrow">RECEPTOR</span>
        <strong>${escapeHtml(receiver.name || 'No informado')}</strong>
        <span>RUC ${escapeHtml(receiver.ruc || '—')}</span>
      </section>
      <section class="invoice-lines-wrap">
        <table class="invoice-lines">
          <thead><tr><th>Documento</th><th>Tipo</th><th>Emisión</th><th>Importe</th><th>Pago</th><th>Tasa</th><th>${adjustmentLabel}</th><th>${netLabel}</th></tr></thead>
          <tbody>${related.length ? related.map((item) => `<tr>
            <td><strong>${escapeHtml(item.number || '—')}</strong>${item.payment_id ? `<small>Pago: ${escapeHtml(item.payment_id)}</small>` : ''}</td>
            <td>${escapeHtml(item.document_type_code || '—')}</td>
            <td>${escapeHtml(formatInvoiceDate(item.issue_date) || '—')}</td>
            <td class="invoice-number">${formatInvoiceAmount(item.invoice_amount, item.currency || currency)}</td>
            <td class="invoice-number">${formatInvoiceAmount(item.paid_amount, item.currency || currency)}</td>
            <td class="invoice-number">${escapeHtml(item.rate || '—')}%</td>
            <td class="invoice-number">${formatInvoiceAmount(item.adjustment_amount, item.currency || currency)}</td>
            <td class="invoice-number"><strong>${formatInvoiceAmount(item.net_amount, item.currency || currency)}</strong></td>
          </tr>`).join('') : '<tr><td colspan="8" class="invoice-empty">El XML no contiene documentos relacionados.</td></tr>'}</tbody>
        </table>
      </section>
      <footer class="invoice-footer">
        <div class="invoice-notes"><span class="invoice-eyebrow">RESUMEN</span><p>Importes declarados por el agente.</p></div>
        <dl class="invoice-totals">
          ${invoiceTotal(document.document_type_code === '20' ? 'Total pagado' : 'Total cobrado', document.totals?.line_extension, currency)}
          <div class="invoice-total-payable"><dt>Total ${adjustmentLabel.toLowerCase()}</dt><dd>${formatInvoiceAmount(document.totals?.tax, currency)}</dd></div>
        </dl>
      </footer>
    </article>`;
  }

  function invoiceMeta(label, value) {
    return `<div><span>${escapeHtml(label)}</span><strong>${escapeHtml(value || '—')}</strong></div>`;
  }

  function invoiceTotal(label, value, currency, optional = false) {
    if (optional && !value) return '';
    return `<div><dt>${escapeHtml(label)}</dt><dd>${formatInvoiceAmount(value, currency)}</dd></div>`;
  }

  function formatInvoiceAmount(value, currency) {
    if (value === undefined || value === null || value === '') return '—';
    const number = Number(String(value).replace(/,/g, ''));
    if (!Number.isFinite(number)) return `${escapeHtml(currency || '')} ${escapeHtml(String(value))}`.trim();
    try {
      return new Intl.NumberFormat('es-PE', { style: 'currency', currency: currency || 'PEN' }).format(number);
    } catch {
      return `${escapeHtml(currency || '')} ${number.toFixed(2)}`.trim();
    }
  }

  function currencyLabel(currency) {
    const labels = { PEN: 'Soles (PEN)', USD: 'Dólares estadounidenses (USD)', EUR: 'Euros (EUR)' };
    return labels[currency] || currency || '—';
  }

  function formatInvoiceDate(value) {
    const match = String(value || '').match(/^(\d{4})-(\d{2})-(\d{2})/);
    return match ? `${match[3]}/${match[2]}/${match[1]}` : String(value || '');
  }

  function formatXMLSource(source) {
    const compact = String(source || '').replace(/>\s*</g, '><').trim();
    let depth = 0;
    return compact.replace(/(<[^>]+>)/g, '\n$1').split('\n').filter(Boolean).map((token) => {
      const closes = /^<\//.test(token);
      const selfClosing = /\/>$/.test(token) || /^<\?/.test(token) || /^<!/.test(token);
      if (closes) depth = Math.max(0, depth - 1);
      const line = `${'  '.repeat(depth)}${token}`;
      if (!closes && !selfClosing && /^<[^/][^>]*>$/.test(token) && !/<\/[^>]+>$/.test(token)) depth++;
      return line;
    }).join('\n').trim();
  }

  function fileKey(comp, tipo) {
    return `${comp?.id || `${comp?.ruc}-${comp?.tipo}-${comp?.serie}-${comp?.numero}`}|${tipo}`;
  }

  function renderLogs(logs) {
    $('terminalLogs').innerHTML = logs.map((line) => `<div class="terminal-line">${escapeHtml(line)}</div>`).join('');
    $('terminalLogs').scrollTop = $('terminalLogs').scrollHeight;
  }

  async function openFolder() {
    const response = await apiFetch('/api/files/open-folder', { method: 'POST' });
    if (!response.ok) alert('No se pudo abrir la carpeta de descargas.');
  }

  // =========================================================================
  // LICENCIA Y SESIÓN
  // =========================================================================

  async function checkSession() {
    try {
      const response = await apiFetch('/api/session');
      const data = await response.json();
      if (response.ok && data.authenticated) {
        showApp();
        await checkLicenseStatus();
        await loadCompanies(true);
        setDefaultProposalPeriod();
      } else {
        showLogin();
      }
    } catch (_) {
      showLogin();
    }
  }

  async function login(event) {
    event.preventDefault();
    $('btnLogin').disabled = true;
    $('loginSpinner').style.display = 'inline-block';
    try {
      const response = await apiFetch('/api/session/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: $('loginUsername').value.trim(),
          password: $('loginPassword').value,
          remember: $('loginRemember').checked
        })
      });
      const data = await response.json();
      if (!response.ok || !data.success) throw new Error(data.error || 'Credenciales inválidas');
      if ($('loginRemember').checked) {
        localStorage.setItem('autosire_user', $('loginUsername').value.trim());
      } else {
        localStorage.removeItem('autosire_user');
      }
      showApp();
      await checkLicenseStatus();
      await loadCompanies(true);
      setDefaultProposalPeriod();
    } catch (error) {
      notifyError('Acceso Denegado', error.message);
    } finally {
      $('btnLogin').disabled = false;
      $('loginSpinner').style.display = 'none';
    }
  }

  async function logout() {
    await apiFetch('/api/session/logout', { method: 'POST' });
    showLogin('Sesión cerrada correctamente');
  }

  async function checkLicenseStatus() {
    try {
      const response = await apiFetch('/api/license/status');
      const data = await response.json();
      if (response.ok && data.active) {
        isLicenseActive = true;
        $('licenseDot').classList.add('active');
        $('licenseLabel').textContent = 'Licencia Activa';
        $('licenseDetailsText').textContent = data.customer || 'AutoSire';
      } else {
        isLicenseActive = false;
        $('licenseDot').classList.remove('active');
        $('licenseLabel').textContent = 'Licencia Inactiva';
        $('licenseDetailsText').textContent = data.message || 'Sin licencia válida';
      }
    } catch (_) {
      isLicenseActive = false;
    }
  }

  async function checkLicenseHeartbeat() {
    if (appShell.style.display === 'none') return;
    await checkLicenseStatus();
  }

  function showLogin(msg) {
    appShell.style.display = 'none';
    loginScreen.style.display = 'flex';
  }

  function showApp() {
    loginScreen.style.display = 'none';
    appShell.style.display = 'flex';
  }

  function restoreRememberedLogin() {
    const saved = localStorage.getItem('autosire_user');
    if (saved && $('loginUsername')) $('loginUsername').value = saved;
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

  async function apiFetch(url, options = {}) {
    return fetch(url, options);
  }

  function toggleMobileDrawer() {
    const open = $('sidebar').classList.toggle('open');
    $('sidebarToggle').setAttribute('aria-expanded', String(open));
  }

  function initTheme() {
    const savedTheme = localStorage.getItem('autosire_theme') || 'light';
    if (savedTheme === 'dark') document.body.classList.add('dark');
  }

  function toggleTheme() {
    const isDark = document.body.classList.toggle('dark');
    localStorage.setItem('autosire_theme', isDark ? 'dark' : 'light');
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
    setInterval(() => {
      fetch('/api/heartbeat', { method: 'POST' }).catch(() => {});
    }, 3000);

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
