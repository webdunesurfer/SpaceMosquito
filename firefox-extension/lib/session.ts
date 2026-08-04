// @ts-nocheck
// Firefox provides browser API natively
import type { Cookie } from './types';

const ATLASIAN_COOKIE_PATTERNS = [
  'session', 'token', 'sso', 'atlassian', 'aui',
  'seraph', 'crowd', 'remember', 'jsession',
];

function shouldKeepCookie(name: string): boolean {
  const lower = name.toLowerCase();
  return ATLASIAN_COOKIE_PATTERNS.some(p => lower.includes(p));
}

function normalizeSameSite(value?: string): string {
  if (!value) return 'Lax';
  if (value === 'no_restriction' || value === 'none') return 'None';
  if (value === 'unspecified' || value === 'lax') return 'Lax';
  if (value === 'strict') return 'Strict';
  return value;
}

function cookieKey(c: any): string {
  return `${c.storeId || ''}|${c.name}|${c.domain}|${c.path}|${JSON.stringify(c.partitionKey || null)}`;
}

function domainVariants(hostname: string): string[] {
  const out = new Set<string>([hostname, '.' + hostname]);
  const parts = hostname.split('.');
  // wiki.example.net → also example.net / .example.net (SSO cookies often land on parent)
  if (parts.length >= 3) {
    const parent = parts.slice(1).join('.');
    out.add(parent);
    out.add('.' + parent);
  }
  return Array.from(out);
}

/**
 * Prefer the tab's cookieStoreId (Multi-Account Containers use
 * firefox-container-N). Also scan other stores / partitions as fallback.
 */
async function getAllCookiesForTab(tabUrl: string, preferredStoreId?: string): Promise<any[]> {
  const hostname = new URL(tabUrl).hostname;
  const domains = domainVariants(hostname);

  let storeIds: string[] = [];
  try {
    const stores: any[] = await browser.cookies.getAllCookieStores();
    storeIds = stores.map((s: any) => s.id).filter(Boolean);
  } catch (err) {
    console.warn('[spacemosquito] getAllCookieStores failed', err);
  }
  if (preferredStoreId && !storeIds.includes(preferredStoreId)) {
    storeIds.unshift(preferredStoreId);
  } else if (preferredStoreId) {
    storeIds = [preferredStoreId, ...storeIds.filter((id) => id !== preferredStoreId)];
  }
  if (storeIds.length === 0) {
    storeIds = preferredStoreId ? [preferredStoreId] : [];
  }

  console.log('[spacemosquito] cookie stores to query', { preferredStoreId, storeIds });

  const queries: Record<string, unknown>[] = [];
  const storeBases: Record<string, unknown>[] =
    storeIds.length > 0 ? storeIds.map((storeId) => ({ storeId })) : [{}];

  for (const base of storeBases) {
    queries.push(
      { ...base, url: tabUrl, partitionKey: {}, firstPartyDomain: null },
      { ...base, url: tabUrl, partitionKey: {} },
      { ...base, url: tabUrl, firstPartyDomain: null },
      { ...base, url: tabUrl },
    );
    for (const domain of domains) {
      queries.push(
        { ...base, domain, partitionKey: {}, firstPartyDomain: null },
        { ...base, domain, partitionKey: {} },
        { ...base, domain, firstPartyDomain: null },
        { ...base, domain },
      );
    }
  }

  const byKey = new Map<string, any>();
  for (const details of queries) {
    try {
      const batch: any[] = await browser.cookies.getAll(details);
      for (const c of batch) {
        if (c?.name) byKey.set(cookieKey(c), c);
      }
    } catch (err) {
      console.warn('[spacemosquito] cookies.getAll skipped', details, err);
    }
  }
  return Array.from(byKey.values());
}

function emptyCookieError(rawNames: string[], cookieStoreId?: string): string {
  const sample = rawNames.slice(0, 12).join(', ') || '(none)';

  let msg =
    `No Confluence session cookies found. Firefox saw: ${sample}. ` +
    `Need names like JSESSIONID, seraph.confluence, crowd.token_key, or tenant.session.token — ` +
    `idp_last_account is only an IdP hint and cannot authenticate.`;

  if (cookieStoreId && cookieStoreId !== 'firefox-default') {
    msg +=
      ` This tab uses Multi-Account Container store "${cookieStoreId}". ` +
      `Reload the extension after updating (needs contextualIdentities), stay logged into Confluence in that container, then capture again.`;
  } else if (rawNames.includes('idp_last_account')) {
    msg +=
      ` If the wiki is in a Multi-Account Container, open the popup from that tab while logged in; ` +
      `or capture from a normal tab.`;
  }

  return msg;
}

export async function captureCookies(
  tabUrl: string,
  cookieStoreId?: string,
): Promise<{ cookies: Cookie[]; rawCount: number; rawNames: string[] }> {
  try {
    const domain = new URL(tabUrl).hostname;
    const rawCookies = await getAllCookiesForTab(tabUrl, cookieStoreId);
    const rawNames = [...new Set(rawCookies.map((c: any) => c.name).filter(Boolean))];
    const filtered = rawCookies.filter((c: any) => c.name && shouldKeepCookie(c.name));

    console.log('[spacemosquito] cookie capture', {
      tabUrl,
      domain,
      cookieStoreId,
      rawCount: rawCookies.length,
      rawNames,
      keptCount: filtered.length,
      keptNames: filtered.map((c: any) => c.name),
    });

    const cookies = filtered.map((c: any) => ({
      name: c.name,
      value: c.value,
      domain: c.domain || domain,
      path: c.path || '/',
      expires: c.expirationDate || c.expiry || undefined,
      secure: c.secure || false,
      httpOnly: c.httpOnly || false,
      sameSite: normalizeSameSite(c.sameSite),
    }));

    return { cookies, rawCount: rawCookies.length, rawNames };
  } catch (error) {
    console.error('[spacemosquito] Failed to capture cookies:', error);
    throw error;
  }
}

export async function captureAndSave(
  confluenceUrl: string,
  apiClient: any,
  cookieStoreId?: string,
): Promise<{ success: boolean; cookieCount: number; error?: string }> {
  try {
    const { cookies, rawNames } = await captureCookies(confluenceUrl, cookieStoreId);

    if (cookies.length === 0) {
      return {
        success: false,
        cookieCount: 0,
        error: emptyCookieError(rawNames, cookieStoreId),
      };
    }

    await apiClient.captureSession(confluenceUrl, cookies);

    return { success: true, cookieCount: cookies.length };
  } catch (error) {
    console.error('[spacemosquito] Failed to capture and save session:', error);
    throw error;
  }
}
