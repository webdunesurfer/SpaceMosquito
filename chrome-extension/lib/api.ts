// @ts-nocheck
import { Cookie, Session, SessionStatus, CrawlJob, CrawlSpace, CronSpaceConfig } from './types';

export class ApiClient {
  private baseUrl: string = 'http://localhost:8081';

  constructor(baseUrl?: string) {
    if (baseUrl) {
      this.baseUrl = baseUrl;
    }
  }

  async getBackendUrl(): Promise<string> {
    return this.baseUrl;
  }

  setBackendUrl(url: string): void {
    this.baseUrl = url.replace(/\/+$/, '');
  }

  private async request<T>(path: string, options?: RequestInit): Promise<T> {
    const url = `${this.baseUrl}${path}`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options?.headers as Record<string, string> || {}),
    };

    try {
      const response = await fetch(url, {
        ...options,
        headers,
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`HTTP ${response.status}: ${errorText}`);
      }

      const contentType = response.headers.get('content-type');
      if (contentType && contentType.includes('application/json')) {
        return await response.json() as T;
      }
      return {} as T;
    } catch (error) {
      console.error(`[spacemosquito] API request failed: ${path}`, error);
      throw error;
    }
  }

  // Session management
  async captureSession(confluenceUrl: string, cookies: Cookie[]): Promise<{ message: string }> {
    return this.request<{ message: string }>('/api/session', {
      method: 'POST',
      body: JSON.stringify({ confluence_url: confluenceUrl, cookies }),
    });
  }

  async getSessionStatus(): Promise<SessionStatus> {
    return this.request<SessionStatus>('/api/session/status');
  }

  async validateSession(): Promise<{ valid: boolean; message: string }> {
    return this.request<{ valid: boolean; message: string }>('/api/session/validate', {
      method: 'POST',
    });
  }

  async deleteSession(): Promise<{ message: string }> {
    return this.request<{ message: string }>('/api/session', {
      method: 'DELETE',
    });
  }

  // Crawl management
  async startCrawl(spaceUrl: string): Promise<{ job_id: string }> {
    return this.request<{ job_id: string }>('/api/crawl', {
      method: 'POST',
      body: JSON.stringify({ space_url: spaceUrl }),
    });
  }

  async listCrawls(): Promise<JobSnapshot> {
    return this.request<JobSnapshot>('/api/crawl');
  }

  async getCrawlStatus(jobId: string): Promise<CrawlJob> {
    return this.request<CrawlJob>(`/api/crawl/status?id=${encodeURIComponent(jobId)}`);
  }

  async cancelCrawl(jobId: string): Promise<void> {
    await this.request('/api/crawl/cancel', {
      method: 'POST',
      body: JSON.stringify({ job_id: jobId }),
    });
  }

  async cleanupCrawls(): Promise<{ message: string }> {
    return this.request<{ message: string }>('/api/crawl/cleanup', {
      method: 'POST',
    });
  }

  // Spaces management
  async addSpace(url: string): Promise<{ message: string; space_key: string }> {
    return this.request<{ message: string; space_key: string }>('/api/spaces', {
      method: 'POST',
      body: JSON.stringify({ url }),
    });
  }

  async listSpaces(): Promise<CrawlSpace[]> {
    return this.request<CrawlSpace[]>('/api/spaces');
  }

  async deleteSpace(spaceKey: string): Promise<void> {
    await this.request(`/api/spaces/${encodeURIComponent(spaceKey)}`, {
      method: 'DELETE',
    });
  }

  async getSpaceStatus(spaceKey: string): Promise<{
    space_key: string;
    space_name: string;
    space_url: string;
    pages_crawled: number;
    last_crawled?: string;
  }> {
    return this.request(`/api/spaces/${encodeURIComponent(spaceKey)}`);
  }

  // Cron management
  async getCronConfig(): Promise<{
    yaml_full_crawl: { enabled: boolean; interval: string; spaces: string[]; max_duration: string };
    yaml_incremental: { enabled: boolean; interval: string; detection: string; spaces: string[]; max_duration: string };
    per_space_overrides: CronSpaceConfig[];
  }> {
    return this.request('/api/cron/config');
  }

  async updateCronConfig(spaceKey: string, config: Partial<CronSpaceConfig>): Promise<{ message: string }> {
    return this.request('/api/cron/config', {
      method: 'POST',
      body: JSON.stringify({ space_key: spaceKey, ...config }),
    });
  }

  async updateSpaceCron(spaceKey: string, config: Partial<CronSpaceConfig>): Promise<{ message: string }> {
    return this.request(`/api/cron/space/${encodeURIComponent(spaceKey)}`, {
      method: 'POST',
      body: JSON.stringify(config),
    });
  }

  async deleteSpaceCron(spaceKey: string): Promise<{ message: string }> {
    return this.request(`/api/cron/space/${encodeURIComponent(spaceKey)}`, {
      method: 'DELETE',
    });
  }

  async getSpaceCronConfig(spaceKey: string): Promise<CronSpaceConfig> {
    return this.request(`/api/cron/space/${encodeURIComponent(spaceKey)}`);
  }

  async reloadCron(): Promise<{ message: string }> {
    return this.request('/api/cron/reload', {
      method: 'POST',
    });
  }

  async listCronJobs(): Promise<{ id: string; next_run: string; last_run: string; disabled: boolean }[]> {
    return this.request('/api/cron');
  }

  async startCronJobs(): Promise<{ status: string }> {
    return this.request('/api/cron/start', {
      method: 'POST',
    });
  }

  // Search
  async searchPages(query: string, spaceKey?: string): Promise<{
    query: string;
    count: number;
    results: {
      confluence_id: number;
      space_key: string;
      title: string;
      excerpt: string;
      similarity: number;
      file_path?: string;
      internal_id?: string;
    }[];
  }> {
    const params = new URLSearchParams({ q: query });
    if (spaceKey) params.set('space_key', spaceKey);
    return this.request(`/api/search?${params.toString()}`);
  }

  async getSearchStats(): Promise<{
    total_spaces: number;
    total_pages: number;
    content_indexing: string;
    last_crawled: string;
  }> {
    return this.request('/api/search/stats');
  }
}
