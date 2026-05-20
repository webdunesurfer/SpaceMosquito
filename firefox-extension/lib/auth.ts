// @ts-nocheck
import browser from 'webextension-polyfill';

export type AuthStatus = 'authenticated' | 'unauthenticated' | 'unknown';

export async function checkAuthStatus(): Promise<AuthStatus> {
  try {
    const sessionCookie = await browser.cookies.get({
      url: 'https://teamnetconomy.atlassian.net',
      name: 'tenant.session.token',
    });

    if (!sessionCookie) {
      return 'unauthenticated';
    }

    // Check if expired (expiry is Unix timestamp in seconds)
    const expiry = (sessionCookie as any).expiry;
    if (expiry && expiry < Math.floor(Date.now() / 1000)) {
      return 'unauthenticated';
    }

    return 'authenticated';
  } catch {
    return 'unknown';
  }
}
