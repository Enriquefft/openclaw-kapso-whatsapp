#!/usr/bin/env node
const { parseArgs, getStringFlag, getNumberFlag } = require('./lib/cli');
const { platformRequest } = require('./lib/http');
const { run } = require('./lib/run');
const { requireFlowId } = require('./lib/whatsapp-flow');

run(async () => {
  const { flags } = parseArgs(process.argv.slice(2));
  const flowId = requireFlowId(flags);
  const versionId = getStringFlag(flags, 'version-id');

  if (versionId) {
    return platformRequest({
      method: 'GET',
      path: `/platform/v1/whatsapp/flows/${flowId}/versions/${versionId}`
    });
  }

  const list = await platformRequest({
    method: 'GET',
    path: `/platform/v1/whatsapp/flows/${flowId}/versions`,
    query: {
      page: getNumberFlag(flags, 'page') || 1,
      per_page: getNumberFlag(flags, 'per-page') || 1
    }
  });

  const versions = Array.isArray(list?.data) ? list.data : [];
  const latest = versions[0];
  if (!latest) {
    return { data: { flow_id: flowId, versions: [] }, meta: list?.meta };
  }

  const detail = await platformRequest({
    method: 'GET',
    path: `/platform/v1/whatsapp/flows/${flowId}/versions/${latest.id}`
  });

  return {
    data: {
      flow_id: flowId,
      version: detail?.data || detail,
      versions
    },
    meta: list?.meta
  };
});
