#!/usr/bin/env node
const { parseArgs } = require('./lib/cli');
const { metaRequest } = require('./lib/http');
const { run } = require('./lib/run');
const { requireFlowId, buildScopeQuery } = require('./lib/whatsapp-flow');

run(async () => {
  const { flags } = parseArgs(process.argv.slice(2));
  const flowId = requireFlowId(flags);
  const query = buildScopeQuery(flags);

  return metaRequest({
    method: 'DELETE',
    path: `/flows/${flowId}`,
    query
  });
});
