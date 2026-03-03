const { parseBoolean, parseNumber, parseJsonObject, parseStringArray } = require('./args');

function buildWebhookPayload(flags) {
  const payload = {};
  if (typeof flags.url === 'string' && flags.url.length > 0) {
    payload.url = flags.url;
  }
  const events = parseStringArray(flags.events, 'events');
  if (events) {
    payload.events = events;
  }
  const active = parseBoolean(flags.active, 'active');
  if (active !== undefined) {
    payload.active = active;
  }
  if (typeof flags.kind === 'string' && flags.kind.length > 0) {
    payload.kind = flags.kind;
  }
  if (typeof flags['payload-version'] === 'string' && flags['payload-version'].length > 0) {
    payload.payload_version = flags['payload-version'];
  }
  const bufferEnabled = parseBoolean(flags['buffer-enabled'], 'buffer-enabled');
  if (bufferEnabled !== undefined) {
    payload.buffer_enabled = bufferEnabled;
  }
  const bufferWindowSeconds = parseNumber(flags['buffer-window-seconds'], 'buffer-window-seconds');
  if (bufferWindowSeconds !== undefined) {
    payload.buffer_window_seconds = bufferWindowSeconds;
  }
  const maxBufferSize = parseNumber(flags['max-buffer-size'], 'max-buffer-size');
  if (maxBufferSize !== undefined) {
    payload.max_buffer_size = maxBufferSize;
  }
  const inactivityMinutes = parseNumber(flags['inactivity-minutes'], 'inactivity-minutes');
  if (inactivityMinutes !== undefined) {
    payload.inactivity_minutes = inactivityMinutes;
  }
  const headers = parseJsonObject(flags.headers, 'headers');
  if (headers) {
    payload.headers = headers;
  }
  return payload;
}

module.exports = {
  buildWebhookPayload
};
