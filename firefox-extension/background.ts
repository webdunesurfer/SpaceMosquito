// @ts-nocheck
import browser from 'webextension-polyfill';
import { ApiClient } from './lib/api';
import { captureAndSave } from './lib/session';
import { checkAuthStatus, AuthStatus } from './lib/auth';
import { SpaceInfo, JobSnapshot } from './lib/types';

const DEFAULT_BACKEND_URL = 'http://localhost:8081';

// Load settings from storage
async function getSettings(): Promise<{ backendUrl: string }> {
  const data: any = await browser.storage.local.get('backend_url');
  return {
    backendUrl: data.backend_url || DEFAULT_BACKEND_URL,
  };
}

// Handle session capture flow
async function handleCaptureSession() {
  try {
    const settings = await getSettings();
    const api = new ApiClient(settings.backendUrl);

    // Get active tab
    const tabs: any[] = await browser.tabs.query({ active: true, currentWindow: true });
    if (!tabs[0] || !tabs[0].url) {
      return { success: false, error: 'No active tab' };
    }

    // Check if on Confluence
    if (!tabs[0].url.includes('atlassian.net')) {
      return { success: false, error: 'Not on a Confluence page' };
    }

    // Capture cookies and save
    const result = await captureAndSave(tabs[0].url, api);

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
      handleCaptureSession().then(sendResponse);
      return true;

    case 'start-crawl':
      handleStartCrawl(msg.spaceUrl).then(sendResponse);
      return true;

    case 'cancel-crawl':
      handleCancelCrawl(msg.jobId).then(sendResponse);
      return true;

    case 'get-space-info':
      browser.tabs.query({ active: true, currentWindow: true }).then((tabs: any[]) => {
        if (tabs[0]?.id) {
          browser.tabs.sendMessage(tabs[0].id, { type: 'get-space-info' }).then(sendResponse);
        }
      });
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
