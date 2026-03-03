export function ok(data) {
  return { ok: true, data };
}

export function blocked(reason, details) {
  return { ok: true, blocked: true, ...details, reason };
}

export function err(message, details) {
  return { ok: false, error: { message, details } };
}

export function printResult(result) {
  const json = JSON.stringify(result, null, 2);

  // Skill runners sometimes only surface stdout. Print errors there too so
  // agents don't see only "exit status 2" with no details.
  // eslint-disable-next-line no-console
  console.log(json);

  return result.ok ? 0 : 2;
}
