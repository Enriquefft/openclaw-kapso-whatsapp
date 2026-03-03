function requireEnv(name) {
  const value = process.env[name];
  if (!value) {
    throw new Error(`Missing required env var: ${name}`);
  }
  return value;
}

function normalizeBaseUrl(raw) {
  return raw.replace(/\/+$/, '');
}

function kapsoConfigFromEnv() {
  return {
    baseUrl: normalizeBaseUrl(requireEnv('KAPSO_API_BASE_URL')),
    apiKey: requireEnv('KAPSO_API_KEY')
  };
}

async function kapsoRequest(config, path, init = {}) {
  const url = `${config.baseUrl}${path}`;
  const headers = new Headers(init.headers || undefined);
  headers.set('X-API-Key', config.apiKey);
  if (!headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  const response = await fetch(url, { ...init, headers });
  const text = await response.text();

  if (!response.ok) {
    throw new Error(`Kapso API request failed (status=${response.status}) body=${text}`);
  }

  return text ? JSON.parse(text) : {};
}

module.exports = {
  kapsoConfigFromEnv,
  kapsoRequest
};
