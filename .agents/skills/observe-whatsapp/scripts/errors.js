const { hasHelpFlag, parseFlags } = require('./lib/triage/args');
const { kapsoConfigFromEnv, kapsoRequest } = require('./lib/triage/kapso-api');

function err(message, details) {
  return { ok: false, error: { message, details } };
}

async function main() {
  const argv = process.argv.slice(2);
  if (hasHelpFlag(argv)) {
    console.log(
      JSON.stringify(
        {
          ok: true,
          usage:
            'node scripts/errors.js [--period <24h|7d|30d>] [--source <message_delivery|api_call|webhook_delivery>] [--limit <n>] [--page <n>] [--phone-number <e164>]',
          env: ['KAPSO_API_BASE_URL', 'KAPSO_API_KEY']
        },
        null,
        2
      )
    );
    return 0;
  }

  try {
    const flags = parseFlags(argv);
    const source = normalizeSource(flags.source);
    const limit = parseNumber(flags.limit, 20, 'limit');
    const page = parseNumber(flags.page, 1, 'page');
    const period = flags.period || '24h';
    const phoneNumber = flags['phone-number'];

    const config = kapsoConfigFromEnv();
    const result = { ok: true, period, sources: {}, notes: [] };

    if (!source || source === 'message_delivery') {
      const params = new URLSearchParams();
      params.set('status', 'failed');
      params.set('direction', 'outbound');
      params.set('per_page', String(limit));
      params.set('page', String(page));
      if (phoneNumber) params.set('phone_number', phoneNumber);

      const path = `/platform/v1/whatsapp/messages?${params.toString()}`;
      result.sources.message_delivery = await kapsoRequest(config, path);
      result.notes.push('Message failures use outbound status=failed and do not support period filtering.');
    }

    if (!source || source === 'api_call') {
      const params = new URLSearchParams();
      params.set('period', period);
      params.set('errors_only', 'true');
      params.set('per_page', String(limit));
      params.set('page', String(page));

      const path = `/platform/v1/api_logs?${params.toString()}`;
      result.sources.api_call = await kapsoRequest(config, path);
    }

    if (!source || source === 'webhook_delivery') {
      const params = new URLSearchParams();
      params.set('period', period);
      params.set('errors_only', 'true');
      params.set('per_page', String(limit));
      params.set('page', String(page));

      const path = `/platform/v1/webhook_deliveries?${params.toString()}`;
      result.sources.webhook_delivery = await kapsoRequest(config, path);
    }

    console.log(JSON.stringify(result, null, 2));
    return 0;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    console.error(JSON.stringify(err('Command failed', { message }), null, 2));
    return 1;
  }
}

function normalizeSource(source) {
  if (!source || source === true) return null;
  const value = String(source).toLowerCase();
  const allowed = ['message_delivery', 'api_call', 'webhook_delivery'];
  if (!allowed.includes(value)) {
    throw new Error(`Invalid --source value: ${source}`);
  }
  return value;
}

function parseNumber(value, fallback, name) {
  if (value === undefined || value === true) return fallback;
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    throw new Error(`Invalid --${name} value: ${value}`);
  }
  return parsed;
}

main().then((code) => process.exit(code));
