import { parseArgs, requireFlag } from './lib/args.mjs';
import { metaProxyRequest } from './lib/request.mjs';
import { err, ok, printResult } from './lib/output.mjs';

function usage() {
  return {
    usage: 'node scripts/list-connected-numbers.mjs --business-account-id <WABA_ID>',
    env: ['KAPSO_API_BASE_URL', 'KAPSO_API_KEY', 'META_GRAPH_VERSION (optional)'],
    hints: [
      'To discover business_account_id (WABA) and phone_number_id, run: node scripts/list-platform-phone-numbers.mjs',
      'Platform API equivalent: GET /platform/v1/whatsapp/phone_numbers'
    ]
  };
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
    const businessAccountId = requireFlag(flags, ['business-account-id', 'business_account_id'], 'business-account-id');
    const response = await metaProxyRequest({
      method: 'GET',
      path: `${businessAccountId}/phone_numbers`
    });

    if (!response.ok) {
      return printResult(err('Meta proxy request failed', { response }));
    }

    return printResult(ok({ response }));
  } catch (error) {
    return printResult(err('Failed to list connected numbers', { message: String(error?.message || error), ...usage() }));
  }
}

main().then((code) => process.exit(code));
