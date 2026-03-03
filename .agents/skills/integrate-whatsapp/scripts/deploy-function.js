#!/usr/bin/env node
const { parseArgs, requireStringFlag } = require('./lib/cli');
const { platformRequest } = require('./lib/http');
const { run } = require('./lib/run');

run(async () => {
  const { flags } = parseArgs(process.argv.slice(2));
  const functionId = requireStringFlag(flags, 'function-id');

  return platformRequest({
    method: 'POST',
    path: `/platform/v1/functions/${functionId}/deploy`
  });
});
