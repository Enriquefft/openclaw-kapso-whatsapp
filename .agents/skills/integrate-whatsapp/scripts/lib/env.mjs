function requireEnv(name) {
  const value = process.env[name];
  if (!value) {
    throw new Error(`Missing required env var: ${name}`);
  }
  return value;
}

function normalizeMetaBase(raw) {
  const trimmed = raw.replace(/\/+$/, '');
  if (trimmed.endsWith('/platform/v1')) {
    return `${trimmed.slice(0, -'/platform/v1'.length)}/meta/whatsapp`;
  }
  const metaMatch = trimmed.match(/^(.*)\/meta\/whatsapp(?:\/v\d+\.\d+)?$/);
  if (metaMatch) {
    return `${metaMatch[1]}/meta/whatsapp`;
  }
  const rawMetaMatch = trimmed.match(/^(.*)\/meta$/);
  if (rawMetaMatch) {
    return `${rawMetaMatch[1]}/meta/whatsapp`;
  }
  return `${trimmed}/meta/whatsapp`;
}

function normalizeGraphVersion(value) {
  if (!value) return 'v24.0';
  return value.startsWith('v') ? value : `v${value}`;
}

export function metaProxyConfig() {
  const rawBase = process.env.KAPSO_META_BASE_URL || requireEnv('KAPSO_API_BASE_URL');
  const baseUrl = normalizeMetaBase(rawBase);
  const apiKey = requireEnv('KAPSO_API_KEY');
  const graphVersion = normalizeGraphVersion(process.env.META_GRAPH_VERSION || 'v24.0');

  return {
    baseUrl,
    apiKey,
    graphVersion
  };
}
