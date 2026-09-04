// @ts-nocheck
// Firefox provides browser API natively
import { ApiClient } from './lib/api';
import { captureAndSave } from './lib/session';
import { checkAuthStatus, AuthStatus } from './lib/auth';
import { SpaceInfo, JobSnapshot } from './lib/types';

const DEFAULT_BACKEND_URL = 'http://localhost:8081';

function isConfluenceUrl(url: string): boolean {
  if (!url) return false;
  const lowerUrl = url.toLowerCase();
  return (
    lowerUrl.includes('atlassian.net') ||
    lowerUrl.includes('/wiki/spaces/') ||
    lowerUrl.includes('/spaces/') ||
    lowerUrl.includes('/display/') ||
    lowerUrl.includes('/pages/viewpage.action') ||
    lowerUrl.includes('/pages/viewspace.action')
  );
}

// Load settings from storage
async function getSettings(): Promise<{ backendUrl: string }> {
  const data: any = await browser.storage.local.get('backend_url');
  return {
    backendUrl: data.backend_url || DEFAULT_BACKEND_URL,
  };
}

// Handle session capture flow
async function handleCaptureSession(tabUrl: string, cookieStoreId?: string) {
  console.log('[spacemosquito] handleCaptureSession(tabUrl):', tabUrl, 'cookieStoreId:', cookieStoreId);
  try {
    const settings = await getSettings();
    console.log('[spacemosquito] settings:', settings);
    const api = new ApiClient(settings.backendUrl);

    if (!tabUrl) {
      return { success: false, error: 'Empty tab URL' };
    }
    if (!isConfluenceUrl(tabUrl)) {
      return { success: false, error: 'Not on a Confluence page' };
    }

    // Capture cookies and save (cookieStoreId = Multi-Account Container jar)
    const result = await captureAndSave(tabUrl, api, cookieStoreId);

    // Update session status
    if (result.success) {
      await browser.storage.local.set({
        session_captured: true,
        session_captured_at: Date.now(),
        session_cookie_count: result.cookieCount,
      });
    }

    return result;
  } catch (error) {
    console.error('[spacemosquito] Session capture failed:', error);
    return { success: false, error: (error as Error).message };
  }
}

// Handle crawl start
async function handleStartCrawl(spaceUrl: string) {
  try {
    const settings = await getSettings();
    const api = new ApiClient(settings.backendUrl);

    const result = await api.startCrawl(spaceUrl);

    await browser.storage.local.set({
      active_crawl: {
        jobId: result.job_id,
        spaceUrl,
        startedAt: Date.now(),
      },
    });

    return { success: true, jobId: result.job_id };
  } catch (error) {
    console.error('[spacemosquito] Crawl start failed:', error);
    return { success: false, error: (error as Error).message };
  }
}

// Handle crawl cancel
async function handleCancelCrawl(jobId: string) {
  try {
    const settings = await getSettings();
    const api = new ApiClient(settings.backendUrl);
    await api.cancelCrawl(jobId);
    await browser.storage.local.remove('active_crawl');
    return { success: true };
  } catch (error) {
    console.error('[spacemosquito] Crawl cancel failed:', error);
    return { success: false, error: (error as Error).message };
  }
}

// Poll session status periodically
async function pollSessionStatus() {
  try {
    const settings = await getSettings();
    const api = new ApiClient(settings.backendUrl);
    const status = await api.getSessionStatus();
    await browser.storage.local.set({ session_status: status });
    return status;
  } catch {
    return null;
  }
}

// Poll active crawl status
async function pollCrawlStatus() {
  try {
    const data: any = await browser.storage.local.get('active_crawl');
    if (!data.active_crawl?.jobId) return null;

    const settings = await getSettings();
    const api = new ApiClient(settings.backendUrl);
    const job = await api.getCrawlStatus(data.active_crawl.jobId);

    if (job.status === 'completed' || job.status === 'failed' || job.status === 'cancelled') {
      await browser.storage.local.set({ active_crawl_progress: job });
      await browser.storage.local.remove('active_crawl');
    } else {
      await browser.storage.local.set({ active_crawl_progress: job });
    }

    return job;
  } catch {
    return null;
  }
}

// Listen for messages from popup and content scripts
browser.runtime.onMessage.addListener((msg: any, sender: any, sendResponse: (response: any) => void) => {
  switch (msg.type) {
    case 'capture-session':
      console.log('[spacemosquito] capture-session received');
      return new Promise(resolve => {
        (async () => {
          let tabUrl: string | undefined;
          let cookieStoreId: string | undefined;
          console.log('[spacemosquito] sender.tab:', sender?.tab);
          if (sender?.tab?.url) {
            tabUrl = sender.tab.url;
            cookieStoreId = sender.tab.cookieStoreId;
          } else {
            console.log('[spacemosquito] No sender.tab, querying windows...');
            try {
              const tabs: any[] = await browser.tabs.query({ active: true, currentWindow: true });
              const tab = tabs[0];
              if (tab?.url) {
                tabUrl = tab.url;
                cookieStoreId = tab.cookieStoreId;
                console.log('[spacemosquito] Found active tab:', tabUrl, 'store:', cookieStoreId);
              } else {
                const windows: any[] = await browser.windows.getAll({ populate: true });
                console.log('[spacemosquito] Found', windows.length, 'windows');
                for (const w of windows) {
                  for (const t of w.tabs || []) {
                    if (t.active && t.url) {
                      tabUrl = t.url;
                      cookieStoreId = t.cookieStoreId;
                      console.log('[spacemosquito] Found active tab:', tabUrl, 'store:', cookieStoreId);
                      break;
                    }
                  }
                  if (tabUrl) break;
                }
              }
            } catch (err) {
              console.error('[spacemosquito] tab lookup failed:', err);
            }
          }
          if (!tabUrl) {
            console.error('[spacemosquito] No active tab URL found');
            resolve({ success: false, error: 'No active tab' });
            return;
          }
          const result = await handleCaptureSession(tabUrl, cookieStoreId);
          console.log('[spacemosquito] capture result:', result);
          resolve(result);
        })().catch(err => {
          console.error('[spacemosquito] capture error:', err);
          resolve({ success: false, error: err.message });
        });
      });

    case 'start-crawl':
      console.log('[spacemosquito] start-crawl received, spaceUrl:', msg.spaceUrl);
      return new Promise(resolve => {
        handleStartCrawl(msg.spaceUrl).then(result => {
          console.log('[spacemosquito] start-crawl result:', result);
          resolve(result);
        }).catch(err => {
          console.error('[spacemosquito] start-crawl error:', err);
          resolve({ success: false, error: err.message });
        });
      });

    case 'cancel-crawl':
      console.log('[spacemosquito] cancel-crawl received, jobId:', msg.jobId);
      return new Promise(resolve => {
        handleCancelCrawl(msg.jobId).then(result => {
          console.log('[spacemosquito] cancel-crawl result:', result);
          resolve(result);
        }).catch(err => {
          console.error('[spacemosquito] cancel-crawl error:', err);
          resolve({ success: false, error: err.message });
        });
      });

    case 'get-space-info':
      console.log('[spacemosquito] get-space-info received');
      return new Promise(resolve => {
        (async () => {
          let tabUrl: string | undefined;
          if (sender?.tab?.url) {
            tabUrl = sender.tab.url;
          } else {
            try {
              const windows: any[] = await browser.windows.getAll({ populate: true });
              for (const w of windows) {
                for (const t of w.tabs || []) {
                  if (t.active && t.url) {
                    tabUrl = t.url;
                    break;
                  }
                }
                if (tabUrl) break;
              }
            } catch (err) {
              console.error('[spacemosquito] get-space-info windows.getAll failed:', err);
            }
          }
          if (!tabUrl || !isConfluenceUrl(tabUrl)) {
            console.log('[spacemosquito] get-space-info: not on Confluence');
            resolve({ spaceKey: '', spaceName: '', spaceURL: '', pageTitle: '' });
            return;
          }
          const hostname = new URL(tabUrl).hostname;
          const protocol = new URL(tabUrl).protocol;
          const parsed = new URL(tabUrl);

          // Match /wiki/spaces/KEY or /spaces/KEY or /display/KEY or ?spaceKey=
          const match = tabUrl.match(/\/(?:wiki\/)?spaces\/([^/?#]+)/) || tabUrl.match(/\/display\/([^/?#]+)/);
          const spaceKey = match ? match[1] : (parsed.searchParams.get('spaceKey') || '');
          const spaceName = spaceKey || 'Unknown';

          let spaceURL = tabUrl;
          if (spaceKey) {
            if (tabUrl.includes('/wiki/spaces/')) {
              spaceURL = `${protocol}//${hostname}/wiki/spaces/${spaceKey}/overview`;
            } else if (tabUrl.includes('/spaces/')) {
              spaceURL = `${protocol}//${hostname}/spaces/${spaceKey}/overview`;
            } else if (tabUrl.includes('/display/')) {
              spaceURL = `${protocol}//${hostname}/display/${spaceKey}`;
            } else if (/\/pages\/view(?:page|space)\.action/i.test(tabUrl)) {
              const path = parsed.pathname;
              const i = path.toLowerCase().indexOf('/pages/');
              const ctx = i >= 0 ? path.slice(0, i) : '';
              spaceURL = `${protocol}//${hostname}${ctx}/display/${spaceKey}`;
            }
          }
          const info = { spaceKey, spaceName, spaceURL, pageTitle: '' };
          console.log('[spacemosquito] get-space-info:', info);
          resolve(info);
        })().catch(err => {
          console.error('[spacemosquito] get-space-info error:', err);
          resolve({ spaceKey: '', spaceName: '', spaceURL: '', pageTitle: '' });
        });
      });

    case 'get-settings':
      getSettings().then(sendResponse);
      return true;

    case 'validate-session':
      (async () => {
        const settings = await getSettings();
        const api = new ApiClient(settings.backendUrl);
        try {
          const result = await api.validateSession();
          await browser.storage.local.set({ session_status: { ...result, exists: true } });
          sendResponse(result);
        } catch (error) {
          sendResponse({ valid: false, message: (error as Error).message });
        }
      })();
      return true;

    default:
      sendResponse({ error: 'unknown message type' });
  }
});

// Periodic polling
setInterval(async () => {
  await pollSessionStatus();
  await pollCrawlStatus();
}, 30 * 1000);

// Initialize on install
browser.runtime.onInstalled.addListener(async () => {
  const data: any = await browser.storage.local.get('backend_url');
  if (!data.backend_url) {
    await browser.storage.local.set({ backend_url: DEFAULT_BACKEND_URL });
  }
});
