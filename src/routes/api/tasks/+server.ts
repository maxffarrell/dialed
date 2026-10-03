
import { orderedTasks } from '#lib/watch.js';
import type { RequestHandler } from './$types';
import type { Task } from '#lib/types.js';

export const POST: RequestHandler = async ({ request, platform }) => {
  const task = (await request.json()) as Task;
  if (!task.title?.trim() || !Number.isFinite(task.durationMinutes)) {
    return Response.json({ error: 'title and durationMinutes are required' }, { status: 400 });
  }
  const db = platform?.env?.DB as D1Database | undefined;
  if (db) {
    await db.prepare('INSERT INTO tasks (id, user_id, title, duration_minutes, completed) VALUES (?, ?, ?, ?, ?)')
      .bind(task.id, 'demo-user', task.title.trim(), task.durationMinutes, task.completed ? 1 : 0).run();
  }
  return Response.json({ ...task, title: task.title.trim(), tasks: orderedTasks([task]) }, { status: 201 });
};
