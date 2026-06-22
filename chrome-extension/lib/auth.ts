
export type AuthStatus = 'authenticated' | 'unauthenticated' | 'unknown';

export async function checkAuthStatus(tabUrl: string): Promise<AuthStatus> {
  try {
    const url = new URL(tabUrl);
    const domain = url.hostname;
    
    // Check for common Confluence session cookies
    const cookies = await chrome.cookies.getAll({ domain });
    const sessionCookie = cookies.find(c => 
      c.name === 'tenant.session.token' || 
      c.name === 'JSESSIONID' || 
      c.name === 'seraph.confluence'
    );

    if (!sessionCookie) {
      return 'unauthenticated';
    }

    // Check if expired (expiry is Unix timestamp in seconds)
    const expiry = (sessionCookie as any).expirationDate; // Chrome uses expirationDate
    if (expiry && expiry < Math.floor(Date.now() / 1000)) {
      return 'unauthenticated';
    }

    return 'authenticated';
  } catch {
    return 'unknown';
  }
}
