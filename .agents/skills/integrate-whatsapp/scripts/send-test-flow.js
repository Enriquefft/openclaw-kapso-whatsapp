#!/usr/bin/env node
const { parseArgs, requireStringFlag, getStringFlag, getBooleanFlag, readFlagJson } = require('./lib/cli');
const { metaRequest } = require('./lib/http');
const { run } = require('./lib/run');

run(async () => {
  const { flags } = parseArgs(process.argv.slice(2));
  const phoneNumberId = requireStringFlag(flags, 'phone-number-id');
  const to = requireStringFlag(flags, 'to');
  const flowId = requireStringFlag(flags, 'flow-id');
  const bodyText = requireStringFlag(flags, 'body-text');
  const flowCta = getStringFlag(flags, 'flow-cta') || 'Open';
  const headerText = getStringFlag(flags, 'header-text');
  const footerText = getStringFlag(flags, 'footer-text');
  const flowToken = getStringFlag(flags, 'flow-token') || flowId;
  const flowAction = getStringFlag(flags, 'flow-action');
  const flowActionPayload = await readFlagJson(flags, 'flow-action-payload', 'flow-action-payload-file');
  const mode = getStringFlag(flags, 'mode');
  const draft = getBooleanFlag(flags, 'draft');

  const payload = {
    messaging_product: 'whatsapp',
    recipient_type: 'individual',
    to,
    type: 'interactive',
    interactive: {
      type: 'flow',
      body: { text: bodyText },
      action: {
        name: 'flow',
        parameters: {
          flow_message_version: '3',
          flow_id: flowId,
          flow_cta: flowCta,
          flow_token: flowToken
        }
      }
    }
  };

  if (headerText) {
    payload.interactive.header = { type: 'text', text: headerText };
  }

  if (footerText) {
    payload.interactive.footer = { text: footerText };
  }

  if (flowAction) {
    payload.interactive.action.parameters.flow_action = flowAction;
  }

  if (flowActionPayload) {
    payload.interactive.action.parameters.flow_action_payload = flowActionPayload;
  }

  if (mode) {
    payload.interactive.action.parameters.mode = mode;
  } else if (draft) {
    payload.interactive.action.parameters.mode = 'draft';
  }

  return metaRequest({
    method: 'POST',
    path: `/${phoneNumberId}/messages`,
    body: payload
  });
});
