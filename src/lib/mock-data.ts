import type { Note, Task, WatchState } from './types';

export const initialTasks: Task[] = [
  { id: 'website', title: 'Refine the website', durationMinutes: 42, completed: false },
  { id: 'ab-test', title: 'Create A/B test', durationMinutes: 28, completed: false },
  { id: 'interview', title: 'Interview Rian', durationMinutes: 19, completed: true }
];

export const initialNotes: Note[] = [
  {
    id: 'note-1',
    createdAt: 'Today · 09:18',
    transcript: 'Keep the first pass focused on the landing page and the signup flow.',
    context: 'Prioritize product surface before experiments.'
  },
  {
    id: 'note-2',
    createdAt: 'Yesterday · 16:42',
    transcript: 'The interview is a quick one. Have the prototype open before the call.',
    context: 'Interview prep is short and time-sensitive.'
  }
];

export function makeWatchState(tasks: Task[] = initialTasks): WatchState {
  return {
    now: '9:41 am',
    remainingMinutes: 19,
    tasks: [...tasks].sort((a, b) => b.durationMinutes - a.durationMinutes),
    selectedTask: 0,
    syncedAt: 'just now',
    version: 1,
    context: 'Keep the first pass focused on the landing page.'
  };
}
