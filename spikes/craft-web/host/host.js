// The parent side of the Craft apps' `?host=parent` protocol, as the cockpit's XlsxPreview speaks
// it: on <app>:ready the file goes to the frame as bytes; every <app>:save is posted to the spike
// server, which writes it to disk. Every message is logged in window.__craft for the driver.
//
//   /?app=pdfcraft&file=/fixtures/documents/Appunti.pdf[&flags=webgl]
const params = new URLSearchParams(location.search);
const app = params.get('app');
const file = params.get('file');
const flags = params.get('flags');
const frame = document.getElementById('app');
const t0 = performance.now();
const at = () => Math.round(performance.now() - t0);
const log = (entry) => window.__craft.events.push({ at: at(), ...entry });

window.__craft = { events: [], saves: [] };

window.addEventListener('message', async (event) => {
  if (event.source !== frame.contentWindow || event.origin !== location.origin) return;
  const message = event.data ?? {};
  const { bytes, ...rest } = message;
  log({ ...rest, bytes: bytes ? bytes.byteLength : undefined });
  if (message.type === `${app}:ready` && file) {
    const name = file.split('/').pop();
    const body = await (await fetch(file)).arrayBuffer();
    // Logged before the transfer detaches the buffer.
    log({ type: 'host:sent', name, bytes: body.byteLength });
    frame.contentWindow.postMessage({ type: `${app}:open`, name, bytes: body }, location.origin, [body]);
  } else if (message.type === `${app}:save` && bytes) {
    const response = await fetch(`/save?name=${encodeURIComponent(message.name)}`, {
      method: 'POST',
      body: bytes,
    });
    window.__craft.saves.push({ name: message.name, saveAs: message.saveAs, bytes: bytes.byteLength, stored: response.ok });
  }
});

frame.src = `/apps/${app}/?host=parent${flags ? `&${flags}` : ''}`;
