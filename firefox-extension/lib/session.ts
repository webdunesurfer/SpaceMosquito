// @ts-nocheck
import browser from 'webextension-polyfill';
import type { Cookie } from './types';

const ATLASIAN_COOKIE_PATTERNS = [
  'session', 'token', 'sso', 'atlassian', 'aui',
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

export async function captureCookies(): Promise<Cookie[]> {
  try {
    const rawCookies: any[] = await browser.cookies.getAll({ domain: '.atlassian.net' });
    const filtered = rawCookies.filter((c: any) => c.name && shouldKeepCookie(c.name));

    return filtered.map((c: any) => ({
      name: c.name,
      value: c.value,
      domain: c.domain || '.atlassian.net',
      path: c.path || '/',
      expires: c.expiry || undefined,
      secure: c.secure || false,
      httpOnly: c.httpOnly || false,
      sameSite: normalizeSameSite(c.sameSite),
    }));
  } catch (error) {
    console.error('[spacemosquito] Failed to capture cookies:', error);
    throw error;
  }
}

export async function captureAndSave(confluenceUrl: string, apiClient: any): Promise<{ success: boolean; cookieCount: number }> {
  try {
    const cookies = await captureCookies();

    if (cookies.length === 0) {
      return { success: false, cookieCount: 0 };
    }

    await apiClient.captureSession(confluenceUrl, cookies);

    return { success: true, cookieCount: cookies.length };
  } catch (error) {
    console.error('[spacemosquito] Failed to capture and save session:', error);
    throw error;
  }
}
