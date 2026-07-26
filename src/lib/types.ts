export type Task = {
  id: string;
  title: string;
  durationMinutes: number;
  completed: boolean;
  note?: string;
};

export type WatchState = {
  now: string;
  remainingMinutes: number;
  tasks: Task[];
  selectedTask: number;
  syncedAt: string;
  version: number;
  context?: string;
};

export type Note = {
  id: string;
  createdAt: string;
  transcript: string;
  context: string;
};
