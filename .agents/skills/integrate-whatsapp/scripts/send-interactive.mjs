import { loadJsonPayload, parseArgs, requireFlag } from './lib/args.mjs';
import { metaProxyRequest } from './lib/request.mjs';
import { err, ok, printResult } from './lib/output.mjs';

function usage() {
  return {
    usage: 'node scripts/send-interactive.mjs --phone-number-id <PHONE_NUMBER_ID> --json <payload> | --file <path>',
    env: ['KAPSO_API_BASE_URL', 'KAPSO_API_KEY', 'META_GRAPH_VERSION (optional)'],
    notes: ['Payload must include messaging_product: "whatsapp" and type: "interactive".'],
    hints: [
      'To discover phone_number_id (Meta phone number id), run: node scripts/list-platform-phone-numbers.mjs',
      'Start from an asset payload in assets/ (send-interactive-*.json) and adjust to/interactive fields.'
    ]
  };
}

function normalizePayload(payload) {
  if (!payload || typeof payload !== 'object') {
    throw new Error('Payload must be a JSON object');
  }

  if (!payload.messaging_product) {
    payload.messaging_product = 'whatsapp';
  }

  if (payload.messaging_product !== 'whatsapp') {
    throw new Error('messaging_product must be whatsapp');
  }

  if (payload.type !== 'interactive') {
    throw new Error('type must be interactive');
  }

  if (!payload.to) {
    throw new Error('to is required');
  }

  if (!payload.interactive || typeof payload.interactive !== 'object') {
    throw new Error('interactive is required and must be an object');
  }

  if (!payload.interactive.type) {
    throw new Error('interactive.type is required');
  }

  return payload;
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
    const phoneNumberId = requireFlag(flags, ['phone-number-id', 'phone_number_id'], 'phone-number-id');
    const payload = normalizePayload(await loadJsonPayload(flags));

    const response = await metaProxyRequest({
      method: 'POST',
      path: `${phoneNumberId}/messages`,
      body: JSON.stringify(payload)
    });

    if (!response.ok) {
      return printResult(err('Meta proxy request failed', { response }));
    }

    return printResult(ok({ response }));
  } catch (error) {
    return printResult(err('Failed to send interactive message', { message: String(error?.message || error), ...usage() }));
  }
}

main().then((code) => process.exit(code));

