const MAX_BODY_BYTES = 14 * 1024 * 1024;
const MAX_MESSAGE_CHARS = 4000;
const MAX_CONTACT_CHARS = 200;
const MAX_META_CHARS = 64;
const MAX_SCREENSHOTS = 4;
const MAX_REPLAYS = 3;
const MAX_ATTACHMENT_BYTES = 9 * 1024 * 1024;
const MAX_REPLAY_NAME_CHARS = 100;
const PER_IP_LIMIT = 5;
const PER_IP_WINDOW_MS = 60 * 1000;

const IMAGE_TYPES = {
  png: { ext: 'png', mime: 'image/png', magic: [0x89, 0x50, 0x4e, 0x47] },
  jpeg: { ext: 'jpg', mime: 'image/jpeg', magic: [0xff, 0xd8, 0xff] },
};

const recentByIP = new Map();

export default {
  async fetch(request, env) {
    if (request.method !== 'POST') {
      return reply(405, 'method not allowed');
    }
    if (!env.DISCORD_WEBHOOK_URL) {
      return reply(503, 'feedback is not configured');
    }
    const ip = request.headers.get('CF-Connecting-IP') || 'unknown';
    if (!(await allow(ip, env))) {
      return reply(429, 'too many submissions, try again in a minute');
    }
    if (Number(request.headers.get('Content-Length') || 0) > MAX_BODY_BYTES) {
      return reply(413, 'submission too large');
    }

    let body;
    try {
      body = await request.json();
    } catch {
      return reply(400, 'invalid JSON');
    }

    const parsed = parseSubmission(body);
    if (parsed.error) {
      return reply(400, parsed.error);
    }

    const discord = await fetch(`${env.DISCORD_WEBHOOK_URL}?wait=true`, {
      method: 'POST',
      body: discordForm(parsed),
    });
    if (!discord.ok) {
      console.log('discord rejected feedback', discord.status, await discord.text());
      return reply(502, 'could not deliver feedback');
    }
    return reply(200, 'ok');
  },
};

async function allow(ip, env) {
  if (env.RATE_LIMITER) {
    const { success } = await env.RATE_LIMITER.limit({ key: ip });
    return success;
  }
  const now = Date.now();
  const recent = (recentByIP.get(ip) || []).filter((t) => now - t < PER_IP_WINDOW_MS);
  if (recent.length >= PER_IP_LIMIT) {
    recentByIP.set(ip, recent);
    return false;
  }
  recent.push(now);
  recentByIP.set(ip, recent);
  return true;
}

function parseSubmission(body) {
  if (!body || typeof body !== 'object') {
    return { error: 'invalid submission' };
  }
  const message = text(body.message, MAX_MESSAGE_CHARS);
  if (!message) {
    return { error: 'message is required' };
  }
  const screenshots = Array.isArray(body.screenshots) ? body.screenshots : [];
  if (screenshots.length > MAX_SCREENSHOTS) {
    return { error: `at most ${MAX_SCREENSHOTS} screenshots` };
  }
  const replays = Array.isArray(body.replays) ? body.replays : [];
  if (replays.length > MAX_REPLAYS) {
    return { error: `at most ${MAX_REPLAYS} replays` };
  }
  const files = [];
  let total = 0;
  for (const [i, encoded] of screenshots.entries()) {
    const image = decodeImage(encoded);
    if (!image) {
      return { error: 'screenshots must be PNG or JPEG' };
    }
    files.push({ bytes: image.bytes, mime: image.type.mime, filename: `screenshot-${i + 1}.${image.type.ext}` });
  }
  for (const replay of replays) {
    const bytes = replay && decodeBase64(replay.data);
    if (!bytes || bytes.length === 0) {
      return { error: 'invalid replay' };
    }
    files.push({ bytes, mime: 'application/octet-stream', filename: replayName(replay.name) });
  }
  for (const file of files) {
    total += file.bytes.length;
  }
  if (total > MAX_ATTACHMENT_BYTES) {
    return { error: 'attachments are too large' };
  }
  return {
    message,
    contact: text(body.contact, MAX_CONTACT_CHARS),
    version: text(body.version, MAX_META_CHARS),
    os: text(body.os, MAX_META_CHARS),
    language: text(body.language, MAX_META_CHARS),
    files,
  };
}

function text(value, max) {
  return typeof value === 'string' ? value.trim().slice(0, max) : '';
}

function decodeBase64(encoded) {
  if (typeof encoded !== 'string') {
    return null;
  }
  try {
    return Uint8Array.from(atob(encoded), (c) => c.charCodeAt(0));
  } catch {
    return null;
  }
}

function replayName(name) {
  const base = (typeof name === 'string' ? name : '').split(/[\\/]/).pop().replace(/\.[^.]*$/, '');
  const cleaned = base.replace(/[^\p{L}\p{N} ._()\[\]-]/gu, '_').trim().slice(0, MAX_REPLAY_NAME_CHARS);
  return `${cleaned && cleaned !== '.' && cleaned !== '..' ? cleaned : 'replay'}.rep`;
}

function decodeImage(encoded) {
  const bytes = decodeBase64(encoded);
  if (!bytes) {
    return null;
  }
  const type = Object.values(IMAGE_TYPES).find((t) => t.magic.every((b, i) => bytes[i] === b));
  return type ? { bytes, type } : null;
}

function discordForm({ message, contact, version, os, language, files }) {
  const firstLine = message.split('\n')[0].trim();
  const threadName = (firstLine.length > 90 ? `${firstLine.slice(0, 90)}…` : firstLine) || 'Feedback';
  const fields = [
    { name: 'Contact', value: contact || 'none (anonymous)', inline: false },
    { name: 'Version', value: version || 'unknown', inline: true },
    { name: 'OS', value: os || 'unknown', inline: true },
    { name: 'Language', value: language || 'unknown', inline: true },
  ];
  const payload = {
    thread_name: threadName,
    allowed_mentions: { parse: [] },
    embeds: [{ description: message, fields }],
    attachments: files.map((file, i) => ({ id: i, filename: file.filename })),
  };
  const form = new FormData();
  form.append('payload_json', JSON.stringify(payload));
  files.forEach((file, i) => {
    form.append(`files[${i}]`, new Blob([file.bytes], { type: file.mime }), file.filename);
  });
  return form;
}

function reply(status, message) {
  return new Response(JSON.stringify({ ok: status === 200, message }), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}
