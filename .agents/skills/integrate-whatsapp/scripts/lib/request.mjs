import { metaProxyConfig } from './env.mjs';

function buildUrl(path, query) {
  const { baseUrl, graphVersion } = metaProxyConfig();
  const cleanedPath = path.replace(/^\/+/, '');
  const url = new URL(`${baseUrl}/${graphVersion}/${cleanedPath}`);

  if (query) {
    Object.entries(query).forEach(([key, value]) => {
      if (value === undefined || value === null || value === '') return;
      url.searchParams.set(key, String(value));
    });
  }

  return url;
}

function shouldParseJson(contentType) {
  return contentType && contentType.toLowerCase().includes('application/json');
}

export async function metaProxyRequest({ method, path, query, headers, body }) {
  const { apiKey } = metaProxyConfig();
  const url = buildUrl(path, query);
  const finalHeaders = new Headers(headers || {});

  finalHeaders.set('X-API-Key', apiKey);

  if (body && !(body instanceof FormData) && !finalHeaders.has('Content-Type')) {
    finalHeaders.set('Content-Type', 'application/json');
  }

  const response = await fetch(url, {
    method,
    headers: finalHeaders,
    body
  });

  const contentType = response.headers.get('content-type') || '';
  const text = await response.text();
  let data = text;

  if (text && shouldParseJson(contentType)) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }

  return {
    ok: response.ok,
    status: response.status,
    url: url.toString(),
    data
  };
}
