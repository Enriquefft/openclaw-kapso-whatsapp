const { hasHelpFlag, parseFlags } = require('./lib/status/args');
const { kapsoConfigFromEnv, kapsoRequest } = require('./lib/status/kapso-api');

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
            'node scripts/overview.js [--period <24h|7d|30d>] [--per-page <n>]',
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
    const period = flags.period || '24h';
    const perPage = parseNumber(flags['per-page'], 50, 'per-page');
    const config = kapsoConfigFromEnv();

    const phoneNumbers = await kapsoRequest(
      config,
      `/platform/v1/whatsapp/phone_numbers?per_page=${perPage}`
    );

    const apiTotals = await kapsoRequest(
      config,
      `/platform/v1/api_logs?period=${encodeURIComponent(period)}&per_page=1`
    );
    const apiErrors = await kapsoRequest(
      config,
      `/platform/v1/api_logs?period=${encodeURIComponent(period)}&errors_only=true&per_page=1`
    );

    const webhookTotals = await kapsoRequest(
      config,
      `/platform/v1/webhook_deliveries?period=${encodeURIComponent(period)}&per_page=1`
    );
    const webhookErrors = await kapsoRequest(
      config,
      `/platform/v1/webhook_deliveries?period=${encodeURIComponent(period)}&errors_only=true&per_page=1`
    );

    const payload = {
      ok: true,
      period,
      phone_numbers: phoneNumbers.data || [],
      api_calls: {
        total: extractCount(apiTotals),
        failed: extractCount(apiErrors)
      },
      webhook_deliveries: {
        total: extractCount(webhookTotals),
        failed: extractCount(webhookErrors)
      },
      notes: [
        'Plan and subscription details are not exposed via the Platform API.',
        'Use whatsapp-health.js per phone number for detailed WhatsApp checks.'
      ]
    };

    console.log(JSON.stringify(payload, null, 2));
    return 0;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    console.error(JSON.stringify(err('Command failed', { message }), null, 2));
    return 1;
  }
}

function parseNumber(value, fallback, name) {
  if (value === undefined || value === true) return fallback;
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    throw new Error(`Invalid --${name} value: ${value}`);
  }
  return parsed;
}

function extractCount(response) {
  if (!response || typeof response !== 'object') return null;
  return response.meta && typeof response.meta.total_count === 'number'
    ? response.meta.total_count
    : null;
}

main().then((code) => process.exit(code));
