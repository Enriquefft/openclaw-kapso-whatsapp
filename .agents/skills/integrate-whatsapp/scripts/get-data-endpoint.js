#!/usr/bin/env node
const { parseArgs } = require('./lib/cli');
const { platformRequest } = require('./lib/http');
const { run } = require('./lib/run');
const { requireFlowId } = require('./lib/whatsapp-flow');

run(async () => {
  const { flags } = parseArgs(process.argv.slice(2));
  const flowId = requireFlowId(flags);

  return platformRequest({
    method: 'GET',
    path: `/platform/v1/whatsapp/flows/${flowId}/data_endpoint`
  });
});
