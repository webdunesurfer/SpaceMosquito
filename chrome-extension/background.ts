import { ApiClient } from './lib/api';
import { captureAndSave } from './lib/session';
import { checkAuthStatus, AuthStatus } from './lib/auth';
import { SpaceInfo, JobSnapshot } from './lib/types';

const DEFAULT_BACKEND_URL = 'http://localhost:8081';

// Load settings from storage
async function getSettings(): Promise<{ backendUrl: string }> {
  const data: any = await chrome.storage.local.get('backend_url');
  return {
    backendUrl: data.backend_url || DEFAULT_BACKEND_URL,
  };
}

function isConfluenceUrl(url: string): boolean {
  if (!url) return false;
  const lowerUrl = url.toLowerCase();
  // Check for common Confluence path patterns
  return (
    lowerUrl.includes('atlassian.net') || 
    lowerUrl.includes('/wiki/spaces/') || 
    lowerUrl.includes('/spaces/') || 
    lowerUrl.includes('/display/')
  );
}

// Handle session capture flow
async function handleCaptureSession(tabUrl: string) {
  console.log('[spacemosquito] handleCaptureSession(tabUrl):', tabUrl);
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

    // Capture cookies and save
    const result = await captureAndSave(tabUrl, api);

    // Update session status
    if (result.success) {
      await chrome.storage.local.set({
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

    await chrome.storage.local.set({
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
    await chrome.storage.local.remove('active_crawl');
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
    await chrome.storage.local.set({ session_status: status });
    return status;
  } catch {
    return null;
  }
}

// Poll active crawl status
async function pollCrawlStatus() {
  try {
    const data: any = await chrome.storage.local.get('active_crawl');
    if (!data.active_crawl?.jobId) return null;

    const settings = await getSettings();
    const api = new ApiClient(settings.backendUrl);
    const job = await api.getCrawlStatus(data.active_crawl.jobId);

    if (job.status === 'completed' || job.status === 'failed' || job.status === 'cancelled') {
      await chrome.storage.local.set({ active_crawl_progress: job });
      await chrome.storage.local.remove('active_crawl');
    } else {
      await chrome.storage.local.set({ active_crawl_progress: job });
    }

    return job;
  } catch {
    return null;
  }
}

// Listen for messages from popup and content scripts
chrome.runtime.onMessage.addListener((msg: any, sender: any, sendResponse: (response: any) => void) => {
  switch (msg.type) {
    case 'capture-session':
      (async () => {
        let tabUrl: string | undefined;
        if (sender?.tab?.url) {
          tabUrl = sender.tab.url;
        } else {
          try {
            const windows: any[] = await chrome.windows.getAll({ populate: true });
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
            console.error('[spacemosquito] windows.getAll failed:', err);
          }
        }
        if (!tabUrl) {
          sendResponse({ success: false, error: 'No active tab' });
          return;
        }
        const result = await handleCaptureSession(tabUrl);
        sendResponse(result);
      })();
      return true;

    case 'start-crawl':
      handleStartCrawl(msg.spaceUrl).then(sendResponse).catch(err => {
        sendResponse({ success: false, error: err.message });
      });
      return true;

    case 'cancel-crawl':
      handleCancelCrawl(msg.jobId).then(sendResponse).catch(err => {
        sendResponse({ success: false, error: err.message });
      });
      return true;

    case 'get-space-info':
      (async () => {
        let tabUrl: string | undefined;
        if (sender?.tab?.url) {
          tabUrl = sender.tab.url;
        } else {
          try {
            const windows: any[] = await chrome.windows.getAll({ populate: true });
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
          sendResponse({ spaceKey: '', spaceName: '', spaceURL: '', pageTitle: '' });
          return;
        }
        const hostname = new URL(tabUrl).hostname;
        const protocol = new URL(tabUrl).protocol;
        
        // Match /wiki/spaces/KEY or /spaces/KEY or /display/KEY
        const match = tabUrl.match(/\/(?:wiki\/)?spaces\/([^/?#]+)/) || tabUrl.match(/\/display\/([^/?#]+)/);
        const spaceKey = match ? match[1] : '';
        const spaceName = spaceKey || 'Unknown';
        
        // Reconstruct space URL based on detected pattern
        let spaceURL = tabUrl;
        if (spaceKey) {
          if (tabUrl.includes('/wiki/spaces/')) {
            spaceURL = `${protocol}//${hostname}/wiki/spaces/${spaceKey}/overview`;
          } else if (tabUrl.includes('/spaces/')) {
            spaceURL = `${protocol}//${hostname}/spaces/${spaceKey}/overview`;
          } else if (tabUrl.includes('/display/')) {
            spaceURL = `${protocol}//${hostname}/display/${spaceKey}`;
          }
        }
        sendResponse({ spaceKey, spaceName, spaceURL, pageTitle: '' });
      })();
      return true;

    case 'get-settings':
      getSettings().then(sendResponse);
      return true;

    case 'validate-session':
      (async () => {
        const settings = await getSettings();
        const api = new ApiClient(settings.backendUrl);
        try {
          const result = await api.validateSession();
          await chrome.storage.local.set({ session_status: { ...result, exists: true } });
          sendResponse(result);
        } catch (error) {
          sendResponse({ valid: false, message: (error as Error).message });
        }
      })();
      return true;

    default:
      sendResponse({ error: 'unknown message type' });
      return false;
  }
});

// Periodic polling
setInterval(async () => {
  await pollSessionStatus();
  await pollCrawlStatus();
}, 30 * 1000);

// Initialize on install
chrome.runtime.onInstalled.addListener(async () => {
  const data: any = await chrome.storage.local.get('backend_url');
  if (!data.backend_url) {
    await chrome.storage.local.set({ backend_url: DEFAULT_BACKEND_URL });
  }
});
