// @ts-nocheck
// Firefox provides browser API natively

export type AuthStatus = 'authenticated' | 'unauthenticated' | 'unknown';

export async function checkAuthStatus(tabUrl: string): Promise<AuthStatus> {
  try {
    const url = new URL(tabUrl);
    const domain = url.hostname;

    const cookies = await browser.cookies.getAll({ domain });
    const sessionCookie = cookies.find((c: any) =>
      c.name === 'tenant.session.token' ||
      c.name === 'JSESSIONID' ||
      c.name === 'seraph.confluence'
    );

    if (!sessionCookie) {
      return 'unauthenticated';
    }

    const expiry = (sessionCookie as any).expirationDate || (sessionCookie as any).expiry;
    if (expiry && expiry < Math.floor(Date.now() / 1000)) {
      return 'unauthenticated';
    }

    return 'authenticated';
  } catch {
    return 'unknown';
  }
}
