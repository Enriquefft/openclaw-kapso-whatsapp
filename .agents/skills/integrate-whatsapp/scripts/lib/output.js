function printOk(data) {
  // eslint-disable-next-line no-console
  console.log(JSON.stringify({ ok: true, data }, null, 2));
}

function printError(message, details) {
  // eslint-disable-next-line no-console
  console.error(JSON.stringify({ ok: false, error: { message, details } }, null, 2));
}

module.exports = {
  printOk,
  printError
};
