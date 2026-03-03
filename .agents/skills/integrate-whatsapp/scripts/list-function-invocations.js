#!/usr/bin/env node
const { parseArgs, getStringFlag, getNumberFlag } = require('./lib/cli');
const { platformRequest } = require('./lib/http');
const { run } = require('./lib/run');

run(async () => {
  const { flags } = parseArgs(process.argv.slice(2));
  const functionId = getStringFlag(flags, 'function-id');
  const flowId = getStringFlag(flags, 'flow-id');

  if (!functionId && !flowId) {
    throw new Error('Provide --function-id or --flow-id');
  }

  if (functionId && flowId) {
    throw new Error('Provide only one of --function-id or --flow-id');
  }

  const query = {
    status: getStringFlag(flags, 'status'),
    limit: getNumberFlag(flags, 'limit')
  };

  return platformRequest({
    method: 'GET',
    path: functionId
      ? `/platform/v1/functions/${functionId}/invocations`
      : `/platform/v1/whatsapp/flows/${flowId}/function_invocations`,
    query
  });
});
