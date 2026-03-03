const { getConfig } = require('./env');

class RequestError extends Error {
  constructor(message, status, body) {
    super(message);
    this.status = status;
    this.body = body;
  }
}

function buildUrl(baseUrl, path, query) {
  const trimmed = baseUrl.replace(/\/+$/, '');
  const safePath = path.startsWith('/') ? path : `/${path}`;
  const url = new URL(`${trimmed}${safePath}`);

  if (query) {
    Object.entries(query).forEach(([key, value]) => {
      if (value === undefined || value === null) return;
      url.searchParams.set(key, String(value));
    });
  }

  return url.toString();
}

function isFormData(body) {
  return typeof FormData !== 'undefined' && body instanceof FormData;
}

function isBlob(body) {
  return typeof Blob !== 'undefined' && body instanceof Blob;
}

function isPlainObject(value) {
  return value !== null && typeof value === 'object' && value.constructor === Object;
}

async function request({ baseUrl, path, method, query, body, headers }) {
  const config = getConfig();
  const url = buildUrl(baseUrl, path, query);
  const finalHeaders = new Headers(headers || {});
  finalHeaders.set('X-API-Key', config.apiKey);

  let finalBody = body;

  if (body !== undefined && body !== null) {
    if (isPlainObject(body)) {
      finalBody = JSON.stringify(body);
      if (!finalHeaders.has('Content-Type')) {
        finalHeaders.set('Content-Type', 'application/json');
      }
    } else if (typeof body === 'string') {
      finalBody = body;
    } else if (isFormData(body) || isBlob(body)) {
      finalBody = body;
    }
  }

  const response = await fetch(url, {
    method,
    headers: finalHeaders,
    body: finalBody
  });

  const contentType = response.headers.get('content-type') || '';
  const text = await response.text();
  const parsed = contentType.includes('application/json') ? safeJson(text) : text;

  if (!response.ok) {
    throw new RequestError(`Request failed (${response.status})`, response.status, parsed);
  }

  return parsed;
}

function safeJson(text) {
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

function metaBaseUrl() {
  const config = getConfig();
  return `${config.baseUrl}/meta/whatsapp/${config.graphVersion}`;
}

function platformBaseUrl() {
  const config = getConfig();
  return config.baseUrl;
}

async function metaRequest(options) {
  return request({ baseUrl: metaBaseUrl(), ...options });
}

async function platformRequest(options) {
  return request({ baseUrl: platformBaseUrl(), ...options });
}

module.exports = {
  RequestError,
  metaRequest,
  platformRequest
};
