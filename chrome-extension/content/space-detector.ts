// @ts-nocheck
export function detectCurrentSpace(): {
  spaceKey: string;
  spaceName: string;
  spaceURL: string;
  pageTitle: string;
} {
  const url = window.location.href;
  const hostname = window.location.hostname;

  // Extract space key from URL
  const match = url.match(/\/wiki\/spaces\/([^/]+)/);
  const spaceKey = match ? match[1] : '';

  // Try to extract space name from DOM
  const selectors = [
    '[data-testid="confluence-space-name"]',
    '.aui-navgroup-label',
    '.header-section h1',
    '.breadcrumbs span:first-of-type',
    '.breadcrumb-header span',
    '[data-testid="sidebar.nav.group.name"]',
  ];

  let spaceName = spaceKey;
  for (const selector of selectors) {
    const el = document.querySelector(selector);
    if (el && el.textContent) {
      const text = el.textContent.trim();
      if (text && text !== spaceKey) {
        spaceName = text;
        break;
      }
    }
  }

  // Build space overview URL
  const spaceURL = spaceKey
    ? `https://${hostname}/wiki/spaces/${spaceKey}/overview`
    : url;

  return { spaceKey, spaceName, spaceURL, pageTitle: document.title };
}

// Listen for messages from popup/background
chrome.runtime.onMessage.addListener((msg: { type: string }, sender: chrome.runtime.MessageSender, sendResponse: (response: any) => void) => {
  if (msg.type === 'get-space-info') {
    const info = detectCurrentSpace();
    sendResponse(info);
  }
  return true; // async response
});

// Also expose for direct calling
(window as any).__spacemosquito = {
  detectSpace: detectCurrentSpace,
};
