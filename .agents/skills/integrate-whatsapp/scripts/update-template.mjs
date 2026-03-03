import { loadJsonPayload, parseArgs, requireFlag } from './lib/args.mjs';
import { metaProxyRequest } from './lib/request.mjs';
import { err, ok, printResult } from './lib/output.mjs';

function usage() {
  return {
    usage: 'node scripts/update-template.mjs --business-account-id <WABA_ID> --hsm-id <template_id> --json <payload> | --file <path>',
    env: ['KAPSO_API_BASE_URL', 'KAPSO_API_KEY', 'META_GRAPH_VERSION (optional)'],
    hints: [
      'To discover business_account_id (WABA), run: node scripts/list-platform-phone-numbers.mjs',
      'Find template id (hsm_id) via list-templates or template-status.'
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
    const hsmId = requireFlag(flags, ['hsm-id', 'hsm_id'], 'hsm-id');
    const payload = await loadJsonPayload(flags);

    const response = await metaProxyRequest({
      method: 'POST',
      path: `${businessAccountId}/message_templates`,
      query: { hsm_id: hsmId },
      body: JSON.stringify(payload)
    });

    if (!response.ok) {
      return printResult(err('Meta proxy request failed', { response }));
    }

    return printResult(ok({ response }));
  } catch (error) {
    return printResult(err('Failed to update template', { message: String(error?.message || error), ...usage() }));
  }
}

main().then((code) => process.exit(code));
