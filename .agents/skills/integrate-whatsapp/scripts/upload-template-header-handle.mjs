import { err, blocked, printResult } from './lib/output.mjs';

function usage() {
  return {
    usage: 'node scripts/upload-template-header-handle.mjs --file <path> --mime-type <mime> --resumable-app-id <app_id>',
    env: ['KAPSO_API_BASE_URL', 'KAPSO_API_KEY', 'META_GRAPH_VERSION (optional)']
  };
}

function main() {
  try {
    return printResult(
      blocked('Meta proxy does not expose the resumable upload endpoints required for header_handle.', {
        missing_endpoints: [
          'POST /api/meta/vXX.X/{app_id}/uploads',
          'POST /api/meta/vXX.X/{upload_session_id}'
        ],
        notes: [
          'Use the Platform media ingest endpoint with delivery: meta_resumable_asset to obtain header_handle.',
          'Current proxy registry only supports POST /{phone_number_id}/media, which returns media_id (send-time only).'
        ],
        usage: usage()
      })
    );
  } catch (error) {
    return printResult(err('Failed to explain header handle limitations', { message: String(error?.message || error) }));
  }
}

main();
