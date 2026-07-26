import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const mockTranscript = 'The next step should feel obvious when I look at the watch.';

function audioFormat(file: File) {
  const extension = file.name.split('.').pop()?.toLowerCase();
  if (extension && ['wav', 'mp3', 'flac', 'm4a', 'ogg', 'webm', 'aac'].includes(extension)) return extension;
  if (file.type.includes('wav')) return 'wav';
  if (file.type.includes('mpeg') || file.type.includes('mp3')) return 'mp3';
  if (file.type.includes('ogg')) return 'ogg';
  return 'webm';
}

async function transcribeAudio(request: Request, env: App.Platform['env']) {
  const form = await request.formData();
  const audio = form.get('file') ?? form.get('audio');
  if (!(audio instanceof File)) return { error: 'an audio file is required' as const };

  const endpoint = env?.PARAKEET_API_URL || 'https://openrouter.ai/api/v1/audio/transcriptions';
  if (env?.PUBLIC_MOCK_MODE !== 'false' && !env?.PARAKEET_API_KEY) return { text: mockTranscript };

  const bytes = new Uint8Array(await audio.arrayBuffer());
  let binary = '';
  const chunkSize = 0x8000;
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + chunkSize));
  }
  const body = JSON.stringify({
    model: env?.PARAKEET_MODEL || 'nvidia/parakeet-tdt-0.6b-v3',
    input_audio: { data: btoa(binary), format: audioFormat(audio) }
  });

  const headers = new Headers();
  headers.set('Content-Type', 'application/json');
  if (env?.PARAKEET_API_KEY) headers.set('Authorization', `Bearer ${env.PARAKEET_API_KEY}`);
  const response = await fetch(endpoint, { method: 'POST', headers, body });
  if (!response.ok) return { error: `Parakeet request failed (${response.status})` as const };
  const result = (await response.json()) as { text?: string };
  if (!result.text?.trim()) return { error: 'Parakeet returned an empty transcript' as const };
  return { text: result.text.trim() };
}

export const POST: RequestHandler = async ({ request, platform }) => {
  let transcript: string | undefined;
  if (request.headers.get('content-type')?.includes('multipart/form-data')) {
    try {
      const result = await transcribeAudio(request, platform?.env);
      if ('error' in result) return json({ error: result.error }, { status: 400 });
      transcript = result.text;
    } catch (error) {
      console.error('Parakeet transcription failed', error);
      return json({ error: 'voice note transcription failed' }, { status: 502 });
    }
  } else {
    const body = (await request.json()) as { transcript?: string };
    transcript = body.transcript?.trim();
  }
  if (!transcript) return json({ error: 'transcript or audio file is required' }, { status: 400 });
  const note = {
    id: crypto.randomUUID(),
    createdAt: new Date().toISOString(),
    transcript,
    context: 'Context extraction: retain the clearest next action.'
  };
  const db = platform?.env?.DB as D1Database | undefined;
  if (db) {
    await db.prepare('INSERT INTO notes (id, user_id, transcript, context) VALUES (?, ?, ?, ?)')
      .bind(note.id, 'demo-user', note.transcript, note.context).run();
  }
  return json(note, { status: 201 });
};
