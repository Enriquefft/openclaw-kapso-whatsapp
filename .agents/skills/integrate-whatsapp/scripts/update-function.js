#!/usr/bin/env node
const { parseArgs, requireStringFlag, getStringFlag, readFlagText, readFlagJson } = require('./lib/cli');
const { platformRequest } = require('./lib/http');
const { run } = require('./lib/run');

run(async () => {
  const { flags } = parseArgs(process.argv.slice(2));
  const functionId = requireStringFlag(flags, 'function-id');
  const code = await readFlagText(flags, 'code', 'code-file');

  if (!code) {
    throw new Error('Missing --code or --code-file');
  }

  const runtimeConfig = await readFlagJson(flags, 'runtime-config', 'runtime-config-file');

  const body = {
    function: {
      name: getStringFlag(flags, 'name'),
      code,
      description: getStringFlag(flags, 'description'),
      runtime_config: runtimeConfig
    }
  };

  return platformRequest({
    method: 'PATCH',
    path: `/platform/v1/functions/${functionId}`,
    body
  });
});
