import { parseArgs, requireFlag } from './lib/args.mjs';
import { metaProxyRequest } from './lib/request.mjs';
import { err, ok, printResult } from './lib/output.mjs';

function usage() {
  return {
    usage: 'node scripts/template-status.mjs --business-account-id <WABA_ID> --name <template_name> [--fields <fields>]',
    env: ['KAPSO_API_BASE_URL', 'KAPSO_API_KEY', 'META_GRAPH_VERSION (optional)'],
    hints: [
      'To discover business_account_id (WABA), run: node scripts/list-platform-phone-numbers.mjs',
      'To list all templates (and get ids/status), use scripts/list-templates.mjs.'
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
    const name = requireFlag(flags, ['name', 'template-name', 'template_name'], 'name');
    const fields = flags.fields || 'name,status,category,language,components';

    const response = await metaProxyRequest({
      method: 'GET',
      path: `${businessAccountId}/message_templates`,
      query: { name, fields }
    });

    if (!response.ok) {
      return printResult(err('Meta proxy request failed', { response }));
    }

    return printResult(ok({ response }));
  } catch (error) {
    return printResult(err('Failed to fetch template status', { message: String(error?.message || error), ...usage() }));
  }
}

main().then((code) => process.exit(code));
