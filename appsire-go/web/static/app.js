document.addEventListener('DOMContentLoaded', () => {
  const $ = (id) => document.getElementById(id);

  function escapeHtml(value) {
    return String(value ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }
  const escapeHTML = escapeHtml;

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
      // Si existía un diálogo previo, cerrarlo y limpiarlo
      const oldDialog = document.getElementById('notionSwalDialog');
      if (oldDialog) {
        try { if (oldDialog.open) oldDialog.close(); } catch (_) {}
        oldDialog.remove();
      }

      // Usar elemento <dialog> nativo para que el navegador lo sitúe en el Top Layer
      // garantizando que aparezca SIEMPRE por encima de cualquier otro modal abierto
      const dialog = document.createElement('dialog');
      dialog.id = 'notionSwalDialog';
      dialog.className = 'notion-swal-dialog';

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

      dialog.innerHTML = `
        <div class="notion-swal-card" role="document">
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

      document.body.appendChild(dialog);

      // showModal() coloca este diálogo al frente del Top Layer, encima de cualquier otro modal
      try {
        dialog.showModal();
      } catch (_) {
        dialog.setAttribute('open', '');
      }

      requestAnimationFrame(() => {
        dialog.classList.add('active');
        const confirmBtn = document.getElementById('notionSwalBtnConfirm');
        if (confirmBtn) confirmBtn.focus();
      });

      let isClosed = false;
      function cleanup(result) {
        if (isClosed) return;
        isClosed = true;
        dialog.classList.remove('active');
        window.removeEventListener('keydown', handleGlobalKeyDown, true);
        setTimeout(() => {
          try {
            if (dialog.open) dialog.close();
          } catch (_) {}
          dialog.remove();
          resolve(result);
        }, 180);
      }

      // Evento cancel del diálogo (tecla Escape)
      dialog.addEventListener('cancel', (e) => {
        e.preventDefault();
        cleanup(false);
      });

      function handleGlobalKeyDown(e) {
        if (e.key === 'Escape') {
          e.preventDefault();
          e.stopPropagation();
          cleanup(false);
        } else if (e.key === 'Enter') {
          e.preventDefault();
          e.stopPropagation();
          cleanup(true);
        }
      }
      window.addEventListener('keydown', handleGlobalKeyDown, true);

      // Botones de acción
      dialog.querySelector('#notionSwalBtnConfirm')?.addEventListener('click', (e) => {
        e.stopPropagation();
        cleanup(true);
      });
      dialog.querySelector('#notionSwalBtnCancel')?.addEventListener('click', (e) => {
        e.stopPropagation();
        cleanup(false);
      });

      // Clic fuera de la tarjeta (en el backdrop del diálogo)
      dialog.addEventListener('click', (e) => {
        const card = dialog.querySelector('.notion-swal-card');
        if (!card) return;
        const rect = card.getBoundingClientRect();
        const isInCard = (
          rect.top <= e.clientY && e.clientY <= rect.bottom &&
          rect.left <= e.clientX && e.clientX <= rect.right
        );
        if (!isInCard) {
          cleanup(false);
        }
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
  const viewArchivador = $('viewArchivador');
  const navLinkEmpresas = $('navLinkEmpresas');
  const navGroupSire = $('navGroupSire');
  const sidebarGroupSire = $('sidebarGroupSire');
  const navLinkRce = $('navLinkRce');
  const navLinkRvie = $('navLinkRvie');
  const navLinkArchivador = $('navLinkArchivador');
  const btnArchiveOpenFolder = $('btnArchiveOpenFolder');
  const btnArchiveRefresh = $('btnArchiveRefresh');
  const btnArchiveExportZip = $('btnArchiveExportZip');
  const txtArchiveSearch = $('txtArchiveSearch');
  const cboArchiveCompany = $('cboArchiveCompany');
  const cboArchiveBook = $('cboArchiveBook');
  const cboArchivePeriod = $('cboArchivePeriod');
  const cboArchiveFormat = $('cboArchiveFormat');
  const archiveTableBody = $('archiveTableBody');
  const archiveStatTotal = $('archiveStatTotal');
  const archiveStatXml = $('archiveStatXml');
  const archiveStatCdr = $('archiveStatCdr');
  const archiveStatPdf = $('archiveStatPdf');
  const archiveStatSize = $('archiveStatSize');
  const archiveFileCountLabel = $('archiveFileCountLabel');
  const btnArchivePrevPage = $('btnArchivePrevPage');
  const btnArchiveNextPage = $('btnArchiveNextPage');
  const archivePageIndicator = $('archivePageIndicator');
  const tabArchivePeriodsBtn = $('tabArchivePeriodsBtn');
  const tabArchiveVouchersBtn = $('tabArchiveVouchersBtn');
  const archivePeriodsView = $('archivePeriodsView');
  const archiveVouchersView = $('archiveVouchersView');
  const archivePeriodsGrid = $('archivePeriodsGrid');
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
      viewRvie: { el: viewRvie, link: navLinkRvie },
      viewArchivador: { el: viewArchivador, link: navLinkArchivador }
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
    } else if (targetViewId === 'viewArchivador') {
      if (sireSharedDownloads) {
        sireSharedDownloads.style.display = 'none';
      }
      initArchiveView();
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
    $('chkVerCpeClientSecret')?.addEventListener('change', (e) => {
      $('txtEmpresaCpeClientSecret').type = e.target.checked ? 'text' : 'password';
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

      // Dropdown de Validaciones
      const btnVal = $(`btnValidaciones${suffix}`);
      const wrapVal = $(`wrapValidaciones${suffix}`);
      if (btnVal && wrapVal) {
        btnVal.addEventListener('click', (e) => {
          e.stopPropagation();
          const isOpen = wrapVal.classList.contains('open');
          document.querySelectorAll('.dropdown-wrap.open').forEach((w) => w.classList.remove('open'));
          if (!isOpen) {
            wrapVal.classList.add('open');
          }
        });
      }
    }

    // Cerrar dropdowns al hacer clic fuera
    document.addEventListener('click', (e) => {
      if (!e.target.closest('.dropdown-wrap')) {
        document.querySelectorAll('.dropdown-wrap.open').forEach((w) => w.classList.remove('open'));
      }
    });

    // Manejo de clic en ítems de validación
    document.querySelectorAll('.dropdown-menu .dropdown-item').forEach((item) => {
      item.addEventListener('click', (e) => {
        e.stopPropagation();
        const wrap = item.closest('.dropdown-wrap');
        if (wrap) wrap.classList.remove('open');
        const action = item.dataset.action;
        const book = item.dataset.book || '';
        if (action === 'val-tc') {
          openValidateTcModal(book);
          return;
        }
        if (action === 'val-cpe') {
          openValidateCpeModal(book);
          return;
        }
        if (action === 'val-ssco') {
          openValidateSscoModal(book);
          return;
        }
        if (action === 'val-cuadre') {
          openCuadreModal(book);
          return;
        }
        if (action === 'val-correl') {
          openCorrelModal(book);
          return;
        }
        const label = item.querySelector('span:last-child')?.textContent || 'Opción';
        showToast(`${label} (${book}) — Función en preparación`, 'info');
      });
    });

    initValidateTcEvents();
    initValidateCpeEvents();
    initValidateSscoEvents();
    initCuadreEvents();
    initCorrelEvents();

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

    // Eventos Archivador Digital
    navLinkArchivador?.addEventListener('click', () => switchView('viewArchivador'));
    btnArchiveRefresh?.addEventListener('click', async () => {
      await loadArchiveTree();
      renderArchivePeriodCards();
      loadArchiveFiles();
      showToast('Archivador digital actualizado', 'success');
    });
    btnArchiveOpenFolder?.addEventListener('click', () => openArchiveFolder());
    btnArchiveExportZip?.addEventListener('click', () => exportArchiveZip());
    tabArchivePeriodsBtn?.addEventListener('click', () => switchArchiveTab('periods'));
    tabArchiveVouchersBtn?.addEventListener('click', () => switchArchiveTab('vouchers'));
    archivePeriodsGrid?.addEventListener('click', handleArchivePeriodClick);
    cboArchiveCompany?.addEventListener('change', () => {
      populateArchivePeriods();
      archiveCurrentPage = 1;
      loadArchiveFiles();
    });
    cboArchiveBook?.addEventListener('change', () => {
      populateArchivePeriods();
      archiveCurrentPage = 1;
      loadArchiveFiles();
    });
    cboArchivePeriod?.addEventListener('change', () => {
      archiveCurrentPage = 1;
      loadArchiveFiles();
    });
    cboArchiveFormat?.addEventListener('change', () => {
      archiveCurrentPage = 1;
      loadArchiveFiles();
    });
    txtArchiveSearch?.addEventListener('input', () => {
      if (archiveSearchTimeout) clearTimeout(archiveSearchTimeout);
      archiveSearchTimeout = setTimeout(() => {
        archiveCurrentPage = 1;
        loadArchiveFiles();
      }, 250);
    });
    btnArchivePrevPage?.addEventListener('click', () => {
      if (archiveCurrentPage > 1) {
        archiveCurrentPage--;
        loadArchiveFiles();
      }
    });
    btnArchiveNextPage?.addEventListener('click', () => {
      if (archiveCurrentPage < archiveTotalPages) {
        archiveCurrentPage++;
        loadArchiveFiles();
      }
    });
    archiveTableBody?.addEventListener('click', handleArchiveTableClick);

    const themeToggle = $('themeToggleBtn');
    if (themeToggle) {
      themeToggle.addEventListener('click', toggleTheme);
    }

    document.querySelectorAll('.tab-btn').forEach((button) => {
      button.addEventListener('click', () => {
        if (!button.dataset.tab) return;
        document.querySelectorAll('.tab-btn').forEach((item) => item.classList.remove('active'));
        document.querySelectorAll('.tab-content').forEach((item) => item.classList.remove('active'));
        button.classList.add('active');
        $(button.dataset.tab)?.classList.add('active');
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
    if ($('chkVerCpeClientSecret')) $('chkVerCpeClientSecret').checked = false;
    $('txtEmpresaClaveSol').type = 'password';
    $('txtEmpresaClientSecret').type = 'password';
    if ($('txtEmpresaCpeClientSecret')) $('txtEmpresaCpeClientSecret').type = 'password';

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
        if ($('txtEmpresaCpeClientId')) $('txtEmpresaCpeClientId').value = c.cpe_client_id || '';
        if ($('txtEmpresaCpeClientSecret')) {
          $('txtEmpresaCpeClientSecret').value = '';
          $('txtEmpresaCpeClientSecret').placeholder = c.cpe_client_secret ? '••••••••••••••••' : 'Ingrese Api Clave CPE';
        }
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
      if ($('txtEmpresaCpeClientId')) $('txtEmpresaCpeClientId').value = '';
      if ($('txtEmpresaCpeClientSecret')) {
        $('txtEmpresaCpeClientSecret').value = '';
        $('txtEmpresaCpeClientSecret').placeholder = '••••••••••••••••';
      }
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
      cpe_client_id: $('txtEmpresaCpeClientId')?.value.trim() || '',
      cpe_client_secret: $('txtEmpresaCpeClientSecret')?.value || '',
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

      clearProposalArtifactStatuses(isRce);
      recordProposalResults(isRce, data.archivos_existentes || []);

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

  function clearProposalArtifactStatuses(isRce) {
    const maps = isRce ? proposalStatusMapsRce : proposalStatusMapsRvie;
    Object.values(maps).forEach((statusMap) => statusMap.clear());
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

  function recordProposalResults(isRce, resultados) {
    if (!resultados || !resultados.length) return;
    for (const res of resultados) {
      if (!res?.comprobante) continue;
      const resultType = String(res.tipo || '').toUpperCase();
      const statusMap = proposalStatusMap(isRce, resultType);
      if (!statusMap) continue;

      const key = getCompKey(res.comprobante);
      const tipo = String(res.comprobante.tipo || '').padStart(2, '0').trim();
      const serie = String(res.comprobante.serie || '').toUpperCase().trim();
      const numero = normalizeNumero(res.comprobante.numero);
      const fallbackKey = `${tipo}-${serie}-${numero}`;
      const val = {
        exito: res.exito,
        error: res.error || (res.exito ? '' : `No se pudo obtener ${proposalDownloadLabel(resultType)} de SUNAT`),
        nom_archivo: res.nom_archivo,
        ruta_local: res.ruta_local,
        descripcion: res.descripcion,
        placa: res.placa,
        estado_cdr: res.estado_cdr,
        codigo_cdr: res.codigo_cdr,
        mensaje_cdr: res.mensaje_cdr
      };
      statusMap.set(key, val);
      statusMap.set(fallbackKey, val);
      if (res.exito && res.ruta_local) {
        downloadedFiles.set(fileKey(res.comprobante, resultType), res.ruta_local);
      }

      if (resultType === 'DESC' && res.xml_ruta) {
        const xmlValue = {
          exito: true,
          error: '',
          nom_archivo: res.xml_nombre || 'XML',
          ruta_local: res.xml_ruta
        };
        const xmlMap = proposalStatusMap(isRce, 'XML');
        xmlMap.set(key, xmlValue);
        xmlMap.set(fallbackKey, xmlValue);
        downloadedFiles.set(fileKey(res.comprobante, 'XML'), res.xml_ruta);
      }
    }
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
      const cleanup = (finalStatus) => {
        if (finished) return;
        finished = true;
        if (sse) { sse.close(); sse = null; }
        if (pollInterval) { clearInterval(pollInterval); pollInterval = null; }
        progressModal.close();

        if (finalStatus?.resultados) {
          recordProposalResults(isRce, finalStatus.resultados);
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
          recordProposalResults(isRce, status.resultados);
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

  // =========================================================================
  // MÓDULO: ARCHIVADOR DIGITAL
  // =========================================================================

  let archiveTree = null;
  let archiveTreeLoaded = false;
  let archiveCurrentPage = 1;
  const archivePageSize = 50;
  let archiveTotalPages = 1;
  let archiveSearchTimeout = null;
  let activeArchiveTab = 'periods';

  async function readArchiveJSON(response, fallbackMessage) {
    const body = await response.text();
    let data = null;
    if (body.trim()) {
      try {
        data = JSON.parse(body);
      } catch (_) {
        if (response.status === 404) {
          throw new Error('El servidor activo no incluye las rutas del Archivador Digital. Actualiza y reinicia el ejecutable.');
        }
        throw new Error(`${fallbackMessage} (respuesta HTTP ${response.status || 'inválida'})`);
      }
    }
    if (!response.ok) {
      throw new Error(data?.error || `${fallbackMessage} (HTTP ${response.status})`);
    }
    if (!data) {
      throw new Error(`${fallbackMessage}: el servidor devolvió una respuesta vacía`);
    }
    return data;
  }

  function switchArchiveTab(tab) {
    activeArchiveTab = tab;
    if (tab === 'periods') {
      tabArchivePeriodsBtn?.classList.add('active');
      tabArchiveVouchersBtn?.classList.remove('active');
      if (archivePeriodsView) archivePeriodsView.style.display = 'block';
      if (archiveVouchersView) archiveVouchersView.style.display = 'none';
      renderArchivePeriodCards();
    } else {
      tabArchivePeriodsBtn?.classList.remove('active');
      tabArchiveVouchersBtn?.classList.add('active');
      if (archivePeriodsView) archivePeriodsView.style.display = 'none';
      if (archiveVouchersView) archiveVouchersView.style.display = 'block';
      loadArchiveFiles();
    }
  }

  async function initArchiveView() {
    await loadArchiveTree();
    // Preseleccionar empresa activa si aplica
    if (activeCompany && cboArchiveCompany && (!cboArchiveCompany.value || cboArchiveCompany.value === '')) {
      const targetOption = Array.from(cboArchiveCompany.options).find(opt => opt.value.startsWith(activeCompany.ruc));
      if (targetOption) {
        cboArchiveCompany.value = targetOption.value;
        populateArchivePeriods();
      }
    }
    renderArchivePeriodCards();
    if (activeArchiveTab === 'vouchers') {
      loadArchiveFiles();
    }
  }

  async function loadArchiveTree() {
    try {
      const res = await apiFetch('/api/archive/tree');
      const data = await readArchiveJSON(res, 'No se pudo leer el archivador');
      if (!data.success) {
        throw new Error(data.error || 'Error al leer árbol del archivador');
      }
      archiveTree = data;
      archiveTreeLoaded = true;

      // Actualizar estadísticas globales
      if (archiveStatTotal) archiveStatTotal.textContent = (data.total_files || 0).toLocaleString();
      if (archiveStatSize) archiveStatSize.textContent = formatBytes(data.total_bytes || 0);

      // Llenar combo de empresas
      if (cboArchiveCompany) {
        const currentVal = cboArchiveCompany.value;
        cboArchiveCompany.innerHTML = '<option value="">(Todas las empresas)</option>';
        (data.companies || []).forEach(comp => {
          const opt = document.createElement('option');
          opt.value = comp.folder;
          opt.textContent = `${comp.ruc} — ${comp.name} (${comp.total_files} docs)`;
          cboArchiveCompany.appendChild(opt);
        });
        if (currentVal) cboArchiveCompany.value = currentVal;
      }

      populateArchivePeriods();
    } catch (err) {
      console.error('Error cargando archivador:', err);
      showToast('No se pudo cargar el archivador: ' + err.message, 'error');
    }
  }

  function populateArchivePeriods() {
    if (!cboArchivePeriod || !archiveTree) return;
    const selectedCompanyFolder = cboArchiveCompany?.value || '';
    const selectedBook = cboArchiveBook?.value || '';
    const currentVal = cboArchivePeriod.value;

    const periodsSet = new Set();

    (archiveTree.companies || []).forEach(comp => {
      if (selectedCompanyFolder && comp.folder !== selectedCompanyFolder) return;
      (comp.books || []).forEach(book => {
        if (selectedBook && !book.name.toLowerCase().includes(selectedBook.toLowerCase())) return;
        (book.periods || []).forEach(p => {
          if (p.period) periodsSet.add(p.period);
        });
      });
    });

    const sortedPeriods = Array.from(periodsSet).sort().reverse();
    cboArchivePeriod.innerHTML = '<option value="">(Todos los periodos)</option>';
    sortedPeriods.forEach(period => {
      const opt = document.createElement('option');
      opt.value = period;
      opt.textContent = formatPeriodLabel(period);
      cboArchivePeriod.appendChild(opt);
    });

    if (currentVal && periodsSet.has(currentVal)) {
      cboArchivePeriod.value = currentVal;
    }
  }

  function formatPeriodLabel(period) {
    if (period && period.length === 6) {
      const y = period.slice(0, 4);
      const m = period.slice(4, 6);
      const meses = ['', 'Ene', 'Feb', 'Mar', 'Abr', 'May', 'Jun', 'Jul', 'Ago', 'Set', 'Oct', 'Nov', 'Dic'];
      const mNum = parseInt(m, 10);
      if (mNum >= 1 && mNum <= 12) {
        return `${meses[mNum]}-${y} (${period})`;
      }
    }
    return period || '-';
  }

  function renderArchivePeriodCards() {
    if (!archivePeriodsGrid) return;
    const periods = archiveTree?.periods || [];
    if (!periods.length) {
      archivePeriodsGrid.innerHTML = `
        <div style="grid-column: 1 / -1; text-align: center; color: var(--notion-text-subtle); padding: 48px 24px;">
          <div style="font-size: 2.2rem; margin-bottom: 12px;">📂</div>
          <strong style="font-size: 1rem; color: var(--notion-text);">No se encontraron descargas organizadas por período.</strong>
          <p style="margin-top: 8px; font-size: 0.85rem; color: var(--notion-text-muted);">
            Las descargas de XML, CDR y PDF realizadas desde las propuestas SIRE aparecerán agrupadas por período en esta sección.
          </p>
        </div>
      `;
      return;
    }

    archivePeriodsGrid.innerHTML = periods.map((p) => {
      const isCompras = (p.book || '').toLowerCase().includes('comp');
      const bookBadge = isCompras
        ? '<span class="notion-tag notion-tag-gray">Compras (RCE)</span>'
        : '<span class="notion-tag notion-tag-gray">Ventas (RVIE)</span>';

      const xmlItem = `<span class="archive-metric-item ${p.xml_count === 0 ? 'empty' : ''}"><strong>${p.xml_count.toLocaleString()}</strong> XML</span>`;
      const cdrItem = `<span class="archive-metric-item ${p.cdr_count === 0 ? 'empty' : ''}"><strong>${p.cdr_count.toLocaleString()}</strong> CDR</span>`;
      const pdfItem = `<span class="archive-metric-item ${p.pdf_count === 0 ? 'empty' : ''}"><strong>${p.pdf_count.toLocaleString()}</strong> PDF</span>`;

      const sizeStr = formatBytes(p.total_bytes);
      const encodedFolder = encodeURIComponent(p.path);

      return `
        <div class="archive-period-row">
          <div class="archive-period-col-main">
            <div class="archive-period-header-line">
              <span class="archive-period-title">${escapeHtml(p.period_label || p.period)}</span>
              ${bookBadge}
            </div>
            <div class="archive-period-company" title="${escapeHtml(p.company_ruc)} - ${escapeHtml(p.company_name)}">
              <code>${escapeHtml(p.company_ruc)}</code> · ${escapeHtml(p.company_name)}
            </div>
          </div>

          <div class="archive-period-col-metrics">
            <div class="archive-period-metric-group">
              ${xmlItem}
              ${cdrItem}
              ${pdfItem}
            </div>
            <div class="archive-period-total-info">
              Total: <strong>${p.total_files.toLocaleString()}</strong> archivos · ${sizeStr}
            </div>
          </div>

          <div class="archive-period-actions">
            <button type="button" class="btn btn-secondary btn-sm" data-period-action="view" data-company="${escapeHtml(p.company_folder)}" data-book="${escapeHtml(p.book)}" data-period="${escapeHtml(p.period)}" title="Ver comprobantes individuales de este período">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>
              <span>Ver Comprobantes</span>
            </button>
            <button type="button" class="btn btn-secondary btn-sm" data-period-action="folder" data-folder="${encodedFolder}" title="Abrir carpeta en el Explorador de Windows">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg>
              <span>Abrir carpeta</span>
            </button>
            <button type="button" class="btn btn-secondary btn-sm" data-period-action="zip" data-company="${escapeHtml(p.company_folder)}" data-book="${escapeHtml(p.book)}" data-period="${escapeHtml(p.period)}" title="Descargar ZIP de este período">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg>
              <span>ZIP</span>
            </button>
          </div>
        </div>
      `;
    }).join('');
  }

  function handleArchivePeriodClick(e) {
    const btn = e.target.closest('[data-period-action]');
    if (!btn) return;
    const action = btn.dataset.periodAction;

    if (action === 'view') {
      const comp = btn.dataset.company || '';
      const book = btn.dataset.book || '';
      const period = btn.dataset.period || '';
      if (cboArchiveCompany && comp) cboArchiveCompany.value = comp;
      if (cboArchiveBook && book) cboArchiveBook.value = book;
      populateArchivePeriods();
      if (cboArchivePeriod && period) cboArchivePeriod.value = period;
      archiveCurrentPage = 1;
      switchArchiveTab('vouchers');
    } else if (action === 'folder') {
      const folder = decodeURIComponent(btn.dataset.folder || '');
      openArchiveFolder(folder);
    } else if (action === 'zip') {
      const comp = btn.dataset.company || '';
      const book = btn.dataset.book || '';
      const period = btn.dataset.period || '';
      const params = new URLSearchParams({ company: comp, book, period });
      window.location.href = `/api/archive/zip?${params.toString()}`;
    }
  }

  async function loadArchiveFiles() {
    if (!archiveTableBody) return;

    archiveTableBody.innerHTML = `
      <tr>
        <td colspan="10" style="text-align: center; color: var(--notion-text-subtle); padding: 32px;">
          <span class="spinner" style="display:inline-block; margin-right:8px;"></span> Consultando archivador digital...
        </td>
      </tr>
    `;

    const company = cboArchiveCompany?.value || '';
    const book = cboArchiveBook?.value || '';
    const period = cboArchivePeriod?.value || '';
    const format = cboArchiveFormat?.value || 'TODOS';
    const search = txtArchiveSearch?.value.trim() || '';

    const params = new URLSearchParams({
      company,
      book,
      period,
      format,
      search,
      page: String(archiveCurrentPage),
      page_size: String(archivePageSize)
    });

    try {
      const res = await apiFetch(`/api/archive/files?${params.toString()}`);
      const data = await readArchiveJSON(res, 'No se pudieron listar los comprobantes');
      if (!data.success) {
        throw new Error(data.error || 'Error al listar comprobantes');
      }

      // Actualizar contadores
      if (data.summary) {
        if (archiveStatTotal) archiveStatTotal.textContent = (data.summary.total_files || 0).toLocaleString();
        if (archiveStatXml) archiveStatXml.textContent = (data.summary.xml_count || 0).toLocaleString();
        if (archiveStatCdr) archiveStatCdr.textContent = (data.summary.cdr_count || 0).toLocaleString();
        if (archiveStatPdf) archiveStatPdf.textContent = (data.summary.pdf_count || 0).toLocaleString();
        if (archiveStatSize) archiveStatSize.textContent = formatBytes(data.summary.total_bytes || 0);
      }

      archiveTotalPages = data.total_pages || 1;
      const count = data.total_vouchers || data.total || 0;
      const shownCount = (data.vouchers && data.vouchers.length) || (data.files && data.files.length) || 0;
      if (archiveFileCountLabel) {
        archiveFileCountLabel.textContent = `${count.toLocaleString()} comprobante(s) encontrado(s) (${shownCount} en pág. ${data.page})`;
      }
      if (archivePageIndicator) {
        archivePageIndicator.textContent = `Pág. ${data.page} / ${archiveTotalPages}`;
      }
      if (btnArchivePrevPage) btnArchivePrevPage.disabled = data.page <= 1;
      if (btnArchiveNextPage) btnArchiveNextPage.disabled = data.page >= archiveTotalPages;

      if ((!data.vouchers || data.vouchers.length === 0) && (!data.files || data.files.length === 0)) {
        archiveTableBody.innerHTML = `
          <tr>
            <td colspan="10" style="text-align: center; color: var(--notion-text-subtle); padding: 36px;">
              No se encontraron comprobantes con los filtros seleccionados en este almacenamiento local.
            </td>
          </tr>
        `;
        return;
      }

      if (data.vouchers && data.vouchers.length > 0) {
        renderArchiveVoucherRows(data.vouchers, (data.page - 1) * data.page_size);
      } else {
        renderArchiveFileRows(data.files, (data.page - 1) * data.page_size);
      }
    } catch (err) {
      archiveTableBody.innerHTML = `
        <tr>
          <td colspan="10" style="text-align: center; color: var(--tag-red-text); padding: 32px;">
            Error consultando archivos: ${escapeHtml(err.message)}
          </td>
        </tr>
      `;
    }
  }

  function renderArchiveVoucherRows(vouchers, startIndex = 0) {
    const rows = vouchers.map((v, idx) => {
      const itemNum = startIndex + idx + 1;
      const compLabel = (v.serie && v.numero) ? `${escapeHtml(v.serie)}-${escapeHtml(v.numero)}` : escapeHtml(v.key || '—');
      const rucLabel = v.ruc || '—';
      const tipoLabel = v.tipo_nombre || (v.tipo === '01' ? 'Factura' : (v.tipo === '03' ? 'Boleta' : (v.tipo === '07' ? 'Nota Crédito' : (v.tipo === '08' ? 'Nota Débito' : `Tipo ${v.tipo}`))));

      let bookBadge = '';
      if (v.book && v.book.toLowerCase().includes('comp')) {
        bookBadge = '<span class="notion-tag notion-tag-yellow">Compras</span>';
      } else if (v.book && v.book.toLowerCase().includes('vent')) {
        bookBadge = '<span class="notion-tag notion-tag-orange">Ventas</span>';
      } else {
        bookBadge = `<span class="notion-tag notion-tag-gray">${escapeHtml(v.book || '—')}</span>`;
      }

      // XML
      let xmlCell = '<span class="archive-artifact-empty">—</span>';
      if (v.has_xml && v.xml_file) {
        const encXmlPath = encodeURIComponent(v.xml_file.path);
        xmlCell = `<button type="button" class="archive-artifact-btn xml" data-archive-action="view" data-view-path="${encXmlPath}" data-view-type="XML" title="Visualizar XML UBL (${formatBytes(v.xml_file.size)})">✓ XML</button>`;
      }

      // CDR
      let cdrCell = '<span class="archive-artifact-empty">—</span>';
      if (v.has_cdr && v.cdr_file) {
        const encCdrPath = encodeURIComponent(v.cdr_file.path);
        cdrCell = `<button type="button" class="archive-artifact-btn cdr" data-archive-action="view" data-view-path="${encCdrPath}" data-view-type="CDR" title="Visualizar CDR SUNAT (${formatBytes(v.cdr_file.size)})">✓ CDR</button>`;
      } else if (String(v.serie || '').toUpperCase().startsWith('E')) {
        cdrCell = '<span class="artifact-badge-na" title="Comprobantes de serie E no tienen CDR">N/A</span>';
      }

      // PDF
      let pdfCell = '<span class="archive-artifact-empty">—</span>';
      if (v.has_pdf && v.pdf_file) {
        const encPdfPath = encodeURIComponent(v.pdf_file.path);
        pdfCell = `<button type="button" class="archive-artifact-btn pdf" data-archive-action="view" data-view-path="${encPdfPath}" data-view-type="PDF" title="Visualizar PDF (${formatBytes(v.pdf_file.size)})">✓ PDF</button>`;
      }

      const anyPath = v.xml_file?.path || v.cdr_file?.path || v.pdf_file?.path || '';
      const encFolder = encodeURIComponent(anyPath);

      return `
        <tr>
          <td style="text-align: center; font-variant-numeric: tabular-nums; color: var(--notion-text-subtle);">${itemNum}</td>
          <td><span style="font-weight: 500;">${escapeHtml(tipoLabel)}</span></td>
          <td><strong>${compLabel}</strong></td>
          <td><span class="font-mono">${escapeHtml(rucLabel)}</span></td>
          <td>${bookBadge}</td>
          <td><span class="font-mono">${escapeHtml(formatPeriodLabel(v.period))}</span></td>
          <td style="text-align: center;">${xmlCell}</td>
          <td style="text-align: center;">${cdrCell}</td>
          <td style="text-align: center;">${pdfCell}</td>
          <td style="text-align: center;">
            <button type="button" class="btn-icon-action" data-archive-action="open-folder" data-folder-path="${encFolder}" title="Abrir carpeta en Explorador">📂</button>
          </td>
        </tr>
      `;
    }).join('');

    archiveTableBody.innerHTML = rows;
  }

  function renderArchiveFileRows(files, startIndex = 0) {
    const rows = files.map((file, idx) => {
      const itemNum = startIndex + idx + 1;
      const compLabel = (file.serie && file.numero) ? `${escapeHtml(file.serie)}-${escapeHtml(file.numero)}` : escapeHtml(file.name);
      const rucLabel = file.ruc || '-';
      const tipoLabel = file.tipo_nombre || 'Comprobante';

      let bookBadge = '';
      if (file.book && file.book.toLowerCase().includes('comp')) {
        bookBadge = '<span class="notion-tag notion-tag-yellow">Compras</span>';
      } else if (file.book && file.book.toLowerCase().includes('vent')) {
        bookBadge = '<span class="notion-tag notion-tag-orange">Ventas</span>';
      } else {
        bookBadge = `<span class="notion-tag notion-tag-gray">${escapeHtml(file.book || '-')}</span>`;
      }

      const encodedPath = encodeURIComponent(file.path);
      let xmlCell = '<span class="archive-artifact-empty">—</span>';
      let cdrCell = '<span class="archive-artifact-empty">—</span>';
      let pdfCell = '<span class="archive-artifact-empty">—</span>';

      if (file.format === 'XML') {
        xmlCell = `<button type="button" class="archive-artifact-btn xml" data-archive-action="view" data-view-path="${encodedPath}" data-view-type="XML">✓ XML</button>`;
      } else if (file.format === 'CDR') {
        cdrCell = `<button type="button" class="archive-artifact-btn cdr" data-archive-action="view" data-view-path="${encodedPath}" data-view-type="CDR">✓ CDR</button>`;
      } else if (file.format === 'PDF') {
        pdfCell = `<button type="button" class="archive-artifact-btn pdf" data-archive-action="view" data-view-path="${encodedPath}" data-view-type="PDF">✓ PDF</button>`;
      }

      return `
        <tr>
          <td style="text-align: center; font-variant-numeric: tabular-nums; color: var(--notion-text-subtle);">${itemNum}</td>
          <td><span style="font-weight: 500;">${escapeHtml(tipoLabel)}</span></td>
          <td><strong>${compLabel}</strong></td>
          <td><span class="font-mono">${escapeHtml(rucLabel)}</span></td>
          <td>${bookBadge}</td>
          <td><span class="font-mono">${escapeHtml(formatPeriodLabel(file.period))}</span></td>
          <td style="text-align: center;">${xmlCell}</td>
          <td style="text-align: center;">${cdrCell}</td>
          <td style="text-align: center;">${pdfCell}</td>
          <td style="text-align: center;">
            <button type="button" class="btn-icon-action" data-archive-action="open-folder" data-folder-path="${encodedPath}" title="Abrir carpeta en Explorador">📂</button>
          </td>
        </tr>
      `;
    }).join('');

    archiveTableBody.innerHTML = rows;
  }

  function handleArchiveTableClick(event) {
    const btn = event.target.closest('[data-archive-action]');
    if (!btn) return;
    const action = btn.dataset.archiveAction;

    if (action === 'view') {
      const viewPath = btn.dataset.viewPath;
      const type = String(btn.dataset.viewType || '').toUpperCase();
      if (type === 'XML' || type === 'CDR') {
        openXMLViewer(viewPath, type);
      } else {
        resetFileViewer();
        $('fileViewerTitle').textContent = `Visor ${type || 'de archivo'}`;
        $('fileViewerFrame').src = `/api/files/view?path=${viewPath}`;
        $('fileViewerFrame').hidden = false;
        $('fileViewer').showModal();
      }
    } else if (action === 'download') {
      const filePath = btn.dataset.filePath;
      window.location.href = `/api/files/view?download=1&path=${filePath}`;
    } else if (action === 'open-folder') {
      const folderPath = decodeURIComponent(btn.dataset.folderPath || '');
      openArchiveFolder(folderPath);
    }
  }

  async function openArchiveFolder(customPath = '') {
    let url = '/api/archive/open';
    let target = customPath;
    if (!target) {
      const comp = cboArchiveCompany?.value || '';
      const book = cboArchiveBook?.value || '';
      const period = cboArchivePeriod?.value || '';
      if (comp) {
        target = 'APP DESCARGAS/CPE/' + comp;
        if (book) {
          target += '/' + book;
          if (period) target += '/' + period;
        }
      }
    }
    if (target) {
      url += '?path=' + encodeURIComponent(target);
    }
    try {
      const res = await apiFetch(url, { method: 'POST' });
      const data = await readArchiveJSON(res, 'No se pudo abrir la carpeta');
      if (data.success) {
        showToast('Carpeta abierta en el Explorador de Windows', 'success');
      } else {
        throw new Error(data.error || 'No se pudo abrir la carpeta');
      }
    } catch (err) {
      notifyError('Error abriendo carpeta', err.message);
    }
  }

  function exportArchiveZip() {
    const comp = cboArchiveCompany?.value || '';
    const book = cboArchiveBook?.value || '';
    const period = cboArchivePeriod?.value || '';
    if (!comp && !period) {
      notifyWarning('Exportación ZIP', 'Selecciona al menos una empresa o un período para empaquetar el ZIP del archivador.');
      return;
    }
    const params = new URLSearchParams({
      company: comp,
      book,
      period
    });
    window.location.href = `/api/archive/zip?${params.toString()}`;
  }

  function formatBytes(bytes, decimals = 1) {
    if (!bytes || bytes === 0) return '0 B';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
  }

  // =========================================================
  // Módulo: Validación de Tipo de Cambio (T.Cambio)
  // =========================================================
  let currentTcBook = 'RCE';
  let currentTcReport = null;
  let currentTcSelectedIndices = new Set();

  function initValidateTcEvents() {
    const modal = $('modalValidarTc');
    if (!modal) return;

    $('btnCerrarModalTc')?.addEventListener('click', () => modal.close());
    $('btnCancelarModalTc')?.addEventListener('click', () => modal.close());
    modal.addEventListener('click', (e) => {
      if (e.target === modal) modal.close();
    });

    // Selector de Cotización (Venta / Compra)
    document.querySelectorAll('input[name="tcRateChoice"]').forEach((radio) => {
      radio.addEventListener('change', () => {
        if (modal.open) {
          fetchAndRenderTcValidation(radio.value);
        }
      });
    });

    // Filtros y Selección
    $('btnTcSelectDiff')?.addEventListener('click', () => {
      currentTcSelectedIndices.clear();
      (currentTcReport?.items || []).forEach((it) => {
        if (it.estado === 'DIFERENTE' || it.estado === 'VACIO') {
          currentTcSelectedIndices.add(it.index);
        }
      });
      syncTcCheckboxUI();
    });

    $('btnTcSelectAll')?.addEventListener('click', () => {
      currentTcSelectedIndices.clear();
      (currentTcReport?.items || []).forEach((it) => currentTcSelectedIndices.add(it.index));
      syncTcCheckboxUI();
    });

    $('btnTcDeselectAll')?.addEventListener('click', () => {
      currentTcSelectedIndices.clear();
      syncTcCheckboxUI();
    });

    $('tcMasterCheck')?.addEventListener('change', (e) => {
      const checked = e.target.checked;
      const visibleCheckboxes = document.querySelectorAll('#tbodyValidarTc .tc-item-check');
      visibleCheckboxes.forEach((cb) => {
        const idx = parseInt(cb.dataset.index, 10);
        cb.checked = checked;
        if (checked) {
          currentTcSelectedIndices.add(idx);
        } else {
          currentTcSelectedIndices.delete(idx);
        }
      });
      updateTcApplyButtonState();
    });

    $('chkOnlyShowDiff')?.addEventListener('change', () => {
      renderTcTableRows();
    });

    $('btnAplicarModalTc')?.addEventListener('click', applyTcChanges);
  }

  async function openValidateTcModal(book) {
    currentTcBook = book || 'RCE';
    const isRce = currentTcBook === 'RCE';
    const allItems = isRce ? currentProposalItemsRce : currentProposalItemsRvie;

    if (!allItems || !allItems.length) {
      notifyWarning(
        'Sin Comprobantes',
        `Primero debes generar y previsualizar la propuesta de ${isRce ? 'Compras (RCE)' : 'Ventas (RVIE)'}.`
      );
      return;
    }

    const hasForeign = allItems.some((it) => {
      const m = (it.moneda || '').trim().toUpperCase();
      return m && m !== 'PEN';
    });

    if (!hasForeign) {
      notifyInfo(
        'Sin Comprobantes en Moneda Extranjera',
        `Todos los comprobantes de la propuesta ${currentTcBook} están en Soles (PEN). No hay operaciones en dólares u otra moneda para auditar.`
      );
      return;
    }

    const modal = $('modalValidarTc');
    if (!modal) return;

    // Título y selector por defecto
    const title = $('modalValidarTcTitle');
    if (title) {
      title.textContent = `Validación de Tipo de Cambio — ${isRce ? 'Compras (RCE)' : 'Ventas (RVIE)'}`;
    }

    const choiceVenta = $('tcChoiceVenta');
    if (choiceVenta) choiceVenta.checked = true;

    const chkDiff = $('chkOnlyShowDiff');
    if (chkDiff) chkDiff.checked = false;

    modal.showModal();
    await fetchAndRenderTcValidation('Venta');
  }

  async function fetchAndRenderTcValidation(tcType = 'Venta') {
    const isRce = currentTcBook === 'RCE';
    const allItems = isRce ? currentProposalItemsRce : currentProposalItemsRvie;
    const tbody = $('tbodyValidarTc');
    const applyBtn = $('btnAplicarModalTc');
    const statusText = $('tcFooterSummary');

    if (tbody) {
      tbody.innerHTML = `
        <tr>
          <td colspan="11" style="text-align:center; padding: 36px; color: var(--notion-text-muted);">
            <span class="spinner" style="display:inline-block; margin-right:8px;"></span>
            Sincronizando cotizaciones oficiales de SUNAT y auditando comprobantes…
          </td>
        </tr>
      `;
    }
    if (applyBtn) applyBtn.disabled = true;
    if (statusText) statusText.textContent = 'Consultando cotizaciones de SUNAT…';

    try {
      const res = await fetch('/api/sire/validate-tc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          book: currentTcBook,
          tc_type: tcType,
          items: allItems
        })
      });

      if (!res.ok) {
        let errMsg = `Error en servidor: ${res.status}`;
        try {
          const errData = await res.json();
          if (errData && errData.error) errMsg = errData.error;
          else if (errData && errData.message) errMsg = errData.message;
        } catch (_) {}
        throw new Error(errMsg);
      }


      const data = await res.json();
      if (!data.success) {
        throw new Error(data.error || 'No se pudo procesar la validación');
      }

      currentTcReport = data.report;

      // Actualizar contadores de métricas
      $('tcMetricTotal').textContent = currentTcReport.total_usd || 0;
      $('tcMetricOk').textContent = currentTcReport.total_ok || 0;
      $('tcMetricDiff').textContent = currentTcReport.total_diferente || 0;
      $('tcMetricVacio').textContent = currentTcReport.total_vacio || 0;

      // Por defecto, pre-seleccionar los que tienen discrepancia (DIFERENTE o VACÍO)
      currentTcSelectedIndices.clear();
      (currentTcReport.items || []).forEach((it) => {
        if (it.estado === 'DIFERENTE' || it.estado === 'VACIO') {
          currentTcSelectedIndices.add(it.index);
        }
      });

      renderTcTableRows();
    } catch (err) {
      if (tbody) {
        tbody.innerHTML = `
          <tr>
            <td colspan="11" style="text-align:center; padding: 28px; color: #dc2626;">
              ⚠️ Error al validar tipo de cambio: ${escapeHtml(err.message)}
            </td>
          </tr>
        `;
      }
      if (statusText) statusText.textContent = 'Ocurrió un error al consultar SUNAT.';
    }
  }

  function renderTcTableRows() {
    const tbody = $('tbodyValidarTc');
    if (!tbody || !currentTcReport) return;

    const items = currentTcReport.items || [];
    const onlyDiff = $('chkOnlyShowDiff')?.checked;

    const visibleItems = onlyDiff
      ? items.filter((it) => it.estado === 'DIFERENTE' || it.estado === 'VACIO' || it.estado === 'SIN_TC')
      : items;

    if (!visibleItems.length) {
      tbody.innerHTML = `
        <tr>
          <td colspan="11" style="text-align:center; padding: 28px; color: var(--notion-text-muted);">
            ${onlyDiff ? 'No hay comprobantes con discrepancias bajo el criterio seleccionado.' : 'No se encontraron comprobantes en moneda extranjera.'}
          </td>
        </tr>
      `;
      updateTcApplyButtonState();
      return;
    }

    const fmtMoney = (n) => Number(n || 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    const fmtTc = (n) => (n > 0 ? Number(n).toFixed(4) : '—');

    tbody.innerHTML = visibleItems.map((it, displayIdx) => {
      const isChecked = currentTcSelectedIndices.has(it.index);
      let rowClass = '';
      let badgeClass = 'tc-badge-ok';
      let badgeLabel = 'OK';

      switch (it.estado) {
        case 'DIFERENTE':
          rowClass = 'row-diff';
          badgeClass = 'tc-badge-diff';
          badgeLabel = 'DIFERENTE';
          break;
        case 'VACIO':
          rowClass = 'row-vacio';
          badgeClass = 'tc-badge-vacio';
          badgeLabel = 'SIN T/C';
          break;
        case 'SIN_TC':
          badgeClass = 'tc-badge-sintc';
          badgeLabel = 'SIN COTIZACIÓN';
          break;
        case 'ERROR_FECHA':
          badgeClass = 'tc-badge-err';
          badgeLabel = 'ERROR FECHA';
          break;
        default:
          badgeClass = 'tc-badge-ok';
          badgeLabel = 'COINCIDE';
      }

      const diffText = it.diferencia !== 0 ? (it.diferencia > 0 ? `+${it.diferencia.toFixed(4)}` : it.diferencia.toFixed(4)) : '0.0000';
      const refNote = it.es_nota_credito ? `<br><small style="color:var(--notion-text-muted);">[NC Ref: ${escapeHtml(it.fecha_usada)}]</small>` : '';

      return `
        <tr class="${rowClass}">
          <td style="text-align:center;">
            <input type="checkbox" class="tc-item-check" data-index="${it.index}" ${isChecked ? 'checked' : ''}>
          </td>
          <td style="text-align:center; color: var(--notion-text-muted);">${displayIdx + 1}</td>
          <td><strong>${escapeHtml(it.comp_pago || '—')}</strong></td>
          <td>${escapeHtml(it.fecha_usada || '—')}${refNote}</td>
          <td style="text-align:center;"><code>${escapeHtml(it.moneda || 'USD')}</code></td>
          <td style="text-align:right; font-family:var(--font-mono);">${fmtTc(it.tc_archivo)}</td>
          <td style="text-align:right; font-family:var(--font-mono); font-weight:600;">${fmtTc(it.tc_sunat)}</td>
          <td style="text-align:right; font-family:var(--font-mono); color:${it.diferencia !== 0 ? '#b45309' : 'inherit'};">${diffText}</td>
          <td style="text-align:center;"><span class="tc-badge-status ${badgeClass}">${badgeLabel}</span></td>
          <td style="text-align:right; font-family:var(--font-mono);">S/ ${fmtMoney(it.importe_original)}</td>
          <td style="text-align:right; font-family:var(--font-mono); font-weight:600; color:${it.importe_original !== it.importe_recalculado ? '#047857' : 'inherit'};">S/ ${fmtMoney(it.importe_recalculado)}</td>
        </tr>
      `;
    }).join('');

    // Listener para checkboxes de filas individuales
    tbody.querySelectorAll('.tc-item-check').forEach((cb) => {
      cb.addEventListener('change', () => {
        const idx = parseInt(cb.dataset.index, 10);
        if (cb.checked) {
          currentTcSelectedIndices.add(idx);
        } else {
          currentTcSelectedIndices.delete(idx);
        }
        updateTcApplyButtonState();
      });
    });

    updateTcApplyButtonState();
  }

  function syncTcCheckboxUI() {
    const checkboxes = document.querySelectorAll('#tbodyValidarTc .tc-item-check');
    checkboxes.forEach((cb) => {
      const idx = parseInt(cb.dataset.index, 10);
      cb.checked = currentTcSelectedIndices.has(idx);
    });
    updateTcApplyButtonState();
  }

  function updateTcApplyButtonState() {
    const applyBtn = $('btnAplicarModalTc');
    const statusText = $('tcFooterSummary');
    const totalSelected = currentTcSelectedIndices.size;

    if (applyBtn) {
      applyBtn.disabled = totalSelected === 0;
      const btnText = applyBtn.querySelector('.btn-text');
      if (btnText) {
        btnText.textContent = totalSelected > 0
          ? `Cambiar T/C y Recalcular (${totalSelected})`
          : 'Cambiar T/C y Recalcular';
      }
    }

    if (statusText) {
      if (totalSelected > 0) {
        statusText.textContent = `${totalSelected} comprobante(s) marcado(s) para actualizar importes.`;
      } else {
        statusText.textContent = 'Seleccione los comprobantes que desea corregir y recalcular.';
      }
    }

    // Actualizar master check
    const masterCheck = $('tcMasterCheck');
    const visibleCheckboxes = document.querySelectorAll('#tbodyValidarTc .tc-item-check');
    if (masterCheck && visibleCheckboxes.length > 0) {
      const allChecked = Array.from(visibleCheckboxes).every((cb) => cb.checked);
      masterCheck.checked = allChecked;
    }
  }

  async function applyTcChanges() {
    if (!currentTcReport || currentTcSelectedIndices.size === 0) return;

    const count = currentTcSelectedIndices.size;
    const confirmed = await notifyConfirm(
      '¿Aplicar Tipo de Cambio SUNAT?',
      `Se actualizará el tipo de cambio oficial y se recalcularán los montos de ${count} comprobante(s) en la propuesta activa.`,
      'Sí, aplicar cambios'
    );
    if (!confirmed) return;

    const isRce = currentTcBook === 'RCE';
    const allItems = isRce ? currentProposalItemsRce : currentProposalItemsRvie;

    // Crear mapa de cambios desde el reporte
    let appliedCount = 0;
    for (const valItem of currentTcReport.items || []) {
      if (currentTcSelectedIndices.has(valItem.index)) {
        if (valItem.index >= 0 && valItem.index < allItems.length) {
          allItems[valItem.index] = valItem.item_recalculado;
          appliedCount++;
        }
      }
    }

    // Re-renderizar la grilla de propuesta en pantalla
    renderProposalTableGrid(isRce, allItems);

    // Cerrar modal
    $('modalValidarTc')?.close();

    notifySuccess(
      'Tipo de Cambio Aplicado',
      `Se aplicaron las cotizaciones oficiales de SUNAT y se recalcularon los importes en ${appliedCount} comprobante(s).`
    );
  }

  // =========================================================
  // Módulo: Validación de Comprobantes de Pago (CPE) en SUNAT
  // =========================================================
  let currentCpeBook = 'RCE';
  let currentCpeReport = null;
  let currentCpeValidatedItems = [];
  let currentCpeAbortController = null;
  let currentCpeFilter = 'all'; // 'all', 'risk', 'ok'
  let currentCpeSearchQuery = '';

  function initValidateCpeEvents() {
    const modal = $('modalValidarCpe');
    if (!modal) return;

    $('btnCerrarModalCpeCross')?.addEventListener('click', () => {
      abortCpeValidation();
      modal.close();
    });
    $('btnCancelarModalCpe')?.addEventListener('click', () => {
      abortCpeValidation();
      modal.close();
    });
    modal.addEventListener('click', (e) => {
      if (e.target === modal) {
        abortCpeValidation();
        modal.close();
      }
    });

    // Filtros por píldoras
    $('btnCpeFilterAll')?.addEventListener('click', () => setCpeFilter('all'));
    $('btnCpeFilterRisk')?.addEventListener('click', () => setCpeFilter('risk'));
    $('btnCpeFilterOk')?.addEventListener('click', () => setCpeFilter('ok'));

    // Búsqueda
    $('txtCpeSearch')?.addEventListener('input', (e) => {
      currentCpeSearchQuery = (e.target.value || '').trim().toLowerCase();
      renderCpeTableRows();
    });

    // Botones de acción
    $('btnIniciarModalCpe')?.addEventListener('click', startCpeValidation);
    $('btnDetenerCpe')?.addEventListener('click', abortCpeValidation);
    $('btnExportarExcelCpe')?.addEventListener('click', exportCpeToExcel);
  }

  function setCpeFilter(filter) {
    currentCpeFilter = filter;
    document.querySelectorAll('.cpe-pill-btn').forEach((btn) => {
      btn.classList.toggle('active', btn.dataset.filter === filter);
    });
    renderCpeTableRows();
  }

  function openValidateCpeModal(book) {
    currentCpeBook = book || 'RCE';
    const isRce = currentCpeBook === 'RCE';
    const allItems = isRce ? currentProposalItemsRce : currentProposalItemsRvie;

    if (!allItems || !allItems.length) {
      notifyWarning(
        'Sin Comprobantes',
        `Primero debes generar y previsualizar la propuesta de ${isRce ? 'Compras (RCE)' : 'Ventas (RVIE)'}.`
      );
      return;
    }

    const modal = $('modalValidarCpe');
    if (!modal) return;

    // Título y subtítulo
    const title = $('modalValidarCpeTitle');
    if (title) {
      title.textContent = `Validación de Comprobantes — ${isRce ? 'Compras (RCE)' : 'Ventas (RVIE)'}`;
    }

    // Configurar selector de alcance
    $('cpeCountAll').textContent = allItems.length;
    $('cpeScopeAll').checked = true;

    // Detectar si hay filas seleccionadas en la grilla principal
    const selectedCheckboxes = document.querySelectorAll(
      isRce ? '#proposalPreviewBodyRce .row-select:checked' : '#proposalPreviewBodyRvie .row-select:checked'
    );
    const lblSelected = $('lblCpeScopeSelected');
    if (selectedCheckboxes.length > 0) {
      lblSelected.style.display = 'inline-flex';
      $('cpeCountSelected').textContent = selectedCheckboxes.length;
    } else {
      lblSelected.style.display = 'none';
    }

    // Resetear métricas a 0
    currentCpeReport = null;
    currentCpeValidatedItems = [];
    currentCpeFilter = 'all';
    currentCpeSearchQuery = '';
    if ($('txtCpeSearch')) $('txtCpeSearch').value = '';
    setCpeFilter('all');

    $('cpeMetricTotal').textContent = '0';
    $('cpeMetricAceptado').textContent = '0';
    $('cpeMetricAnulado').textContent = '0';
    $('cpeMetricNoExiste').textContent = '0';
    $('cpeMetricRiesgo').textContent = '0';

    $('cpePillCountAll').textContent = '0';
    $('cpePillCountRisk').textContent = '0';
    $('cpePillCountOk').textContent = '0';

    $('cpeProgressContainer').style.display = 'none';
    $('cpeProgressBarFill').style.width = '0%';
    $('btnExportarExcelCpe').style.display = 'none';
    $('btnDetenerCpe').style.display = 'none';
    $('btnIniciarModalCpe').style.display = 'inline-flex';
    $('btnIniciarModalCpe').disabled = false;
    $('spinnerModalCpe').style.display = 'none';

    $('cpeFooterSummary').textContent = `${allItems.length} comprobante(s) listos para auditar en SUNAT.`;

    const tbody = $('tbodyValidarCpe');
    if (tbody) {
      tbody.innerHTML = `
        <tr>
          <td colspan="9" style="text-align:center; padding: 36px; color: var(--notion-text-subtle);">
            Haga clic en <strong>"Iniciar Validación"</strong> para auditar la validez fiscal contra SUNAT.
          </td>
        </tr>
      `;
    }

    modal.showModal();
  }

  function abortCpeValidation() {
    if (currentCpeAbortController) {
      currentCpeAbortController.abort();
      currentCpeAbortController = null;
    }
    const btnIniciar = $('btnIniciarModalCpe');
    const btnDetener = $('btnDetenerCpe');
    const spinner = $('spinnerModalCpe');

    if (btnIniciar) {
      btnIniciar.style.display = 'inline-flex';
      btnIniciar.disabled = false;
    }
    if (btnDetener) btnDetener.style.display = 'none';
    if (spinner) spinner.style.display = 'none';
    const progressText = $('cpeProgressText');
    if (progressText) progressText.textContent = 'Validación detenida';
  }

  async function startCpeValidation() {
    const isRce = currentCpeBook === 'RCE';
    const allItems = isRce ? currentProposalItemsRce : currentProposalItemsRvie;
    if (!allItems || !allItems.length) return;

    let targetItems = allItems;
    const isSelectedScope = $('cpeScopeSelected')?.checked;
    if (isSelectedScope) {
      const selectedCheckboxes = document.querySelectorAll(
        isRce ? '#proposalPreviewBodyRce .row-select:checked' : '#proposalPreviewBodyRvie .row-select:checked'
      );
      const selectedIndices = new Set(Array.from(selectedCheckboxes).map((cb) => parseInt(cb.dataset.index, 10)));
      targetItems = allItems.filter((_, idx) => selectedIndices.has(idx));
    }

    if (!targetItems.length) {
      notifyWarning('Sin Comprobantes', 'No hay comprobantes seleccionados para validar.');
      return;
    }

    // UI en estado de ejecución
    const btnIniciar = $('btnIniciarModalCpe');
    const btnDetener = $('btnDetenerCpe');
    const spinner = $('spinnerModalCpe');
    const progressContainer = $('cpeProgressContainer');
    const progressBar = $('cpeProgressBarFill');
    const progressText = $('cpeProgressText');
    const progressPct = $('cpeProgressPct');
    const footerSummary = $('cpeFooterSummary');
    const tbody = $('tbodyValidarCpe');

    btnIniciar.style.display = 'none';
    btnDetener.style.display = 'inline-flex';
    spinner.style.display = 'inline-block';
    progressContainer.style.display = 'block';
    progressBar.style.width = '0%';
    progressPct.textContent = '0%';
    progressText.textContent = `Iniciando consulta de ${targetItems.length} comprobante(s) en SUNAT...`;
    footerSummary.textContent = 'Conectando con la API de validación de SUNAT...';

    currentCpeValidatedItems = [];
    currentCpeReport = null;
    currentCpeAbortController = new AbortController();

    if (tbody) {
      tbody.innerHTML = `
        <tr>
          <td colspan="9" style="text-align:center; padding: 36px; color: var(--notion-text-muted);">
            <span class="spinner" style="display:inline-block; margin-right:8px;"></span>
            Consultando estado oficial en SUNAT en tiempo real...
          </td>
        </tr>
      `;
    }

    let itemsMap = new Map();
    let totalItems = targetItems.length;

    try {
      const res = await fetch('/api/sire/validate-cpe', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          book: currentCpeBook,
          items: targetItems,
          stream: true
        }),
        signal: currentCpeAbortController.signal
      });

      if (!res.ok) {
        let errMsg = `Error en servidor: ${res.status}`;
        try {
          const errData = await res.json();
          if (errData && errData.error) errMsg = errData.error;
          else if (errData && errData.message) errMsg = errData.message;
        } catch (_) {}
        throw new Error(errMsg);
      }


      const contentType = res.headers.get('Content-Type') || '';
      if (contentType.includes('application/x-ndjson')) {
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop(); // la última línea puede estar incompleta

          for (const line of lines) {
            if (!line.trim()) continue;
            try {
              const msg = JSON.parse(line);
              if (msg.type === 'item') {
                const item = msg.item;
                itemsMap.set(item.index, item);
                currentCpeValidatedItems = Array.from(itemsMap.values());

                const current = msg.current;
                const total = msg.total || totalItems;
                const pct = Math.round((current / total) * 100);

                progressBar.style.width = `${pct}%`;
                progressPct.textContent = `${pct}%`;
                progressText.textContent = `Validando comprobante ${current} de ${total}...`;

                updateCpeLiveMetrics(currentCpeValidatedItems);

                // Re-renderizado de tabla cada 5 comprobantes o en los primeros para fluidez
                if (current <= 5 || current % 5 === 0 || current === total) {
                  renderCpeTableRows();
                }
              } else if (msg.type === 'report') {
                currentCpeReport = msg.report;
                currentCpeValidatedItems = currentCpeReport.items || [];
              } else if (msg.type === 'error') {
                throw new Error(msg.error || 'Error reportado por el servidor');
              }
            } catch (parseErr) {
              console.warn('Línea ndjson no parseable:', line, parseErr);
            }
          }
        }
      } else {
        // Modo JSON directo (fallback)
        const data = await res.json();
        if (!data.success) throw new Error(data.error || 'Error al validar comprobantes');
        currentCpeReport = data.report;
        currentCpeValidatedItems = currentCpeReport.items || [];
      }

      // Finalización
      progressBar.style.width = '100%';
      progressPct.textContent = '100%';
      progressText.textContent = `Validación completada (${currentCpeValidatedItems.length} comprobantes)`;

      updateCpeFinalMetrics(currentCpeReport || { items: currentCpeValidatedItems });
      renderCpeTableRows();

      $('btnExportarExcelCpe').style.display = 'inline-flex';
      const conRiesgo = (currentCpeReport?.total_con_riesgo) || currentCpeValidatedItems.filter(it => it.es_riesgo).length;
      footerSummary.textContent = `Validación finalizada. ${currentCpeValidatedItems.length} comprobante(s) procesados.` +
        (conRiesgo > 0 ? ` (${conRiesgo} con riesgo fiscal detectado)` : ' (Todos conformes)');

      if (conRiesgo > 0) {
        notifyWarning(
          'Validación Finalizada con Observaciones',
          `Se auditaron ${currentCpeValidatedItems.length} comprobantes. Se detectaron ${conRiesgo} con observaciones o riesgo fiscal.`
        );
      } else {
        notifySuccess(
          'Validación Conforme',
          `Todos los ${currentCpeValidatedItems.length} comprobantes fueron validados y se encuentran conformes en SUNAT.`
        );
      }
    } catch (err) {
      if (err.name === 'AbortError') {
        notifyInfo('Validación Cancelada', 'La auditoría de comprobantes fue interrumpida por el usuario.');
      } else {
        const errorMsg = (err.message || '').toLowerCase().includes('network error')
          ? 'Se interrumpió la conexión con el servidor o con SUNAT durante la validación.'
          : err.message;
        notifyError('Error al Validar CPE', errorMsg);
        if (tbody && currentCpeValidatedItems.length === 0) {
          tbody.innerHTML = `
            <tr>
              <td colspan="9" style="text-align:center; padding: 24px; color: var(--notion-red, #e03e3e);">
                ⚠️ Error al validar comprobantes: ${escapeHtml(errorMsg)}
              </td>
            </tr>
          `;
        } else if (currentCpeValidatedItems.length > 0) {
          footerSummary.textContent = `Validación interrumpida. Se lograron auditar ${currentCpeValidatedItems.length} comprobante(s).`;
          renderCpeTableRows();
        }
      }
    } finally {
      btnIniciar.style.display = 'inline-flex';
      btnDetener.style.display = 'none';
      spinner.style.display = 'none';
      currentCpeAbortController = null;
    }
  }

  function updateCpeLiveMetrics(items) {
    let aceptados = 0;
    let anulados = 0;
    let noExiste = 0;
    let alertaRuc = 0;

    for (const it of items) {
      const cp = (it.estado_cp || '').toUpperCase();
      const ruc = (it.estado_ruc || '').toUpperCase();
      const domi = (it.cond_domicilio || '').toUpperCase();

      if (cp === 'ACEPTADO' || cp === 'AUTORIZADO') aceptados++;
      else if (cp === 'ANULADO') anulados++;
      else if (cp === 'NO EXISTE' || cp === 'NO AUTORIZADO') noExiste++;

      if (ruc.includes('BAJA') || ruc === 'SUSPENSION TEMPORAL' || domi === 'NO HABIDO' || domi === 'NO HALLADO') {
        alertaRuc++;
      }
    }

    $('cpeMetricTotal').textContent = items.length;
    $('cpeMetricAceptado').textContent = aceptados;
    $('cpeMetricAnulado').textContent = anulados;
    $('cpeMetricNoExiste').textContent = noExiste;
    $('cpeMetricRiesgo').textContent = alertaRuc;

    const riskCount = items.filter(it => it.es_riesgo).length;
    $('cpePillCountAll').textContent = items.length;
    $('cpePillCountRisk').textContent = riskCount;
    $('cpePillCountOk').textContent = aceptados;
  }

  function updateCpeFinalMetrics(report) {
    const items = report.items || [];
    $('cpeMetricTotal').textContent = report.total_auditados || items.length;
    $('cpeMetricAceptado').textContent = report.total_aceptados || items.filter(it => it.estado_cp === 'ACEPTADO' || it.estado_cp === 'AUTORIZADO').length;
    $('cpeMetricAnulado').textContent = report.total_anulados || items.filter(it => it.estado_cp === 'ANULADO').length;
    $('cpeMetricNoExiste').textContent = report.total_no_existe || items.filter(it => it.estado_cp === 'NO EXISTE' || it.estado_cp === 'NO AUTORIZADO').length;
    $('cpeMetricRiesgo').textContent = (report.total_baja || 0) + (report.total_no_habido || 0);

    const riskCount = report.total_con_riesgo !== undefined ? report.total_con_riesgo : items.filter(it => it.es_riesgo).length;
    $('cpePillCountAll').textContent = items.length;
    $('cpePillCountRisk').textContent = riskCount;
    $('cpePillCountOk').textContent = $('cpeMetricAceptado').textContent;
  }

  function renderCpeTableRows() {
    const tbody = $('tbodyValidarCpe');
    if (!tbody) return;

    let items = currentCpeValidatedItems;
    if (!items.length) return;

    // 1. Filtrar por píldora activa
    if (currentCpeFilter === 'risk') {
      items = items.filter((it) => it.es_riesgo);
    } else if (currentCpeFilter === 'ok') {
      items = items.filter((it) => it.estado_cp === 'ACEPTADO' || it.estado_cp === 'AUTORIZADO');
    }

    // 2. Filtrar por texto de búsqueda
    if (currentCpeSearchQuery) {
      const q = currentCpeSearchQuery;
      items = items.filter((it) => {
        return (
          (it.ruc_emisor || '').toLowerCase().includes(q) ||
          (it.razon_social || '').toLowerCase().includes(q) ||
          (it.serie || '').toLowerCase().includes(q) ||
          (it.numero || '').toLowerCase().includes(q) ||
          (it.comp_pago || '').toLowerCase().includes(q) ||
          (it.estado_cp || '').toLowerCase().includes(q)
        );
      });
    }

    if (items.length === 0) {
      tbody.innerHTML = `
        <tr>
          <td colspan="9" style="text-align:center; padding: 28px; color: var(--notion-text-subtle);">
            No hay comprobantes que coincidan con el filtro seleccionado.
          </td>
        </tr>
      `;
      return;
    }

    tbody.innerHTML = items.map((it, idx) => {
      const rowRiskClass = it.es_riesgo ? 'row-cpe-risk' : '';

      // Badges
      let cpBadgeClass = 'badge-cpe-error';
      const cp = (it.estado_cp || '').toUpperCase();
      if (cp === 'ACEPTADO') cpBadgeClass = 'badge-cpe-aceptado';
      else if (cp === 'AUTORIZADO') cpBadgeClass = 'badge-cpe-autorizado';
      else if (cp === 'ANULADO') cpBadgeClass = 'badge-cpe-anulado';
      else if (cp === 'NO EXISTE') cpBadgeClass = 'badge-cpe-noexiste';
      else if (cp === 'NO AUTORIZADO') cpBadgeClass = 'badge-cpe-noautorizado';

      let rucBadgeClass = 'badge-ruc';
      const ruc = (it.estado_ruc || '').toUpperCase();
      if (ruc === 'ACTIVO') rucBadgeClass = 'badge-ruc badge-ruc-activo';
      else if (ruc.includes('BAJA') || ruc.includes('SUSPENSION') || ruc.includes('INHABILITADO')) rucBadgeClass = 'badge-ruc badge-ruc-baja';

      let domiBadgeClass = 'badge-domi';
      const domi = (it.cond_domicilio || '').toUpperCase();
      if (domi === 'HABIDO') domiBadgeClass = 'badge-domi badge-domi-habido';
      else if (domi === 'NO HABIDO' || domi === 'NO HALLADO') domiBadgeClass = 'badge-domi badge-domi-nohabido';
      else if (domi === 'PENDIENTE') domiBadgeClass = 'badge-domi badge-domi-pendiente';

      const montoFormatted = (it.monto_original || 0).toLocaleString('es-PE', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
      const compLabel = `${it.tipo ? it.tipo + '-' : ''}${it.serie || ''}-${it.numero || ''}`;

      return `
        <tr class="${rowRiskClass}">
          <td style="text-align:center; color: var(--notion-text-muted);">${idx + 1}</td>
          <td class="cpe-comp-num">${escapeHtml(compLabel)}</td>
          <td style="text-align:center;">${escapeHtml(it.fecha || '')}</td>
          <td>
            <div class="cpe-ruc" style="font-weight:600;">${escapeHtml(it.ruc_emisor || '')}</div>
            <div style="font-size:0.73rem; color:var(--notion-text-muted); max-width:260px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">
              ${escapeHtml(it.razon_social || '')}
            </div>
          </td>
          <td style="text-align:right; font-family:var(--font-mono, monospace); font-weight:600;">
            ${escapeHtml(it.moneda || 'PEN')} ${montoFormatted}
          </td>
          <td style="text-align:center;">
            <span class="badge-cpe ${cpBadgeClass}">${escapeHtml(it.estado_cp || 'SIN DATOS')}</span>
          </td>
          <td style="text-align:center;">
            <span class="${rucBadgeClass}">${escapeHtml(it.estado_ruc || '—')}</span>
          </td>
          <td style="text-align:center;">
            <span class="${domiBadgeClass}">${escapeHtml(it.cond_domicilio || '—')}</span>
          </td>
          <td style="font-size:0.74rem; color:var(--notion-text-muted);">
            ${escapeHtml(it.observaciones || '—')}
          </td>
        </tr>
      `;
    }).join('');
  }

  function exportCpeToExcel() {
    if (!currentCpeValidatedItems.length) {
      notifyWarning('Sin Datos', 'No hay resultados auditados para exportar.');
      return;
    }

    const headers = [
      'N°',
      'Tipo Comp.',
      'Serie',
      'Número',
      'Fecha Emisión',
      'RUC Emisor',
      'Razón Social',
      'Moneda',
      'Monto Original',
      'Estado Comprobante SUNAT',
      'Estado RUC',
      'Condición Domicilio',
      'Observaciones SUNAT',
      'Riesgo Fiscal'
    ];

    const rows = currentCpeValidatedItems.map((it, idx) => [
      idx + 1,
      it.tipo || '',
      it.serie || '',
      it.numero || '',
      it.fecha || '',
      `="${it.ruc_emisor || ''}"`,
      `"${(it.razon_social || '').replace(/"/g, '""')}"`,
      it.moneda || 'PEN',
      (it.monto_original || 0).toFixed(2),
      it.estado_cp || '',
      it.estado_ruc || '',
      it.cond_domicilio || '',
      `"${(it.observaciones || '').replace(/"/g, '""')}"`,
      it.es_riesgo ? 'SI' : 'NO'
    ]);

    const csvContent = '\uFEFF' + [headers.join(','), ...rows.map(r => r.join(','))].join('\r\n');
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    const now = new Date();
    const dateStr = now.toISOString().slice(0, 10).replace(/-/g, '');
    a.download = `Validacion_CPE_SUNAT_${currentCpeBook}_${dateStr}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);

    notifySuccess('Reporte Descargado', 'Se descargó el reporte de auditoría en formato CSV (compatible con Excel).');
  }

  // =========================================================
  // Módulo: Validación SSCO (Sujetos Sin Capacidad Operativa)
  // =========================================================
  let currentSscoBook = 'RCE';
  let currentSscoReport = null;
  let currentSscoValidatedItems = [];
  let currentSscoFilter = 'all'; // 'all', 'risk', 'ok'
  let currentSscoSearchQuery = '';

  function initValidateSscoEvents() {
    const modal = $('modalValidarSsco');
    if (!modal) return;

    $('btnCerrarModalSscoCross')?.addEventListener('click', () => modal.close());
    $('btnCancelarModalSsco')?.addEventListener('click', () => modal.close());
    modal.addEventListener('click', (e) => {
      if (e.target === modal) modal.close();
    });

    // Filtros por píldoras
    $('btnSscoFilterAll')?.addEventListener('click', () => setSscoFilter('all'));
    $('btnSscoFilterRisk')?.addEventListener('click', () => setSscoFilter('risk'));
    $('btnSscoFilterOk')?.addEventListener('click', () => setSscoFilter('ok'));

    // Búsqueda
    $('txtSscoSearch')?.addEventListener('input', (e) => {
      currentSscoSearchQuery = (e.target.value || '').trim().toLowerCase();
      renderSscoTableRows();
    });

    // Botones de acción
    $('btnIniciarModalSsco')?.addEventListener('click', startSscoValidation);
    $('btnActualizarPadronSsco')?.addEventListener('click', refreshSscoPadron);
    $('btnExportarExcelSsco')?.addEventListener('click', exportSscoToExcel);
  }

  function setSscoFilter(filter) {
    currentSscoFilter = filter;
    $('btnSscoFilterAll')?.classList.toggle('active', filter === 'all');
    $('btnSscoFilterRisk')?.classList.toggle('active', filter === 'risk');
    $('btnSscoFilterOk')?.classList.toggle('active', filter === 'ok');
    renderSscoTableRows();
  }

  function openValidateSscoModal(book) {
    currentSscoBook = book || 'RCE';
    if (currentSscoBook !== 'RCE') {
      notifyWarning(
        'Validación de Compras',
        'La validación contra el padrón de SSCO (D.L. 1532) aplica a las COMPRAS (RCE), ya que afecta el crédito fiscal y la deducción de costos/gastos.'
      );
      return;
    }

    const allItems = currentProposalItemsRce;
    if (!allItems || !allItems.length) {
      notifyWarning(
        'Sin Comprobantes',
        'Primero debes generar y previsualizar la propuesta de Compras (RCE).'
      );
      return;
    }

    const modal = $('modalValidarSsco');
    if (!modal) return;

    // Configurar selector de alcance
    $('sscoCountAll').textContent = allItems.length;
    $('sscoScopeAll').checked = true;

    const selectedCheckboxes = document.querySelectorAll('#proposalPreviewBodyRce .row-select:checked');
    const lblSelected = $('lblSscoScopeSelected');
    if (selectedCheckboxes.length > 0) {
      lblSelected.style.display = 'inline-flex';
      $('sscoCountSelected').textContent = selectedCheckboxes.length;
    } else {
      lblSelected.style.display = 'none';
    }

    // Resetear métricas a 0
    currentSscoReport = null;
    currentSscoValidatedItems = [];
    currentSscoFilter = 'all';
    currentSscoSearchQuery = '';
    if ($('txtSscoSearch')) $('txtSscoSearch').value = '';
    setSscoFilter('all');

    $('sscoMetricTotal').textContent = '0';
    $('sscoMetricCritico').textContent = '0';
    $('sscoMetricWarning').textContent = '0';
    $('sscoMetricOk').textContent = '0';
    $('sscoMetricIgvRiesgo').textContent = 'S/ 0.00';

    $('sscoPillCountAll').textContent = '0';
    $('sscoPillCountRisk').textContent = '0';
    $('sscoPillCountOk').textContent = '0';

    $('btnExportarExcelSsco').style.display = 'none';
    const btnIniciar = $('btnIniciarModalSsco');
    if (btnIniciar) {
      btnIniciar.disabled = false;
      const btnText = btnIniciar.querySelector('.btn-text');
      if (btnText) btnText.textContent = 'Iniciar Validación SSCO';
    }
    $('spinnerModalSsco').style.display = 'none';

    $('sscoFooterSummary').textContent = `${allItems.length} comprobante(s) listos para auditar contra padrón SUNAT.`;

    const tbody = $('tbodyValidarSsco');
    if (tbody) {
      tbody.innerHTML = `
        <tr>
          <td colspan="10" style="text-align:center; padding: 36px; color: var(--notion-text-subtle);">
            Haga clic en <strong>"Iniciar Validación SSCO"</strong> para cruzar sus compras con el padrón oficial.
          </td>
        </tr>
      `;
    }

    modal.showModal();
  }

  async function refreshSscoPadron() {
    const btn = $('btnActualizarPadronSsco');
    const statusText = $('sscoPadronStatusText');
    if (!btn) return;

    btn.disabled = true;
    const origHTML = btn.innerHTML;
    btn.innerHTML = '<span class="spinner" style="display:inline-block; margin-right:4px;"></span> Actualizando...';

    try {
      const res = await apiFetch('/api/sire/ssco/refresh', { method: 'POST' });
      if (!res.ok) {
        let errMsg = `Error en servidor (${res.status})`;
        try {
          const errData = await res.json();
          if (errData && errData.error) errMsg = errData.error;
        } catch (_) {}
        throw new Error(errMsg);
      }
      const data = await res.json();
      if (!data.success) throw new Error(data.error || 'Error al actualizar padrón');

      if (statusText) {
        statusText.innerHTML = `Sujetos registrados: <strong>${(data.total_sujetos || 0).toLocaleString()}</strong> | Actualizado al: <strong>${data.fecha_actualizacion || 'Reciente'}</strong> ${data.desde_cache ? '<em>(Copia local)</em>' : ''}`;
      }
      notifySuccess('Padrón Actualizado', `Se sincronizó el padrón oficial de SUNAT con ${(data.total_sujetos || 0).toLocaleString()} sujetos.`);
    } catch (err) {
      notifyError('Actualización Fallida', err.message);
    } finally {
      btn.disabled = false;
      btn.innerHTML = origHTML;
    }
  }

  async function startSscoValidation() {
    const allItems = currentProposalItemsRce;
    if (!allItems || !allItems.length) return;

    let targetItems = allItems;
    const isSelectedScope = $('sscoScopeSelected')?.checked;
    if (isSelectedScope) {
      const selectedCheckboxes = document.querySelectorAll('#proposalPreviewBodyRce .row-select:checked');
      const selectedIndices = new Set(Array.from(selectedCheckboxes).map((cb) => parseInt(cb.dataset.index, 10)));
      targetItems = allItems.filter((_, idx) => selectedIndices.has(idx));
    }

    if (!targetItems.length) {
      notifyWarning('Sin Comprobantes', 'No hay comprobantes seleccionados para validar.');
      return;
    }

    const btnIniciar = $('btnIniciarModalSsco');
    const spinner = $('spinnerModalSsco');
    const footerSummary = $('sscoFooterSummary');
    const tbody = $('tbodyValidarSsco');

    btnIniciar.disabled = true;
    spinner.style.display = 'inline-block';
    const btnText = btnIniciar.querySelector('.btn-text');
    if (btnText) btnText.textContent = 'Auditando compras...';
    footerSummary.textContent = `Cruzando ${targetItems.length} comprobante(s) contra el padrón oficial SSCO...`;

    if (tbody) {
      tbody.innerHTML = `
        <tr>
          <td colspan="10" style="text-align:center; padding: 36px; color: var(--notion-text-muted);">
            <span class="spinner" style="display:inline-block; margin-right:8px;"></span>
            Descargando y cruzando compras con el Padrón Oficial SSCO de SUNAT...
          </td>
        </tr>
      `;
    }

    try {
      const res = await apiFetch('/api/sire/validate-ssco', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          book: 'RCE',
          items: targetItems
        })
      });

      if (!res.ok) {
        let errMsg = `Error en servidor (${res.status})`;
        try {
          const errData = await res.json();
          if (errData && errData.error) errMsg = errData.error;
          else if (errData && errData.message) errMsg = errData.message;
        } catch (_) {
          if (res.status === 404) {
            errMsg = 'El servicio de validación SSCO no está disponible en este servidor. Por favor reinicie el servidor appsire-server.exe.';
          }
        }
        throw new Error(errMsg);
      }

      const data = await res.json();
      if (!data.success) {
        throw new Error(data.error || 'Error al procesar validación SSCO');
      }

      currentSscoReport = data.report;
      currentSscoValidatedItems = (data.report && data.report.items) || [];

      // Actualizar contadores métricos
      $('sscoMetricTotal').textContent = currentSscoReport.total_items || 0;
      $('sscoMetricCritico').textContent = currentSscoReport.count_ssco || 0;
      $('sscoMetricWarning').textContent = currentSscoReport.count_warning || 0;
      $('sscoMetricOk').textContent = currentSscoReport.count_ok || 0;
      $('sscoMetricIgvRiesgo').textContent = `S/ ${(currentSscoReport.total_igv_riesgo || 0).toLocaleString('es-PE', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;

      const totalRisk = (currentSscoReport.count_ssco || 0) + (currentSscoReport.count_warning || 0);
      $('sscoPillCountAll').textContent = currentSscoReport.total_items || 0;
      $('sscoPillCountRisk').textContent = totalRisk;
      $('sscoPillCountOk').textContent = currentSscoReport.count_ok || 0;

      // Actualizar barra de estado del padrón
      const statusText = $('sscoPadronStatusText');
      if (statusText) {
        statusText.innerHTML = `Sujetos registrados: <strong>${(currentSscoReport.padron_total || 0).toLocaleString()}</strong> | Actualizado al: <strong>${currentSscoReport.padron_fecha || 'Reciente'}</strong> ${currentSscoReport.desde_cache ? '<em>(Copia local)</em>' : ''}`;
      }

      // Renderizar tabla
      renderSscoTableRows();

      // Botón exportar
      $('btnExportarExcelSsco').style.display = 'inline-flex';

      if (totalRisk > 0) {
        footerSummary.innerHTML = `<span style="color:#b91c1c; font-weight:600;">⚠️ ALERTA: Se detectaron ${currentSscoReport.count_ssco} comprobante(s) con pérdida de crédito fiscal y ${currentSscoReport.count_warning} por revisar.</span>`;
        notifyWarning(
          'Riesgo Fiscal SSCO Detectado',
          `Se detectaron ${currentSscoReport.count_ssco} comprobante(s) de Sujetos Sin Capacidad Operativa con S/ ${currentSscoReport.total_igv_riesgo.toFixed(2)} de crédito fiscal en riesgo (D.L. 1532).`
        );
      } else {
        footerSummary.innerHTML = `<span style="color:#047857; font-weight:600;">✓ Conforme: Ningún proveedor auditado figura en el padrón de SSCO.</span>`;
        notifySuccess('Auditoría Conforme', 'Ninguno de sus proveedores de compras figura como Sujeto Sin Capacidad Operativa.');
      }

    } catch (err) {
      notifyError('Error de Validación', err.message);
      footerSummary.textContent = `Error: ${err.message}`;
      if (tbody) {
        tbody.innerHTML = `
          <tr>
            <td colspan="10" style="text-align:center; padding: 36px; color: var(--danger);">
              ${escapeHtml(err.message)}
            </td>
          </tr>
        `;
      }
    } finally {
      btnIniciar.disabled = false;
      spinner.style.display = 'none';
      if (btnText) btnText.textContent = 'Iniciar Validación SSCO';
    }
  }

  function renderSscoTableRows() {
    const tbody = $('tbodyValidarSsco');
    if (!tbody) return;

    if (!currentSscoValidatedItems.length) {
      tbody.innerHTML = `
        <tr>
          <td colspan="10" style="text-align:center; padding: 36px; color: var(--notion-text-subtle);">
            No hay resultados disponibles.
          </td>
        </tr>
      `;
      return;
    }

    let filtered = currentSscoValidatedItems;
    if (currentSscoFilter === 'risk') {
      filtered = filtered.filter((it) => it.riesgo === 'CRITICAL' || it.riesgo === 'WARNING');
    } else if (currentSscoFilter === 'ok') {
      filtered = filtered.filter((it) => it.riesgo === 'OK');
    }

    if (currentSscoSearchQuery) {
      const q = currentSscoSearchQuery;
      filtered = filtered.filter((it) =>
        (it.ruc && it.ruc.toLowerCase().includes(q)) ||
        (it.razon_social && it.razon_social.toLowerCase().includes(q)) ||
        (it.serie && it.serie.toLowerCase().includes(q)) ||
        (it.numero && it.numero.toLowerCase().includes(q)) ||
        (it.resolucion && it.resolucion.toLowerCase().includes(q))
      );
    }

    if (!filtered.length) {
      tbody.innerHTML = `
        <tr>
          <td colspan="10" style="text-align:center; padding: 32px; color: var(--notion-text-muted);">
            No se encontraron comprobantes que coincidan con el filtro actual.
          </td>
        </tr>
      `;
      return;
    }

    tbody.innerHTML = filtered.map((it, idx) => {
      let badgeHtml = '';
      let rowClass = '';
      if (it.riesgo === 'CRITICAL') {
        badgeHtml = '<span class="ssco-badge ssco-badge-critico">⚠️ SIN CAPACIDAD</span>';
        rowClass = 'row-ssco-critical';
      } else if (it.riesgo === 'WARNING') {
        badgeHtml = '<span class="ssco-badge ssco-badge-warning">🔍 REVISAR NOMBRE</span>';
        rowClass = 'row-ssco-warning';
      } else {
        badgeHtml = '<span class="ssco-badge ssco-badge-ok">✓ NO REGISTRADO</span>';
      }

      const compLabel = (it.serie && it.numero) ? `${it.serie}-${it.numero}` : (it.comp_pago || '—');
      const fmtMonto = (it.importe_total || 0).toLocaleString('es-PE', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
      const fmtIgv = (it.igv || 0).toLocaleString('es-PE', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
      const igvColor = it.riesgo === 'CRITICAL' ? 'color:#b91c1c; font-weight:700;' : '';

      return `
        <tr class="${rowClass}">
          <td style="text-align:center; color: var(--notion-text-subtle);">${idx + 1}</td>
          <td><strong>${escapeHtml(compLabel)}</strong></td>
          <td style="text-align:center;">${escapeHtml(it.fecha || '—')}</td>
          <td><span class="font-mono" style="font-weight:600;">${escapeHtml(it.ruc || '—')}</span></td>
          <td><span title="${escapeHtml(it.razon_social || '')}">${escapeHtml(it.razon_social || '—')}</span></td>
          <td style="text-align:right; font-family:var(--font-mono);">S/ ${fmtMonto}</td>
          <td style="text-align:right; font-family:var(--font-mono); ${igvColor}">S/ ${fmtIgv}</td>
          <td style="text-align:center;">${badgeHtml}</td>
          <td style="text-align:center;"><span class="notion-tag">${escapeHtml(it.coincide_por || '-')}</span></td>
          <td style="font-size:0.75rem; color: ${it.riesgo === 'CRITICAL' ? '#b91c1c' : 'var(--notion-text-muted)'};">
            ${escapeHtml(it.detalle || '—')}
          </td>
        </tr>
      `;
    }).join('');
  }

  function exportSscoToExcel() {
    if (!currentSscoValidatedItems.length) {
      notifyWarning('Sin Datos', 'No hay resultados auditados para exportar.');
      return;
    }

    const headers = [
      'N°',
      'Comprobante',
      'Fecha Emisión',
      'RUC Proveedor',
      'Razón Social Proveedor',
      'Importe Total',
      'IGV (Crédito Fiscal)',
      'Estado SSCO',
      'Coincide Por',
      'Resolución SUNAT',
      'Fecha Firme',
      'Fecha Publicación',
      'Detalle',
      'Riesgo Fiscal'
    ];

    const rows = currentSscoValidatedItems.map((it, idx) => [
      idx + 1,
      `"${it.serie ? it.serie + '-' + it.numero : it.comp_pago || ''}"`,
      it.fecha || '',
      `="${it.ruc || ''}"`,
      `"${(it.razon_social || '').replace(/"/g, '""')}"`,
      (it.importe_total || 0).toFixed(2),
      (it.igv || 0).toFixed(2),
      it.estado || '',
      it.coincide_por || '',
      `"${(it.resolucion || '').replace(/"/g, '""')}"`,
      it.fecha_firme || '',
      it.fecha_publicacion || '',
      `"${(it.detalle || '').replace(/"/g, '""')}"`,
      it.riesgo || 'OK'
    ]);

    const csvContent = '\uFEFF' + [headers.join(','), ...rows.map(r => r.join(','))].join('\r\n');
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    const now = new Date();
    const dateStr = now.toISOString().slice(0, 10).replace(/-/g, '');
    a.download = `Auditoria_SSCO_Compras_RCE_${dateStr}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);

    notifySuccess('Reporte Descargado', 'Se descargó el reporte de auditoría SSCO en formato compatible con Excel.');
  }

  // =========================================================
  // Módulo: Cuadre de Importes (Compras RCE / Ventas RVIE)
  // =========================================================
  let currentCuadreBook = 'RCE';
  let currentCuadreReport = null;
  let currentCuadreItems = [];
  let currentCuadreFilter = 'all'; // 'all', 'exceso', 'defecto'
  let currentCuadreSearchQuery = '';
  let currentCuadreGlobalCol = 'bi_gravada';

  function initCuadreEvents() {
    const modal = $('modalCuadrarImportes');
    if (!modal) return;

    $('btnCerrarModalCuadreCross')?.addEventListener('click', () => modal.close());
    $('btnCancelarModalCuadre')?.addEventListener('click', () => modal.close());
    modal.addEventListener('click', (e) => {
      if (e.target === modal) modal.close();
    });

    // Selector global de columna
    $('cboCuadreGlobalCol')?.addEventListener('change', (e) => {
      currentCuadreGlobalCol = e.target.value;
    });

    $('btnCuadreAplicarGlobal')?.addEventListener('click', () => {
      const selCol = $('cboCuadreGlobalCol')?.value || 'bi_gravada';
      currentCuadreGlobalCol = selCol;
      for (const it of currentCuadreItems) {
        recalculateCuadreItem(it, selCol);
      }
      renderCuadreRows();
      notifySuccess('Columna Aplicada', `Se asignó la columna "${selCol}" como objetivo de ajuste para todas las filas.`);
    });

    // Píldoras de filtro
    $('btnCuadreFilterAll')?.addEventListener('click', () => setCuadreFilter('all'));
    $('btnCuadreFilterExceso')?.addEventListener('click', () => setCuadreFilter('exceso'));
    $('btnCuadreFilterDefecto')?.addEventListener('click', () => setCuadreFilter('defecto'));

    // Búsqueda
    $('txtCuadreSearch')?.addEventListener('input', (e) => {
      currentCuadreSearchQuery = (e.target.value || '').trim().toLowerCase();
      renderCuadreRows();
    });

    // Acciones principales
    $('btnAplicarModalCuadre')?.addEventListener('click', applyCuadreAdjustments);
    $('btnExportarExcelCuadre')?.addEventListener('click', exportCuadreToExcel);

    // Cambio de columna en fila individual (delegado en tbody)
    $('tbodyCuadrarImportes')?.addEventListener('change', (e) => {
      const select = e.target.closest('.cuadre-row-select');
      if (!select) return;
      const rowIdx = parseInt(select.dataset.cuadreIndex, 10);
      const it = currentCuadreItems[rowIdx];
      if (!it) return;
      const newCol = select.value;
      recalculateCuadreItem(it, newCol);

      // Actualizar visualmente la celda de Antes -> Ajustado sin re-renderizar toda la tabla
      const cellPreview = document.getElementById(`cuadrePreview_${rowIdx}`);
      if (cellPreview) {
        cellPreview.innerHTML = `<span class="cuadre-val-prev">${it.valor_actual.toFixed(2)}</span> &rarr; <strong class="cuadre-val-next">${it.valor_ajustado.toFixed(2)}</strong>`;
      }
    });
  }

  function setCuadreFilter(filter) {
    currentCuadreFilter = filter;
    $('btnCuadreFilterAll')?.classList.toggle('active', filter === 'all');
    $('btnCuadreFilterExceso')?.classList.toggle('active', filter === 'exceso');
    $('btnCuadreFilterDefecto')?.classList.toggle('active', filter === 'defecto');
    renderCuadreRows();
  }

  function recalculateCuadreItem(it, colId) {
    const isRce = currentCuadreBook === 'RCE';
    const allItems = isRce ? currentProposalItemsRce : currentProposalItemsRvie;
    const orig = allItems[it.index];
    if (!orig) return;

    let currentVal = 0;
    if (colId === 'bi_gravada') currentVal = parseFloat(orig.bi_gravada || 0) || 0;
    else if (colId === 'igv') currentVal = parseFloat(orig.igv || 0) || 0;
    else if (colId === 'adq_no_gravada') currentVal = parseFloat(orig.adq_no_gravada || 0) || 0;
    else if (colId === 'bi_grav_y_no_grav') currentVal = parseFloat(orig.bi_grav_y_no_grav || 0) || 0;
    else if (colId === 'bi_no_gravada') currentVal = parseFloat(orig.bi_no_gravada || 0) || 0;
    else if (colId === 'otros_conceptos') currentVal = parseFloat(orig.otros_conceptos || 0) || 0;
    else currentVal = parseFloat(orig.bi_gravada || 0) || 0;

    const dif = it.diferencia;
    const newVal = Math.round((currentVal + dif) * 100) / 100;

    it.columna_ajuste = colId;
    it.valor_actual = currentVal;
    it.valor_ajustado = newVal;

    const cloned = { ...orig };
    if (colId === 'bi_gravada') cloned.bi_gravada = newVal.toFixed(2);
    else if (colId === 'igv') cloned.igv = newVal.toFixed(2);
    else if (colId === 'adq_no_gravada') cloned.adq_no_gravada = newVal.toFixed(2);
    else if (colId === 'bi_grav_y_no_grav') cloned.bi_grav_y_no_grav = newVal.toFixed(2);
    else if (colId === 'bi_no_gravada') cloned.bi_no_gravada = newVal.toFixed(2);
    else if (colId === 'otros_conceptos') cloned.otros_conceptos = newVal.toFixed(2);
    else cloned.bi_gravada = newVal.toFixed(2);

    it.item_recalculado = cloned;
  }

  async function openCuadreModal(book) {
    currentCuadreBook = book || 'RCE';
    const isRce = currentCuadreBook === 'RCE';
    const allItems = isRce ? currentProposalItemsRce : currentProposalItemsRvie;

    if (!allItems || !allItems.length) {
      notifyWarning(
        'Sin Comprobantes',
        `Primero debes generar y previsualizar la propuesta de ${isRce ? 'Compras (RCE)' : 'Ventas (RVIE)'}.`
      );
      return;
    }

    const modal = $('modalCuadrarImportes');
    if (!modal) return;

    // Configurar título y subtítulo
    const titleEl = $('modalCuadrarImportesTitle');
    if (titleEl) titleEl.textContent = `Cuadre de Importes - ${isRce ? 'Compras (RCE)' : 'Ventas (RVIE)'}`;

    // Resetear estado
    currentCuadreReport = null;
    currentCuadreItems = [];
    currentCuadreFilter = 'all';
    currentCuadreSearchQuery = '';
    if ($('txtCuadreSearch')) $('txtCuadreSearch').value = '';
    setCuadreFilter('all');

    $('cuadreMetricDescuadres').textContent = '0';
    $('cuadreMetricDiferenciaNeta').textContent = 'S/ 0.00';
    $('cuadreMetricExceso').textContent = '0';
    $('cuadreMetricDefecto').textContent = '0';

    $('cuadrePillCountAll').textContent = '0';
    $('cuadrePillCountExceso').textContent = '0';
    $('cuadrePillCountDefecto').textContent = '0';

    $('btnExportarExcelCuadre').style.display = 'none';
    const btnAplicar = $('btnAplicarModalCuadre');
    if (btnAplicar) {
      btnAplicar.disabled = true;
      const btnText = btnAplicar.querySelector('.btn-text');
      if (btnText) btnText.textContent = 'Cuadrar y Aplicar Ajustes';
    }
    $('spinnerModalCuadre').style.display = 'none';

    const tbody = $('tbodyCuadrarImportes');
    if (tbody) {
      tbody.innerHTML = `
        <tr>
          <td colspan="9" style="text-align:center; padding: 36px; color: var(--notion-text-subtle);">
            <div class="spinner" style="display:inline-block; margin-right:8px; vertical-align:middle;"></div>
            Analizando consistencia de importes en ${allItems.length} comprobante(s)...
          </td>
        </tr>
      `;
    }

    $('cuadreFooterSummary').textContent = `Verificando ${allItems.length} comprobante(s)...`;
    modal.showModal();

    try {
      const res = await apiFetch('/api/sire/cuadre/detect', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ book: currentCuadreBook, items: allItems })
      });

      if (!res.ok) {
        let errMsg = `Error en servidor (${res.status})`;
        try {
          const errData = await res.json();
          if (errData && errData.error) errMsg = errData.error;
        } catch (_) {
          if (res.status === 404) {
            errMsg = 'El servicio de cuadre de importes no está disponible. Por favor reinicie el servidor appsire-server.exe.';
          }
        }
        throw new Error(errMsg);
      }

      const data = await res.json();
      if (!data.success) {
        throw new Error(data.error || 'Error al analizar cuadre de importes');
      }

      currentCuadreReport = data.report;
      currentCuadreItems = (data.report && data.report.items) ? data.report.items.map(it => ({ ...it })) : [];
      currentCuadreGlobalCol = data.report.columna_default || 'bi_gravada';

      // Poblar selector global de columnas
      const cboGlobal = $('cboCuadreGlobalCol');
      if (cboGlobal && data.report.columnas_disponibles) {
        cboGlobal.innerHTML = data.report.columnas_disponibles
          .map(c => `<option value="${escapeHtml(c.id)}" ${c.id === currentCuadreGlobalCol ? 'selected' : ''}>${escapeHtml(c.nombre)}</option>`)
          .join('');
      }

      // Actualizar contadores métricos
      $('cuadreMetricDescuadres').textContent = currentCuadreReport.total_descuadres || 0;
      $('cuadreMetricDiferenciaNeta').textContent = `S/ ${(currentCuadreReport.diferencia_neta || 0).toLocaleString('es-PE', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
      $('cuadreMetricExceso').textContent = currentCuadreReport.total_por_exceso || 0;
      $('cuadreMetricDefecto').textContent = currentCuadreReport.total_por_defecto || 0;

      $('cuadrePillCountAll').textContent = currentCuadreReport.total_descuadres || 0;
      $('cuadrePillCountExceso').textContent = currentCuadreReport.total_por_exceso || 0;
      $('cuadrePillCountDefecto').textContent = currentCuadreReport.total_por_defecto || 0;

      if (currentCuadreReport.total_descuadres === 0) {
        if (tbody) {
          tbody.innerHTML = `
            <tr>
              <td colspan="9" style="text-align:center; padding: 48px 20px; color: #047857;">
                <div style="font-size: 24px; margin-bottom: 8px;">✓</div>
                <div style="font-size: 14px; font-weight: 600;">Todos los importes están cuadrados</div>
                <div style="font-size: 12px; color: var(--notion-text-subtle); margin-top: 4px;">
                  Los ${allItems.length} comprobante(s) coinciden exactamente entre su Importe Total y la suma de sus componentes.
                </div>
              </td>
            </tr>
          `;
        }
        $('cuadreFooterSummary').innerHTML = `<span style="color:#047857; font-weight:600;">✓ Conforme: Todos los comprobantes (${allItems.length}) cuadran al 100%.</span>`;
        notifySuccess('Importes Cuadrados', 'Todos los comprobantes de la propuesta cuadran exactamente.');
      } else {
        renderCuadreRows();
        $('btnExportarExcelCuadre').style.display = 'inline-flex';
        btnAplicar.disabled = false;
        $('cuadreFooterSummary').innerHTML = `<span style="color:#b91c1c; font-weight:600;">⚠ Se detectaron ${currentCuadreReport.total_descuadres} comprobante(s) descuadrado(s) por un neto de S/ ${(currentCuadreReport.diferencia_neta || 0).toFixed(2)}.</span>`;
        notifyWarning(
          'Descuadre Detectado',
          `Se detectaron ${currentCuadreReport.total_descuadres} comprobante(s) con diferencias aritméticas respecto a su Importe Total.`
        );
      }
    } catch (err) {
      if (tbody) {
        tbody.innerHTML = `
          <tr>
            <td colspan="9" style="text-align:center; padding: 36px; color: #b91c1c;">
              Error al analizar consistencia: ${escapeHtml(err.message)}
            </td>
          </tr>
        `;
      }
      notifyError('Error en Cuadre', err.message);
    }
  }

  function renderCuadreRows() {
    const tbody = $('tbodyCuadrarImportes');
    if (!tbody) return;

    if (!currentCuadreItems.length) {
      tbody.innerHTML = `
        <tr>
          <td colspan="9" style="text-align:center; padding: 36px; color: var(--notion-text-subtle);">
            No hay comprobantes descuadrados.
          </td>
        </tr>
      `;
      return;
    }

    let filtered = currentCuadreItems;
    if (currentCuadreFilter === 'exceso') {
      filtered = filtered.filter(it => it.diferencia > 0);
    } else if (currentCuadreFilter === 'defecto') {
      filtered = filtered.filter(it => it.diferencia < 0);
    }

    if (currentCuadreSearchQuery) {
      const q = currentCuadreSearchQuery;
      filtered = filtered.filter(it =>
        (it.ruc && it.ruc.toLowerCase().includes(q)) ||
        (it.razon_social && it.razon_social.toLowerCase().includes(q)) ||
        (it.serie && it.serie.toLowerCase().includes(q)) ||
        (it.numero && it.numero.toLowerCase().includes(q)) ||
        (it.comp_pago && it.comp_pago.toLowerCase().includes(q))
      );
    }

    if (!filtered.length) {
      tbody.innerHTML = `
        <tr>
          <td colspan="9" style="text-align:center; padding: 36px; color: var(--notion-text-subtle);">
            No hay comprobantes que coincidan con los filtros aplicados.
          </td>
        </tr>
      `;
      return;
    }

    const cols = currentCuadreReport?.columnas_disponibles || [
      { id: 'bi_gravada', nombre: 'Base Imponible Gravada' }
    ];

    tbody.innerHTML = filtered.map((it) => {
      // Encontrar el índice original en currentCuadreItems
      const origItemIdx = currentCuadreItems.findIndex(x => x.index === it.index);
      const isExceso = it.diferencia > 0;
      const diffSign = isExceso ? `+${it.diferencia.toFixed(2)}` : it.diferencia.toFixed(2);
      const diffBadge = isExceso
        ? `<span class="cuadre-badge cuadre-diff-exceso">${diffSign}</span>`
        : `<span class="cuadre-badge cuadre-diff-defecto">${diffSign}</span>`;

      const optionsHtml = cols.map(c =>
        `<option value="${escapeHtml(c.id)}" ${c.id === it.columna_ajuste ? 'selected' : ''}>${escapeHtml(c.nombre)}</option>`
      ).join('');

      return `
        <tr>
          <td style="text-align: center; color: var(--notion-text-subtle);">${origItemIdx + 1}</td>
          <td>
            <div style="display: flex; align-items: center; gap: 4px;">
              <span class="cpe-tipo-tag">${escapeHtml(it.tipo || '01')}</span>
              <strong>${escapeHtml(it.serie ? `${it.serie}-${it.numero}` : it.comp_pago || '-')}</strong>
            </div>
          </td>
          <td style="text-align: center;">${escapeHtml(it.fecha || '-')}</td>
          <td>
            <div style="font-weight: 500; max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" title="${escapeHtml(it.razon_social)}">
              ${escapeHtml(it.razon_social || '-')}
            </div>
            <div style="font-size: 11px; color: var(--notion-text-subtle); font-family: monospace;">
              RUC: ${escapeHtml(it.ruc || '-')}
            </div>
          </td>
          <td style="text-align: right; font-family: monospace;">S/ ${(it.suma_componentes || 0).toFixed(2)}</td>
          <td style="text-align: right; font-family: monospace; font-weight: 600;">S/ ${(it.importe_total || 0).toFixed(2)}</td>
          <td style="text-align: center;">${diffBadge}</td>
          <td>
            <select class="form-control form-control-sm cuadre-row-select" data-cuadre-index="${origItemIdx}">
              ${optionsHtml}
            </select>
          </td>
          <td style="text-align: right; font-family: monospace;" id="cuadrePreview_${origItemIdx}">
            <span class="cuadre-val-prev">${(it.valor_actual || 0).toFixed(2)}</span> &rarr; <strong class="cuadre-val-next">${(it.valor_ajustado || 0).toFixed(2)}</strong>
          </td>
        </tr>
      `;
    }).join('');
  }

  async function applyCuadreAdjustments() {
    if (!currentCuadreItems || !currentCuadreItems.length) return;

    const count = currentCuadreItems.length;
    const isRce = currentCuadreBook === 'RCE';
    const confirmed = await notifyConfirm(
      '¿Cuadrar Importes?',
      `Se actualizarán los montos de ${count} comprobante(s) en la propuesta de ${isRce ? 'Compras (RCE)' : 'Ventas (RVIE)'}. La columna seleccionada absorberá la diferencia de redondeo y el Importe Total se mantendrá intacto.`,
      'Sí, aplicar cuadre'
    );
    if (!confirmed) return;

    const allItems = isRce ? currentProposalItemsRce : currentProposalItemsRvie;
    let applied = 0;

    for (const it of currentCuadreItems) {
      if (it.index >= 0 && it.index < allItems.length) {
        allItems[it.index] = it.item_recalculado;
        applied++;
      }
    }

    // Re-renderizar la grilla de propuesta en pantalla
    renderProposalTableGrid(isRce, allItems);

    // Cerrar modal
    $('modalCuadrarImportes')?.close();

    notifySuccess(
      'Importes Cuadrados',
      `Se cuadraron exitosamente ${applied} comprobante(s) en la propuesta activa.`
    );
  }

  function exportCuadreToExcel() {
    if (!currentCuadreItems.length) {
      notifyWarning('Sin Datos', 'No hay comprobantes descuadrados para exportar.');
      return;
    }

    const colNames = {};
    (currentCuadreReport?.columnas_disponibles || []).forEach(c => colNames[c.id] = c.nombre);

    const headers = [
      'N°',
      'Comprobante',
      'Tipo',
      'Serie',
      'Número',
      'Fecha Emisión',
      'RUC',
      'Razón Social',
      'Suma Componentes',
      'Importe Total',
      'Diferencia',
      'Tipo Descuadre',
      'Columna Ajustada',
      'Valor Anterior',
      'Valor Ajustado'
    ];

    const rows = currentCuadreItems.map((it, idx) => [
      idx + 1,
      `"${it.serie ? it.serie + '-' + it.numero : it.comp_pago || ''}"`,
      it.tipo || '',
      it.serie || '',
      it.numero || '',
      it.fecha || '',
      `="${it.ruc || ''}"`,
      `"${(it.razon_social || '').replace(/"/g, '""')}"`,
      (it.suma_componentes || 0).toFixed(2),
      (it.importe_total || 0).toFixed(2),
      (it.diferencia || 0).toFixed(2),
      it.diferencia > 0 ? 'Exceso (+)' : 'Defecto (-)',
      `"${colNames[it.columna_ajuste] || it.columna_ajuste}"`,
      (it.valor_actual || 0).toFixed(2),
      (it.valor_ajustado || 0).toFixed(2)
    ]);

    const csvContent = '\uFEFF' + [headers.join(','), ...rows.map(r => r.join(','))].join('\r\n');
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    const now = new Date();
    const dateStr = now.toISOString().slice(0, 10).replace(/-/g, '');
    a.download = `Cuadre_Importes_${currentCuadreBook}_${dateStr}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);

    notifySuccess('Reporte Descargado', 'Se descargó el reporte de cuadre de importes en formato compatible con Excel.');
  }

  // =========================================================
  // Módulo: Validación de Correlativos RVIE (SIRE)
  // =========================================================
  let currentCorrelReport = null;
  let currentCorrelItems = [];
  let currentCorrelSelectedIndices = new Set();
  let currentCorrelSerieFilter = 'all';
  let currentCorrelSearchQuery = '';

  function initCorrelEvents() {
    const modal = $('modalValidarCorrelativos');
    if (!modal) return;

    $('btnCerrarModalCorrelCross')?.addEventListener('click', () => modal.close());
    $('btnCancelarModalCorrel')?.addEventListener('click', () => modal.close());
    modal.addEventListener('click', (e) => {
      if (e.target === modal) modal.close();
    });

    // Filtro por serie
    $('cboCorrelSerieFilter')?.addEventListener('change', (e) => {
      currentCorrelSerieFilter = e.target.value;
      renderCorrelRows();
    });

    // Marcar / Desmarcar todos
    $('btnCorrelMarcarTodos')?.addEventListener('click', () => {
      currentCorrelSelectedIndices = new Set(currentCorrelItems.map((_, i) => i));
      updateCorrelCheckboxesUI();
    });

    $('btnCorrelDesmarcarTodos')?.addEventListener('click', () => {
      currentCorrelSelectedIndices.clear();
      updateCorrelCheckboxesUI();
    });

    // Master check
    $('correlMasterCheck')?.addEventListener('change', (e) => {
      const checked = e.target.checked;
      const visibleCheckboxes = document.querySelectorAll('#tbodyCorrelativos .correl-item-check');
      visibleCheckboxes.forEach((cb) => {
        const idx = parseInt(cb.dataset.correlIndex, 10);
        cb.checked = checked;
        if (checked) currentCorrelSelectedIndices.add(idx);
        else currentCorrelSelectedIndices.delete(idx);
      });
      updateCorrelButtonState();
    });

    // Check individual (delegado)
    $('tbodyCorrelativos')?.addEventListener('change', (e) => {
      if (!e.target.classList.contains('correl-item-check')) return;
      const idx = parseInt(e.target.dataset.correlIndex, 10);
      if (e.target.checked) currentCorrelSelectedIndices.add(idx);
      else currentCorrelSelectedIndices.delete(idx);

      // Actualizar master check
      const masterCheck = $('correlMasterCheck');
      const visibleCheckboxes = document.querySelectorAll('#tbodyCorrelativos .correl-item-check');
      if (masterCheck && visibleCheckboxes.length > 0) {
        masterCheck.checked = Array.from(visibleCheckboxes).every((cb) => cb.checked);
      }
      updateCorrelButtonState();
    });

    // Búsqueda
    $('txtCorrelSearch')?.addEventListener('input', (e) => {
      currentCorrelSearchQuery = (e.target.value || '').trim().toLowerCase();
      renderCorrelRows();
    });

    // Acciones principales
    $('btnCompletarModalCorrel')?.addEventListener('click', completeCorrelAsAnulado);
    $('btnExportarExcelCorrel')?.addEventListener('click', exportCorrelToExcel);
  }

  function updateCorrelCheckboxesUI() {
    const visibleCheckboxes = document.querySelectorAll('#tbodyCorrelativos .correl-item-check');
    visibleCheckboxes.forEach((cb) => {
      const idx = parseInt(cb.dataset.correlIndex, 10);
      cb.checked = currentCorrelSelectedIndices.has(idx);
    });
    const masterCheck = $('correlMasterCheck');
    if (masterCheck && visibleCheckboxes.length > 0) {
      masterCheck.checked = Array.from(visibleCheckboxes).every((cb) => cb.checked);
    }
    updateCorrelButtonState();
  }

  function updateCorrelButtonState() {
    const btn = $('btnCompletarModalCorrel');
    if (!btn) return;
    const count = currentCorrelSelectedIndices.size;
    btn.disabled = count === 0;
    const btnText = btn.querySelector('.btn-text');
    if (btnText) {
      btnText.textContent = count > 0
        ? `Completar ${count} Seleccionado(s) como ANULADO`
        : 'Completar Seleccionados como ANULADO';
    }
  }

  async function openCorrelModal(book) {
    if (book !== 'RVIE') {
      notifyWarning(
        'Validación de Ventas',
        'La validación de correlativos aplica exclusivamente a las VENTAS (RVIE), donde la empresa emisora está obligada a declarar numeración correlativa estricta sin saltos.'
      );
      return;
    }

    const allItems = currentProposalItemsRvie;
    if (!allItems || !allItems.length) {
      notifyWarning(
        'Sin Comprobantes',
        'Primero debes generar y previsualizar la propuesta de Ventas (RVIE).'
      );
      return;
    }

    const modal = $('modalValidarCorrelativos');
    if (!modal) return;

    // Resetear estado
    currentCorrelReport = null;
    currentCorrelItems = [];
    currentCorrelSelectedIndices = new Set();
    currentCorrelSerieFilter = 'all';
    currentCorrelSearchQuery = '';
    if ($('txtCorrelSearch')) $('txtCorrelSearch').value = '';

    $('correlMetricFaltantes').textContent = '0';
    $('correlMetricSeries').textContent = '0';
    $('correlMetricTotalSeries').textContent = '0';
    $('correlMetricTotalDocs').textContent = '0';

    $('btnExportarExcelCorrel').style.display = 'none';
    const btnCompletar = $('btnCompletarModalCorrel');
    if (btnCompletar) {
      btnCompletar.disabled = true;
      const btnText = btnCompletar.querySelector('.btn-text');
      if (btnText) btnText.textContent = 'Completar Seleccionados como ANULADO';
    }

    const tbody = $('tbodyCorrelativos');
    if (tbody) {
      tbody.innerHTML = `
        <tr>
          <td colspan="8" style="text-align:center; padding: 36px; color: var(--notion-text-subtle);">
            <div class="spinner" style="display:inline-block; margin-right:8px; vertical-align:middle;"></div>
            Analizando correlatividad de numeración en ${allItems.length} comprobante(s)...
          </td>
        </tr>
      `;
    }

    $('correlFooterSummary').textContent = `Auditando ${allItems.length} comprobante(s) de Ventas (RVIE)...`;
    modal.showModal();

    try {
      const res = await apiFetch('/api/sire/correlativos/detect', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ book: 'RVIE', items: allItems })
      });

      if (!res.ok) {
        let errMsg = `Error en servidor (${res.status})`;
        try {
          const errData = await res.json();
          if (errData && errData.error) errMsg = errData.error;
        } catch (_) {
          if (res.status === 404) {
            errMsg = 'El servicio de validación de correlativos no está disponible en este servidor. Por favor reinicie el servidor appsire-server.exe.';
          }
        }
        throw new Error(errMsg);
      }

      const data = await res.json();
      if (!data.success) {
        throw new Error(data.error || 'Error al analizar correlativos');
      }

      currentCorrelReport = data.report;
      currentCorrelItems = (data.report && data.report.faltantes) || [];
      currentCorrelSelectedIndices = new Set(currentCorrelItems.map((_, i) => i));

      // Actualizar contadores
      $('correlMetricFaltantes').textContent = currentCorrelReport.total_faltantes || 0;
      $('correlMetricSeries').textContent = currentCorrelReport.series_con_faltantes || 0;
      $('correlMetricTotalSeries').textContent = currentCorrelReport.total_series || 0;
      $('correlMetricTotalDocs').textContent = currentCorrelReport.total_documentos || 0;

      // Poblar selector de series
      const cboSeries = $('cboCorrelSerieFilter');
      if (cboSeries) {
        const seriesConHuecos = Array.from(new Set(currentCorrelItems.map(f => `${f.tipo} - ${f.serie}`))).sort();
        let opts = `<option value="all">Todas las series (${currentCorrelReport.series_con_faltantes || 0} con huecos)</option>`;
        seriesConHuecos.forEach(s => {
          opts += `<option value="${escapeHtml(s)}">${escapeHtml(s)}</option>`;
        });
        cboSeries.innerHTML = opts;
      }

      if (currentCorrelReport.total_faltantes === 0) {
        if (tbody) {
          tbody.innerHTML = `
            <tr>
              <td colspan="8" style="text-align:center; padding: 48px 20px; color: #047857;">
                <div style="font-size: 24px; margin-bottom: 8px;">✓</div>
                <div style="font-size: 14px; font-weight: 600;">Sin huecos en la numeración</div>
                <div style="font-size: 12px; color: var(--notion-text-subtle); margin-top: 4px;">
                  Se revisaron ${currentCorrelReport.total_series} series (${currentCorrelReport.total_documentos} documentos). Todas las series son correlativas y continuas.
                </div>
              </td>
            </tr>
          `;
        }
        $('correlFooterSummary').innerHTML = `<span style="color:#047857; font-weight:600;">✓ Conforme: Ninguna serie de ventas presenta saltos de numeración.</span>`;
        notifySuccess('Correlativos Conformes', 'Todas las series de ventas están correlativas y completas.');
      } else {
        renderCorrelRows();
        $('btnExportarExcelCorrel').style.display = 'inline-flex';
        updateCorrelButtonState();
        $('correlFooterSummary').innerHTML = `<span style="color:#b91c1c; font-weight:600;">⚠ Se detectaron ${currentCorrelReport.total_faltantes} correlativo(s) faltante(s) en ${currentCorrelReport.series_con_faltantes} serie(s).</span>`;
        notifyWarning(
          'Saltos de Correlativo Detectados',
          `Se detectaron ${currentCorrelReport.total_faltantes} comprobante(s) faltante(s) en la propuesta de Ventas. Puede completarlos como ANULADOS para evitar observaciones de SUNAT.`
        );
      }

      if (currentCorrelReport.advertencias && currentCorrelReport.advertencias.length > 0) {
        notifyWarning('Aviso de Correlativos', currentCorrelReport.advertencias.join(' '));
      }
    } catch (err) {
      if (tbody) {
        tbody.innerHTML = `
          <tr>
            <td colspan="8" style="text-align:center; padding: 36px; color: #b91c1c;">
              Error al analizar correlativos: ${escapeHtml(err.message)}
            </td>
          </tr>
        `;
      }
      notifyError('Error en Correlativos', err.message);
    }
  }

  function renderCorrelRows() {
    const tbody = $('tbodyCorrelativos');
    if (!tbody) return;

    if (!currentCorrelItems.length) {
      tbody.innerHTML = `
        <tr>
          <td colspan="8" style="text-align:center; padding: 36px; color: var(--notion-text-subtle);">
            No hay correlativos faltantes.
          </td>
        </tr>
      `;
      return;
    }

    let filtered = currentCorrelItems.map((item, originalIdx) => ({ item, originalIdx }));

    if (currentCorrelSerieFilter !== 'all') {
      filtered = filtered.filter(({ item }) => `${item.tipo} - ${item.serie}` === currentCorrelSerieFilter);
    }

    if (currentCorrelSearchQuery) {
      const q = currentCorrelSearchQuery;
      filtered = filtered.filter(({ item }) =>
        (item.serie && item.serie.toLowerCase().includes(q)) ||
        (item.numero && item.numero.toLowerCase().includes(q)) ||
        (item.tipo && item.tipo.toLowerCase().includes(q))
      );
    }

    if (!filtered.length) {
      tbody.innerHTML = `
        <tr>
          <td colspan="8" style="text-align:center; padding: 36px; color: var(--notion-text-subtle);">
            No hay correlativos faltantes que coincidan con los filtros aplicados.
          </td>
        </tr>
      `;
      return;
    }

    tbody.innerHTML = filtered.map(({ item, originalIdx }, rowNum) => {
      const isChecked = currentCorrelSelectedIndices.has(originalIdx);
      return `
        <tr>
          <td style="text-align: center;">
            <input type="checkbox" class="tc-item-check correl-item-check" data-correl-index="${originalIdx}" ${isChecked ? 'checked' : ''}>
          </td>
          <td style="text-align: center; color: var(--notion-text-subtle);">${rowNum + 1}</td>
          <td>
            <span class="cpe-tipo-tag">${escapeHtml(item.tipo || '01')}</span>
          </td>
          <td>
            <strong>${escapeHtml(item.serie)}</strong>
          </td>
          <td style="text-align: right; font-family: monospace; font-weight: 700; color: #b91c1c;">
            ${escapeHtml(item.numero)}
          </td>
          <td style="text-align: center;">${escapeHtml(item.fecha_referencia || '-')}</td>
          <td style="text-align: center;">
            <span class="correl-badge-anulado">ANULADO</span>
          </td>
          <td style="text-align: right; font-family: monospace; color: var(--notion-text-subtle);">
            S/ 0.00
          </td>
        </tr>
      `;
    }).join('');

    // Actualizar estado del master check
    const masterCheck = $('correlMasterCheck');
    const visibleCheckboxes = document.querySelectorAll('#tbodyCorrelativos .correl-item-check');
    if (masterCheck && visibleCheckboxes.length > 0) {
      masterCheck.checked = Array.from(visibleCheckboxes).every((cb) => cb.checked);
    }
  }

  async function completeCorrelAsAnulado() {
    if (currentCorrelSelectedIndices.size === 0) {
      notifyWarning('Sin Selección', 'Marque al menos un correlativo faltante para completar como ANULADO.');
      return;
    }

    const count = currentCorrelSelectedIndices.size;
    const confirmed = await notifyConfirm(
      '¿Completar como ANULADO?',
      `Se insertarán ${count} comprobante(s) en la propuesta de Ventas (RVIE) con estado ANULADO (importes en S/ 0.00, identidad 0 / 0001) para asegurar la correlatividad completa ante SUNAT.`,
      'Sí, completar correlativos'
    );
    if (!confirmed) return;

    // Obtener los ítems seleccionados
    const itemsToInsert = [];
    currentCorrelItems.forEach((it, idx) => {
      if (currentCorrelSelectedIndices.has(idx)) {
        itemsToInsert.push(it.item_propuesto);
      }
    });

    if (!itemsToInsert.length) return;

    // Agregar a la propuesta activa de Ventas
    currentProposalItemsRvie = currentProposalItemsRvie.concat(itemsToInsert);

    // Ordenar propuesta por TipoDoc, Serie y Número (numérico)
    currentProposalItemsRvie.sort((a, b) => {
      const tipoA = (a.tipo || '').trim();
      const tipoB = (b.tipo || '').trim();
      if (tipoA !== tipoB) return tipoA.localeCompare(tipoB);

      const serieA = (a.serie || '').trim();
      const serieB = (b.serie || '').trim();
      if (serieA !== serieB) return serieA.localeCompare(serieB);

      const numA = parseInt(a.numero || '0', 10) || 0;
      const numB = parseInt(b.numero || '0', 10) || 0;
      return numA - numB;
    });

    // Re-renderizar grilla principal
    renderProposalTableGrid(false, currentProposalItemsRvie);

    // Cerrar modal
    $('modalValidarCorrelativos')?.close();

    notifySuccess(
      'Correlativos Completados',
      `Se insertaron exitosamente ${count} comprobante(s) ANULADOS en la propuesta de Ventas (RVIE).`
    );
  }

  function exportCorrelToExcel() {
    if (!currentCorrelItems.length) {
      notifyWarning('Sin Datos', 'No hay correlativos faltantes para exportar.');
      return;
    }

    const headers = [
      'N°',
      'Tipo Doc.',
      'Serie',
      'Número Faltante',
      'Fecha Referencial',
      'Tipo Doc Identidad',
      'Nro Doc Identidad',
      'Razón Social Propuesta',
      'Importe Total'
    ];

    const rows = currentCorrelItems.map((it, idx) => [
      idx + 1,
      `"${it.tipo || ''}"`,
      `"${it.serie || ''}"`,
      `"${it.numero || ''}"`,
      it.fecha_referencia || '',
      `"0"`,
      `"0001"`,
      `"ANULADO"`,
      `0.00`
    ]);

    const csvContent = '\uFEFF' + [headers.join(','), ...rows.map(r => r.join(','))].join('\r\n');
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    const now = new Date();
    const dateStr = now.toISOString().slice(0, 10).replace(/-/g, '');
    a.download = `Correlativos_Faltantes_RVIE_${dateStr}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);

    notifySuccess('Reporte Descargado', 'Se descargó el reporte de correlativos faltantes en formato compatible con Excel.');
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
