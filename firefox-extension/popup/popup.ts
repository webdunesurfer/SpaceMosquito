// @ts-nocheck
// Firefox provides browser API natively
import { ApiClient } from '../lib/api';
import { CrawlJob, CrawlSpace, CronSpaceConfig, ExtensionSettings } from '../lib/types';

let api: ApiClient;
let activeJobId: string | null = null;
let pollInterval: number | null = null;
let currentSpaceInfo: { spaceKey: string; spaceName: string; spaceURL: string } | null = null;

// Initialize
async function init() {
  const settings = await getSettings();
  api = new ApiClient(settings.backend_url);

  setupTabs();
  
  // Run these in parallel and don't let one failure stop the others
  Promise.allSettled([
    loadSessionStatus(),
    detectCurrentSpace(),
    loadSpaces(),
    loadCronConfig()
  ]).catch(console.error);
}

async function getSettings(): Promise<ExtensionSettings & { backend_url: string }> {
  const data: any = await browser.storage.local.get('backend_url');
  return {
    backend_url: data.backend_url || 'http://localhost:8081',
    crawl_depth: 'all',
  };
}

// Tab switching
function setupTabs() {
  document.querySelectorAll('.tab-btn').forEach((btn: HTMLElement) => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.tab-btn').forEach(t => t.classList.remove('active'));
      btn.classList.add('active');

      document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
      const tabId = `tab-${btn.dataset.tab}`;
      document.getElementById(tabId)?.classList.add('active');
    });
  });
}

// Session tab
async function loadSessionStatus() {
  const dot = document.getElementById('session-dot');
  const statusText = document.getElementById('session-status-text') as HTMLParagraphElement;
  const captureBtn = document.getElementById('btn-capture') as HTMLButtonElement;
  const validateBtn = document.getElementById('btn-validate') as HTMLButtonElement;
  const deleteBtn = document.getElementById('btn-delete') as HTMLButtonElement;
  const sessionInfo = document.getElementById('session-info');

  try {
    const status = await api.getSessionStatus();

    if (status.exists) {
      dot?.classList.add('connected');
      dot?.classList.remove('connected', 'disconnected', 'checking');
      statusText.textContent = status.message || 'Session stored';
      if (status.valid) {
        captureBtn.classList.add('hidden');
      } else {
        captureBtn.classList.remove('hidden');
        captureBtn.textContent = 'Re-Capture Session';
      }
      validateBtn.classList.remove('hidden');
      deleteBtn.classList.remove('hidden');

      sessionInfo?.classList.remove('hidden');
      const data: any = await browser.storage.local.get(['session_cookie_count', 'session_captured_at']);
      if (data.session_cookie_count) {
        (document.getElementById('cookie-count') as HTMLSpanElement).textContent = String(data.session_cookie_count);
      }
      if (data.session_captured_at) {
        const date = new Date(data.session_captured_at);
        (document.getElementById('session-expires') as HTMLSpanElement).textContent = date.toLocaleString();
      }
    } else {
      dot?.classList.add('disconnected');
      dot?.classList.remove('connected', 'checking');
      statusText.textContent = status.message || 'No session';
      captureBtn.classList.remove('hidden');
      validateBtn.classList.add('hidden');
      deleteBtn.classList.add('hidden');
      sessionInfo?.classList.add('hidden');
    }
  } catch {
    dot?.classList.add('disconnected');
    dot?.classList.remove('connected', 'checking');
    statusText.textContent = 'Backend not reachable';
    captureBtn.classList.add('hidden');
    validateBtn.classList.add('hidden');
    deleteBtn.classList.add('hidden');
    sessionInfo?.classList.add('hidden');
  }
}

(document.getElementById('btn-capture') as HTMLButtonElement)?.addEventListener('click', async () => {
  console.log('[spacemosquito] popup: capture clicked');
  const btn = document.getElementById('btn-capture') as HTMLButtonElement;
  btn.textContent = 'Capturing...';
  btn.disabled = true;

  try {
    console.log('[spacemosquito] popup: sending capture-session message');
    const result: any = await browser.runtime.sendMessage({ type: 'capture-session' });
    console.log('[spacemosquito] popup: got response:', result);
    if (result?.success) {
      const statusText = document.getElementById('session-status-text') as HTMLParagraphElement;
      statusText.textContent = `Session captured (${result.cookieCount} cookies)`;
      (document.getElementById('cookie-count') as HTMLSpanElement).textContent = String(result.cookieCount);
      await loadSessionStatus();
    } else {
      alert(result?.error || 'Capture failed');
    }
  } catch (error) {
    alert('Capture failed: ' + (error as Error).message);
  } finally {
    btn.textContent = 'Capture Session';
    btn.disabled = false;
  }
});

(document.getElementById('btn-validate') as HTMLButtonElement)?.addEventListener('click', async () => {
  const btn = document.getElementById('btn-validate') as HTMLButtonElement;
  btn.textContent = 'Validating...';
  btn.disabled = true;

  try {
    const result: any = await browser.runtime.sendMessage({ type: 'validate-session' });
    const statusText = document.getElementById('session-status-text') as HTMLParagraphElement;
    if (result?.valid) {
      statusText.textContent = `Valid: ${result.message}`;
    } else {
      statusText.textContent = `Invalid: ${result.message}`;
    }
  } catch (error) {
    alert('Validation failed: ' + (error as Error).message);
  } finally {
    btn.textContent = 'Validate Session';
    btn.disabled = false;
  }
});

(document.getElementById('btn-delete') as HTMLButtonElement)?.addEventListener('click', async () => {
  if (!confirm('Delete stored session?')) return;
  try {
    await api.deleteSession();
    await loadSessionStatus();
  } catch (error) {
    alert('Delete failed: ' + (error as Error).message);
  }
});

// Crawl tab
async function detectCurrentSpace() {
  const nameEl = document.getElementById('current-space-name') as HTMLParagraphElement;

  try {
    const info: any = await browser.runtime.sendMessage({ type: 'get-space-info' });
    if (info?.spaceKey) {
      currentSpaceInfo = info;
      nameEl.textContent = `${info.spaceName} (${info.spaceKey})`;
    } else {
      currentSpaceInfo = null;
      nameEl.textContent = 'Not on a Confluence page';
    }
  } catch {
    currentSpaceInfo = null;
    nameEl.textContent = 'Detection failed';
  }
}

(document.getElementById('btn-start-crawl') as HTMLButtonElement)?.addEventListener('click', async () => {
  console.log('[spacemosquito] popup: start-crawl clicked');
  console.log('[spacemosquito] popup: currentSpaceInfo:', currentSpaceInfo);
  if (!currentSpaceInfo) {
    alert('Not on a Confluence page');
    return;
  }

  const btn = document.getElementById('btn-start-crawl') as HTMLButtonElement;
  btn.textContent = 'Starting...';
  btn.disabled = true;

  try {
    const payload = {
      type: 'start-crawl',
      spaceUrl: currentSpaceInfo.spaceURL,
    };
    console.log('[spacemosquito] popup: sending start-crawl with payload:', payload);
    const result: any = await browser.runtime.sendMessage(payload);
    console.log('[spacemosquito] popup: start-crawl result:', result);

    if (result?.success) {
      activeJobId = result.jobId;
      (document.getElementById('crawl-progress') as HTMLDivElement)?.classList.remove('hidden');
      (document.getElementById('btn-cancel-crawl') as HTMLButtonElement)?.classList.remove('hidden');
      startCrawlPolling();
    } else {
      alert(result?.error || 'Crawl failed');
    }
  } catch (error) {
    alert('Crawl failed: ' + (error as Error).message);
  } finally {
    btn.textContent = 'Crawl Current Space';
    btn.disabled = false;
  }
});

function startCrawlPolling() {
  if (pollInterval) clearInterval(pollInterval);

  pollInterval = window.setInterval(async () => {
    if (!activeJobId) return;

   try {
      console.log('[spacemosquito] crawl poll requesting:', api.getBackendUrl(), '/api/crawl/status?id=' + activeJobId);
      const job = await api.getCrawlStatus(activeJobId);
      console.log('[spacemosquito] crawl poll got:', job);
      updateCrawlProgress(job);

      if (job.status !== 'running') {
        clearInterval(pollInterval!);
        pollInterval = null;
        activeJobId = null;
        (document.getElementById('btn-cancel-crawl') as HTMLButtonElement)?.classList.add('hidden');
        (document.getElementById('btn-cleanup') as HTMLButtonElement)?.classList.remove('hidden');
      }
    } catch (error) {
      console.error('[spacemosquito] crawl poll error:', error, 'jobId:', activeJobId);
    }
  }, 3000);
}

function updateCrawlProgress(job: CrawlJob) {
  const percent = job.total_pages > 0 ? Math.round((job.completed / job.total_pages) * 100) : 0;

  const statusEl = document.getElementById('crawl-status') as HTMLSpanElement;
  const percentEl = document.getElementById('crawl-percent') as HTMLSpanElement;
  const fillEl = document.getElementById('progress-fill') as HTMLDivElement;
  const pagesEl = document.getElementById('crawl-pages') as HTMLSpanElement;
  const completedEl = document.getElementById('crawl-completed') as HTMLSpanElement;
  const failedEl = document.getElementById('crawl-failed') as HTMLSpanElement;

  if (statusEl) {
    statusEl.textContent =
      job.status === 'running' ? 'Running...' :
      job.status === 'completed' ? 'Completed' :
      job.status === 'failed' ? `Failed: ${job.error || 'unknown'}` :
      job.status;
  }
  if (percentEl) percentEl.textContent = `${percent}%`;
  if (fillEl) fillEl.style.width = `${percent}%`;
  if (pagesEl) pagesEl.textContent = `Page ${job.completed}/${job.total_pages}`;
  if (completedEl) completedEl.textContent = `Completed: ${job.completed}`;
  if (failedEl) failedEl.textContent = `Failed: ${job.failed}`;

  const errorEl = document.getElementById('crawl-error');
  if (job.error && job.status === 'failed' && errorEl) {
    errorEl.textContent = job.error;
    errorEl.classList.remove('hidden');
  }

  if (job.status === 'completed' && fillEl) {
    fillEl.style.background = 'var(--success)';
  } else if (job.status === 'failed' && fillEl) {
    fillEl.style.background = 'var(--danger)';
  }
}

(document.getElementById('btn-cancel-crawl') as HTMLButtonElement)?.addEventListener('click', async () => {
  if (!activeJobId) return;
  try {
    await browser.runtime.sendMessage({ type: 'cancel-crawl', jobId: activeJobId });
    (document.getElementById('crawl-progress') as HTMLDivElement)?.classList.add('hidden');
  } catch (error) {
    alert('Cancel failed: ' + (error as Error).message);
  }
});

(document.getElementById('btn-cleanup') as HTMLButtonElement)?.addEventListener('click', async () => {
  try {
    await api.cleanupCrawls();
    (document.getElementById('crawl-progress') as HTMLDivElement)?.classList.add('hidden');
    (document.getElementById('btn-cleanup') as HTMLButtonElement)?.classList.add('hidden');
  } catch (error) {
    alert('Cleanup failed: ' + (error as Error).message);
  }
});

// Settings tab
(document.getElementById('btn-save-url') as HTMLButtonElement)?.addEventListener('click', async () => {
  const input = document.getElementById('backend-url') as HTMLInputElement;
  const url = input.value.trim();
  if (!url) return;

  try {
    api.setBackendUrl(url);
    await browser.storage.local.set({ backend_url: url });
    alert('Backend URL saved');
  } catch (error) {
    alert('Save failed: ' + (error as Error).message);
  }
});

(document.getElementById('btn-add-space') as HTMLButtonElement)?.addEventListener('click', async () => {
  const input = document.getElementById('add-space-url') as HTMLInputElement;
  const url = input.value.trim();
  if (!url) return;

  try {
    await api.addSpace(url);
    input.value = '';
    await loadSpaces();
  } catch (error) {
    alert('Add failed: ' + (error as Error).message);
  }
});

async function loadSpaces() {
  const list = document.getElementById('spaces-list');
  const crawlAllBtn = document.getElementById('btn-crawl-all');

  if (!list || !crawlAllBtn) return;

  try {
    const spaces = await api.listSpaces();
    list.innerHTML = '';

    if (spaces.length > 0) {
      crawlAllBtn.classList.remove('hidden');

      spaces.forEach((space: CrawlSpace) => {
        const li = document.createElement('li');
        li.innerHTML = `
          <div class="space-item-info">
            <div class="space-item-key">${space.space_key}</div>
            <div class="space-item-meta">${space.pages_crawled} pages · ${space.last_crawled ? new Date(space.last_crawled).toLocaleDateString() : 'never'}</div>
          </div>
          <div class="space-item-actions">
            <button class="btn btn-secondary btn-small btn-crawl-space" data-key="${space.space_key}" data-url="${space.space_url}">Crawl</button>
            <button class="btn btn-secondary btn-small btn-manage-cron" data-key="${space.space_key}" data-url="${space.space_url}">⏱</button>
            <button class="btn btn-danger btn-small btn-remove-space" data-key="${space.space_key}">✕</button>
          </div>
        `;
        list.appendChild(li);
      });

      list.querySelectorAll('.btn-crawl-space').forEach((btn: HTMLElement) => {
        btn.addEventListener('click', async (e: Event) => {
          const key = (e.target as HTMLElement).dataset.key!;
          const url = (e.target as HTMLElement).dataset.url!;
          try {
            const result: any = await browser.runtime.sendMessage({ type: 'start-crawl', spaceUrl: url });
            if (result?.success) {
              activeJobId = result.jobId;
              document.querySelector('.tab-btn[data-tab="crawl"]')?.dispatchEvent(new Event('click'));
              (document.getElementById('crawl-progress') as HTMLDivElement)?.classList.remove('hidden');
              (document.getElementById('btn-cancel-crawl') as HTMLButtonElement)?.classList.remove('hidden');
              startCrawlPolling();
            }
          } catch (error) {
            alert('Crawl failed: ' + (error as Error).message);
          }
        });
      });

      list.querySelectorAll('.btn-manage-cron').forEach((btn: HTMLElement) => {
        btn.addEventListener('click', async (e: Event) => {
          const key = (e.target as HTMLElement).dataset.key!;
          const url = (e.target as HTMLElement).dataset.url!;
          showSpaceCronConfig(key, url);
        });
      });

      list.querySelectorAll('.btn-remove-space').forEach((btn: HTMLElement) => {
        btn.addEventListener('click', async (e: Event) => {
          const key = (e.target as HTMLElement).dataset.key!;
          if (!confirm(`Remove space ${key}?`)) return;
          try {
            await api.deleteSpace(key);
            await loadSpaces();
            await loadCronConfig();
          } catch (error) {
            alert('Remove failed: ' + (error as Error).message);
          }
        });
      });
    } else {
      crawlAllBtn.classList.add('hidden');
      list.innerHTML = '<li class="no-spaces">No spaces configured</li>';
    }
  } catch (error) {
    list.innerHTML = `<li class="crawl-error">Failed to load: ${(error as Error).message}</li>`;
  }
}

(document.getElementById('btn-crawl-all') as HTMLButtonElement)?.addEventListener('click', async () => {
  try {
    const spaces = await api.listSpaces();
    for (const space of spaces) {
      const result: any = await browser.runtime.sendMessage({ type: 'start-crawl', spaceUrl: space.space_url });
      if (result?.success) {
        activeJobId = result.jobId;
        document.querySelector('.tab-btn[data-tab="crawl"]')?.dispatchEvent(new Event('click'));
        (document.getElementById('crawl-progress') as HTMLDivElement)?.classList.remove('hidden');
        (document.getElementById('btn-cancel-crawl') as HTMLButtonElement)?.classList.remove('hidden');
        startCrawlPolling();
        await new Promise(r => setTimeout(r, 2000));
      }
    }
  } catch (error) {
    alert('Crawl all failed: ' + (error as Error).message);
  }
});

// Cron config management
async function loadCronConfig() {
  const container = document.getElementById('cron-configs');
  const saveBtn = document.getElementById('btn-save-cron');

  if (!container || !saveBtn) return;
  container.innerHTML = '';
  saveBtn.classList.add('hidden');

  try {
    const config = await api.getCronConfig();
    const overrides = config.per_space_overrides;

    if (overrides.length === 0) {
      container.innerHTML = '<p class="info-text">No per-space cron configs. Click the ⏱ button on a space to configure.</p>';
      return;
    }

    overrides.forEach((ov: CronSpaceConfig) => {
      const div = document.createElement('div');
      div.className = 'cron-item';
      div.innerHTML = `
        <div class="cron-item-header">
          <span class="cron-item-key">${ov.space_key}</span>
          <button class="btn btn-danger btn-small btn-remove-cron" data-key="${ov.space_key}">✕</button>
        </div>
        <div class="cron-fields">
          <div class="cron-field">
            <label>Full Crawl</label>
            <select data-key="${ov.space_key}" data-field="full_crawl_enabled">
              <option value="true" ${ov.full_crawl_enabled ? 'selected' : ''}>Enabled</option>
              <option value="false" ${!ov.full_crawl_enabled ? 'selected' : ''}>Disabled</option>
            </select>
          </div>
          <div class="cron-field">
            <label>Full Interval</label>
            <select data-key="${ov.space_key}" data-field="full_crawl_interval">
              <option value="1h" ${ov.full_crawl_interval === '1h' ? 'selected' : ''}>1 hour</option>
              <option value="6h" ${ov.full_crawl_interval === '6h' ? 'selected' : ''}>6 hours</option>
              <option value="12h" ${ov.full_crawl_interval === '12h' ? 'selected' : ''}>12 hours</option>
              <option value="24h" ${ov.full_crawl_interval === '24h' ? 'selected' : ''}>24 hours</option>
              <option value="48h" ${ov.full_crawl_interval === '48h' ? 'selected' : ''}>48 hours</option>
              <option value="7d" ${ov.full_crawl_interval === '7d' ? 'selected' : ''}>7 days</option>
            </select>
          </div>
          <div class="cron-field">
            <label>Incremental</label>
            <select data-key="${ov.space_key}" data-field="incr_crawl_enabled">
              <option value="true" ${ov.incr_crawl_enabled ? 'selected' : ''}>Enabled</option>
              <option value="false" ${!ov.incr_crawl_enabled ? 'selected' : ''}>Disabled</option>
            </select>
          </div>
          <div class="cron-field">
            <label>Incr Interval</label>
            <select data-key="${ov.space_key}" data-field="incr_crawl_interval">
              <option value="30m" ${ov.incr_crawl_interval === '30m' ? 'selected' : ''}>30 min</option>
              <option value="1h" ${ov.incr_crawl_interval === '1h' ? 'selected' : ''}>1 hour</option>
              <option value="2h" ${ov.incr_crawl_interval === '2h' ? 'selected' : ''}>2 hours</option>
              <option value="4h" ${ov.incr_crawl_interval === '4h' ? 'selected' : ''}>4 hours</option>
              <option value="6h" ${ov.incr_crawl_interval === '6h' ? 'selected' : ''}>6 hours</option>
              <option value="12h" ${ov.incr_crawl_interval === '12h' ? 'selected' : ''}>12 hours</option>
            </select>
          </div>
        </div>
      `;
      container.appendChild(div);
    });

    saveBtn.classList.remove('hidden');

    container.querySelectorAll('.btn-remove-cron').forEach((btn: HTMLElement) => {
      btn.addEventListener('click', async (e: Event) => {
        const key = (e.target as HTMLElement).dataset.key!;
        if (!confirm(`Remove cron config for ${key}?`)) return;
        try {
          await api.deleteSpaceCron(key);
          await loadCronConfig();
        } catch (error) {
          alert('Delete failed: ' + (error as Error).message);
        }
      });
    });

  } catch (error) {
    container.innerHTML = `<p class="info-text">Failed to load: ${(error as Error).message}</p>`;
  }
}

function showSpaceCronConfig(spaceKey: string, spaceURL: string) {
  const container = document.getElementById('cron-configs');
  const saveBtn = document.getElementById('btn-save-cron');
  if (!container || !saveBtn) return;

  container.innerHTML = '';
  saveBtn.classList.add('hidden');

  const div = document.createElement('div');
  div.className = 'cron-item';
  div.innerHTML = `
    <div class="cron-item-header">
      <span class="cron-item-key">${spaceKey}</span>
      <span class="info-text" style="font-size:10px; color: var(--text-muted);">Editing this space</span>
    </div>
    <div class="cron-fields">
      <div class="cron-field">
        <label>Full Crawl</label>
        <select data-key="${spaceKey}" data-field="full_crawl_enabled">
          <option value="true">Enabled</option>
          <option value="false">Disabled</option>
        </select>
      </div>
      <div class="cron-field">
        <label>Full Interval</label>
        <select data-key="${spaceKey}" data-field="full_crawl_interval">
          <option value="1h">1 hour</option>
          <option value="6h">6 hours</option>
          <option value="12h">12 hours</option>
          <option value="24h" selected>24 hours</option>
          <option value="48h">48 hours</option>
          <option value="7d">7 days</option>
        </select>
      </div>
      <div class="cron-field">
        <label>Incremental</label>
        <select data-key="${spaceKey}" data-field="incr_crawl_enabled">
          <option value="true">Enabled</option>
          <option value="false">Disabled</option>
        </select>
      </div>
      <div class="cron-field">
        <label>Incr Interval</label>
        <select data-key="${spaceKey}" data-field="incr_crawl_interval">
          <option value="30m">30 min</option>
          <option value="1h" selected>1 hour</option>
          <option value="2h">2 hours</option>
          <option value="4h">4 hours</option>
          <option value="6h">6 hours</option>
          <option value="12h">12 hours</option>
        </select>
      </div>
    </div>
  `;
  container.appendChild(div);
  saveBtn.classList.remove('hidden');
  saveBtn.textContent = 'Save & Reload';

  const spacesList = document.getElementById('spaces-list');
  spacesList?.querySelectorAll('li').forEach(li => {
    if (li.querySelector(`[data-key="${spaceKey}"]`)) {
      li.classList.add('hidden');
    }
  });
}

(document.getElementById('btn-save-cron') as HTMLButtonElement)?.addEventListener('click', async () => {
  const btn = document.getElementById('btn-save-cron') as HTMLButtonElement;
  btn.textContent = 'Saving...';
  btn.disabled = true;

  try {
    const changes = new Map<string, Partial<CronSpaceConfig>>();
    const selects = document.querySelectorAll('#cron-configs select[data-key]');
    selects.forEach((sel: HTMLSelectElement) => {
      const key = sel.dataset.key!;
      const field = sel.dataset.field!;
      const value = sel.value;

      if (!changes.has(key)) {
        changes.set(key, { space_key: key });
      }
      const change = changes.get(key)!;

      if (field === 'full_crawl_enabled') change.full_crawl_enabled = value === 'true';
      if (field === 'full_crawl_interval') change.full_crawl_interval = value;
      if (field === 'incr_crawl_enabled') change.incr_crawl_enabled = value === 'true';
      if (field === 'incr_crawl_interval') change.incr_crawl_interval = value;
    });

    const promises: Promise<any>[] = [];
    for (const [key, change] of changes) {
      promises.push(api.updateSpaceCron(key, change));
    }
    await Promise.all(promises);

    await api.reloadCron();

    alert('Cron config saved and scheduler reloaded');
    await loadCronConfig();
    await loadSpaces();
  } catch (error) {
    alert('Save failed: ' + (error as Error).message);
  } finally {
    btn.textContent = 'Save & Reload Cron';
    btn.disabled = false;
  }
});

// Initialize on load
init();
