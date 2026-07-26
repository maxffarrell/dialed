import { json } from '@sveltejs/kit';
import { makeWatchState } from '$lib/mock-data';
import type { RequestHandler } from './$types';
import type { Task, WatchState } from '$lib/types';

let mockState: WatchState = makeWatchState();

export const GET: RequestHandler = async ({ platform }) => {
  const db = platform?.env?.DB as D1Database | undefined;
  if (!db) return json(mockState);

  const rows = await db.prepare(
    'SELECT id, title, duration_minutes as durationMinutes, completed FROM tasks WHERE user_id = ? ORDER BY duration_minutes DESC'
  ).bind('demo-user').all<Task>();
  return json({ ...mockState, tasks: rows.results });
};

export const PUT: RequestHandler = async ({ request, platform }) => {
  const incoming = (await request.json()) as Partial<WatchState>;
  mockState = { ...mockState, ...incoming, version: mockState.version + 1, syncedAt: 'just now' };
  const db = platform?.env?.DB as D1Database | undefined;
  if (db && incoming.tasks) {
    for (const task of incoming.tasks) {
      await db.prepare('UPDATE tasks SET completed = ? WHERE id = ? AND user_id = ?')
        .bind(task.completed ? 1 : 0, task.id, 'demo-user').run();
    }
  }
  return json(mockState);
};
