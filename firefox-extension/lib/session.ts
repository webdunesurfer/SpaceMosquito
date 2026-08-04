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
  return `${c.name}|${c.domain}|${c.path}`;
}

/**
 * Firefox differs from Chrome: getAll({ domain }) often misses host-only
 * cookies, and with first-party isolation you must pass firstPartyDomain
 * (null = any). Prefer url=tabUrl and merge several queries.
 */
async function getAllCookiesForTab(tabUrl: string): Promise<any[]> {
  const hostname = new URL(tabUrl).hostname;
  const queries: Record<string, unknown>[] = [
    { url: tabUrl, firstPartyDomain: null },
    { url: tabUrl },
    { domain: hostname, firstPartyDomain: null },
    { domain: hostname },
    { domain: '.' + hostname, firstPartyDomain: null },
    { domain: '.' + hostname },
  ];

  const byKey = new Map<string, any>();
  for (const details of queries) {
    try {
      const batch: any[] = await browser.cookies.getAll(details);
      for (const c of batch) {
        if (c?.name) byKey.set(cookieKey(c), c);
      }
    } catch (err) {
      // firstPartyDomain unsupported or invalid filter combination
      console.warn('[spacemosquito] cookies.getAll skipped', details, err);
    }
  }
  return Array.from(byKey.values());
}

export async function captureCookies(tabUrl: string): Promise<{ cookies: Cookie[]; rawCount: number; rawNames: string[] }> {
  try {
    const domain = new URL(tabUrl).hostname;
    const rawCookies = await getAllCookiesForTab(tabUrl);
    const rawNames = rawCookies.map((c: any) => c.name).filter(Boolean);
    const filtered = rawCookies.filter((c: any) => c.name && shouldKeepCookie(c.name));

    console.log('[spacemosquito] cookie capture', {
      tabUrl,
      domain,
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

export async function captureAndSave(confluenceUrl: string, apiClient: any): Promise<{ success: boolean; cookieCount: number; error?: string }> {
  try {
    const { cookies, rawCount, rawNames } = await captureCookies(confluenceUrl);

    if (cookies.length === 0) {
      const sample = rawNames.slice(0, 8).join(', ') || '(none)';
      return {
        success: false,
        cookieCount: 0,
        error: `No session cookies matched (raw ${rawCount}: ${sample})`,
      };
    }

    await apiClient.captureSession(confluenceUrl, cookies);

    return { success: true, cookieCount: cookies.length };
  } catch (error) {
    console.error('[spacemosquito] Failed to capture and save session:', error);
    throw error;
  }
}
