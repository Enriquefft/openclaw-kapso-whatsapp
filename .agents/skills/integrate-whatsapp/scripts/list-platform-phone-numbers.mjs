import { getFlag, parseArgs } from './lib/args.mjs';
import { err, ok, printResult } from './lib/output.mjs';

function usage() {
  return {
    usage: 'node scripts/list-platform-phone-numbers.mjs [--page <n>] [--per-page <n>] [--phone-number-id <id>] [--business-account-id <id>]',
    env: ['KAPSO_API_BASE_URL', 'KAPSO_API_KEY'],
    notes: [
      'This calls the Platform API: GET /platform/v1/whatsapp/phone_numbers.',
      'Use this to discover business_account_id (WABA) and phone_number_id (Meta phone number id).'
    ]
  };
}

function requireEnv(name) {
  const value = process.env[name];
  if (!value) {
    throw new Error(`Missing required env var: ${name}`);
  }
  return value;
}

function normalizePlatformBase(raw) {
  const trimmed = raw.replace(/\/+$/, '');
  if (trimmed.endsWith('/platform/v1')) {
    return trimmed.slice(0, -'/platform/v1'.length);
  }
  const metaIndex = trimmed.indexOf('/meta');
  if (metaIndex >= 0) {
    return trimmed.slice(0, metaIndex);
  }
  return trimmed;
}

function buildUrl(path, query) {
  const baseUrl = normalizePlatformBase(requireEnv('KAPSO_API_BASE_URL'));
  const cleanedPath = path.replace(/^\/+/, '');
  const url = new URL(`${baseUrl}/platform/v1/${cleanedPath}`);

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

async function platformRequest({ method, path, query }) {
  const apiKey = requireEnv('KAPSO_API_KEY');
  const url = buildUrl(path, query);
  const headers = new Headers();

  headers.set('X-API-Key', apiKey);

  const response = await fetch(url, { method, headers });
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

function summarizeIds(platformResponse) {
  const configs = platformResponse?.data?.data;
  if (!Array.isArray(configs)) return [];

  return configs.map((config) => ({
    name: config.name,
    display_phone_number: config.display_phone_number,
    phone_number_id: config.phone_number_id || config.id,
    business_account_id: config.business_account_id
  }));
}

async function main() {
  const { flags, errors } = parseArgs(process.argv.slice(2));
  if (flags.help) {
    return printResult(ok(usage()));
  }
  if (errors.length > 0) {
    return printResult(err('Invalid arguments', { errors, ...usage() }));
  }

  try {
    const query = {
      page: getFlag(flags, ['page']),
      per_page: getFlag(flags, ['per-page', 'per_page']),
      phone_number_id: getFlag(flags, ['phone-number-id', 'phone_number_id']),
      business_account_id: getFlag(flags, ['business-account-id', 'business_account_id'])
    };

    const response = await platformRequest({
      method: 'GET',
      path: 'whatsapp/phone_numbers',
      query
    });

    if (!response.ok) {
      return printResult(err('Platform API request failed', { response, ...usage() }));
    }

    return printResult(ok({ response, ids: summarizeIds(response) }));
  } catch (error) {
    return printResult(err('Failed to list platform WhatsApp phone numbers', { message: String(error?.message || error), ...usage() }));
  }
}

main().then((code) => process.exit(code));

