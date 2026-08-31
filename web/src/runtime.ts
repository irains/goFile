export interface RuntimeMetadata {
  basePath: string;
  csrfNonce: string;
  language: 'en' | 'zh';
  loginNext: string;
  appName: string;
}

const trimBasePath = (value: string) => {
  const path = value.trim().replace(/^https?:\/\/[^/]+/i, '').replace(/\/+$/, '');
  return path === '/' ? '' : path.startsWith('/') ? path : path ? `/${path}` : '';
};

function readMeta(name: string): string {
  return document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)?.content ?? '';
}

export function getRuntime(): RuntimeMetadata {
  const basePath = trimBasePath(readMeta('fileharbor-base'));
  const locale = readMeta('fileharbor-locale');
  return {
    basePath,
    csrfNonce: readMeta('fileharbor-nonce'),
    language: locale === 'zh-CN' || (locale !== 'en' && document.documentElement.lang.toLowerCase().startsWith('zh')) ? 'zh' : 'en',
    loginNext: readMeta('fileharbor-login-next') || baseUrlFrom(basePath),
    appName: 'FileHarbor'
  };
}

function baseUrlFrom(basePath: string, path = ''): string {
  const suffix = path ? `/${path.replace(/^\/+/, '')}` : '';
  return `${basePath}${suffix}` || '/';
}

export function baseUrl(path = ''): string {
  return baseUrlFrom(getRuntime().basePath, path);
}

export function routeUrl(path: string, query?: Record<string, string | undefined>): string {
  const url = new URL(baseUrl(path), window.location.origin);
  for (const [key, value] of Object.entries(query ?? {})) if (value !== undefined) url.searchParams.set(key, value);
  return `${url.pathname}${url.search}`;
}

export function itemUrl(prefix: string, rootRelativePath: string): string {
  if (prefix === 'd' && !rootRelativePath) return baseUrl();
  const encodedPath = rootRelativePath.split('/').filter(Boolean).map(encodeURIComponent).join('/');
  return baseUrl(`${prefix}/${encodedPath}`);
}
