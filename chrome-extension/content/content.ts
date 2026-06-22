// @ts-nocheck
import { detectCurrentSpace } from './space-detector';

// Log content script injection
console.log('[spacemosquito] Content script injected on', window.location.href);

// Listen for messages from popup/background
chrome.runtime.onMessage.addListener((msg: { type: string }, sender: chrome.runtime.MessageSender, sendResponse: (response: any) => void) => {
  switch (msg.type) {
    case 'get-space-info':
      const info = detectCurrentSpace();
      sendResponse(info);
      break;

    case 'capture-session':
      // Trigger cookie capture from content script context
      const spaceInfo = detectCurrentSpace();
      sendResponse({ spaceInfo, url: window.location.href });
      break;

    default:
      sendResponse({ error: 'unknown message type' });
  }
  return true; // async response
});
