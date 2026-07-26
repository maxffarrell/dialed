import type { Task, WatchState } from './types';

export function formatDuration(minutes: number) {
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  return hours ? `${hours}h ${mins.toString().padStart(2, '0')}m` : `${mins}m`;
}

export function completeTask(state: WatchState, taskId: string): WatchState {
  const tasks = state.tasks.map((task) => (task.id === taskId ? { ...task, completed: !task.completed } : task));
  return { ...state, tasks, version: state.version + 1, syncedAt: 'just now' };
}

export function orderedTasks(tasks: Task[]) {
  return [...tasks].sort((a, b) => b.durationMinutes - a.durationMinutes);
}
